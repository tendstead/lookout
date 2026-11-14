CREATE TABLE metadata_connections (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), kind text NOT NULL, base_url text NOT NULL,
 token_cipher bytea NOT NULL, path_mappings jsonb NOT NULL DEFAULT '[]',
 verified boolean NOT NULL DEFAULT false, last_error text NOT NULL DEFAULT '', checked_at timestamptz,
 UNIQUE(kind,base_url)
);
ALTER TABLE library_titles ADD COLUMN metadata_url text NOT NULL DEFAULT '';
ALTER TABLE library_titles ADD COLUMN artwork_connection uuid REFERENCES metadata_connections(id);
ALTER TABLE library_titles ADD COLUMN artwork_path text NOT NULL DEFAULT '';
ALTER TABLE library_titles ADD COLUMN artwork_url text NOT NULL DEFAULT '';
CREATE TABLE metadata_images (
 title_id uuid PRIMARY KEY REFERENCES library_titles(id) ON DELETE CASCADE,
 mime text NOT NULL, content bytea NOT NULL CHECK(octet_length(content)<=2097152), cached_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE metadata_snapshots (
 title_id uuid NOT NULL REFERENCES library_titles(id) ON DELETE CASCADE,
 provider text NOT NULL, provider_id text NOT NULL, metadata jsonb NOT NULL, refreshed_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(title_id,provider)
);
