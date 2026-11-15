<script lang="ts">
  import { tick } from 'svelte';
  import { translate, type Locale, type Key } from './i18n';
  let {
    locale,
    category = '',
    value = $bindable(''),
    onSearch,
  }: {
    locale: Locale;
    category?: string;
    value?: string;
    onSearch: () => void;
  } = $props();
  const t = (key: Key) => translate(locale, key);
  type Suggestion = {
    id: string;
    name: string;
    category: string;
    year?: number;
  };
  let focused = $state(false);
  let items = $state<Suggestion[]>([]);
  let open = $state(false);
  let active = $state(-1);
  let failed = $state(false);
  let pending = $state(false);
  let generation = 0;
  let controller: AbortController | undefined;
  const categoryKey: Record<string, Key> = {
    movie: 'movieCategory',
    series: 'seriesCategory',
    anime_movie: 'animeMovieCategory',
    anime_series: 'animeSeriesCategory',
  };
  function close() {
    generation++;
    controller?.abort();
    open = false;
    active = -1;
    pending = false;
    failed = false;
  }
  $effect(() => {
    const term = value.trim();
    const filter = category;
    const current = ++generation;
    controller?.abort();
    items = [];
    active = -1;
    failed = false;
    open = false;
    pending = false;
    if (!focused || [...term].length < 2) return;
    const timer = setTimeout(async () => {
      if (current !== generation) return;
      const request = new AbortController();
      controller = request;
      pending = true;
      try {
        const response = await fetch(
          '/api/v1/titles/suggestions?' +
            new URLSearchParams({ q: term, category: filter }),
          {
            cache: 'no-store',
            signal: AbortSignal.any([
              request.signal,
              AbortSignal.timeout(5000),
            ]),
          },
        );
        if (!response.ok) throw new Error();
        const data = await response.json();
        if (current === generation) {
          items = data.items;
          open = true;
        }
      } catch {
        if (current === generation) {
          failed = true;
          items = [];
        }
      } finally {
        if (current === generation) pending = false;
      }
    }, 200);
    return () => {
      clearTimeout(timer);
      controller?.abort();
    };
  });
  async function keys(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      event.preventDefault();
      close();
    } else if (
      (event.key === 'ArrowDown' || event.key === 'ArrowUp') &&
      items.length
    ) {
      event.preventDefault();
      open = true;
      active =
        event.key === 'ArrowDown'
          ? (active + 1) % items.length
          : active < 0
            ? items.length - 1
            : (active - 1 + items.length) % items.length;
      await tick();
      document
        .getElementById('local-title-option-' + active)
        ?.scrollIntoView({ block: 'nearest' });
    } else if (event.key === 'Enter' && open && active >= 0 && items[active]) {
      event.preventDefault();
      window.location.assign('/titles/' + encodeURIComponent(items[active].id));
    }
  }
</script>

<form
  class="library-search"
  onsubmit={(event) => {
    event.preventDefault();
    close();
    onSearch();
  }}
  onfocusout={(event) => {
    if (!event.currentTarget.contains(event.relatedTarget as Node)) {
      focused = false;
      close();
    }
  }}
>
  <input
    type="search"
    role="combobox"
    aria-label={t('searchLibrary')}
    placeholder={t('searchLibrary')}
    autocomplete="off"
    aria-autocomplete="list"
    aria-expanded={open && items.length > 0}
    aria-controls={open && items.length ? 'local-title-suggestions' : undefined}
    aria-activedescendant={open && active >= 0
      ? 'local-title-option-' + active
      : undefined}
    bind:value
    onkeydown={keys}
    onfocus={() => {
      focused = true;
      if (items.length) open = true;
    }}
  />
  <button type="submit">{t('search')}</button>
  {#if open && items.length}<ul
      id="local-title-suggestions"
      class="suggestions"
      role="listbox"
      aria-label={t('localSuggestions')}
    >
      {#each items as item, index}<li role="presentation">
          <a
            id={'local-title-option-' + index}
            role="option"
            tabindex="-1"
            aria-selected={active === index}
            class:chosen={active === index}
            href={'/titles/' + encodeURIComponent(item.id)}
            onpointerdown={(event) => {
              if (event.button === 0) event.preventDefault();
            }}
            onpointermove={() => (active = index)}
            ><span>{item.name}</span><small
              >{t(categoryKey[item.category] || 'all')}{item.year
                ? ' · ' + item.year
                : ''}</small
            ></a
          >
        </li>{/each}
    </ul>{/if}
  {#if pending}<span class="suggestion-note" role="status"
      >{t('searchingLocal')}</span
    >
  {:else if failed}<span class="suggestion-note" role="status"
      >{t('suggestionsUnavailable')}</span
    >
  {:else if open && !items.length}<span class="suggestion-note" role="status"
      >{t('noLocalSuggestions')}</span
    >{/if}
</form>

<style>
  .library-search {
    position: relative;
    flex-wrap: wrap;
  }
  .library-search input {
    width: 0;
    min-width: 0;
  }
  .suggestions {
    position: absolute;
    z-index: 10;
    top: 100%;
    left: 0;
    right: 0;
    margin: 6px 0 0;
    padding: 5px;
    list-style: none;
    border: 1px solid var(--ts-border);
    border-radius: 10px;
    background: var(--ts-surface);
    box-shadow: 0 6px 18px #00000018;
    max-height: 350px;
    overflow: auto;
  }
  .suggestions li {
    display: block;
    border: 0;
    padding: 0;
  }
  .suggestions a {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 12px;
    padding: 11px 10px;
    border-radius: 6px;
    text-decoration: none;
    color: var(--ts-text);
    font-size: 13px;
    overflow-wrap: anywhere;
  }
  .suggestions a:hover,
  .suggestions a.chosen {
    background: var(--ts-selection);
    color: var(--ts-on-selection);
  }
  .suggestions small {
    font-size: 10px;
    color: var(--ts-muted);
    flex-shrink: 0;
    max-width: 40%;
    text-align: right;
  }
  .suggestion-note {
    font-size: 11px;
    flex-basis: 100%;
    color: var(--ts-muted);
  }
</style>
