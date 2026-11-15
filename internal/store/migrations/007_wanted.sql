CREATE TABLE wanted_entries (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),name text NOT NULL,category text NOT NULL CHECK(category IN('movie','series')),
 year integer,provider_ids jsonb NOT NULL DEFAULT '{}',season integer CHECK(season>=0),episode integer CHECK(episode>0),
 created_at timestamptz NOT NULL DEFAULT now(),CHECK(episode IS NULL OR season IS NOT NULL),CHECK(category='series' OR season IS NULL)
);
CREATE TABLE wanted_arrivals (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,wanted_id uuid NOT NULL REFERENCES wanted_entries(id) ON DELETE CASCADE,
 server_id text NOT NULL REFERENCES library_servers(id),content_key text NOT NULL,description text NOT NULL,
 status text NOT NULL DEFAULT 'held',attempts integer NOT NULL DEFAULT 0,next_attempt timestamptz NOT NULL DEFAULT now(),
 claimed_until timestamptz,sent_at timestamptz,last_error text NOT NULL DEFAULT '',created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(wanted_id,server_id,content_key)
);
