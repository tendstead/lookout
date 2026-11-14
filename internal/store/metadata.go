// SPDX-License-Identifier: Elastic-2.0
package store

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/tendstead/lookout/internal/metadata"
	"strings"
)

var ErrMetadataConflict = errors.New("metadata identities conflict; review required")

type MetadataUpdate struct {
	metadata.Candidate
	ConnectionID string
	ArtworkPath  string
	Manual       bool
}

func (s *Store) MetadataTargets(ctx context.Context, source, relative, kind string) ([]string, error) {
	query := "SELECT id::text FROM library_titles WHERE source_id=$1 AND group_key=$2"
	if kind == "movie" {
		query = "SELECT DISTINCT title_id::text FROM library_files WHERE source_id=$1 AND relative_path=$2"
	}
	rows, e := s.pool.Query(ctx, query, source, relative)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			return nil, e
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
func (s *Store) ApplyMetadata(ctx context.Context, id string, u MetadataUpdate) error {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var raw []byte
	var provenance string
	if e = tx.QueryRow(ctx, "SELECT provider_ids,provenance FROM library_titles WHERE id::text=$1 FOR UPDATE", id).Scan(&raw, &provenance); e != nil {
		return e
	}
	old := map[string]string{}
	if e = json.Unmarshal(raw, &old); e != nil {
		return e
	}
	if strings.HasPrefix(provenance, "manual:") && !u.Manual {
		return nil
	}
	if !u.Manual {
		for key, value := range u.ProviderIDs {
			if previous := old[key]; previous != "" && previous != value {
				if _, e = tx.Exec(ctx, "UPDATE library_titles SET match_status='review' WHERE id::text=$1", id); e != nil {
					return e
				}
				if e = tx.Commit(ctx); e != nil {
					return e
				}
				return ErrMetadataConflict
			}
		}
	}
	ids := old
	if u.Manual {
		ids = map[string]string{}
	}
	for k, v := range u.ProviderIDs {
		ids[k] = v
	}
	raw, e = json.Marshal(ids)
	if e != nil {
		return e
	}
	displaySource := u.Provider
	if u.Manual {
		displaySource = "manual:" + displaySource
	}
	preserve := !u.Manual && u.Provider == "tvmaze" && (provenance == "jellyfin" || provenance == "plex" || provenance == "nfo")
	_, e = tx.Exec(ctx, `UPDATE library_titles SET name=CASE WHEN $9 OR $2='' THEN name ELSE $2 END,year=COALESCE(NULLIF($3,0),year),
 overview=CASE WHEN $9 AND overview<>'' OR $4='' THEN overview ELSE $4 END,provider_ids=$5,
 provenance=CASE WHEN $9 THEN provenance ELSE $6 END,match_status='matched',metadata_url=CASE WHEN $7='' THEN metadata_url ELSE $7 END,
 artwork_connection=CASE WHEN $9 OR $8='' THEN artwork_connection ELSE NULLIF($8,'')::uuid END,
 artwork_path=CASE WHEN $9 OR $10='' THEN artwork_path ELSE $10 END,
 artwork_url=CASE WHEN $9 OR $11='' THEN artwork_url ELSE $11 END,updated_at=now() WHERE id::text=$1`, id, u.Name, u.Year, u.Overview, raw, displaySource, u.URL, u.ConnectionID, preserve, u.ArtworkPath, u.ImageURL)
	if e != nil {
		return e
	}
	snapshot, e := json.Marshal(u.Candidate)
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, `INSERT INTO metadata_snapshots(title_id,provider,provider_id,metadata) VALUES($1,$2,$3,$4) ON CONFLICT(title_id,provider) DO UPDATE SET provider_id=excluded.provider_id,metadata=excluded.metadata,refreshed_at=now()`, id, u.Provider, u.ID, snapshot)
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}

type ArtworkReference struct {
	ConnectionID string
	Path         string
	URL          string
}

func (s *Store) Artwork(ctx context.Context, id string) (ArtworkReference, error) {
	var a ArtworkReference
	e := s.pool.QueryRow(ctx, "SELECT COALESCE(artwork_connection::text,''),artwork_path,artwork_url FROM library_titles WHERE id::text=$1", id).Scan(&a.ConnectionID, &a.Path, &a.URL)
	return a, e
}
func (s *Store) CachedArtwork(ctx context.Context, id string) ([]byte, string, error) {
	var b []byte
	var mime string
	e := s.pool.QueryRow(ctx, "SELECT content,mime FROM metadata_images WHERE title_id::text=$1", id).Scan(&b, &mime)
	return b, mime, e
}
func (s *Store) CacheArtwork(ctx context.Context, id, mime string, b []byte) error {
	if len(b) > 2*1024*1024 {
		return errors.New("artwork too large")
	}
	_, e := s.pool.Exec(ctx, `INSERT INTO metadata_images(title_id,mime,content) VALUES($1,$2,$3) ON CONFLICT(title_id) DO UPDATE SET mime=excluded.mime,content=excluded.content,cached_at=now()`, id, mime, b)
	return e
}
