CREATE TABLE library_sources (
 id text PRIMARY KEY, label text NOT NULL, root_path text NOT NULL, category text NOT NULL,
 status text NOT NULL DEFAULT 'pending', last_scan uuid, last_success timestamptz,
 last_error text NOT NULL DEFAULT '', file_count bigint NOT NULL DEFAULT 0,
 stats jsonb NOT NULL DEFAULT '{}', UNIQUE(root_path)
);
CREATE TABLE library_scans (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), source_id text NOT NULL REFERENCES library_sources(id),
 status text NOT NULL DEFAULT 'running', started_at timestamptz NOT NULL DEFAULT now(), finished_at timestamptz,
 observed bigint NOT NULL DEFAULT 0, stats jsonb NOT NULL DEFAULT '{}', error text NOT NULL DEFAULT ''
);
CREATE TABLE library_titles (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), source_id text NOT NULL REFERENCES library_sources(id),
 group_key text NOT NULL, category text NOT NULL, name text NOT NULL, year integer,
 overview text NOT NULL DEFAULT '', provider_ids jsonb NOT NULL DEFAULT '{}', provenance text NOT NULL DEFAULT 'filename',
 match_status text NOT NULL DEFAULT 'unmatched', created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(source_id,group_key)
);
CREATE TABLE library_files (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 source_id text NOT NULL REFERENCES library_sources(id), title_id uuid NOT NULL REFERENCES library_titles(id),
 relative_path text NOT NULL, size bigint NOT NULL, modified_at timestamptz NOT NULL,
 season integer, episodes integer[] NOT NULL DEFAULT '{}', numbering text NOT NULL DEFAULT 'unknown', episode_title text NOT NULL DEFAULT '',
 first_seen timestamptz NOT NULL DEFAULT now(), last_seen timestamptz NOT NULL DEFAULT now(), last_scan uuid NOT NULL REFERENCES library_scans(id),
 observation text NOT NULL DEFAULT 'observed', absent_scans integer NOT NULL DEFAULT 0,
 UNIQUE(source_id,relative_path)
);
CREATE INDEX library_files_title ON library_files(title_id);
CREATE INDEX library_files_scan ON library_files(source_id,last_scan);
CREATE INDEX library_titles_name ON library_titles(lower(name),id);
CREATE INDEX library_titles_category ON library_titles(category);
