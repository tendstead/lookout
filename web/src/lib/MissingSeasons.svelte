<script lang="ts">
  import { onMount } from 'svelte';
  import { translate, type Locale, type Key } from './i18n';
  let { locale }: { locale: Locale } = $props();
  const t = (key: Key) => translate(locale, key);
  import SeasonGapCard from './SeasonGapCard.svelte';
  import type { Gap, Kind } from './season-gaps';
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
      {#each items as item (item.title_id + ':' + item.kind + ':' + item.season)}<li
        >
          <SeasonGapCard {item} {locale} />
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
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 1rem;
    align-items: start;
  }
  .gap-list > li {
    display: block;
    padding: 0;
    border: 0;
    min-width: 0;
  }
  @media (max-width: 767px) {
    .gap-list {
      grid-template-columns: minmax(0, 1fr);
    }
  }
</style>
