<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { translate, type Locale, type Key } from './i18n';
  import { api, type Server } from './server-types';
  import TitleCard from './TitleCard.svelte';
  import Icon from './Icon.svelte';
  import ConfirmDialog from './ConfirmDialog.svelte';
  import ServerBadges from './ServerBadges.svelte';
  import TrackingNumber from './TrackingNumber.svelte';

  type AlertMode = 'episode' | 'season' | 'none';
  type Category = 'movie' | 'series';
  type ProviderIDs = Record<string, string>;
  type Episode = {
    id: string;
    name: string;
    season: number;
    number: number | null;
    air_date?: string;
    servers: string[];
  };
  type Draft = {
    name: string;
    category: Category;
    year?: number;
    season?: number;
    episode?: number;
    local_title_id?: string;
    provider_ids?: ProviderIDs;
    source_url?: string;
    alert_mode?: AlertMode;
    air_alerts?: boolean;
    expected_episodes?: number;
    server_id?: string;
    paused?: boolean;
  };
  type Entry = Draft & {
    id?: string;
    status: string;
    season_end_known?: boolean;
    episodes?: Episode[];
    season_names?: Record<number, string>;
    artwork_url?: string;
    servers: {
      server_id: string;
      name: string;
      status: string;
      content: string[];
    }[];
  };
  type Suggestion = {
    id: string;
    name: string;
    category: string;
    year?: number;
  };
  type Candidate = {
    name: string;
    year?: number;
    provider_ids: ProviderIDs;
    url?: string;
  };
  type Preview = {
    draft: Entry;
    candidates?: Candidate[];
    existing_id?: string;
  };

  let {
    locale,
    readOnly,
    showHeading = true,
  }: { locale: Locale; readOnly: boolean; showHeading?: boolean } = $props();
  const t = (key: Key) => translate(locale, key);
  const params = new URLSearchParams(window.location.search);
  const localTitle = params.get('title');
  const trackedID = params.get('tracked');
  const startWithAirAlerts = params.get('air') === '1';
  let started = $state(false);
  let items = $state<Entry[]>([]);
  let input = $state('');
  let servers = $state<Server[]>([]);
  let searchServer = $state<string>();
  let confirmation = $state<{ kind: 'stop' | 'remove'; entry: Entry }>();
  let suggestions = $state<Suggestion[]>([]);
  let focused = $state(false);
  let active = $state(-1);
  let draft = $state<Entry | null>(null);
  let linkedTarget = $state<Entry | null>(null);
  let candidates = $state<Candidate[]>([]);
  let existingID = $state<string | undefined>();
  let loadError = $state(false);
  let actionError = $state<Key>();
  let notice = $state<Key>();
  let busy = $state(false);
  let editingIdentity = $state(false);

  const availability = (status: string) =>
    t(
      status === 'available'
        ? 'availabilityAvailable'
        : status === 'missing'
          ? 'availabilityMissing'
          : 'availabilityUncertain',
    );
  const titleLabel = (entry: { name: string; year?: number }) =>
    entry.year ? entry.name + ' (' + entry.year + ')' : entry.name;
  function publicLink(value: string | undefined) {
    if (!value) return '';
    try {
      const link = new URL(value);
      return link.protocol === 'https:' && !link.username && !link.password
        ? link.href
        : '';
    } catch {
      return '';
    }
  }
  function episodeLabel(episode: Episode) {
    return episode.number == null
      ? t('special')
      : 'S' +
          String(episode.season).padStart(2, '0') +
          'E' +
          String(episode.number).padStart(2, '0');
  }
  function scopedEpisodes(entry: Entry) {
    return (entry.episodes ?? []).filter(
      (episode) =>
        (entry.season == null || episode.season === entry.season) &&
        (entry.episode == null || episode.number === entry.episode),
    );
  }
  function progress(episodes: Episode[]) {
    return t('trackingEpisodeProgress')
      .replace(
        '{available}',
        String(episodes.filter((ep) => ep.servers?.length).length),
      )
      .replace('{total}', String(episodes.length));
  }

  function scopedServerContent(entry: Entry, content: string[]) {
    if (entry.category !== 'series' || entry.season == null) return content;
    const season = 'S' + String(entry.season).padStart(2, '0') + 'E';
    const episode =
      entry.episode == null ? '' : String(entry.episode).padStart(2, '0');
    return content.filter((item) =>
      episode ? item === season + episode : item.startsWith(season),
    );
  }

  function parseTrackingInput(value: string) {
    const input = value.trim();
    if (/^[a-z][a-z0-9+.-]*:\/\//i.test(input)) return { input };
    const match =
      /^(.+?)\s+(?:s(\d{1,4})(?:\s*e(\d{1,6}))?|(?:season|staffel)\s+(\d{1,4}))$/i.exec(
        input,
      );
    if (!match) return { input };
    const season = Number(match[2] ?? match[4]);
    const episode = match[3] ? Number(match[3]) : undefined;
    if (
      season > 1000 ||
      (episode !== undefined && (episode < 1 || episode > 100000))
    )
      return { input };
    return { input: match[1].trim(), season, episode };
  }

  function inventoryUncertain(entry: Entry) {
    return (
      entry.status === 'uncertain' ||
      entry.servers?.some((server) => server.status === 'uncertain')
    );
  }

  const seasonOptions = $derived.by(() => {
    if (!draft) return [];
    const grouped = new Map<number, Episode[]>();
    for (const episode of draft.episodes ?? []) {
      if (episode.number == null) continue;
      const group = grouped.get(episode.season) ?? [];
      group.push(episode);
      grouped.set(episode.season, group);
    }
    const names = draft.season_names ?? {};
    return [...new Set([...grouped.keys(), ...Object.keys(names).map(Number)])]
      .sort((a, b) => a - b)
      .map((number) => {
        const name = names[number];
        const label = t('season') + ' ' + number;
        const episodes = grouped.get(number) ?? [];
        return {
          number,
          label:
            name && name.toLowerCase() !== label.toLowerCase()
              ? label + ' · ' + name
              : label,
          detail: episodes.length ? progress(episodes) : '',
        };
      });
  });

  const episodeOptions = $derived(
    (draft?.episodes ?? [])
      .filter(
        (episode) => episode.season === draft?.season && episode.number != null,
      )
      .map((episode) => ({
        number: episode.number as number,
        label:
          'E' +
          String(episode.number).padStart(2, '0') +
          (episode.name ? ' · ' + episode.name : ''),
        detail: t(
          episode.servers?.length
            ? 'trackingAvailable'
            : draft && inventoryUncertain(draft)
              ? 'trackingUncertain'
              : 'trackingMissing',
        ),
      })),
  );

  function changeSeason(season: number | undefined) {
    if (!draft || draft.season === season) return;
    draft.season = season;
    draft.episode = undefined;
    draft.expected_episodes = undefined;
  }

  function changeEpisode(episode: number | undefined) {
    if (!draft) return;
    draft.episode = episode;
    if (episode != null && draft.alert_mode === 'season')
      draft.alert_mode = 'episode';
  }

  $effect(() => {
    const parsed = parseTrackingInput(input);
    const server = searchServer;
    const query = parsed.input;
    suggestions = [];
    active = -1;
    if (!focused || query.length < 2 || /^[a-z][a-z0-9+.-]*:\/\//i.test(query))
      return;
    const request = new AbortController();
    const timer = setTimeout(async () => {
      try {
        const result = await fetch(
          '/api/v1/titles/suggestions?' +
            new URLSearchParams({ q: query, ...(server ? { server } : {}) }),
          {
            signal: AbortSignal.any([
              request.signal,
              AbortSignal.timeout(5000),
            ]),
          },
        );
        if (!result.ok) return;
        const data = await result.json();
        if (!request.signal.aborted) {
          suggestions =
            parsed.season === undefined
              ? data.items
              : data.items.filter((item: Suggestion) =>
                  item.category.includes('series'),
                );
        }
      } catch {
        // Free-text entry is available when the library lookup fails.
      }
    }, 200);
    return () => {
      clearTimeout(timer);
      request.abort();
    };
  });

  $effect(() => {
    if (!started && !readOnly && localTitle) {
      started = true;
      void preview({ local_title_id: localTitle });
    }
  });

  async function load() {
    try {
      items = (await api('/api/v1/wanted')).items;
      loadError = false;
    } catch {
      loadError = true;
    }
  }
  async function preview(selection: Partial<Draft> = {}) {
    busy = true;
    actionError = undefined;
    notice = undefined;
    focused = false;
    try {
      const parsed = parseTrackingInput(input);
      const result: Preview = await api('/api/v1/wanted/preview', {
        ...parsed,
        category: parsed.season === undefined ? undefined : 'series',
        server_id: searchServer,
        ...selection,
      });
      if (!input && result.draft.name) input = result.draft.name;
      const proposed = result.draft;
      if (
        result.existing_id &&
        !items.some((item) => item.id === result.existing_id)
      ) {
        await load();
      }
      const existing = items.find((item) => item.id === result.existing_id);
      if (result.existing_id && !existing) {
        throw new Error('Existing tracking settings could not be loaded');
      }
      linkedTarget =
        existing &&
        proposed.season != null &&
        (proposed.season !== existing.season ||
          proposed.episode !== existing.episode)
          ? proposed
          : null;
      draft = existing
        ? {
            ...existing,
            episodes: proposed.episodes ?? existing.episodes,
            season_names: proposed.season_names,
            artwork_url: proposed.artwork_url ?? existing.artwork_url,
          }
        : {
            ...proposed,
            alert_mode: proposed.alert_mode ?? 'episode',
            air_alerts:
              (startWithAirAlerts && selection.local_title_id === localTitle) ||
              !!proposed.air_alerts,
            source_url: proposed.source_url || publicLink(input),
          };
      existingID = result.existing_id;
      if (parsed.season !== undefined && linkedTarget) useLinkedTarget();
      candidates = result.candidates || [];
      editingIdentity =
        !draft.local_title_id &&
        (!draft.provider_ids || !Object.keys(draft.provider_ids).length);
    } catch {
      actionError = 'trackingPreviewError';
    } finally {
      busy = false;
    }
  }
  function selectSuggestion(suggestion: Suggestion) {
    if (parseTrackingInput(input).season === undefined) input = suggestion.name;
    void preview({ local_title_id: suggestion.id });
  }
  function handleKeys(event: KeyboardEvent) {
    if (!suggestions.length) return;
    if (event.key === 'Escape') {
      event.preventDefault();
      focused = false;
    } else if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      const delta = event.key === 'ArrowDown' ? 1 : -1;
      active =
        active < 0 && delta < 0
          ? suggestions.length - 1
          : (active + delta + suggestions.length) % suggestions.length;
    } else if (event.key === 'Enter' && active >= 0) {
      event.preventDefault();
      selectSuggestion(suggestions[active]);
    }
  }
  function useLinkedTarget() {
    if (!linkedTarget || !draft) return;
    draft = {
      ...draft,
      season: linkedTarget.season,
      episode: linkedTarget.episode,
      episodes: linkedTarget.episodes,
      servers: linkedTarget.servers,
      expected_episodes: undefined,
      alert_mode:
        linkedTarget.episode != null && draft.alert_mode === 'season'
          ? 'episode'
          : draft.alert_mode,
    };
    linkedTarget = null;
  }
  function clearDraft() {
    draft = null;
    linkedTarget = null;
    candidates = [];
    existingID = undefined;
  }
  const serverChoices = $derived(
    servers.map((server) => ({ server_id: server.id, name: server.name })),
  );

  function selectDraftServer(id: string | undefined) {
    if (!draft || draft.server_id === id) return;
    draft.server_id = id;
    if (id) draft.air_alerts = false;
  }

  async function edit(entry: Entry, server = entry.server_id) {
    draft = { ...entry };
    selectDraftServer(server);
    linkedTarget = null;
    existingID = entry.id;
    candidates = [];
    editingIdentity = false;
    actionError = undefined;
    notice = undefined;
    await tick();
    document
      .getElementById('tracking-preview')
      ?.scrollIntoView({ block: 'nearest' });
    if (entry.category === 'series') {
      try {
        const result: Preview = await api('/api/v1/wanted/preview', {
          input: entry.source_url || entry.name,
          name: entry.name,
          category: entry.category,
          provider_ids: entry.provider_ids,
          local_title_id: entry.local_title_id,
          year: entry.year,
          server_id: server,
        });
        if (draft && existingID === entry.id && draft.server_id === server) {
          draft.episodes = result.draft.episodes ?? draft.episodes;
          draft.season_names = result.draft.season_names;
          draft.artwork_url = result.draft.artwork_url ?? draft.artwork_url;
          draft.servers = result.draft.servers ?? draft.servers;
        }
      } catch {
        // Existing episode evidence and manual scope entry remain available.
      }
    }
  }
  function settings(entry: Draft): Draft {
    const series = entry.category === 'series';
    return {
      name: entry.name,
      category: entry.category,
      year: entry.year,
      provider_ids: entry.provider_ids,
      local_title_id: entry.local_title_id,
      source_url: entry.source_url,
      season: series ? entry.season : undefined,
      episode: series && entry.season != null ? entry.episode : undefined,
      alert_mode: entry.alert_mode ?? 'episode',
      air_alerts: series && !entry.server_id && !!entry.air_alerts,
      server_id: entry.server_id,
      paused: !!entry.paused,
      expected_episodes:
        series &&
        entry.alert_mode === 'season' &&
        (entry.season ?? 0) > 0 &&
        entry.episode == null
          ? entry.expected_episodes
          : undefined,
    };
  }

  async function save() {
    if (!draft) return;
    busy = true;
    actionError = undefined;
    const data = settings(draft);
    try {
      await api(
        existingID
          ? '/api/v1/wanted/' + encodeURIComponent(existingID) + '/settings'
          : '/api/v1/wanted',
        data,
      );
      clearDraft();
      input = '';
      notice = 'trackingSaved';
      await load();
    } catch {
      actionError = 'trackingSaveError';
    } finally {
      busy = false;
    }
  }
  async function remove(id: string) {
    busy = true;
    actionError = undefined;
    try {
      await api('/api/v1/wanted/' + encodeURIComponent(id) + '/remove', {});
      if (existingID === id) clearDraft();
      await load();
    } catch {
      actionError = 'trackingSaveError';
    } finally {
      busy = false;
    }
  }
  async function setPaused(entry: Entry, paused: boolean) {
    if (!entry.id) return;
    busy = true;
    actionError = undefined;
    try {
      await api(
        '/api/v1/wanted/' + encodeURIComponent(entry.id) + '/settings',
        settings({ ...entry, paused }),
      );
      if (draft && existingID === entry.id) draft.paused = paused;
      await load();
    } catch {
      actionError = 'trackingSaveError';
    } finally {
      busy = false;
    }
  }

  function confirm() {
    const action = confirmation;
    confirmation = undefined;
    if (!action?.entry.id) return;
    if (action.kind === 'stop') void setPaused(action.entry, true);
    else void remove(action.entry.id);
  }

  function selectSearchServer(id: string | undefined) {
    searchServer = id;
    clearDraft();
  }

  function scopeBadges(entry: Entry) {
    const options = new Map(
      serverChoices.map((server) => [server.server_id, server]),
    );
    for (const server of entry.servers ?? [])
      options.set(server.server_id, server);
    return [...options.values()];
  }

  function artwork(value?: string) {
    return value?.startsWith('/api/v1/') ? value : undefined;
  }

  onMount(() => {
    void load();
    void api('/api/v1/servers')
      .then((data) => {
        servers = data.items ?? [];
      })
      .catch(() => {});

    const timer = setInterval(() => void load(), 30000);
    return () => clearInterval(timer);
  });
</script>

{#snippet evidence(entry: Entry)}
  {@const episodes = scopedEpisodes(entry).map((episode) => {
    if (!entry.server_id) return episode;
    const server = entry.servers?.find(
      (server) => server.server_id === entry.server_id,
    );
    return {
      ...episode,
      servers: server?.content?.includes(episodeLabel(episode))
        ? [server.name]
        : [],
    };
  })}
  {@const uncertain = inventoryUncertain(entry)}
  {#if episodes.length}
    <details open={entry === draft}>
      <summary>{progress(episodes)}</summary>
      <ul class="tracked-episodes">
        {#each episodes as episode (episode.id)}
          <li>
            <span><strong>{episodeLabel(episode)}</strong> {episode.name}</span>
            <small
              >{episode.servers?.length
                ? episode.servers.join(', ')
                : t(
                    uncertain ? 'availabilityUncertain' : 'availabilityMissing',
                  )}</small
            >
          </li>
        {/each}
      </ul>
    </details>
  {:else if entry.category === 'series'}
    <p class="metadata-note">{t('trackingNoSchedule')}</p>
  {/if}
  {#each (entry.servers || []).filter((server) => !entry.server_id || server.server_id === entry.server_id) as server (server.server_id)}
    {@const content = scopedServerContent(entry, server.content ?? [])}
    {@const status =
      server.status === 'available' && entry.season != null && !content.length
        ? uncertain
          ? 'uncertain'
          : 'missing'
        : server.status}
    <div class="wanted-server">
      <strong>{server.name}</strong><span>{availability(status)}</span>
      {#if content.length}
        <p>
          {content.slice(0, 12).join(', ')}{content.length > 12 ? ' …' : ''}
        </p>
      {/if}
    </div>
  {/each}
{/snippet}

<section aria-label={t('watchlist')}>
  <div class="catalogue-intro">
    {#if showHeading}<h2>{t('watchlist')}</h2>{/if}
    <p>{t('trackingHint')}</p>
  </div>

  {#if !readOnly}
    <form
      class="source-card connection-form"
      onsubmit={(event) => {
        event.preventDefault();
        void preview();
      }}
      onfocusout={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget as Node | null))
          focused = false;
      }}
    >
      {#if serverChoices.length}
        <div
          class="tracking-scope"
          role="group"
          aria-label={t('trackingServerScope')}
        >
          <button
            type="button"
            class="server-badge"
            aria-pressed={!searchServer}
            onclick={() => selectSearchServer(undefined)}
            >{t('allServers')}</button
          >
          <ServerBadges
            servers={serverChoices}
            {locale}
            selected={searchServer}
            onSelect={selectSearchServer}
            disabled={busy}
          />
        </div>
      {/if}
      <label for="tracking-input">{t('trackingInput')}</label>
      <div class="tracking-search">
        <div class="tracking-combobox">
          <input
            id="tracking-input"
            type="search"
            role="combobox"
            autocomplete="off"
            aria-autocomplete="list"
            aria-expanded={suggestions.length > 0}
            aria-controls={suggestions.length
              ? 'tracking-suggestions'
              : undefined}
            aria-activedescendant={active >= 0
              ? 'tracking-option-' + active
              : undefined}
            bind:value={input}
            oninput={clearDraft}
            onfocus={() => (focused = true)}
            onkeydown={handleKeys}
            required
            maxlength="2048"
            disabled={busy}
          />
          {#if suggestions.length}
            <ul
              id="tracking-suggestions"
              class="tracking-suggestions"
              role="listbox"
              aria-label={t('localSuggestions')}
            >
              {#each suggestions as suggestion, index (suggestion.id)}
                <li role="presentation">
                  <button
                    type="button"
                    role="option"
                    id={'tracking-option-' + index}
                    tabindex="-1"
                    aria-selected={active === index}
                    onpointerdown={(event) => {
                      if (event.button === 0) event.preventDefault();
                    }}
                    onclick={() => selectSuggestion(suggestion)}
                  >
                    {titleLabel(suggestion)}
                    <small
                      >{t(
                        suggestion.category.includes('series')
                          ? 'seriesCategory'
                          : 'movieCategory',
                      )}</small
                    >
                  </button>
                </li>
              {/each}
            </ul>
          {/if}
        </div>
        <button class="catalogue-button" disabled={busy}
          >{t('trackingPreview')}</button
        >
      </div>
    </form>
    {#if draft}
      <form
        id="tracking-preview"
        class="source-card connection-form"
        onsubmit={(event) => {
          event.preventDefault();
          void save();
        }}
      >
        <div class="tracking-preview-heading">
          {#if artwork(draft.artwork_url)}
            <img
              src={artwork(draft.artwork_url)}
              alt=""
              onerror={(event) =>
                ((event.currentTarget as HTMLImageElement).hidden = true)}
            />
          {/if}
          <h3>{draft.name ? titleLabel(draft) : t('trackingPreview')}</h3>
        </div>
        {#if scopeBadges(draft).length}
          <div
            class="tracking-scope"
            role="group"
            aria-label={t('trackingServerScope')}
          >
            <button
              type="button"
              class="server-badge"
              aria-pressed={!draft.server_id}
              onclick={() => selectDraftServer(undefined)}
              >{t('allServers')}</button
            >
            <ServerBadges
              servers={scopeBadges(draft)}
              {locale}
              selected={draft.server_id}
              onSelect={selectDraftServer}
              disabled={busy}
            />
          </div>
        {/if}
        {#if existingID}<p class="metadata-note">
            {t('trackingAlreadyTracked')}
          </p>{/if}
        {#if linkedTarget}
          <button
            class="text-button"
            type="button"
            disabled={busy}
            onclick={useLinkedTarget}
          >
            {t(
              linkedTarget.episode == null
                ? 'trackingUseLinkedSeason'
                : 'trackingUseLinkedEpisode',
            )}
          </button>
        {/if}
        {#if !draft.server_id && publicLink(draft.source_url)}
          <p>
            <a
              href={publicLink(draft.source_url)}
              target="_blank"
              rel="noreferrer">{t('trackingSource')} ↗</a
            >
          </p>
        {/if}
        {#if candidates.length}
          <details>
            <summary>{t('trackingOtherMatches')}</summary>
            <ul class="tracked-episodes">
              {#each candidates as candidate}
                <li>
                  <button
                    class="text-button"
                    type="button"
                    disabled={busy}
                    onclick={() =>
                      void preview({
                        name: candidate.name,
                        year: candidate.year,
                        category: 'series',
                        provider_ids: candidate.provider_ids,
                        source_url: publicLink(input) || candidate.url,
                      })}>{titleLabel(candidate)}</button
                  >
                </li>
              {/each}
            </ul>
          </details>
        {/if}
        {#if editingIdentity || !draft.name}
          <div class="tracking-fields">
            <label
              >{t('wantedTitle')}<input
                required
                maxlength="300"
                bind:value={draft.name}
              /></label
            >
            <label
              >{t('trackingCategory')}<select
                bind:value={draft.category}
                onchange={() => {
                  if (
                    draft &&
                    draft.category === 'movie' &&
                    draft.alert_mode === 'season'
                  )
                    draft.alert_mode = 'episode';
                }}
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
                bind:value={draft.year}
              /></label
            >
          </div>
        {:else}
          <button
            type="button"
            class="text-button"
            onclick={() => {
              if (!draft) return;
              editingIdentity = true;
              draft = {
                ...draft,
                provider_ids: undefined,
                local_title_id: undefined,
                episodes: [],
                servers: [],
              };
            }}>{t('trackingEditTitle')}</button
          >
        {/if}
        {#if draft.category === 'series'}
          <div class="tracking-fields scope-fields">
            <TrackingNumber
              id="tracking-season"
              label={t('wantedSeason')}
              value={draft.season}
              min={0}
              max={1000}
              options={seasonOptions}
              onChange={changeSeason}
            />
            <TrackingNumber
              id="tracking-episode"
              label={t('wantedEpisode')}
              value={draft.episode}
              min={1}
              max={100000}
              disabled={draft.season === undefined}
              options={episodeOptions}
              onChange={changeEpisode}
            />
          </div>
        {/if}
        {@render evidence(draft)}
        <div class="tracking-fields">
          <label
            >{t('trackingLibraryAlerts')}<select bind:value={draft.alert_mode}>
              <option value="episode">{t('trackingEpisodeAlert')}</option>
              {#if draft.category === 'series' && draft.episode == null}<option
                  value="season">{t('trackingSeasonAlert')}</option
                >{/if}
              <option value="none">{t('trackingNoAlerts')}</option>
            </select></label
          >
        </div>
        {#if draft.alert_mode === 'season' && (draft.season ?? 0) > 0 && draft.episode == null}
          <label
            >{t('trackingExpectedEpisodes')}
            <input
              type="number"
              min="1"
              max="100000"
              bind:value={draft.expected_episodes}
            />
          </label>
          <p class="metadata-note">{t('trackingExpectedHint')}</p>
        {/if}
        {#if draft.category === 'series' && !draft.server_id}
          <label class="tracking-checkbox"
            ><input type="checkbox" bind:checked={draft.air_alerts} />
            {t('trackingAirAlerts')}</label
          >
        {/if}
        <div class="tracking-actions">
          <button class="catalogue-button" disabled={busy}
            >{existingID ? t('saveSettings') : t('trackingSubmit')}</button
          >
          <button
            type="button"
            class="text-button"
            disabled={busy}
            onclick={clearDraft}>{t('trackingCancel')}</button
          >
        </div>
      </form>
    {/if}
  {/if}

  {#if loadError}<p role="alert">{t('libraryError')}</p>{/if}
  {#if actionError}<p role="alert">{t(actionError)}</p>{/if}
  {#if notice}<p role="status">{t(notice)}</p>{/if}
  {#if !items.length && !loadError}<p class="metadata-note">
      {t('trackingEmpty')}
    </p>{/if}
  <div class="title-grid">
    {#each items as item (item.id)}
      <TitleCard
        name={titleLabel(item)}
        category={item.category}
        categoryLabel={t(
          item.category === 'series' ? 'seriesCategory' : 'movieCategory',
        )}
        artwork={artwork(item.artwork_url)}
        selected={item.id === trackedID}
        onOpen={!readOnly && item.id ? () => void edit(item) : undefined}
      >
        {#if item.season !== undefined}
          <p>
            {t('season')}
            {item.season}{item.episode !== undefined
              ? ' · ' + t('episode') + ' ' + item.episode
              : ''}
          </p>
        {/if}
        <ServerBadges
          servers={item.servers}
          {locale}
          selected={item.server_id}
          onSelect={!readOnly && item.id
            ? (server) => void edit(item, server)
            : undefined}
          disabled={busy}
        />
        <p>{item.paused ? t('trackingStopped') : availability(item.status)}</p>
        {#if !item.server_id && publicLink(item.source_url)}
          <p>
            <a
              href={publicLink(item.source_url)}
              target="_blank"
              rel="noreferrer">{t('trackingSource')} ↗</a
            >
          </p>
        {/if}
        {#if item.alert_mode === 'season' && item.season_end_known === false && !item.paused}
          <p>{t('trackingWaitingForSeason')}</p>
        {/if}
        {#snippet actions()}
          {#if !readOnly && item.id}
            <div class="tracking-actions">
              <button
                type="button"
                class="icon-button"
                disabled={busy}
                title={t(item.paused ? 'trackingResume' : 'trackingRemove')}
                aria-label={t(
                  item.paused ? 'trackingResume' : 'trackingRemove',
                )}
                onclick={() =>
                  item.paused
                    ? void setPaused(item, false)
                    : (confirmation = { kind: 'stop', entry: item })}
              >
                <Icon kind={item.paused ? 'play' : 'stop'} size={19} />
              </button>
              <button
                type="button"
                class="icon-button"
                disabled={busy}
                title={t('trackingDelete')}
                aria-label={t('trackingDelete')}
                onclick={() => (confirmation = { kind: 'remove', entry: item })}
              >
                <Icon kind="trash" size={19} />
              </button>
            </div>
          {/if}
        {/snippet}
      </TitleCard>
    {/each}
  </div>
  <ConfirmDialog
    open={!!confirmation}
    title={t('trackingConfirm')}
    confirmLabel={t(
      confirmation?.kind === 'remove' ? 'trackingDelete' : 'trackingRemove',
    )}
    cancelLabel={t('trackingCancel')}
    onCancel={() => (confirmation = undefined)}
    onConfirm={confirm}
  >
    {#if confirmation}<p>
        {t(
          confirmation.kind === 'remove'
            ? 'trackingDeletePrompt'
            : 'trackingStopPrompt',
        ).replace('{title}', confirmation.entry.name)}
      </p>{/if}
  </ConfirmDialog>
</section>

<style>
  .connection-form {
    margin-bottom: 14px;
  }
  .tracking-search,
  .tracking-actions {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
  }
  .tracking-combobox {
    position: relative;
    min-width: min(100%, 14rem);
    flex: 1;
  }
  .tracking-combobox input {
    width: 100%;
    box-sizing: border-box;
  }
  .tracking-suggestions {
    position: absolute;
    z-index: 10;
    top: 100%;
    inset-inline: 0;
    list-style: none;
    padding: 4px;
    margin-top: 4px;
    max-height: 16rem;
    overflow-y: auto;
    background: var(--ts-surface);
    border: 1px solid var(--ts-border);
    border-radius: 8px;
    box-shadow: 0 6px 18px #00000018;
  }
  .tracking-suggestions button {
    display: flex;
    width: 100%;
    gap: 8px;
    justify-content: space-between;
    padding: 8px;
    border: 0;
    border-radius: 5px;
    background: transparent;
    color: var(--ts-text);
    font: inherit;
    text-align: left;
  }
  .tracking-suggestions button:hover,
  .tracking-suggestions button[aria-selected='true'] {
    background: var(--ts-selection);
    color: var(--ts-on-selection);
  }
  .tracking-fields {
    display: flex;
    flex-wrap: wrap;
    gap: 8px 14px;
  }
  @media (max-width: 560px) {
    .scope-fields {
      flex-direction: column;
    }
  }
  .tracking-fields label {
    min-width: 0;
  }
  .tracking-fields input,
  .tracking-fields select {
    box-sizing: border-box;
    max-width: 100%;
  }
  .tracking-checkbox {
    display: flex;
    align-items: baseline;
    gap: 8px;
  }
  .tracking-checkbox input {
    width: auto;
  }
  .tracked-episodes {
    margin: 8px 0;
    padding: 0;
    list-style: none;
  }
  .tracked-episodes li {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    gap: 4px 12px;
    padding: 6px 0;
    border-top: 1px solid var(--ts-border);
  }
  .tracked-episodes small {
    color: var(--ts-muted);
  }
  .tracking-actions .icon-button {
    min-width: 44px;
    min-height: 44px;
  }
  .tracking-preview-heading {
    display: flex;
    gap: 12px;
    align-items: center;
  }
  .tracking-preview-heading img {
    width: 56px;
    aspect-ratio: 2 / 3;
    object-fit: cover;
    border-radius: 5px;
  }
  .tracking-scope {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
    align-items: center;
    margin: 8px 0;
  }
  .tracking-scope > button {
    cursor: pointer;
  }
  .tracking-scope > button[aria-pressed='true'] {
    border-color: var(--ts-primary);
    background: var(--ts-selection);
    color: var(--ts-on-selection);
  }
</style>
