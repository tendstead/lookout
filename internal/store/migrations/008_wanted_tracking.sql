DELETE FROM wanted_entries;

ALTER TABLE wanted_entries
    ADD COLUMN source_url text NOT NULL DEFAULT '',
    ADD COLUMN alert_mode text NOT NULL DEFAULT 'episode' CHECK (alert_mode IN ('episode', 'season', 'none')),
    ADD COLUMN air_alerts boolean NOT NULL DEFAULT false,
    ADD COLUMN local_title_id text NOT NULL DEFAULT '',
    ADD COLUMN expected_episodes integer CHECK (expected_episodes BETWEEN 1 AND 100000);
