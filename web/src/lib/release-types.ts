export type UpcomingEpisode = {
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
