<script lang="ts">
  import { onMount } from 'svelte';
  import { translate, type Locale, type Key } from './i18n';
  let {
    locale,
    titleID,
    readOnly,
  }: { locale: Locale; titleID: string; readOnly: boolean } = $props();
  const t = (key: Key) => translate(locale, key);
  let enabled = $state(false);
  let initialized = $state(false);
  let ready = $state(false);
  let busy = $state(false);
  let failed = $state(false);
  async function load() {
    try {
      const r = await fetch(
        '/api/v1/titles/' + encodeURIComponent(titleID) + '/follow',
        { signal: AbortSignal.timeout(8000) },
      );
      if (!r.ok) return;
      const v = await r.json();
      enabled = v.enabled;
      initialized = v.initialized;
      ready = true;
    } catch {
      /* Catalogue detail remains visible during a follow-settings failure. */
    }
  }
  async function unfollow() {
    busy = true;
    failed = false;
    try {
      const r = await fetch(
        '/api/v1/titles/' + encodeURIComponent(titleID) + '/follow',
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ enabled: false }),
          signal: AbortSignal.timeout(8000),
        },
      );
      if (!r.ok) throw new Error();
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

{#if !readOnly || (ready && enabled)}
  <section class="follow-controls">
    {#if !readOnly}
      <a
        class="catalogue-button"
        href={'/wanted?title=' + encodeURIComponent(titleID) + '&air=1'}
      >
        {enabled ? t('trackingEdit') : t('followReleases')}
      </a>
      {#if enabled}
        <button
          class="text-button"
          disabled={busy}
          onclick={() => void unfollow()}
        >
          {t('unfollowReleases')}
        </button>
      {/if}
    {/if}
    {#if enabled}
      <p class="metadata-note">
        {t('followingReleases')}{#if !initialized}
          · {t('followPending')}{/if}
      </p>
    {/if}
    {#if failed}<p role="alert">{t('followError')}</p>{/if}
  </section>
{/if}
