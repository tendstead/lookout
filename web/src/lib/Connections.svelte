<script lang="ts">
  import { onMount } from 'svelte';
  import Telegram from './Telegram.svelte';
  import { translate, type Locale, type Key } from './i18n';
  type Mapping = {
    server_prefix: string;
    source_id: string;
    source_prefix?: string;
  };
  type Connection = {
    id: string;
    kind: string;
    url: string;
    verified: boolean;
    path_mappings: Mapping[];
  };
  type Source = { id: string; label: string; path: string };
  let { locale }: { locale: Locale } = $props();
  const t = (key: Key) => translate(locale, key);
  let items = $state<Connection[]>([]);
  let sources = $state<Source[]>([]);
  let kind = $state('jellyfin');
  let serverURL = $state('');
  let token = $state('');
  let mappings = $state<Mapping[]>([]);
  let busy = $state(false);
  let ready = $state(false);
  let publicMetadata = $state(false);
  let notice = $state<Key | null>(null);
  let failed = $state(false);
  let progress = $state({
    running: false,
    matched: 0,
    review: 0,
    unmatched: 0,
  });
  async function request(url: string, body?: unknown) {
    const r = await fetch(url, {
      method: body === undefined ? 'GET' : 'POST',
      headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
      cache: 'no-store',
      signal: AbortSignal.timeout(12000),
    });
    if (!r.ok) throw new Error('connection request failed');
    return r.json();
  }
  async function load() {
    try {
      const [data, root] = await Promise.all([
        request('/api/v1/connections'),
        request('/api/v1/sources'),
      ]);
      items = data.items;
      ready = data.capabilities.connections_ready;
      publicMetadata = data.capabilities.public_metadata_enabled;
      progress = data.progress;
      sources = root.items;
    } catch {
      failed = true;
    }
  }
  async function save() {
    busy = true;
    notice = null;
    failed = false;
    try {
      await request('/api/v1/connections', {
        kind,
        url: serverURL,
        token,
        path_mappings: mappings
          .filter((m) => m.server_prefix && m.source_id)
          .map((m) => ({
            ...m,
            source_prefix:
              sources.find((s) => s.id === m.source_id)?.path || '',
          })),
      });
      token = '';
      notice = 'savedConnection';
      await load();
    } catch {
      failed = true;
    } finally {
      busy = false;
    }
  }
  async function action(id: string, action: string) {
    busy = true;
    notice = null;
    failed = false;
    try {
      await request(
        '/api/v1/connections/' + encodeURIComponent(id) + '/' + action,
        {},
      );
      notice = action === 'verify' ? 'connectionVerified' : 'metadataQueued';
      await load();
    } catch {
      failed = true;
    } finally {
      busy = false;
    }
  }
  async function official() {
    busy = true;
    notice = null;
    failed = false;
    try {
      await request('/api/v1/metadata/official', {});
      notice = 'metadataQueued';
      await load();
    } catch {
      failed = true;
    } finally {
      busy = false;
    }
  }
  onMount(() => {
    void load();
    const timer = setInterval(() => void load(), 4000);
    return () => clearInterval(timer);
  });
</script>

<section class="catalogue-intro">
  <h2>{t('metadataConnections')}</h2>
  <p>{t('connectionIntro')}</p>
</section>
{#if failed}<p class="catalogue-message" role="alert">
    {t('connectionError')}
  </p>{/if}
{#if notice}<p class="catalogue-message" role="status">{t(notice)}</p>{/if}
{#if progress.running}<p class="catalogue-message" role="status">
    {t('metadataRunning')}
    {progress.matched || 0}
    {t('matched')} · {progress.review || 0}
    {t('needsReview')}
  </p>{/if}
<div class="source-list">
  {#each items as item}<article class="source-card">
      <div class="section-heading">
        <h3>{item.kind === 'jellyfin' ? 'Jellyfin' : 'Plex'}</h3>
        <span class="source-status"
          >{item.verified ? t('verified') : t('notVerified')}</span
        >
      </div>
      <p class="source-path">{item.url}</p>
      {#each item.path_mappings || [] as m}<p class="source-date">
          {m.server_prefix} → {sources.find((s) => s.id === m.source_id)
            ?.label || m.source_id}
        </p>{/each}
      <div class="connection-actions">
        <button
          class="catalogue-button"
          disabled={busy}
          onclick={() => void action(item.id, 'verify')}
          >{t('verifyConnection')}</button
        ><button
          class="catalogue-button"
          disabled={busy || progress.running || !item.path_mappings?.length}
          onclick={() => void action(item.id, 'import')}
          >{t('importMetadata')}</button
        >
      </div>
    </article>{/each}
</div>
{#if !items.length}<p class="metadata-note">{t('noConnections')}</p>{/if}
<section class="source-card connection-form">
  <form
    onsubmit={(e) => {
      e.preventDefault();
      void save();
    }}
  >
    <label
      >{t('serverType')}<select bind:value={kind} disabled={!ready}
        ><option value="jellyfin">Jellyfin</option><option value="plex"
          >Plex</option
        ></select
      ></label
    ><label
      >{t('serverURL')}<input
        type="url"
        bind:value={serverURL}
        required
        placeholder="http://media-server:8096"
        disabled={!ready}
      /></label
    ><label
      >{t('accessToken')}<input
        type="password"
        bind:value={token}
        required
        autocomplete="new-password"
        disabled={!ready}
      /></label
    >
    <p class="metadata-note">{t('tokenNote')}</p>
    <p class="metadata-note">{t('mappingNote')}</p>
    {#each mappings as m, i}<div class="path-mapping">
        <label
          >{t('serverPrefix')}<input
            bind:value={m.server_prefix}
            placeholder="/data/Movies"
            required
          /></label
        ><label
          >{t('localSource')}<select bind:value={m.source_id} required
            ><option value="">—</option>{#each sources as source}<option
                value={source.id}>{source.label}</option
              >{/each}</select
          ></label
        ><button
          type="button"
          class="mapping-remove"
          aria-label={t('removeMapping')}
          onclick={() =>
            (mappings = mappings.filter((_, index) => index !== i))}>×</button
        >
      </div>{/each}
    <div class="connection-actions">
      <button
        type="button"
        class="catalogue-button secondary"
        disabled={!ready}
        onclick={() =>
          (mappings = [...mappings, { server_prefix: '', source_id: '' }])}
        >{t('addMapping')}</button
      ><button type="submit" class="catalogue-button" disabled={!ready || busy}
        >{t('saveConnection')}</button
      >
    </div>
    {#if !ready}<p class="metadata-note">{t('connectionsDisabled')}</p>{/if}
  </form>
</section>
{#if publicMetadata}<section class="source-card connection-form">
    <h3>TVmaze</h3>
    <p>{t('officialPrivacy')}</p>
    <button
      class="catalogue-button"
      disabled={busy || progress.running}
      onclick={() => void official()}>{t('refreshOfficial')}</button
    >
    <p class="metadata-note">
      <a href="https://www.tvmaze.com" target="_blank" rel="noreferrer"
        >TVmaze</a
      >
      ·
      <a
        href="https://creativecommons.org/licenses/by-sa/4.0/"
        target="_blank"
        rel="noreferrer">CC BY-SA</a
      >
    </p>
  </section>{/if}

<Telegram {locale} />
