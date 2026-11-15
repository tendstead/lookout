export type Kind = 'missing' | 'partial' | 'review';
export type SeasonDifference = {
  season: number;
  kind: Kind;
  aired: number;
  present: number;
  missing_episodes: number[];
};
export type Gap = {
  seasons?: SeasonDifference[];
  title_id: string;
  title: string;
  category: string;
  kind: Kind;
  season: number;
  latest_aired_season: number;
  local_seasons: number[];
  aired: number;
  present: number;
  missing_episodes: number[];
  reasons: string[];
  checked_at: string | null;
  inventory_at: string | null;
};
