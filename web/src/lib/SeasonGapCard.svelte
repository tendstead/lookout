<script lang="ts">
  import Icon from './Icon.svelte';
  import { translate, type Locale, type Key } from './i18n';
  import type { Gap } from './season-gaps';
  let { item, locale }: { item: Gap; locale: Locale } = $props();
  const t = (key: Key) => translate(locale, key);
  let artworkFailed = $state(false);
  let expanded = $state(false);
  let selectedSeason = $state<number | null>(null);
  const seasons = $derived(item.seasons?.length ? item.seasons : [item]);
  const current = $derived(
    seasons.find((season) => season.season === selectedSeason) ?? seasons[0],
  );
  const percent = $derived(
    current.aired > 0
      ? Math.max(0, Math.min(100, (100 * current.present) / current.aired))
      : 0,
  );
  const stateLabel = $derived(
    item.kind === 'review'
      ? 'gapReviewTab'
      : item.kind === 'missing'
        ? seasons.length > 1
          ? 'gapMissingTab'
          : 'gapMissing'
        : seasons.length > 1
          ? 'gapPartialTab'
          : 'gapPartial',
  );
  const reasons: Record<string, Key> = {
    schedule_unavailable: 'gapScheduleReason',
    inventory_unavailable: 'gapInventoryReason',
    metadata_conflict: 'gapMetadataReason',
    anime_ordering: 'gapAnimeReason',
    unknown_numbering: 'gapNumberingReason',
    unconfirmed_files: 'gapFilesReason',
  };
  function date(value: string) {
    return new Intl.DateTimeFormat(locale, {
      dateStyle: 'medium',
      timeStyle: 'short',
    }).format(new Date(value));
  }
</script>

<article class="season-card" class:needs-review={item.kind === 'review'}>
  <header class="card-heading">
    <a
      class="season-art"
      href={'/titles/' + item.title_id}
      aria-label={t('openTitle') + ': ' + item.title}
    >
      <Icon kind="library" size={30} />
      {#if artworkFailed}<span class="art-fallback">{t('gapNoArtwork')}</span>
      {:else}<img
          src={'/api/v1/titles/' + item.title_id + '/artwork'}
          alt={t('gapSeriesArtwork') + ' ' + item.title}
          loading="lazy"
          onerror={() => (artworkFailed = true)}
        />{/if}
    </a>
    <div class="card-identity">
      <span class="state-badge"
        ><Icon kind={item.kind === 'review' ? 'eye' : 'library'} size={13} />{t(
          stateLabel,
        )}</span
      >
      <a class="release-title card-title" href={'/titles/' + item.title_id}
        >{item.title}</a
      >
      {#if item.kind !== 'review'}
        {#if seasons.length > 1}<p class="season-number season-count">
            <strong>{seasons.length}</strong><span
              >{t(
                item.kind === 'missing' ? 'gapMissingTab' : 'gapPartialTab',
              )}</span
            >
          </p>
        {:else}<p class="season-number">
            <span>{t('season')}</span><strong
              >{current.season.toString().padStart(2, '0')}</strong
            >
          </p>{/if}
      {/if}
      {#if item.latest_aired_season > 0}<p class="latest-season">
          {t('gapLatest')}: {item.latest_aired_season}
        </p>{/if}
    </div>
  </header>
  <div class="card-body">
    {#if item.kind === 'review'}
      <ul class="review-reasons">
        {#each item.reasons as reason}<li>
            {t(reasons[reason] || 'reviewStatus')}
          </li>{/each}
      </ul>
    {:else}
      {#if seasons.length > 1}<div
          class="season-selector"
          role="group"
          aria-label={t(
            item.kind === 'missing' ? 'gapMissingTab' : 'gapPartialTab',
          )}
        >
          {#each seasons as season}<button
              class:chosen={current.season === season.season}
              aria-pressed={current.season === season.season}
              onclick={() => {
                selectedSeason = season.season;
                expanded = false;
              }}>{t('season')} {season.season}</button
            >{/each}
        </div>
        <p class="selected-season">{t('season')} {current.season}</p>{/if}
      {#key current.season}
        <div class="coverage">
          <p class="coverage-copy">
            <span>{current.present} / {current.aired} {t('gapPresent')}</span
            ><span class="coverage-percent" aria-hidden="true"
              >{Math.round(percent)}%</span
            >
          </p>
          <div
            class="coverage-track"
            role="progressbar"
            aria-label={t('gapCoverage')}
            aria-valuemin="0"
            aria-valuemax={current.aired}
            aria-valuenow={current.present}
            aria-valuetext={current.present +
              ' / ' +
              current.aired +
              ' ' +
              t('gapPresent')}
          >
            <span style:width={percent + '%'}></span>
          </div>
        </div>
        <details
          class="episode-details"
          ontoggle={(event) => (expanded = event.currentTarget.open)}
        >
          <summary
            >{t('gapEpisodeNumbers')} ({current.missing_episodes
              .length})</summary
          >
          {#if expanded}
            <!-- svelte-ignore a11y_no_noninteractive_tabindex (Scrollable number groups need keyboard focus in browsers without automatic scroll-container focus.) -->
            <div
              class="episode-tiles"
              role="group"
              tabindex="0"
              aria-label={t('gapEpisodeNumbers')}
            >
              {#each current.missing_episodes as number}<span
                  class="episode-tile">{number}</span
                >{/each}
            </div>{/if}
        </details>
      {/key}
    {/if}
    <div class="local-context">
      <p class="context-label">
        <Icon kind="library" size={14} />{t('gapLocal')}
      </p>
      <!-- svelte-ignore a11y_no_noninteractive_tabindex (Scrollable number groups need keyboard focus in browsers without automatic scroll-container focus.) -->
      <div
        class="local-seasons"
        role="group"
        tabindex="0"
        aria-label={t('gapLocal') +
          ': ' +
          (item.local_seasons.length
            ? item.local_seasons.join(', ')
            : t('gapNoLocal'))}
      >
        {#each item.local_seasons as season}<span
            class="season-chip"
            title={t('season') + ' ' + season}>{season}</span
          >
        {:else}<span class="no-local">{t('gapNoLocal')}</span>{/each}
      </div>
    </div>
    {#if item.checked_at || item.inventory_at}<details class="evidence">
        <summary>{t('gapEvidence')}</summary>
        {#if item.checked_at}<p>
            {t('gapChecked')}: {date(item.checked_at)}
          </p>{/if}
        {#if item.inventory_at}<p>
            {t('gapScanned')}: {date(item.inventory_at)}
          </p>{/if}
      </details>{/if}
  </div>
</article>

<style>
  .season-card {
    border: 1px solid var(--ts-border);
    border-radius: 14px;
    overflow: hidden;
    background: var(--ts-surface);
    min-width: 0;
    overflow-wrap: anywhere;
  }
  .card-heading {
    display: grid;
    grid-template-columns: 104px minmax(0, 1fr);
    gap: 16px;
    padding: 18px;
    background: var(--ts-surface-subtle);
    align-items: start;
  }
  .season-art {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: 8px;
    align-items: center;
    justify-content: center;
    aspect-ratio: 2/3;
    width: 104px;
    min-width: 0;
    border-radius: 9px;
    overflow: hidden;
    background: var(--ts-selection);
    color: var(--ts-on-selection);
    text-decoration: none;
    box-shadow: 0 2px 7px #00000016;
  }
  .season-art img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .art-fallback {
    font-size: 11px;
    text-align: center;
    padding: 0 8px;
  }
  .card-identity {
    min-width: 0;
  }
  .state-badge {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    max-width: 100%;
    font-size: 10px;
    line-height: 1.4;
    font-weight: 600;
    padding: 4px 7px;
    border-radius: 6px;
    background: var(--ts-selection);
    color: var(--ts-on-selection);
  }
  .needs-review .state-badge {
    background: var(--ts-surface);
    color: var(--ts-muted);
    border: 1px solid var(--ts-border);
  }
  .card-title {
    display: block;
    font-size: 17px;
    line-height: 1.3;
    margin: 10px 0 0;
    text-wrap: pretty;
  }
  .season-number {
    display: flex;
    align-items: baseline;
    gap: 8px;
    margin: 8px 0 0;
    line-height: 1;
  }
  .season-number span {
    font-size: 12px;
    color: var(--ts-muted);
  }
  .season-number strong {
    font-size: 34px;
    font-weight: 650;
    letter-spacing: -1px;
    color: var(--ts-primary);
  }
  .season-count {
    align-items: center;
  }
  .season-count span {
    line-height: 1.4;
  }
  .season-selector {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-bottom: 14px;
  }
  .season-selector button {
    font-size: 11px;
    min-height: 32px;
    padding: 5px 8px;
    border: 1px solid var(--ts-border);
    border-radius: 6px;
    background: var(--ts-surface);
    color: var(--ts-text);
    cursor: pointer;
  }
  .season-selector button.chosen {
    background: var(--ts-selection);
    border-color: var(--ts-primary);
    color: var(--ts-on-selection);
  }
  .selected-season {
    font-size: 12px;
    font-weight: 600;
    margin: 0 0 8px;
  }
  .latest-season {
    font-size: 10px;
    line-height: 1.5;
    margin: 7px 0 0;
    color: var(--ts-muted);
  }
  .card-body {
    padding: 16px 18px;
  }
  .coverage-copy {
    display: flex;
    justify-content: space-between;
    gap: 10px;
    font-size: 12px;
    line-height: 1.5;
    margin: 0 0 8px;
  }
  .coverage-percent {
    color: var(--ts-primary);
    font-weight: 600;
    flex-shrink: 0;
  }
  .coverage-track {
    height: 9px;
    overflow: hidden;
    border-radius: 4px;
    background: var(--ts-surface-subtle);
    border: 1px solid var(--ts-border);
  }
  .coverage-track > span {
    display: block;
    height: 100%;
    background: var(--ts-primary);
  }
  summary {
    cursor: pointer;
    line-height: 1.5;
  }
  .episode-details {
    margin: 12px 0 0;
    font-size: 11px;
    color: var(--ts-muted);
  }
  .episode-tiles {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    max-height: 180px;
    overflow: auto;
    margin-top: 10px;
    padding: 2px;
  }
  .episode-tiles:focus-visible,
  .local-seasons:focus-visible {
    outline: 2px solid var(--ts-focus);
    outline-offset: 2px;
  }
  .episode-tile {
    display: inline-grid;
    place-items: center;
    min-width: 30px;
    height: 30px;
    padding: 0 5px;
    border: 1px dashed var(--ts-primary);
    border-radius: 6px;
    color: var(--ts-primary);
    font-size: 11px;
    font-variant-numeric: tabular-nums;
  }
  .local-context {
    margin-top: 16px;
    padding-top: 12px;
    border-top: 1px solid var(--ts-border);
  }
  .context-label {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 11px;
    color: var(--ts-muted);
    margin: 0 0 8px;
  }
  .local-seasons {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    max-height: 92px;
    overflow: auto;
    padding: 2px;
  }
  .season-chip {
    display: inline-grid;
    place-items: center;
    min-width: 28px;
    height: 26px;
    padding: 0 6px;
    border-radius: 5px;
    background: var(--ts-selection);
    color: var(--ts-on-selection);
    font-size: 11px;
    font-weight: 600;
    font-variant-numeric: tabular-nums;
  }
  .no-local {
    font-size: 12px;
    color: var(--ts-muted);
  }
  .evidence {
    margin-top: 14px;
    color: var(--ts-muted);
    font-size: 10px;
  }
  .evidence p {
    margin: 6px 0 0;
    font-size: 10px;
    line-height: 1.5;
  }
  .review-reasons {
    padding-left: 16px;
    margin: 0;
    font-size: 12px;
    color: var(--ts-muted);
    line-height: 1.6;
  }
  .review-reasons li {
    display: list-item;
    border: 0;
    padding: 3px 0;
  }
  @media (max-width: 400px) {
    .card-heading {
      grid-template-columns: 92px minmax(0, 1fr);
      gap: 14px;
      padding: 16px;
    }
    .season-art {
      width: 92px;
    }
    .card-title {
      font-size: 16px;
    }
  }
</style>
