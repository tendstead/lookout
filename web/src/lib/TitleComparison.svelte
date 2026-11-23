<script lang="ts">
  import FileComparison from './FileComparison.svelte';
  import {
    confirmedEpisodes,
    confirmedNumbering,
    type FileItem,
  } from './file-comparison';
  import { translate, type Key, type Locale } from './i18n';

  let {
    files,
    isSeries,
    locale,
    showFlags,
  }: {
    files: FileItem[];
    isSeries: boolean;
    locale: Locale;
    showFlags: boolean;
  } = $props();
  const t = (key: Key) => translate(locale, key);
  let selectedSeason = $state<number>();
  let selectedEpisode = $state<number>();
  const active = $derived(
    files.filter(
      (file) =>
        file.observation === 'observed' || file.observation === 'stable',
    ),
  );
  const seasons = $derived(confirmedEpisodes(active));
  const season = $derived(
    seasons.find((option) => option.season === selectedSeason) ?? seasons[0],
  );
  const episode = $derived(
    season?.episodes.includes(selectedEpisode ?? -1)
      ? selectedEpisode
      : season?.episodes[0],
  );
  const copies = $derived(
    isSeries
      ? active.filter(
          (file) =>
            confirmedNumbering(file) &&
            file.season === season?.season &&
            episode != null &&
            file.episodes?.includes(episode),
        )
      : active,
  );
  let open = $state(false);
</script>

<details class="title-comparison" bind:open>
  <summary>{t('compareFiles')}</summary>
  {#if open}
    {#if isSeries && season}
      <div class="selectors">
        <label>
          {t('season')}
          <select
            value={season.season}
            onchange={(event) => {
              selectedSeason = Number(event.currentTarget.value);
              selectedEpisode = undefined;
            }}
          >
            {#each seasons as option (option.season)}
              <option value={option.season}>{option.season}</option>
            {/each}
          </select>
        </label>
        <label>
          {t('episode')}
          <select
            value={episode}
            onchange={(event) =>
              (selectedEpisode = Number(event.currentTarget.value))}
          >
            {#each season.episodes as option (option)}
              <option value={option}>{option}</option>
            {/each}
          </select>
        </label>
      </div>
    {/if}
    {#if copies.length}
      <FileComparison
        files={copies}
        {locale}
        {showFlags}
        crossServerOnly={false}
      />
    {:else}
      <p>{t(isSeries ? 'noComparableFiles' : 'noDifferences')}</p>
    {/if}
  {/if}
</details>

<style>
  .title-comparison {
    margin: 18px 0;
    padding: 12px;
    border: 1px solid var(--ts-border);
    border-radius: 12px;
    min-width: 0;
  }
  summary {
    cursor: pointer;
    padding: 4px;
    font-weight: 650;
  }
  .title-comparison[open] summary {
    margin-bottom: 12px;
  }
  .selectors {
    display: flex;
    flex-wrap: wrap;
    gap: 8px 16px;
    margin-bottom: 12px;
  }
  label {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  select {
    min-height: 40px;
    min-width: 56px;
    padding: 6px;
    border: 1px solid var(--ts-border);
    border-radius: 6px;
    background: var(--ts-surface);
    color: var(--ts-text);
    font: inherit;
  }
</style>
