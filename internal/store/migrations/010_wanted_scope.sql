ALTER TABLE wanted_entries
 ADD COLUMN server_id text REFERENCES library_servers(id),
 ADD COLUMN paused boolean NOT NULL DEFAULT false,
 ADD CONSTRAINT wanted_scoped_air_alert CHECK (server_id IS NULL OR NOT air_alerts);
