<script lang="ts">
  import { onMount } from 'svelte';
  import { translate, type Locale, type Key } from './i18n';
  let { locale, readOnly }: { locale: Locale; readOnly: boolean } = $props();
  const t = (key: Key) => translate(locale, key);
  let configured = $state(false);
  let verified = $state(false);
  let enabled = $state(false);
  let busy = $state(false);
  let failed = $state(false);
  async function load() {
    try {
      const r = await fetch('/api/v1/telegram/status', {
        cache: 'no-store',
        signal: AbortSignal.timeout(8000),
      });
      if (!r.ok) return;
      const v = await r.json();
      configured = v.configured;
      verified = v.verified;
      enabled = v.delivery_enabled;
    } catch {
      /* Connection panel remains usable when verification is unavailable. */
    }
  }
  async function verify() {
    busy = true;
    failed = false;
    try {
      const r = await fetch('/api/v1/telegram/verify', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: '{}',
        signal: AbortSignal.timeout(12000),
      });
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
    const timer = setInterval(() => void load(), 15000);
    return () => clearInterval(timer);
  });
</script>

<section class="source-card connection-form">
  <h3>{t('telegramConnection')}</h3>
  {#if configured}<p>
      {t('telegramState')}:
      <strong data-testid="telegram-verified"
        >{verified ? t('verified') : t('notVerified')}</strong
      >
    </p>
    <p>{enabled ? t('telegramEnabled') : t('telegramPaused')}</p>
    {#if !readOnly}<button
        class="catalogue-button"
        disabled={busy}
        onclick={() => void verify()}>{t('verifyTelegram')}</button
      >{/if}{:else}<p>{t('telegramNotConfigured')}</p>{/if}
  {#if !readOnly}<p class="metadata-note">{t('telegramVerifyNote')}</p>{/if}
  {#if failed}<p role="alert">{t('telegramError')}</p>{/if}
</section>
