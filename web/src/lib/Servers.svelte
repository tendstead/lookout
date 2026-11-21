<script lang="ts">
  import { onMount } from 'svelte';
  import { translate, type Locale, type Key } from './i18n';
  import { api, type Server } from './server-types';
  let { locale, readOnly }: { locale: Locale; readOnly: boolean } = $props();
  const t = (key: Key) => translate(locale, key);
  const sourceTypes = {
    jellyfin: 'Jellyfin',
    plex: 'Plex',
    sftp: 'SFTP',
    ssh: 'SSH',
  };
  let items = $state<Server[]>([]);
  let failed = $state(false);
  let saved = $state('');
  let busy = $state(false);
  async function load() {
    try {
      items = (await api('/api/v1/servers')).items;
      failed = false;
    } catch {
      failed = true;
    }
  }
  async function save(v: Server) {
    busy = true;
    failed = false;
    saved = '';
    try {
      await api('/api/v1/servers/' + encodeURIComponent(v.id), {
        name: v.name,
        scan_hours: Number(v.scan_hours),
        stale_hours: Number(v.stale_hours),
      });
      saved = v.id;
      await load();
    } catch {
      failed = true;
    } finally {
      busy = false;
    }
  }
  $effect(() => {
    if (!readOnly) return;
    const timer = setInterval(() => void load(), 10000);
    return () => clearInterval(timer);
  });
  onMount(() => {
    void load();
  });
</script>

{#snippet summary(item: Server)}
  <h3>{item.name}</h3>
  {#if item.type}<p>{sourceTypes[item.type]}</p>{/if}
  <p>
    {t(
      item.status === 'ready'
        ? 'readyStatus'
        : item.status === 'scanning'
          ? 'scanningStatus'
          : item.status === 'pending'
            ? 'pendingStatus'
            : item.status === 'error'
              ? 'errorStatus'
              : 'availabilityUncertain',
    )}
  </p>
  <p class="metadata-note">
    {t('lastScan')}: {item.last_success
      ? new Date(item.last_success).toLocaleString(locale)
      : t('neverScanned')}
  </p>
{/snippet}

<section class="catalogue-intro">
  <h2>{t('servers')}</h2>
  <p>{t(readOnly ? 'configFileHint' : 'serversHint')}</p>
</section>
{#if failed}<p role="alert">{t('libraryError')}</p>{/if}
<div class="source-list">
  {#each items as item}
    {#if readOnly}
      <article class="source-card">
        {@render summary(item)}
        <p>{t('scanHours')}: {item.scan_hours}</p>
        <p>{t('staleHours')}: {item.stale_hours}</p>
      </article>
    {:else}
      <form
        class="source-card connection-form"
        onsubmit={(e) => {
          e.preventDefault();
          void save(item);
        }}
      >
        {@render summary(item)}
        <label
          >{t('serverName')}<input
            bind:value={item.name}
            required
            maxlength="100"
          /></label
        >
        <label
          >{t('scanHours')}<input
            type="number"
            min="1"
            max="168"
            bind:value={item.scan_hours}
            required
          /></label
        >
        <label
          >{t('staleHours')}<input
            type="number"
            min={item.scan_hours}
            max="720"
            bind:value={item.stale_hours}
            required
          /></label
        >
        <button class="catalogue-button" disabled={busy}
          >{t('saveSettings')}</button
        >
        {#if saved === item.id}<p role="status">{t('settingsSaved')}</p>{/if}
      </form>
    {/if}
  {/each}
</div>
