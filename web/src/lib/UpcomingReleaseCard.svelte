<script lang="ts">
  import Icon from './Icon.svelte';
  import { translate, type Locale, type Key } from './i18n';
  import type { UpcomingEpisode } from './release-types';
  let { episode, locale }: { episode: UpcomingEpisode; locale: Locale } =
    $props();
  const t = (key: Key) => translate(locale, key);
  let failed = $state(false);
  const dateOnly = $derived(episode.precision === 'date');
  const stamp = $derived(
    dateOnly
      ? episode.air_date
        ? new Date(episode.air_date + 'T12:00:00Z')
        : new Date(new Date(episode.scheduled_at).getTime() - 86400000)
      : new Date(episode.scheduled_at),
  );
  const zone = $derived(dateOnly ? { timeZone: 'UTC' } : {});
  const premiere = $derived(episode.season > 0 && episode.number === 1);
  function safeURL(value: string) {
    try {
      const u = new URL(value);
      return u.protocol === 'https:' ? u.href : '#';
    } catch {
      return '#';
    }
  }
</script>

<article class="upcoming-card">
  <header class="schedule-banner">
    <time
      datetime={episode.air_date && dateOnly
        ? episode.air_date
        : episode.scheduled_at}
      class="calendar-date"
      ><strong
        >{new Intl.DateTimeFormat(locale, { day: '2-digit', ...zone }).format(
          stamp,
        )}</strong
      ><span
        >{new Intl.DateTimeFormat(locale, {
          month: 'short',
          year: 'numeric',
          ...zone,
        }).format(stamp)}</span
      ></time
    >
    <div class="schedule-state">
      <span>{t(premiere ? 'seasonPremiere' : 'upcomingEpisode')}</span>
      <p>
        {dateOnly
          ? t('dateOnly')
          : new Intl.DateTimeFormat(locale, { timeStyle: 'short' }).format(
              stamp,
            )}
      </p>
    </div>
  </header>
  <div class="scheduled-title">
    <a
      class="upcoming-art"
      href={'/titles/' + episode.title_id}
      aria-label={t('openTitle') + ': ' + episode.title}
      ><Icon kind="library" size={25} />{#if !failed}<img
          src={'/api/v1/titles/' + episode.title_id + '/artwork'}
          alt={t('gapSeriesArtwork') + ' ' + episode.title}
          loading="lazy"
          onerror={() => (failed = true)}
        />{/if}</a
    >
    <div>
      <a class="release-title" href={'/titles/' + episode.title_id}
        >{episode.title}</a
      >
      <p>{episode.name}</p>
      <small
        >{episode.number !== null
          ? t('season') +
            ' ' +
            episode.season +
            ' · ' +
            t('episode') +
            ' ' +
            episode.number
          : t('special')}</small
      >
    </div>
  </div>
  <footer>
    <span
      >{new Intl.DateTimeFormat(locale, {
        dateStyle: 'medium',
        ...zone,
      }).format(stamp)}</span
    >{#if episode.url}<a
        href={safeURL(episode.url)}
        target="_blank"
        rel="noreferrer">TVmaze ↗</a
      >{/if}
  </footer>
</article>

<style>
  .upcoming-card {
    min-width: 0;
    overflow: hidden;
    border: 1px solid var(--ts-border);
    border-top: 3px solid var(--ts-primary);
    border-radius: 12px;
    background: var(--ts-surface);
    overflow-wrap: anywhere;
  }
  .schedule-banner {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 14px;
    padding: 14px 18px;
    background: var(--ts-surface-subtle);
  }
  .calendar-date {
    display: flex;
    flex-direction: column;
    gap: 3px;
    color: var(--ts-primary);
    font-variant-numeric: tabular-nums;
  }
  .calendar-date strong {
    font-size: 30px;
    font-weight: 650;
    line-height: 1;
  }
  .calendar-date span {
    font-size: 11px;
  }
  .schedule-state {
    text-align: right;
  }
  .schedule-state > span {
    display: inline-block;
    border: 1px dashed var(--ts-primary);
    padding: 4px 7px;
    border-radius: 5px;
    font-size: 10px;
    color: var(--ts-primary);
  }
  .schedule-state p {
    font-size: 11px;
    margin: 7px 0 0;
    color: var(--ts-muted);
  }
  .scheduled-title {
    display: grid;
    grid-template-columns: 64px minmax(0, 1fr);
    gap: 14px;
    padding: 16px 18px;
  }
  .upcoming-art {
    display: grid;
    place-items: center;
    position: relative;
    width: 64px;
    aspect-ratio: 2/3;
    background: var(--ts-selection);
    color: var(--ts-on-selection);
    border-radius: 6px;
    overflow: hidden;
  }
  .upcoming-art img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .scheduled-title p {
    font-size: 12px;
    color: var(--ts-muted);
    line-height: 1.5;
    margin: 7px 0;
  }
  .scheduled-title small {
    font-size: 11px;
    color: var(--ts-muted);
  }
  footer {
    display: flex;
    justify-content: space-between;
    gap: 14px;
    margin: 0 18px;
    padding: 12px 0;
    border-top: 1px solid var(--ts-border);
    font-size: 11px;
    color: var(--ts-muted);
  }
  footer a {
    color: var(--ts-primary);
  }
</style>
