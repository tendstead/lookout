export type Kind = 'missing' | 'partial' | 'review';
export type Gap = {
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
