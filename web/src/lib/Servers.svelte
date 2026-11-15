<script lang="ts">
  import { onMount } from 'svelte';
  import { translate, type Locale, type Key } from './i18n';
  import { api, type Server } from './server-types';
  let { locale }: { locale: Locale } = $props();
  const t = (key: Key) => translate(locale, key);
  let items = $state<Server[]>([]);
  let failed = $state(false);
  let saved = $state('');
  let busy = $state(false);
  async function load() {
    try {
      items = (await api('/api/v1/servers')).items;
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
  onMount(() => {
    void load();
  });
</script>

<section class="catalogue-intro">
  <h2>{t('servers')}</h2>
  <p>{t('serversHint')}</p>
</section>
{#if failed}<p role="alert">{t('libraryError')}</p>{/if}
<div class="source-list">
  {#each items as item}<form
      class="source-card connection-form"
      onsubmit={(e) => {
        e.preventDefault();
        void save(item);
      }}
    >
      <h3>{item.name}</h3>
      <p>
        {t(
          item.status === 'ready'
            ? 'readyStatus'
            : item.status === 'scanning'
              ? 'scanningStatus'
              : item.status === 'pending'
                ? 'pendingStatus'
                : 'availabilityUncertain',
        )}
      </p>
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
      <p class="metadata-note">
        {t('lastScan')}: {item.last_success
          ? new Date(item.last_success).toLocaleString(locale)
          : t('neverScanned')}
      </p>
      <button class="catalogue-button" disabled={busy}
        >{t('saveSettings')}</button
      >
      {#if saved === item.id}<p role="status">{t('settingsSaved')}</p>{/if}
    </form>{/each}
</div>
