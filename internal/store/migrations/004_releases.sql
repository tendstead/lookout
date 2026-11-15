CREATE TABLE release_follows (
 title_id uuid PRIMARY KEY REFERENCES library_titles(id) ON DELETE CASCADE, enabled boolean NOT NULL DEFAULT true,
 provider_id text NOT NULL DEFAULT '', initialized boolean NOT NULL DEFAULT false, baseline_at timestamptz NOT NULL DEFAULT now(),
 next_refresh_at timestamptz NOT NULL DEFAULT now(), checked_at timestamptz, last_error text NOT NULL DEFAULT ''
);
CREATE TABLE provider_episodes (
 title_id uuid NOT NULL REFERENCES library_titles(id) ON DELETE CASCADE, show_id text NOT NULL, episode_id text NOT NULL,
 name text NOT NULL, season integer NOT NULL, number integer, air_date text NOT NULL, air_stamp text NOT NULL,
 scheduled_at timestamptz, precision text NOT NULL, provider_url text NOT NULL, active boolean NOT NULL DEFAULT true,
 PRIMARY KEY(title_id,show_id,episode_id)
);
CREATE INDEX provider_episodes_schedule ON provider_episodes(scheduled_at) WHERE active;
CREATE TABLE release_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, title_id uuid NOT NULL REFERENCES library_titles(id) ON DELETE CASCADE,
 title_name text NOT NULL, category text NOT NULL, show_id text NOT NULL, episode_id text NOT NULL, kind text NOT NULL,
 episode_name text NOT NULL, season integer NOT NULL, number integer, reported_at timestamptz NOT NULL,
 precision text NOT NULL, provider_url text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(title_id,show_id,episode_id,kind)
);
CREATE INDEX release_events_recent ON release_events(created_at DESC,id DESC);
CREATE TABLE notification_outbox (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, event_id bigint NOT NULL UNIQUE REFERENCES release_events(id) ON DELETE CASCADE,
 status text NOT NULL DEFAULT 'pending', attempts integer NOT NULL DEFAULT 0, next_attempt timestamptz NOT NULL DEFAULT now(),
 claimed_until timestamptz, sent_at timestamptz, last_error text NOT NULL DEFAULT ''
);
