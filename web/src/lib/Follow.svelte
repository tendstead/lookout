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
  async function toggle() {
    busy = true;
    failed = false;
    try {
      const r = await fetch(
        '/api/v1/titles/' + encodeURIComponent(titleID) + '/follow',
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ enabled: !enabled }),
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

{#if ready && (enabled || !readOnly)}<section class="follow-controls">
    {#if !readOnly}<button
        class="catalogue-button"
        aria-pressed={enabled}
        disabled={busy}
        onclick={() => void toggle()}
        >{enabled ? t('unfollowReleases') : t('followReleases')}</button
      >{/if}
    <p class="metadata-note">
      {enabled
        ? t('followingReleases')
        : t('baselineNote')}{#if enabled && !initialized}
        · {t('followPending')}{/if}
    </p>
    {#if failed}<p role="alert">{t('followError')}</p>{/if}
  </section>{/if}
