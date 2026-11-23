// SPDX-License-Identifier: Elastic-2.0
package store

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/tendstead/lookout/internal/library"
	"time"
)

type Source struct {
	library.Root
	Status      string            `json:"status"`
	LastSuccess *time.Time        `json:"last_success"`
	LastError   string            `json:"last_error,omitempty"`
	FileCount   int64             `json:"file_count"`
	Stats       library.ScanStats `json:"stats"`
}

func (s *Store) EnsureSource(ctx context.Context, root library.Root) error {
	if root.ServerID == "" {
		root.ServerID = "legacy"
	}
	tag, e := s.pool.Exec(ctx, `INSERT INTO library_sources(id,label,root_path,category,server_id) VALUES($1,$2,$3,$4,$5) ON CONFLICT(id) DO UPDATE SET label=excluded.label WHERE library_sources.root_path=excluded.root_path AND library_sources.category=excluded.category AND library_sources.server_id=excluded.server_id`, root.ID, root.Label, root.Path, root.Category, root.ServerID)
	if e == nil && tag.RowsAffected() == 0 {
		return errors.New("source ID already belongs to another server, path or category")
	}
	return e
}
func (s *Store) StartScan(ctx context.Context, source string) (string, error) {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return "", e
	}
	defer tx.Rollback(ctx)
	var status string
	if e = tx.QueryRow(ctx, "SELECT status FROM library_sources WHERE id=$1 FOR UPDATE", source).Scan(&status); e != nil {
		return "", e
	}
	if status == "scanning" {
		return "", errors.New("source scan already running")
	}
	var id string
	if e = tx.QueryRow(ctx, "INSERT INTO library_scans(source_id) VALUES($1) RETURNING id::text", source).Scan(&id); e != nil {
		return "", e
	}
	_, e = tx.Exec(ctx, "UPDATE library_sources SET status='scanning',last_scan=$2,last_error='' WHERE id=$1", source, id)
	if e != nil {
		return "", e
	}
	return id, tx.Commit(ctx)
}
func (s *Store) Observe(ctx context.Context, scan string, f library.FileObservation) error {
	storage := f.Storage
	if storage == "" {
		storage = "file"
	}
	ids := f.Title.ProviderIDs
	if ids == nil {
		ids = map[string]string{}
	}
	raw, e := json.Marshal(ids)
	if e != nil {
		return e
	}
	episodes := f.Episodes
	if episodes == nil {
		episodes = []int{}
	}
	var season *int
	if len(episodes) > 0 {
		season = &f.Season
	}
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var title string
	e = tx.QueryRow(ctx, `INSERT INTO library_titles(source_id,group_key,category,name,year,overview,provider_ids,provenance)
 VALUES($1,$2,$3,$4,NULLIF($5,0),$6,$7,$8) ON CONFLICT(source_id,group_key) DO UPDATE SET updated_at=now(),
 provider_ids=excluded.provider_ids||library_titles.provider_ids,
 name=CASE WHEN library_titles.provenance='filename' THEN excluded.name ELSE library_titles.name END,
 year=COALESCE(library_titles.year,excluded.year),overview=CASE WHEN library_titles.overview='' THEN excluded.overview ELSE library_titles.overview END,
 provenance=CASE WHEN library_titles.provenance='filename' THEN excluded.provenance ELSE library_titles.provenance END RETURNING id::text`, f.SourceID, f.Group, f.Category, f.Title.Title, f.Title.Year, f.Title.Overview, raw, f.Title.Provenance).Scan(&title)
	if e != nil {
		return e
	}
	itemKeys := f.APIItemKeys
	if itemKeys == nil {
		itemKeys = []string{}
	}
	_, e = tx.Exec(ctx, `INSERT INTO library_files(source_id,title_id,relative_path,size,modified_at,season,episodes,numbering,episode_title,last_scan,storage,archive_member,technical,api_item_keys,reported_path,provider_revision)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16) ON CONFLICT(source_id,relative_path,archive_member) DO UPDATE SET title_id=excluded.title_id,size=excluded.size,modified_at=excluded.modified_at,
 season=excluded.season,episodes=excluded.episodes,numbering=excluded.numbering,episode_title=excluded.episode_title,last_seen=now(),last_scan=excluded.last_scan,
 technical=CASE WHEN cardinality(excluded.api_item_keys)>0 THEN excluded.technical
   WHEN library_files.size=excluded.size AND library_files.modified_at=excluded.modified_at THEN library_files.technical END,
 api_item_keys=excluded.api_item_keys,reported_path=excluded.reported_path,provider_revision=excluded.provider_revision,
 observation=CASE WHEN library_files.size=excluded.size AND library_files.modified_at=excluded.modified_at THEN 'stable' ELSE 'observed' END,absent_scans=0`, f.SourceID, title, f.RelativePath, f.Size, f.Modified, season, episodes, f.Numbering, f.EpisodeTitle, scan, storage, f.ArchiveMember, f.Technical, itemKeys, f.ReportedPath, f.ProviderRevision)
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, "UPDATE library_scans SET observed=observed+1 WHERE id=$1", scan)
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (s *Store) FinishScan(ctx context.Context, scan, source string, stats library.ScanStats, scanErr error) error {
	raw, e := json.Marshal(stats)
	if e != nil {
		return e
	}
	status := "ready"
	scanStatus := "complete"
	message := ""
	if scanErr != nil {
		status = "error"
		scanStatus = "failed"
		message = "Scan incomplete; previously observed files retained"
	}
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var previous int64
	if e = tx.QueryRow(ctx, "SELECT file_count FROM library_sources WHERE id=$1 FOR UPDATE", source).Scan(&previous); e != nil {
		return e
	}
	if scanErr == nil {
		// Item-level placeholders can be retired as soon as a full import
		// identifies the actual files. Several placeholders can resolve to
		// one physical file, so exclude them from the inventory-drop guard.
		retired, err := tx.Exec(ctx, `DELETE FROM library_files old USING library_files copy
 WHERE old.source_id=$1 AND old.last_scan<>$2 AND copy.source_id=old.source_id AND copy.last_scan=$2
 AND old.relative_path=ANY(copy.api_item_keys)`, source, scan)
		if err != nil {
			return err
		}
		previous -= retired.RowsAffected()
	}
	if scanErr == nil && previous > 0 && int64(stats.Media+stats.ArchivedMedia)*10 < previous*8 {
		status = "review"
		scanStatus = "review"
		message = "Large or empty inventory change requires review; known files retained"
	}
	if scanErr == nil && status == "ready" {
		_, e = tx.Exec(ctx, "UPDATE library_files SET absent_scans=absent_scans+1,observation=CASE WHEN absent_scans>=1 THEN 'missing' ELSE 'unconfirmed' END WHERE source_id=$1 AND last_scan<>$2", source, scan)
		if e != nil {
			return e
		}
	}
	if scanErr == nil && status == "ready" {
		_, e = tx.Exec(ctx, `DELETE FROM library_titles t WHERE t.source_id=$1 AND t.provenance='filename' AND NOT EXISTS(SELECT 1 FROM library_files f WHERE f.title_id=t.id)`, source)
		if e != nil {
			return e
		}
	}
	_, e = tx.Exec(ctx, "UPDATE library_scans SET status=$2,finished_at=now(),stats=$3,error=$4 WHERE id=$1", scan, scanStatus, raw, message)
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, `UPDATE library_sources SET status=$2,last_error=$3,stats=$4,file_count=(SELECT count(*) FROM library_files WHERE source_id=$1),last_success=CASE WHEN $5 THEN now() ELSE last_success END WHERE id=$1`, source, status, message, raw, scanErr == nil && status == "ready")
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (s *Store) RecoverScans(ctx context.Context) error {
	_, e := s.pool.Exec(ctx, "UPDATE library_scans SET status='interrupted',finished_at=now(),error='Application restarted; inventory retained' WHERE status='running'; UPDATE library_sources SET status='interrupted',last_error='Application restarted; scan can be retried' WHERE status='scanning'")
	return e
}
func (s *Store) ListSources(ctx context.Context) ([]Source, error) {
	rows, e := s.pool.Query(ctx, `SELECT s.id,s.label,s.root_path,s.category,s.server_id,s.status,s.last_success,s.last_error,s.file_count,s.stats FROM library_sources s ORDER BY s.label`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Source{}
	for rows.Next() {
		var v Source
		var raw []byte
		if e = rows.Scan(&v.ID, &v.Label, &v.Path, &v.Category, &v.ServerID, &v.Status, &v.LastSuccess, &v.LastError, &v.FileCount, &raw); e != nil {
			return nil, e
		}
		_ = json.Unmarshal(raw, &v.Stats)
		out = append(out, v)
	}
	return out, rows.Err()
}

// AttachTechnicalProfile associates provider data only with an exact observed
// plain file on the configured server. A later size or mtime change clears it.
func (s *Store) AttachTechnicalProfile(ctx context.Context, source, server, relative string, profile *library.TechnicalProfile) error {
	_, err := s.pool.Exec(ctx, `UPDATE library_files f SET technical=$4
 FROM library_sources src WHERE src.id=f.source_id AND src.id=$1 AND src.server_id=$2
 AND f.relative_path=$3 AND f.storage='file' AND f.archive_member=''
 AND f.observation IN ('observed','stable')
 AND ($5::bigint IS NULL OR f.size=$5) AND f.last_seen<=$6`, source, server, relative, profile, profile.SizeBytes, profile.ObservedAt)
	return err
}
