<script lang="ts">
  import { onMount } from 'svelte';
  import { translate, type Locale, type Key } from './i18n';
  import { api } from './server-types';
  type Entry = {
    id: string;
    name: string;
    category: string;
    year?: number;
    season?: number;
    episode?: number;
    status: string;
    servers: {
      server_id: string;
      name: string;
      status: string;
      content: string[];
    }[];
  };
  let { locale, readOnly }: { locale: Locale; readOnly: boolean } = $props();
  const t = (key: Key) => translate(locale, key);
  let items = $state<Entry[]>([]);
  let name = $state('');
  let category = $state('movie');
  let year = $state<number | undefined>();
  let season = $state<number | undefined>();
  let episode = $state<number | undefined>();
  let failed = $state(false);
  let busy = $state(false);
  const label = (status: string) =>
    t(
      status === 'available'
        ? 'availabilityAvailable'
        : status === 'missing'
          ? 'availabilityMissing'
          : 'availabilityUncertain',
    );
  async function load() {
    try {
      items = (await api('/api/v1/wanted')).items;
    } catch {
      failed = true;
    }
  }
  async function add() {
    busy = true;
    failed = false;
    try {
      await api('/api/v1/wanted', {
        name,
        category,
        year: year || undefined,
        season: category === 'series' ? season : undefined,
        episode: category === 'series' ? episode : undefined,
      });
      name = '';
      year = undefined;
      season = undefined;
      episode = undefined;
      await load();
    } catch {
      failed = true;
    } finally {
      busy = false;
    }
  }
  async function remove(id: string) {
    busy = true;
    try {
      await api('/api/v1/wanted/' + id + '/remove', {});
      await load();
    } catch {
      failed = true;
    } finally {
      busy = false;
    }
  }
  onMount(() => {
    void load();
    const timer = setInterval(() => void load(), 30000);
    return () => clearInterval(timer);
  });
</script>

<section class="catalogue-intro">
  <h2>{t('wanted')}</h2>
  <p>{t('wantedHint')}</p>
</section>
{#if failed}<p role="alert">{t('libraryError')}</p>{/if}
{#if !readOnly}<form
    class="source-card connection-form wanted-form"
    onsubmit={(e) => {
      e.preventDefault();
      void add();
    }}
  >
    <label
      >{t('wantedTitle')}<input
        bind:value={name}
        required
        maxlength="300"
      /></label
    >
    <label
      >{t('serverType')}<select bind:value={category}
        ><option value="movie">{t('movieCategory')}</option><option
          value="series">{t('seriesCategory')}</option
        ></select
      ></label
    >
    <label
      >{t('wantedYear')}<input
        type="number"
        min="1800"
        max="3000"
        bind:value={year}
      /></label
    >
    {#if category === 'series'}<label
        >{t('wantedSeason')}<input
          type="number"
          min="0"
          max="1000"
          bind:value={season}
        /></label
      ><label
        >{t('wantedEpisode')}<input
          type="number"
          min="1"
          max="100000"
          bind:value={episode}
          disabled={season === undefined}
        /></label
      >{/if}
    <button class="catalogue-button" disabled={busy}>{t('addWanted')}</button>
  </form>{/if}
<div class="source-list">
  {#each items as item}<article class="source-card wanted-card">
      <h3>{item.name}{item.year ? ' (' + item.year + ')' : ''}</h3>
      <p>
        {#if item.season !== undefined}{t('season')}
          {item.season}{item.episode !== undefined
            ? ' · ' + t('episode') + ' ' + item.episode
            : ''} ·
        {/if}<strong>{label(item.status)}</strong>
      </p>
      {#each item.servers as server}<div class="wanted-server">
          <strong>{server.name}</strong><span>{label(server.status)}</span
          >{#if server.content.length}<p>
              {server.content.slice(0, 12).join(', ')}{server.content.length >
              12
                ? ' …'
                : ''}
            </p>{/if}
        </div>{/each}
      {#if !readOnly}<button
          class="text-button"
          disabled={busy}
          onclick={() => void remove(item.id)}>{t('removeWanted')}</button
        >{/if}
    </article>{/each}
</div>
