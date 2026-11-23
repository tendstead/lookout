// SPDX-License-Identifier: Elastic-2.0
package store

import (
	"context"
	"errors"
	"strings"
	"time"
)

type Server struct {
	ID          string     `json:"id"`
	Type        string     `json:"type,omitempty"`
	Name        string     `json:"name"`
	ScanHours   int        `json:"scan_hours"`
	StaleHours  int        `json:"stale_hours"`
	Status      string     `json:"status"`
	LastSuccess *time.Time `json:"last_success"`
}

func (s *Store) EnsureServer(ctx context.Context, id, name string) error {
	if name == "" {
		name = id
	}
	_, e := s.pool.Exec(ctx, `INSERT INTO library_servers(id,name) VALUES($1,$2) ON CONFLICT(id) DO UPDATE SET name=excluded.name WHERE NOT library_servers.name_custom`, id, name)
	return e
}
func (s *Store) SaveServer(ctx context.Context, v Server) error {
	v.Name = strings.TrimSpace(v.Name)
	if v.ID == "" || len(v.ID) > 100 || v.Name == "" || len(v.Name) > 100 || v.ScanHours < 1 || v.ScanHours > 168 || v.StaleHours < v.ScanHours || v.StaleHours > 720 {
		return errors.New("invalid server settings")
	}
	tag, e := s.pool.Exec(ctx, `UPDATE library_servers SET name=$2,name_custom=true,scan_hours=$3,stale_hours=$4 WHERE id=$1`, v.ID, v.Name, v.ScanHours, v.StaleHours)
	if e == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return e
}
func (s *Store) ListServers(ctx context.Context) ([]Server, error) {
	return s.ListServersForSources(ctx, nil)
}

// A nil source list includes historical roots for legacy clients.
func (s *Store) ListServersForSources(ctx context.Context, sources []string) ([]Server, error) {
	rows, e := s.pool.Query(ctx, `SELECT v.id,v.name,v.scan_hours,v.stale_hours,
 CASE WHEN count(s.id)=0 THEN 'pending' WHEN bool_or(s.status IN('error','interrupted','review')) THEN 'error'
 WHEN bool_or(s.last_success IS NULL OR s.last_success<now()-make_interval(hours=>v.stale_hours)) THEN 'stale'
 WHEN bool_or(s.status='scanning') THEN 'scanning' ELSE 'ready' END,min(s.last_success)
 FROM library_servers v LEFT JOIN library_sources s ON s.server_id=v.id AND ($1::text[] IS NULL OR s.id=ANY($1)) GROUP BY v.id ORDER BY lower(v.name),v.id`, sources)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Server{}
	for rows.Next() {
		var v Server
		if e = rows.Scan(&v.ID, &v.Name, &v.ScanHours, &v.StaleHours, &v.Status, &v.LastSuccess); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) SourceDue(ctx context.Context, id string) (bool, error) {
	var due bool
	e := s.pool.QueryRow(ctx, `SELECT s.status<>'scanning' AND (s.last_success IS NULL OR s.last_success<now()-make_interval(hours=>v.scan_hours)) AND NOT EXISTS(SELECT 1 FROM library_scans j WHERE j.source_id=s.id AND j.started_at>now()-interval '1 hour') FROM library_sources s JOIN library_servers v ON v.id=s.server_id WHERE s.id=$1`, id).Scan(&due)
	return due, e
}
func (s *Store) SetSourceAdapter(ctx context.Context, id, adapter string) error {
	_, e := s.pool.Exec(ctx, `UPDATE library_sources SET adapter=$2 WHERE id=$1`, id, adapter)
	return e
}
func (s *Store) ConnectionDue(ctx context.Context, id string) bool {
	var due bool
	e := s.pool.QueryRow(ctx, `SELECT (c.last_import_success IS NULL OR c.last_import_success<now()-make_interval(hours=>v.scan_hours)) AND (c.last_import_attempt IS NULL OR c.last_import_attempt<now()-interval '1 hour') FROM metadata_connections c JOIN library_servers v ON v.id=c.server_id WHERE c.id::text=$1`, id).Scan(&due)
	return e == nil && due
}

func (s *Store) ConnectionRetryAllowed(ctx context.Context, id string) bool {
	var allowed bool
	e := s.pool.QueryRow(ctx, `SELECT last_import_attempt IS NULL OR last_import_attempt<now()-interval '1 hour' FROM metadata_connections WHERE id::text=$1`, id).Scan(&allowed)
	return e == nil && allowed
}
func (s *Store) ConnectionImportStarted(ctx context.Context, id string) error {
	_, e := s.pool.Exec(ctx, `UPDATE metadata_connections SET last_import_attempt=now() WHERE id::text=$1`, id)
	return e
}
func (s *Store) ConnectionImportFinished(ctx context.Context, id string) error {
	_, e := s.pool.Exec(ctx, `UPDATE metadata_connections SET last_import_success=now() WHERE id::text=$1`, id)
	return e
}
func (s *Store) FailConnectionInventory(ctx context.Context, id string) error {
	_, e := s.pool.Exec(ctx, `UPDATE library_sources SET status='error',last_error='API inventory unavailable; last known copies retained' WHERE adapter IN('jellyfin','plex') AND id IN($1,$2)`, "api-"+id+"-movie", "api-"+id+"-series")
	return e
}
