<script lang="ts">
  import { onMount } from 'svelte';
  import { translate, type Locale, type Key } from './i18n';
  let { locale }: { locale: Locale } = $props();
  const t = (key: Key) => translate(locale, key);
  type Kind = 'missing' | 'partial' | 'review';
  type Gap = {
    title_id: string;
    title: string;
    category: string;
    kind: Kind;
    season: number;
    latest_aired_season: number;
    local_seasons: number[];
    aired: number;
    present: number;
    missing_episodes: number[];
    reasons: string[];
    checked_at: string | null;
    inventory_at: string | null;
  };
  let kind = $state<Kind>('missing');
  let query = $state('');
  let search = $state('');
  let offset = $state(0);
  let items = $state<Gap[]>([]);
  let total = $state(0);
  let counts = $state({ missing: 0, partial: 0, review: 0 });
  let loading = $state(true);
  let failed = $state(false);
  const limit = 48;
  let generation = 0;
  const reasons: Record<string, Key> = {
    schedule_unavailable: 'gapScheduleReason',
    inventory_unavailable: 'gapInventoryReason',
    metadata_conflict: 'gapMetadataReason',
    anime_ordering: 'gapAnimeReason',
    unknown_numbering: 'gapNumberingReason',
    unconfirmed_files: 'gapFilesReason',
  };
  async function load() {
    const current = ++generation;
    loading = true;
    failed = false;
    try {
      const params = new URLSearchParams({
        kind,
        q: search,
        limit: String(limit),
        offset: String(offset),
      });
      const response = await fetch('/api/v1/releases/gaps?' + params, {
        cache: 'no-store',
        signal: AbortSignal.timeout(8000),
      });
      if (!response.ok) throw new Error();
      const data = await response.json();
      if (current !== generation) return;
      if (offset > 0 && offset >= data.total) {
        offset = Math.max(0, Math.floor((data.total - 1) / limit) * limit);
        await load();
        return;
      }
      items = data.items;
      total = data.total;
      counts = {
        missing: data.missing,
        partial: data.partial,
        review: data.review,
      };
    } catch {
      if (current === generation) {
        failed = true;
        items = [];
      }
    } finally {
      if (current === generation) loading = false;
    }
  }
  function choose(value: Kind) {
    kind = value;
    offset = 0;
    items = [];
    void load();
  }
  function date(value: string) {
    return new Intl.DateTimeFormat(locale, {
      dateStyle: 'medium',
      timeStyle: 'short',
    }).format(new Date(value));
  }
  onMount(() => {
    void load();
    const timer = setInterval(() => void load(), 30000);
    return () => {
      clearInterval(timer);
      generation++;
    };
  });
</script>

<section
  class="release-section season-gaps"
  aria-labelledby="season-gaps-heading"
>
  <h3 id="season-gaps-heading">{t('seasonGapsHeading')}</h3>
  <p class="metadata-note">{t('seasonGapsHint')}</p>
  <div class="gap-tabs" aria-label={t('seasonGapsHeading')}>
    <button
      class="catalogue-button"
      class:gap-selected={kind === 'missing'}
      aria-pressed={kind === 'missing'}
      onclick={() => choose('missing')}
      >{t('gapMissingTab')} · {counts.missing}</button
    >
    <button
      class="catalogue-button"
      class:gap-selected={kind === 'partial'}
      aria-pressed={kind === 'partial'}
      onclick={() => choose('partial')}
      >{t('gapPartialTab')} · {counts.partial}</button
    >
    <button
      class="catalogue-button"
      class:gap-selected={kind === 'review'}
      aria-pressed={kind === 'review'}
      onclick={() => choose('review')}
      >{t('gapReviewTab')} · {counts.review}</button
    >
  </div>
  <form
    class="gap-search"
    onsubmit={(event) => {
      event.preventDefault();
      search = query.trim();
      offset = 0;
      void load();
    }}
  >
    <input
      type="search"
      aria-label={t('gapSearch')}
      placeholder={t('gapSearch')}
      bind:value={query}
    />
    <button class="catalogue-button" type="submit">{t('search')}</button>
  </form>
  {#if failed}<p role="alert">{t('gapError')}</p>
    <button class="catalogue-button" onclick={() => void load()}
      >{t('refresh')}</button
    >
  {:else if loading && !items.length}<p role="status">{t('checking')}</p>
  {:else if !items.length}<p class="metadata-note">
      {t(
        kind === 'missing'
          ? 'gapNoMissing'
          : kind === 'partial'
            ? 'gapNoPartial'
            : 'gapNoReview',
      )}
    </p>
  {:else}<ul class="gap-list" aria-busy={loading}>
      {#each items as item}<li>
          <a class="release-title" href={'/titles/' + item.title_id}
            >{item.title}</a
          >
          {#if item.kind === 'review'}
            <strong>{t('gapReviewTab')}</strong>
            <ul>
              {#each item.reasons as reason}<li>
                  {t(reasons[reason] || 'reviewStatus')}
                </li>{/each}
            </ul>
          {:else}
            <strong
              >{t('season')}
              {item.season} · {t(
                item.kind === 'missing' ? 'gapMissing' : 'gapPartial',
              )}</strong
            >
            <p>{item.present} / {item.aired} {t('gapPresent')}</p>
            <details>
              <summary
                >{t('gapEpisodeNumbers')} ({item.missing_episodes
                  .length})</summary
              >
              <p>{item.missing_episodes.join(', ')}</p>
            </details>
          {/if}
          {#if item.latest_aired_season > 0}<p>
              {t('gapLatest')}: {item.latest_aired_season}
            </p>{/if}
          <p>
            {t('gapLocal')}: {item.local_seasons.length
              ? item.local_seasons.join(', ')
              : t('gapNoLocal')}
          </p>
          <small
            >{#if item.checked_at}{t('gapChecked')}: {date(
                item.checked_at,
              )}{/if}{#if item.inventory_at}<br />{t('gapScanned')}: {date(
                item.inventory_at,
              )}{/if}</small
          >
        </li>{/each}
    </ul>{/if}
  {#if total > limit}<nav
      class="pagination"
      aria-label={t('seasonGapsHeading')}
    >
      <button
        disabled={!offset || loading}
        onclick={() => {
          offset = Math.max(0, offset - limit);
          void load();
        }}>{t('previous')}</button
      >
      <span>{Math.floor(offset / limit) + 1} / {Math.ceil(total / limit)}</span>
      <button
        disabled={offset + limit >= total || loading}
        onclick={() => {
          offset += limit;
          void load();
        }}>{t('nextPage')}</button
      >
    </nav>{/if}
  <p class="metadata-note">{t('gapStaleness')}</p>
</section>

<style>
  .gap-tabs {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    margin: 1rem 0;
  }
  .gap-tabs button {
    background: var(--ts-surface);
    color: var(--ts-text);
    border: 1px solid var(--ts-border);
  }
  .gap-tabs .gap-selected {
    background: var(--ts-primary);
    color: var(--ts-on-primary);
  }
  .gap-search {
    display: flex;
    gap: 0.5rem;
    margin: 1rem 0;
  }
  .gap-search input {
    min-width: 0;
    flex: 1;
    padding: 0.75rem;
    border: 1px solid var(--ts-border);
    border-radius: 0.5rem;
    background: var(--ts-surface);
    color: var(--ts-text);
    font: inherit;
  }
  .gap-list {
    padding: 0;
    list-style: none;
    display: grid;
    gap: 1rem;
  }
  .gap-list > li {
    display: block;
    padding: 1rem;
    border: 1px solid var(--ts-border);
    border-radius: 0.75rem;
    overflow-wrap: anywhere;
    min-width: 0;
  }
  .gap-list ul {
    padding-left: 1.25rem;
    margin: 0.5rem 0;
  }
  .gap-list ul li {
    display: list-item;
    padding: 0.25rem 0;
    border: 0;
  }
  .gap-list strong {
    display: block;
    margin: 0.5rem 0;
  }
  .gap-list p {
    margin: 0.5rem 0;
  }
  .gap-list small {
    color: var(--ts-muted);
  }
  .gap-list details {
    margin: 0.75rem 0;
  }
  .gap-list summary {
    cursor: pointer;
  }
</style>
