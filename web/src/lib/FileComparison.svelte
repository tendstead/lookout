<script lang="ts">
  import type { Snippet } from 'svelte';
  import {
    compareCopies,
    comparableCopies,
    episodeCode,
    fileSize,
    reportedText,
    scalarFields,
    type FileItem,
  } from './file-comparison';
  import { translate, type Key, type Locale } from './i18n';
  import ComparisonTracks from './ComparisonTracks.svelte';

  let {
    files,
    locale,
    showFlags,
    copyActions,
    crossServerOnly = true,
  }: {
    files: FileItem[];
    locale: Locale;
    showFlags: boolean;
    copyActions?: Snippet<[FileItem]>;
    crossServerOnly?: boolean;
  } = $props();
  const t = (key: Key) => translate(locale, key);
  const differences = $derived(compareCopies(files, crossServerOnly));
  type ScalarField = keyof typeof scalarFields;
  const fields: ScalarField[] = [
    'resolution',
    'videoCodec',
    'videoBitrate',
    'fileSize',
    'hdr',
    'edition',
    'container',
    'fileDuration',
    'videoProfile',
    'overallBitrate',
    'frameRate',
    'bitDepth',
  ];
  const shownFields = $derived(
    fields.filter(
      (field, index) =>
        index < 5 ||
        files.some((file) =>
          scalarFields[field].some((read) => {
            const value = read(file);
            return typeof value === 'string'
              ? Boolean(reportedText(value))
              : value != null;
          }),
        ),
    ),
  );

  function quantity(value: number | undefined, unit: string, divisor = 1) {
    return value == null
      ? undefined
      : `${(value / divisor).toLocaleString(locale, { maximumFractionDigits: 2 })} ${unit}`;
  }

  function size(bytes: number | undefined) {
    if (bytes == null) return undefined;
    if (bytes >= 1024 ** 4) return quantity(bytes, 'TiB', 1024 ** 4);
    if (bytes >= 1024 ** 3) return quantity(bytes, 'GiB', 1024 ** 3);
    if (bytes >= 1024 ** 2) return quantity(bytes, 'MiB', 1024 ** 2);
    if (bytes >= 1024) return quantity(bytes, 'KiB', 1024);
    return quantity(bytes, 'B');
  }

  function scalarLabel(file: FileItem, field: ScalarField) {
    const profile = file.technical;
    const unknown = t('comparisonUnknown');
    switch (field) {
      case 'resolution': {
        const width = profile?.width;
        const height = profile?.height;
        if (width == null && height == null) return undefined;
        return `${width ?? unknown} × ${height ?? unknown}`;
      }
      case 'videoCodec':
        return reportedText(profile?.video_codec);
      case 'videoBitrate':
        return quantity(profile?.video_bitrate_bps, 'Mb/s', 1e6);
      case 'fileSize':
        return size(fileSize(file));
      case 'hdr':
        return reportedText(profile?.hdr);
      case 'edition':
        return reportedText(profile?.edition);
      case 'container':
        return reportedText(profile?.container);
      case 'fileDuration':
        return quantity(profile?.duration_ms, 's', 1000);
      case 'videoProfile':
        return reportedText(profile?.video_profile);
      case 'overallBitrate':
        return quantity(profile?.bitrate_bps, 'Mb/s', 1e6);
      case 'frameRate':
        return quantity(profile?.frame_rate, 'fps');
      case 'bitDepth':
        return quantity(profile?.bit_depth, 'bit');
    }
  }

  function pixels(file: FileItem) {
    const { width, height } = file.technical ?? {};
    return width == null || height == null ? undefined : width * height;
  }

  function higher(file: FileItem, read: (f: FileItem) => number | undefined) {
    const value = read(file);
    return (
      value != null &&
      files.some((other) => {
        const compared = read(other);
        return (
          comparableCopies(file, other, crossServerOnly) &&
          compared != null &&
          value > compared
        );
      })
    );
  }
</script>

<div class="scroll-frame">
  <!-- svelte-ignore a11y_no_noninteractive_tabindex (keyboard scrolling) -->
  <div
    class="comparison-scroll"
    role="region"
    aria-label={t('compareFiles')}
    tabindex="0"
  >
    <table>
      <thead>
        <tr>
          <th scope="col" class="property">{t('comparisonProperty')}</th>
          {#each files as file (file.id)}
            {@const profile = file.technical}
            <th scope="col">
              <div>
                {file.server_name || file.server_id || t('comparisonUnknown')}
              </div>
              <div class="copy-path">
                {file.reported_path ||
                  file.relative_path}{#if file.archive_member}!/{file.archive_member}{/if}
              </div>
              {#if file.season != null && file.episodes?.length}
                <div class="copy-detail">
                  {episodeCode(file.season, file.episodes)}
                </div>
              {/if}
              {#if profile?.part_number != null}
                <div class="copy-detail">
                  {t('streamPart')}
                  {profile.part_number}{profile.part_count != null
                    ? ' / ' + profile.part_count
                    : ''}
                </div>
              {/if}
              {#if copyActions}{@render copyActions(file)}{/if}
            </th>
          {/each}
        </tr>
      </thead>
      <tbody>
        {#each shownFields as field}
          <tr>
            <th scope="row" class="property">{t(field)}</th>
            {#each files as file (file.id)}
              {@const value = scalarLabel(file, field)}
              <td class:different={differences.get(file.id)?.has(field)}>
                <span class:unknown={!value}
                  >{value || t('comparisonUnknown')}</span
                >
                {#if field === 'resolution' && higher(file, pixels)}
                  <span class="highlight">{t('higherResolution')}</span>
                {:else if field === 'videoBitrate' && higher(file, (item) => item.technical?.video_bitrate_bps)}
                  <span class="highlight">{t('higherVideoBitrate')}</span>
                {/if}
              </td>
            {/each}
          </tr>
        {/each}
        {#each ['audioTracks', 'subtitleTracks'] as const as field}
          <tr>
            <th scope="row" class="property">{t(field)}</th>
            {#each files as file (file.id)}
              <td class:different={differences.get(file.id)?.has(field)}>
                {#if field === 'audioTracks'}
                  <ComparisonTracks
                    audio={file.technical?.audio}
                    {locale}
                    {showFlags}
                  />
                {:else}
                  <ComparisonTracks
                    subtitles={file.technical?.subtitles}
                    {locale}
                    {showFlags}
                  />
                {/if}
              </td>
            {/each}
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>

<style>
  .scroll-frame {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    min-width: 0;
  }
  .comparison-scroll {
    overflow: auto;
    border: 1px solid var(--ts-border);
    border-radius: 10px;
  }
  .comparison-scroll:focus-visible {
    outline: 3px solid var(--ts-focus);
    outline-offset: 3px;
  }
  table {
    border-collapse: separate;
    border-spacing: 0;
    width: 100%;
    font-size: 0.875rem;
  }
  th,
  td {
    min-width: 7.5rem;
    padding: 10px 12px;
    border-bottom: 1px solid var(--ts-border);
    text-align: start;
    vertical-align: top;
    overflow-wrap: anywhere;
  }
  th {
    background: var(--ts-surface);
    font-weight: 650;
  }
  tr:last-child td,
  tbody tr:last-child th {
    border-bottom: 0;
  }
  td + td,
  th + th,
  th + td {
    border-inline-start: 1px solid var(--ts-border);
  }
  .property {
    position: sticky;
    inset-inline-start: 0;
    z-index: 1;
    min-width: 6rem;
    max-width: 8.5rem;
  }
  .copy-path,
  .copy-detail {
    color: var(--ts-muted);
    font-size: 0.75rem;
    font-weight: normal;
    margin-top: 4px;
  }
  .copy-path {
    max-width: 22ch;
  }
  .unknown {
    color: var(--ts-muted);
  }
  .different {
    background: var(--ts-selection);
    color: var(--ts-on-selection);
  }
  .highlight {
    display: block;
    font-size: 0.75rem;
    font-weight: 650;
    margin-top: 4px;
  }

  @media (max-width: 600px) {
    th,
    td {
      padding: 8px;
    }
  }
</style>
