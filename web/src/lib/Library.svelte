<script lang="ts">
  import { onMount } from 'svelte';
  import { translate, type Locale, type Key } from './i18n';
  import Icon from './Icon.svelte';
  import MetadataReview from './MetadataReview.svelte';
  import Follow from './Follow.svelte';
  type Title = {
    id: string;
    name: string;
    category: string;
    year?: number;
    overview?: string;
    provenance: string;
    provider_ids: Record<string, string>;
    match_status: string;
    file_count: number;
    source_label: string;
    has_artwork?: boolean;
    metadata_url?: string;
  };
  type FileItem = {
    storage?: string;
    archive_member?: string;
    id: number;
    relative_path: string;
    size: number;
    season: number | null;
    episodes: number[];
    numbering: string;
    observation: string;
  };
  type Source = {
    id: string;
    label: string;
    path: string;
    category: string;
    status: string;
    file_count: number;
    last_success: string | null;
    last_error: string;
    stats: { media?: number };
  };
  let {
    locale,
    mode = 'overview',
    titleID = '',
  }: {
    locale: Locale;
    mode?: 'overview' | 'sources' | 'detail';
    titleID?: string;
  } = $props();
  const t = (key: Key) => translate(locale, key);
  let titles = $state<Title[]>([]);
  let sources = $state<Source[]>([]);
  let detail = $state<(Title & { files: FileItem[] }) | null>(null);
  let search = $state('');
  let category = $state('');
  let offset = $state(0);
  let total = $state(0);
  let loading = $state(true);
  let error = $state('');
  let notice = $state('');
  const limit = 48;
  let version = 0;
  const categories: [string, Key][] = [
    ['', 'all'],
    ['movie', 'movieCategory'],
    ['series', 'seriesCategory'],
    ['anime_movie', 'animeMovieCategory'],
    ['anime_series', 'animeSeriesCategory'],
  ];
  const categoryLabel = (value: string) =>
    t(categories.find(([id]) => id === value)?.[1] || 'all');
  const statusLabel = (value: string) =>
    t(
      (
        {
          ready: 'readyStatus',
          pending: 'pendingStatus',
          scanning: 'scanningStatus',
          error: 'errorStatus',
          review: 'reviewStatus',
          interrupted: 'interruptedStatus',
        } as Record<string, Key>
      )[value] || 'pendingStatus',
    );
  async function json(url: string, init?: RequestInit) {
    const r = await fetch(url, {
      ...init,
      cache: 'no-store',
      signal: AbortSignal.timeout(10000),
    });
    if (!r.ok) throw new Error('request failed');
    return r.json();
  }
  async function load() {
    const current = ++version;
    loading = true;
    error = '';
    try {
      if (mode === 'detail') {
        detail = await json('/api/v1/titles/' + encodeURIComponent(titleID));
      } else if (mode === 'overview') {
        const q = new URLSearchParams({
          q: search,
          category,
          limit: String(limit),
          offset: String(offset),
        });
        const data = await json('/api/v1/titles?' + q);
        if (current !== version) return;
        titles = data.items;
        total = data.total;
      }
      const data = await json('/api/v1/sources');
      if (current === version) sources = data.items;
    } catch {
      if (current === version) error = 'libraryError';
    } finally {
      if (current === version) loading = false;
    }
  }
  async function filter(value: string) {
    category = value;
    offset = 0;
    await load();
  }
  async function scan(id: string) {
    notice = '';
    try {
      await json('/api/v1/sources/' + encodeURIComponent(id) + '/scan', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: '{}',
      });
      notice = 'scanQueued';
      await load();
    } catch {
      error = 'libraryError';
    }
  }
  onMount(() => {
    void load();
    const timer = setInterval(() => {
      if (mode === 'sources') void load();
    }, 15000);
    return () => clearInterval(timer);
  });
</script>

{#if error}<p class="catalogue-message" role="alert">
    {t(error as Key)}
    <button onclick={() => void load()}>{t('refresh')}</button>
  </p>{/if}
{#if notice}<p class="catalogue-message" role="status">
    {t(notice as Key)}
  </p>{/if}
{#if mode === 'sources'}
  <section class="catalogue-intro">
    <h2>{t('sources')}</h2>
    <p>
      {t('readOnly')} · {sources
        .reduce((n, s) => n + s.file_count, 0)
        .toLocaleString(locale)}
      {t('files')}
    </p>
  </section>
  <div class="source-list">
    {#each sources as source}<article class="source-card">
        <div class="section-heading">
          <h3>{source.label}</h3>
          <span
            class="source-status"
            class:source-warning={source.status === 'error' ||
              source.status === 'review'}>{statusLabel(source.status)}</span
          >
        </div>
        <p class="source-path">{source.path}</p>
        <p>
          {categoryLabel(source.category)} · {source.file_count.toLocaleString(
            locale,
          )}
          {t('files')}
        </p>
        <p class="source-date">
          {t('lastScan')}: {source.last_success
            ? new Intl.DateTimeFormat(locale, {
                dateStyle: 'medium',
                timeStyle: 'short',
              }).format(new Date(source.last_success))
            : t('neverScanned')}
        </p>
        {#if source.last_error}<p class="source-warning">
            {source.last_error}
          </p>{/if}<button
          class="catalogue-button"
          disabled={source.status === 'scanning'}
          onclick={() => void scan(source.id)}
          ><Icon kind="refresh" size={16} />{t('scanNow')}</button
        >
      </article>{/each}
  </div>
  {#if !loading && !sources.length}<p>{t('sourceEmpty')}</p>{/if}
{:else if mode === 'detail'}
  <a class="back-link" href="/">← {t('backLibrary')}</a>
  {#if detail}<section class="catalogue-intro">
      <p class="eyebrow">{categoryLabel(detail.category)}</p>
      <h2>{detail.name}</h2>
      <p>
        {detail.year || t('unknownYear')} · {detail.file_count.toLocaleString(
          locale,
        )}
        {t('files')} · {detail.source_label}
      </p>
      {#if detail.overview}<p class="title-description">
          {detail.overview}
        </p>{/if}
      <p class="metadata-note">
        {t('metadataSource')}: {detail.provenance === 'filename'
          ? t('unknownMetadata')
          : detail.provenance}
      </p>
    </section>
    <section>
      {#if detail.metadata_url}<p class="metadata-note">
          <a href={detail.metadata_url} target="_blank" rel="noreferrer"
            >{t('providerAttribution')} ↗</a
          >
        </p>{/if}{#if detail.category === 'series' || detail.category === 'anime_series'}<Follow
          {locale}
          titleID={detail.id}
        /><MetadataReview
          {locale}
          titleID={detail.id}
          name={detail.name}
        />{/if}
      <h3>{t('observedFiles')}</h3>
      {#if detail.files.some((f) => f.storage === 'zip')}<p
          class="metadata-note"
        >
          {t('archivedNote')}
        </p>{/if}
      <ul class="observed-files">
        {#each detail.files as file}<li>
            <div class="file-name">
              {file.relative_path}{#if file.archive_member}!/{file.archive_member}{/if}
            </div>
            <div class="file-note">
              {#if file.storage === 'zip'}<strong>{t('archivedMedia')}</strong> ·
              {/if}
              {#if file.episodes?.length}{t('season')}
                {file.season} · {t('episode')}
                {file.episodes.join(', ')}{:else}{t('unknownNumbering')}{/if} · {(
                file.size /
                1024 /
                1024 /
                1024
              ).toLocaleString(locale, { maximumFractionDigits: 2 })} GiB
            </div>
          </li>{/each}
      </ul>
    </section>{/if}
{:else}
  <section class="library-toolbar" aria-label={t('library')}>
    <div class="library-totals">
      <strong data-testid="library-total">{total.toLocaleString(locale)}</strong
      >
      {t('titles')}<span>
        · {sources.reduce((n, s) => n + s.file_count, 0).toLocaleString(locale)}
        {t('files')}</span
      >
    </div>
    <form
      class="library-search"
      onsubmit={(e) => {
        e.preventDefault();
        offset = 0;
        void load();
      }}
    >
      <input
        type="search"
        aria-label={t('searchLibrary')}
        placeholder={t('searchLibrary')}
        bind:value={search}
      /><button type="submit">{t('search')}</button>
    </form>
    <div class="category-filters" role="group" aria-label={t('library')}>
      {#each categories as [id, key]}<button
          class:chosen={category === id}
          aria-pressed={category === id}
          onclick={() => void filter(id)}>{t(key)}</button
        >{/each}
    </div>
  </section>
  {#if loading}<p class="catalogue-message" role="status">
      {t('loadingLibrary')}
    </p>{/if}
  <div class="title-grid">
    {#each titles as title}<a class="title-card" href={'/titles/' + title.id}
        ><div class="title-art" aria-hidden="true">
          {#if title.has_artwork}<img
              src={'/api/v1/titles/' + title.id + '/artwork'}
              alt=""
              loading="lazy"
              onerror={(e) =>
                ((e.currentTarget as HTMLImageElement).style.display = 'none')}
            />{/if}
          <Icon
            kind={title.category.includes('series') ? 'system' : 'library'}
            size={34}
          /><span>{categoryLabel(title.category)}</span>
        </div>
        <div class="title-card-text">
          <h3>{title.name}</h3>
          <p>
            {title.year ? title.year + ' · ' : ''}{title.file_count}
            {t('files')}
          </p>
          <small
            >{title.provenance === 'filename'
              ? t('unknownMetadata')
              : title.provenance}</small
          >
        </div></a
      >{/each}
  </div>
  {#if !loading && !titles.length}<div class="empty-card">
      <h3>{sources.length ? t('noResults') : t('sourceEmpty')}</h3>
      <p>{sources.length ? t('noResultsHint') : t('sourceEmptyHint')}</p>
    </div>{/if}
  {#if total > limit}<nav class="pagination" aria-label={t('page')}>
      <button
        disabled={offset === 0 || loading}
        onclick={() => {
          offset = Math.max(0, offset - limit);
          void load();
          window.scrollTo(0, 0);
        }}>{t('previous')}</button
      ><span
        >{Math.floor(offset / limit) + 1}
        {t('of')}
        {Math.ceil(total / limit)}</span
      ><button
        disabled={offset + limit >= total || loading}
        onclick={() => {
          offset += limit;
          void load();
          window.scrollTo(0, 0);
        }}>{t('nextPage')}</button
      >
    </nav>{/if}
{/if}
