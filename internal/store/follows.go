// SPDX-License-Identifier: Elastic-2.0
package store

import (
	"context"
	"github.com/jackc/pgx/v5"
	"time"
)

type Follow struct {
	TitleID     string     `json:"title_id"`
	Name        string     `json:"name"`
	Category    string     `json:"category"`
	Enabled     bool       `json:"enabled"`
	ProviderID  string     `json:"provider_id"`
	Initialized bool       `json:"initialized"`
	BaselineAt  time.Time  `json:"baseline_at"`
	NextRefresh time.Time  `json:"next_refresh"`
	CheckedAt   *time.Time `json:"checked_at"`
	LastError   string     `json:"last_error,omitempty"`
}

const followColumns = `t.id::text,t.name,t.category,COALESCE(f.enabled,false),COALESCE(f.provider_id,t.provider_ids->>'tvmaze',''),COALESCE(f.initialized,false),COALESCE(f.baseline_at,now()),COALESCE(f.next_refresh_at,now()),f.checked_at,COALESCE(f.last_error,'')`

func readFollow(row pgx.Row) (Follow, error) {
	var f Follow
	e := row.Scan(&f.TitleID, &f.Name, &f.Category, &f.Enabled, &f.ProviderID, &f.Initialized, &f.BaselineAt, &f.NextRefresh, &f.CheckedAt, &f.LastError)
	return f, e
}
func (s *Store) GetFollow(ctx context.Context, id string) (Follow, error) {
	canonical, err := s.CanonicalID(ctx, id)
	if err != nil {
		return Follow{}, err
	}
	id = canonical
	f, e := readFollow(s.pool.QueryRow(ctx, "SELECT "+followColumns+" FROM library_titles t LEFT JOIN release_follows f ON f.title_id=t.id WHERE t.id::text=$1", id))
	if e == pgx.ErrNoRows {
		return f, ErrNotFound
	}
	return f, e
}
func (s *Store) SetFollow(ctx context.Context, id string, enabled bool, now time.Time) error {
	canonical, err := s.CanonicalID(ctx, id)
	if err != nil {
		return err
	}
	id = canonical
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	result, e := tx.Exec(ctx, `INSERT INTO release_follows(title_id,enabled,provider_id,baseline_at,next_refresh_at)
 SELECT t.id,$2,COALESCE(t.provider_ids->>'tvmaze',''),$3,$3 FROM library_titles t JOIN title_identity i ON i.id=t.id WHERE i.canonical_id::text=$1 AND t.category IN('series','anime_series')
 ON CONFLICT(title_id) DO UPDATE SET enabled=excluded.enabled,provider_id=excluded.provider_id,
 initialized=CASE WHEN release_follows.provider_id<>excluded.provider_id OR NOT release_follows.enabled AND excluded.enabled THEN false ELSE release_follows.initialized END,
 baseline_at=CASE WHEN release_follows.provider_id<>excluded.provider_id OR NOT release_follows.enabled AND excluded.enabled THEN excluded.baseline_at ELSE release_follows.baseline_at END,next_refresh_at=excluded.next_refresh_at`, id, enabled, now)
	if e != nil {
		return e
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	if !enabled {
		_, e = tx.Exec(ctx, `UPDATE notification_outbox o SET status='cancelled' FROM release_events e WHERE o.event_id=e.id AND e.title_id IN(SELECT id FROM title_identity WHERE canonical_id::text=$1) AND o.status IN('pending','retry')`, id)
		if e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}
func (s *Store) SeedFollows(ctx context.Context) error {
	_, e := s.pool.Exec(ctx, `INSERT INTO release_follows(title_id,provider_id) SELECT id,COALESCE(provider_ids->>'tvmaze','') FROM library_titles WHERE canonical_id IS NULL AND category IN('series','anime_series') ON CONFLICT DO NOTHING`)
	return e
}
func (s *Store) SyncFollowProviders(ctx context.Context, now time.Time) error {
	_, e := s.pool.Exec(ctx, `UPDATE release_follows f SET provider_id=COALESCE(t.provider_ids->>'tvmaze',''),initialized=false,baseline_at=$1,next_refresh_at=$1,last_error='' FROM library_titles t WHERE t.id=f.title_id AND f.provider_id<>COALESCE(t.provider_ids->>'tvmaze','')`, now)
	return e
}
func (s *Store) ListFollows(ctx context.Context) ([]Follow, error) {
	rows, e := s.pool.Query(ctx, "SELECT "+followColumns+" FROM release_follows f JOIN library_titles t ON t.id=f.title_id WHERE t.canonical_id IS NULL ORDER BY t.id")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Follow{}
	for rows.Next() {
		f, e := readFollow(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
func (s *Store) ScheduleFailure(ctx context.Context, id string, now time.Time) error {
	_, e := s.pool.Exec(ctx, "UPDATE release_follows SET next_refresh_at=$2::timestamptz+interval '30 minutes',last_error='Schedule provider unavailable; cached data retained' WHERE title_id::text=$1", id, now)
	return e
}
func (s *Store) CachedDue(ctx context.Context, f Follow, now time.Time) (bool, error) {
	var due bool
	e := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM provider_episodes p WHERE p.title_id=$1 AND p.show_id=$2 AND p.active AND p.scheduled_at>$3 AND p.scheduled_at<=$4 AND NOT EXISTS(SELECT 1 FROM release_events e WHERE e.title_id=p.title_id AND e.show_id=p.show_id AND e.episode_id=p.episode_id AND e.kind='episode'))`, f.TitleID, f.ProviderID, f.BaselineAt, now).Scan(&due)
	return due, e
}
