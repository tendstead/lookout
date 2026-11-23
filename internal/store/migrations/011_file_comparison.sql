ALTER TABLE library_files
    ADD COLUMN technical jsonb,
    ADD COLUMN provider_revision text NOT NULL DEFAULT '',
    ADD COLUMN api_item_keys text[] NOT NULL DEFAULT '{}';
