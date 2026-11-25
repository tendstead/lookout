ALTER TABLE wanted_entries ADD COLUMN release_baseline_at timestamptz NOT NULL DEFAULT now();

CREATE TABLE wanted_schedules (
 show_id text PRIMARY KEY,
 checked_at timestamptz,
 next_refresh_at timestamptz NOT NULL DEFAULT now(),
 last_error boolean NOT NULL DEFAULT false
);
CREATE TABLE wanted_provider_episodes (
 show_id text NOT NULL REFERENCES wanted_schedules(show_id) ON DELETE CASCADE,
 episode_id text NOT NULL,
 name text NOT NULL,
 season integer NOT NULL,
 number integer,
 air_date text NOT NULL,
 air_stamp text NOT NULL,
 scheduled_at timestamptz,
 precision text NOT NULL,
 provider_url text NOT NULL,
 PRIMARY KEY(show_id,episode_id)
);
CREATE INDEX wanted_provider_episodes_schedule ON wanted_provider_episodes(scheduled_at);
CREATE TABLE wanted_provider_seasons (
 show_id text NOT NULL REFERENCES wanted_schedules(show_id) ON DELETE CASCADE,
 number integer NOT NULL,
 episode_order integer NOT NULL,
 end_date text NOT NULL,
 PRIMARY KEY(show_id,number)
);
CREATE TABLE wanted_air_outbox (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 wanted_id uuid NOT NULL REFERENCES wanted_entries(id) ON DELETE CASCADE,
 show_id text NOT NULL,
 episode_id text NOT NULL,
 name text NOT NULL,
 season integer NOT NULL,
 number integer,
 provider_url text NOT NULL,
 kind text NOT NULL,
 aired_at timestamptz NOT NULL,
 status text NOT NULL,
 attempts integer NOT NULL DEFAULT 0,
 next_attempt timestamptz NOT NULL,
 claimed_until timestamptz,
 sent_at timestamptz,
 UNIQUE(wanted_id,show_id,episode_id)
);
CREATE INDEX wanted_air_outbox_claim ON wanted_air_outbox(status,next_attempt);
