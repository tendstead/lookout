export type UpcomingEpisode = {
  title_id: string;
  tracked_id?: string;
  availability?: string;
  title: string;
  name: string;
  season: number;
  number: number | null;
  scheduled_at: string;
  air_date?: string;
  precision: string;
  url: string;
};

export function releaseTitleHref(
  episode: Pick<UpcomingEpisode, 'title_id' | 'tracked_id'>,
) {
  return episode.tracked_id
    ? '/wanted?tracked=' + encodeURIComponent(episode.tracked_id)
    : '/titles/' + encodeURIComponent(episode.title_id);
}

export function releaseAvailability(status: string) {
  if (status === 'available') return 'trackingAvailable';
  if (status === 'missing') return 'trackingMissing';
  return 'trackingUncertain';
}
