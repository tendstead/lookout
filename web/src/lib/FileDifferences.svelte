<script lang="ts">
  import { onMount } from 'svelte';
  import FileComparison from './FileComparison.svelte';
  import Icon from './Icon.svelte';
  import {
    compareCopies,
    copyPreferenceKey,
    episodeCode,
    type DifferenceGroup,
    type FileItem,
  } from './file-comparison';
  import { translate, type Key, type Locale } from './i18n';
  import { api } from './server-types';

  let { locale, showFlags }: { locale: Locale; showFlags: boolean } = $props();
  const t = (key: Key) => translate(locale, key);
  const storageKey = 'lookout.hiddenCopies';
  const limit = 50;
  let groups = $state<DifferenceGroup[]>([]);
  let hidden = $state(new Set<string>());
  let showHidden = $state(false);
  let loading = $state(true);
  let failed = $state(false);
  let hasMore = $state(false);
  let offset = 0;
  let dialog: HTMLDialogElement;
  let activeTitleID = $state<string>();
  let selectedIndex = $state(0);
  const hiddenCount = $derived(
    new Set(
      groups.flatMap((group) =>
        group.files.filter(isHidden).map(copyPreferenceKey),
      ),
    ).size,
  );
  const visibleGroups = $derived.by(() => {
    if (showHidden) return groups.filter((group) => group.files.some(isHidden));
    return groups
      .map((group) => ({
        ...group,
        files: group.files.filter((file) => !isHidden(file)),
      }))
      .filter((group) => compareCopies(group.files).size > 0);
  });

  const cards = $derived.by(() => {
    const cards = new Map<string, DifferenceGroup[]>();
    for (const group of visibleGroups) {
      const existing = cards.get(group.title_id);
      if (existing) existing.push(group);
      else cards.set(group.title_id, [group]);
    }
    return cards;
  });
  const activeGroups = $derived(
    activeTitleID ? cards.get(activeTitleID) : undefined,
  );
  const active = $derived(activeGroups?.[selectedIndex] ?? activeGroups?.[0]);

  $effect(() => {
    if (activeTitleID && !active) dialog.close();
  });

  function openCard(group: DifferenceGroup) {
    activeTitleID = group.title_id;
    selectedIndex = 0;
    dialog.showModal();
  }

  function isHidden(file: FileItem) {
    const key = copyPreferenceKey(file);
    return key != null && hidden.has(key);
  }
  function setHidden(file: FileItem, shouldHide: boolean) {
    const key = copyPreferenceKey(file);
    if (!key) return;
    const next = new Set(hidden);
    if (shouldHide) next.add(key);
    else next.delete(key);
    hidden = next;
    try {
      localStorage.setItem(storageKey, JSON.stringify([...next]));
    } catch {
      /* Keep the preferences for this visit. */
    }
  }
  async function load() {
    loading = true;
    failed = false;
    try {
      const data: { items?: DifferenceGroup[]; has_more: boolean } = await api(
        `/api/v1/file-differences?limit=${limit}&offset=${offset}`,
      );
      const incoming = data.items ?? [];
      groups = [...groups, ...incoming];
      offset += incoming.length;
      hasMore = incoming.length > 0 && data.has_more;
    } catch {
      failed = true;
    } finally {
      loading = false;
    }
  }
  onMount(() => {
    try {
      const saved: unknown = JSON.parse(
        localStorage.getItem(storageKey) ?? '[]',
      );
      if (Array.isArray(saved))
        hidden = new Set(
          saved.filter((item): item is string => typeof item === 'string'),
        );
    } catch {
      /* Use the default view when browser storage is unavailable. */
    }
    void load();
  });
</script>

<section aria-label={t('fileDifferences')}>
  <div class="file-toolbar">
    <button
      type="button"
      class="catalogue-button secondary"
      aria-pressed={showHidden}
      onclick={() => (showHidden = !showHidden)}
    >
      {showHidden
        ? t('showDifferences')
        : `${t('showHidden')} (${hiddenCount})`}
    </button>
  </div>
  <div class="title-grid">
    {#each cards as [titleID, entries] (titleID)}
      {@const group = entries[0]}
      <button
        type="button"
        class="title-card difference-card"
        onclick={() => openCard(group)}
      >
        <span class="title-art" aria-hidden="true">
          {#if group.has_artwork}
            <img
              src={'/api/v1/titles/' + encodeURIComponent(titleID) + '/artwork'}
              alt=""
              loading="lazy"
              onerror={(event) =>
                ((event.currentTarget as HTMLImageElement).style.display =
                  'none')}
            />
          {/if}
          <Icon kind={group.episode == null ? 'library' : 'system'} size={34} />
        </span>
        <span class="title-card-text">
          <span class="card-title">{group.title_name}</span>
          {#if entries.length > 1}
            <small>{entries.length} {t('episodeComparisons')}</small>
          {:else if group.season != null && group.episode != null}
            <small>{episodeCode(group.season, [group.episode])}</small>
          {/if}
        </span>
      </button>
    {/each}
  </div>

  <dialog
    bind:this={dialog}
    aria-labelledby="difference-dialog-title"
    onclose={() => (activeTitleID = undefined)}
  >
    {#if active}
      <div class="dialog-heading">
        <h2 id="difference-dialog-title">{active.title_name}</h2>
        <button
          type="button"
          class="catalogue-button secondary"
          onclick={() => dialog.close()}>{t('closeComparison')}</button
        >
      </div>
      {#if activeGroups && activeGroups.length > 1}
        <label class="episode-picker">
          {t('episode')}
          <select
            value={activeGroups.indexOf(active)}
            onchange={(event) =>
              (selectedIndex = Number(event.currentTarget.value))}
          >
            {#each activeGroups as group, index}
              <option value={index}
                >{episodeCode(group.season ?? 0, [group.episode ?? 0])}</option
              >
            {/each}
          </select>
        </label>
      {:else if active.season != null && active.episode != null}
        <p class="episode-picker">
          {episodeCode(active.season, [active.episode])}
        </p>
      {/if}
      <FileComparison files={active.files} {locale} {showFlags}>
        {#snippet copyActions(file)}
          {#if isHidden(file)}
            <div class="hidden-label">{t('hiddenCopy')}</div>
            <button
              type="button"
              class="copy-action"
              onclick={() => setHidden(file, false)}>{t('restoreCopy')}</button
            >
          {:else if !showHidden && copyPreferenceKey(file)}
            <button
              type="button"
              class="copy-action"
              onclick={() => setHidden(file, true)}>{t('hideCopy')}</button
            >
          {/if}
        {/snippet}
      </FileComparison>
      <a
        class="title-link"
        href={'/titles/' + encodeURIComponent(active.title_id)}
        >{t('openComparisonTitle')}</a
      >
    {/if}
  </dialog>
  {#if failed}
    <p class="catalogue-message" role="alert">{t('libraryError')}</p>
    <button class="catalogue-button" onclick={() => void load()}
      >{t('refresh')}</button
    >
  {:else if loading}
    <p class="catalogue-message" role="status">{t('loadingLibrary')}</p>
  {:else}
    {#if !visibleGroups.length && !hasMore}
      <p role="status">{t(showHidden ? 'noHiddenCopies' : 'noDifferences')}</p>
    {/if}
    {#if hasMore}
      <button class="catalogue-button" onclick={() => void load()}
        >{t('loadMoreDifferences')}</button
      >
    {/if}
  {/if}
</section>

<style>
  .file-toolbar {
    display: flex;
    justify-content: flex-end;
    margin-bottom: 12px;
  }
  .file-toolbar button[aria-pressed='true'] {
    background: var(--ts-selection);
    color: var(--ts-on-selection);
  }
  .difference-card {
    text-align: start;
    padding: 0;
    font: inherit;
    cursor: pointer;
  }
  .title-card-text,
  .card-title {
    display: block;
  }
  .card-title {
    font-size: 0.875rem;
    font-weight: 600;
    overflow-wrap: anywhere;
  }
  dialog {
    width: min(960px, calc(100% - 24px));
    max-height: calc(100% - 24px);
    max-width: none;
    box-sizing: border-box;
    padding: 16px;
    border: 1px solid var(--ts-border);
    border-radius: 12px;
    background: var(--ts-surface);
    color: var(--ts-text);
  }
  dialog::backdrop {
    background: rgb(0 0 0 / 60%);
  }
  .dialog-heading {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 12px;
    margin-bottom: 12px;
  }
  h2 {
    font-size: 1.125rem;
    margin: 0;
    overflow-wrap: anywhere;
  }
  .episode-picker,
  .title-link {
    display: block;
    margin: 12px 0;
  }
  .episode-picker select {
    margin-inline-start: 6px;
  }
  .copy-action {
    min-height: 34px;
    padding: 4px 6px;
    border: 1px solid var(--ts-border);
    border-radius: 6px;
    background: var(--ts-surface);
    color: var(--ts-text);
    margin-top: 8px;
    cursor: pointer;
    font: inherit;
    font-size: 0.75rem;
  }
  .hidden-label {
    margin-top: 6px;
    font-size: 0.75rem;
    font-weight: normal;
    color: var(--ts-muted);
  }
</style>
