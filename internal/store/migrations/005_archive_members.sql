ALTER TABLE library_files ADD COLUMN storage text NOT NULL DEFAULT 'file';
ALTER TABLE library_files ADD COLUMN archive_member text NOT NULL DEFAULT '';
ALTER TABLE library_files DROP CONSTRAINT library_files_source_id_relative_path_key;
ALTER TABLE library_files ADD UNIQUE(source_id,relative_path,archive_member);
