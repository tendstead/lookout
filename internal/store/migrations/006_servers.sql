CREATE TABLE library_servers (
 id text PRIMARY KEY, name text NOT NULL, name_custom boolean NOT NULL DEFAULT false, scan_hours integer NOT NULL DEFAULT 6 CHECK(scan_hours BETWEEN 1 AND 168),
 stale_hours integer NOT NULL DEFAULT 24 CHECK(stale_hours BETWEEN 1 AND 720 AND stale_hours>=scan_hours)
);
INSERT INTO library_servers(id,name) VALUES('legacy','NAS');
ALTER TABLE library_sources ADD COLUMN server_id text NOT NULL DEFAULT 'legacy' REFERENCES library_servers(id);
ALTER TABLE library_sources ADD COLUMN adapter text NOT NULL DEFAULT 'sftp' CHECK(adapter IN('sftp','jellyfin','plex'));
ALTER TABLE library_sources DROP CONSTRAINT library_sources_root_path_key;
CREATE UNIQUE INDEX library_sources_server_path ON library_sources(server_id,root_path);
ALTER TABLE metadata_connections ADD COLUMN server_id text REFERENCES library_servers(id);
ALTER TABLE library_titles ADD COLUMN canonical_id uuid REFERENCES library_titles(id) ON DELETE SET NULL;
ALTER TABLE library_titles ADD COLUMN identity_manual boolean NOT NULL DEFAULT false;
CREATE INDEX library_titles_canonical ON library_titles(canonical_id);
CREATE VIEW title_identity AS SELECT id,COALESCE(canonical_id,id) AS canonical_id FROM library_titles;

UPDATE metadata_connections SET server_id='legacy' WHERE jsonb_array_length(path_mappings)>0;
ALTER TABLE metadata_connections ADD COLUMN last_import_attempt timestamptz;
ALTER TABLE metadata_connections ADD COLUMN last_import_success timestamptz;
ALTER TABLE library_files ADD COLUMN reported_path text NOT NULL DEFAULT '';
