<script lang="ts">
  import TrackLanguage from './TrackLanguage.svelte';
  import { translate, type Key, type Locale } from './i18n';
  import {
    reportedText,
    type AudioTrack,
    type SubtitleTrack,
  } from './file-comparison';

  let {
    audio,
    subtitles,
    locale,
    showFlags,
  }: {
    audio?: AudioTrack[] | null;
    subtitles?: SubtitleTrack[] | null;
    locale: Locale;
    showFlags: boolean;
  } = $props();
  const t = (key: Key) => translate(locale, key);
  const tracks = $derived(audio ?? subtitles);
  const audioCodecNames: Record<string, string> = {
    aac: 'AAC',
    ac3: 'Dolby Digital (AC-3)',
    eac3: 'Dolby Digital Plus (E-AC-3)',
    truehd: 'Dolby TrueHD',
    dts: 'DTS',
    flac: 'FLAC',
    mp3: 'MP3',
    opus: 'Opus',
    vorbis: 'Vorbis',
  };
  function codec(value?: string) {
    const reported = reportedText(value);
    if (!reported) return t('comparisonUnknown');
    return audio
      ? (audioCodecNames[reported.toLowerCase()] ?? reported)
      : reported;
  }

  function flag(value?: boolean) {
    return value == null
      ? t('comparisonUnknown')
      : t(value ? 'comparisonYes' : 'comparisonNo');
  }
</script>

{#if !tracks}
  <span class="unknown">{t('comparisonUnknown')}</span>
{:else if !tracks.length}
  {t('noTracks')}
{:else}
  <ul class="track-labels">
    {#each tracks as track}
      <li><TrackLanguage reported={track.language} {locale} {showFlags} /></li>
    {/each}
  </ul>
  <details>
    <summary>{t('streamDetails')}</summary>
    <ul class="track-details">
      {#each tracks as track}
        <li>
          <TrackLanguage reported={track.language} {locale} {showFlags} />
          <dl>
            <div>
              <dt>{t('trackCodec')}</dt>
              <dd>{codec(track.codec)}</dd>
            </div>
            {#if audio}
              {@const stream = track as AudioTrack}
              <div>
                <dt>{t('trackChannels')}</dt>
                <dd>{stream.channels ?? t('comparisonUnknown')}</dd>
              </div>
              <div>
                <dt>{t('trackBitrate')}</dt>
                <dd>
                  {stream.bitrate_bps == null
                    ? t('comparisonUnknown')
                    : (stream.bitrate_bps / 1000).toLocaleString(locale, {
                        maximumFractionDigits: 2,
                      }) + ' kb/s'}
                </dd>
              </div>
            {:else}
              {@const stream = track as SubtitleTrack}
              <div>
                <dt>{t('trackForced')}</dt>
                <dd>{flag(stream.forced)}</dd>
              </div>
              <div>
                <dt>{t('trackDefault')}</dt>
                <dd>{flag(stream.default)}</dd>
              </div>
              <div>
                <dt>{t('trackHearingImpaired')}</dt>
                <dd>{flag(stream.hearing_impaired)}</dd>
              </div>
            {/if}
          </dl>
        </li>
      {/each}
    </ul>
  </details>
{/if}

<style>
  ul {
    padding: 0;
    list-style: none;
    margin: 0;
  }
  .track-labels {
    display: flex;
    flex-wrap: wrap;
    gap: 4px 8px;
  }
  .track-details li {
    border-top: 1px solid var(--ts-border);
    margin-top: 8px;
    padding-top: 8px;
  }
  .unknown,
  dt {
    color: var(--ts-muted);
  }
  details {
    margin-top: 6px;
    font-size: 0.8rem;
  }
  summary {
    cursor: pointer;
  }
  dl {
    margin: 6px 0 0;
  }
  dd {
    margin: 0;
  }
  dl > div {
    display: flex;
    align-items: baseline;
    flex-wrap: wrap;
    gap: 2px 6px;
    margin-top: 4px;
  }
</style>
