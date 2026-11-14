<script lang="ts">
  import { untrack } from 'svelte';
  import { translate, type Locale, type Key } from './i18n';
  let {
    locale,
    titleID,
    name,
  }: { locale: Locale; titleID: string; name: string } = $props();
  const t = (key: Key) => translate(locale, key);
  type Candidate = {
    id: string;
    provider: string;
    name: string;
    year?: number;
    overview: string;
    url?: string;
  };
  let query = $state(untrack(() => name));
  let items = $state<Candidate[]>([]);
  let open = $state(false);
  let busy = $state(false);
  let failed = $state(false);
  async function search() {
    open = true;
    busy = true;
    failed = false;
    try {
      const r = await fetch(
        '/api/v1/titles/' +
          encodeURIComponent(titleID) +
          '/metadata/search?q=' +
          encodeURIComponent(query),
        { signal: AbortSignal.timeout(10000) },
      );
      if (!r.ok) throw new Error();
      items = (await r.json()).items;
    } catch {
      failed = true;
    } finally {
      busy = false;
    }
  }
  async function match(item: Candidate) {
    busy = true;
    failed = false;
    try {
      const r = await fetch(
        '/api/v1/titles/' + encodeURIComponent(titleID) + '/metadata',
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ provider: item.provider, id: item.id }),
          signal: AbortSignal.timeout(10000),
        },
      );
      if (!r.ok) throw new Error();
      window.location.reload();
    } catch {
      failed = true;
      busy = false;
    }
  }
</script>

<section class="metadata-review">
  <button
    class="catalogue-button secondary"
    disabled={busy}
    onclick={() => void search()}>{t('findMetadata')}</button
  >{#if open}<p class="metadata-note">{t('matchHint')}</p>
    <form
      class="library-search"
      onsubmit={(e) => {
        e.preventDefault();
        void search();
      }}
    >
      <input
        type="search"
        aria-label={t('searchMetadata')}
        bind:value={query}
      /><button type="submit" disabled={busy}>{t('search')}</button>
    </form>
    {#if failed}<p role="alert">{t('metadataSearchError')}</p>{/if}
    <ul class="match-results">
      {#each items as item}<li>
          <div>
            <strong>{item.name}</strong>
            {item.year || ''}
            <p>{item.overview}</p>
          </div>
          <button
            class="catalogue-button"
            disabled={busy}
            onclick={() => void match(item)}>{t('selectMatch')}</button
          >
        </li>{/each}
    </ul>{/if}
</section>
