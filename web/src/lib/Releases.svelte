<script lang="ts">
  import { onMount } from 'svelte';
  import { translate, type Locale, type Key } from './i18n';
  let { locale }: { locale: Locale } = $props();
  const t = (key: Key) => translate(locale, key);
  type Episode = {
    title_id: string;
    title: string;
    name: string;
    season: number;
    number: number | null;
    scheduled_at: string;
    air_date?: string;
    precision: string;
    url: string;
  };
  type Event = Omit<Episode, 'scheduled_at'> & {
    id: number;
    kind: string;
    reported_at: string;
    delivery: string;
  };
  let upcoming = $state<Episode[]>([]);
  let unmatchedTitles = $state<{ title_id: string; name: string }[]>([]);
  let recent = $state<Event[]>([]);
  let following = $state(0);
  let ready = $state(0);
  let unmatched = $state(0);
  let running = $state(false);
  let allowed = $state(false);
  let failed = $state(false);
  let offset = $state(0);
  let upcomingTotal = $state(0);
  let recentTotal = $state(0);
  const limit = 48;
  async function load() {
    try {
      const response = await fetch(
        '/api/v1/releases?limit=' + limit + '&offset=' + offset,
        { cache: 'no-store', signal: AbortSignal.timeout(8000) },
      );
      if (!response.ok) throw new Error();
      const { data, progress } = await response.json();
      upcoming = data.upcoming;
      recent = data.recent;
      following = data.following;
      ready = data.ready;
      unmatched = data.unmatched;
      unmatchedTitles = data.unmatched_titles || [];
      upcomingTotal = data.upcoming_total;
      recentTotal = data.recent_total;
      running = progress.running;
      allowed = progress.allowed;
      failed = false;
    } catch {
      failed = true;
    }
  }
  async function refresh() {
    try {
      const r = await fetch('/api/v1/releases/refresh', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: '{}',
        signal: AbortSignal.timeout(8000),
      });
      if (!r.ok) throw new Error();
      running = true;
      await load();
    } catch {
      failed = true;
    }
  }
  function date(stamp: string, precision: string, airDate?: string) {
    if (precision === 'date') {
      const value = airDate
        ? new Date(airDate + 'T12:00:00Z')
        : new Date(new Date(stamp).getTime() - 86400000);
      return (
        new Intl.DateTimeFormat(locale, {
          dateStyle: 'medium',
          timeZone: 'UTC',
        }).format(value) +
        ' · ' +
        t('dateOnly')
      );
    }
    return new Intl.DateTimeFormat(locale, {
      dateStyle: 'medium',
      timeStyle: 'short',
    }).format(new Date(stamp));
  }
  function safeURL(value: string) {
    try {
      const u = new URL(value);
      return u.protocol === 'https:' ? u.href : '#';
    } catch {
      return '#';
    }
  }
  onMount(() => {
    void load();
    const timer = setInterval(() => void load(), 10000);
    return () => clearInterval(timer);
  });
</script>

<section class="catalogue-intro">
  <h2>{t('releases')}</h2>
  <p>{t('releaseNotice')}</p>
  <p>
    {following}
    {t('releaseFollowing')} · {ready}
    {t('scheduleReady')} · {unmatched}
    {t('matchNeeded')}
  </p>
  <button
    class="catalogue-button"
    disabled={!allowed || running}
    onclick={() => void refresh()}
    >{running ? t('checkingButton') : t('refreshReleases')}</button
  >
</section>
{#if failed}<p class="catalogue-message" role="alert">
    {t('releaseError')}
  </p>{/if}
{#if unmatchedTitles.length}<details class="unmatched-titles">
    <summary>{unmatched} {t('matchNeeded')}</summary>
    <ul>
      {#each unmatchedTitles as title}<li>
          <a href={'/titles/' + title.title_id}>{title.name}</a>
        </li>{/each}
    </ul>
  </details>{/if}
<section class="release-section">
  <h3>{t('upcomingEpisodes')}</h3>
  {#if !upcoming.length}<p class="metadata-note">{t('noUpcoming')}</p>{/if}
  <ul class="release-list">
    {#each upcoming as episode}<li>
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
        <div class="release-date">
          <time datetime={episode.scheduled_at}
            >{date(
              episode.scheduled_at,
              episode.precision,
              episode.air_date,
            )}</time
          >{#if episode.url}<a
              href={safeURL(episode.url)}
              target="_blank"
              rel="noreferrer">TVmaze ↗</a
            >{/if}
        </div>
      </li>{/each}
  </ul>
</section>
<section class="release-section">
  <h3>{t('reportedAired')}</h3>
  {#if !recent.length}<p class="metadata-note">{t('noAnnouncements')}</p>{/if}
  <ul class="release-list">
    {#each recent as event}<li>
        <div>
          <span class="release-kind"
            >{event.kind === 'season' ? t('newSeason') : t('newEpisode')}</span
          ><a class="release-title" href={'/titles/' + event.title_id}
            >{event.title}</a
          >
          <p>{event.name}</p>
          <small
            >{event.number !== null
              ? t('season') +
                ' ' +
                event.season +
                ' · ' +
                t('episode') +
                ' ' +
                event.number
              : t('special')}</small
          >
        </div>
        <div class="release-date">
          <time datetime={event.reported_at}
            >{date(event.reported_at, event.precision)}</time
          >{#if event.url}<a
              href={safeURL(event.url)}
              target="_blank"
              rel="noreferrer">TVmaze ↗</a
            >{/if}
        </div>
      </li>{/each}
  </ul>
</section>
{#if Math.max(upcomingTotal, recentTotal) > limit}<nav
    class="pagination"
    aria-label={t('page')}
  >
    <button
      disabled={!offset}
      onclick={() => {
        offset = Math.max(0, offset - limit);
        void load();
      }}>{t('previous')}</button
    ><span>{Math.floor(offset / limit) + 1}</span><button
      disabled={offset + limit >= Math.max(upcomingTotal, recentTotal)}
      onclick={() => {
        offset += limit;
        void load();
      }}>{t('nextPage')}</button
    >
  </nav>{/if}
<p class="metadata-note">{t('baselineNote')}</p>
<p class="metadata-note">
  <a href="https://www.tvmaze.com" target="_blank" rel="noreferrer">TVmaze</a> ·
  <a
    href="https://creativecommons.org/licenses/by-sa/4.0/"
    target="_blank"
    rel="noreferrer">CC BY-SA</a
  >
</p>
