// SPDX-License-Identifier: Elastic-2.0
package store

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/tendstead/lookout/internal/metadata"
	"github.com/tendstead/lookout/internal/releases"
	"time"
)

func (s *Store) SaveEpisodeSnapshot(ctx context.Context, follow Follow, episodes []metadata.Episode, now time.Time, notify bool) (int, error) {
	if len(episodes) > 10000 {
		return 0, errors.New("episode schedule exceeds limit")
	}
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback(ctx)
	var initialized, enabled bool
	var baseline time.Time
	var provider string
	if e = tx.QueryRow(ctx, "SELECT initialized,baseline_at,provider_id,enabled FROM release_follows WHERE title_id=$1 FOR UPDATE", follow.TitleID).Scan(&initialized, &baseline, &provider, &enabled); e != nil {
		return 0, e
	}
	if provider != follow.ProviderID || !enabled {
		return 0, nil
	}
	emitted := map[string]bool{}
	rows, e := tx.Query(ctx, "SELECT e.kind||':'||e.episode_id FROM release_events e WHERE e.show_id=$2 AND (e.title_id IN(SELECT id FROM title_identity WHERE canonical_id=(SELECT canonical_id FROM title_identity WHERE id=$1)) OR EXISTS(SELECT 1 FROM release_delivery_receipts sent WHERE sent.show_id=e.show_id AND sent.episode_id=e.episode_id AND sent.kind=e.kind))", follow.TitleID, provider)
	if e != nil {
		return 0, e
	}
	for rows.Next() {
		var key string
		if e = rows.Scan(&key); e != nil {
			rows.Close()
			return 0, e
		}
		emitted[key] = true
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return 0, e
	}
	state, events := releases.Evaluate(releases.State{Initialized: initialized, BaselineAt: baseline, Emitted: emitted}, episodes, now)
	if _, e = tx.Exec(ctx, "UPDATE provider_episodes SET active=false WHERE title_id=$1 AND show_id=$2", follow.TitleID, provider); e != nil {
		return 0, e
	}
	batch := &pgx.Batch{}
	for _, episode := range episodes {
		if episode.ID == "" {
			continue
		}
		stamp, precision, known := releases.ReportedTime(episode)
		var scheduled *time.Time
		if known {
			scheduled = &stamp
		}
		batch.Queue(`INSERT INTO provider_episodes(title_id,show_id,episode_id,name,season,number,air_date,air_stamp,scheduled_at,precision,provider_url) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT(title_id,show_id,episode_id) DO UPDATE SET name=excluded.name,season=excluded.season,number=excluded.number,air_date=excluded.air_date,air_stamp=excluded.air_stamp,scheduled_at=excluded.scheduled_at,precision=excluded.precision,provider_url=excluded.provider_url,active=true`, follow.TitleID, provider, episode.ID, episode.Name, episode.Season, episode.Number, episode.AirDate, episode.AirStamp, scheduled, precision, episode.URL)
	}
	results := tx.SendBatch(ctx, batch)
	if e = results.Close(); e != nil {
		return 0, e
	}
	count := 0
	for _, event := range events {
		var eventID int64
		e = tx.QueryRow(ctx, `INSERT INTO release_events(title_id,title_name,category,show_id,episode_id,kind,episode_name,season,number,reported_at,precision,provider_url) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) ON CONFLICT DO NOTHING RETURNING id`, follow.TitleID, follow.Name, follow.Category, provider, event.EpisodeID, event.Kind, event.Name, event.Season, event.Number, event.ReportedAt, event.Precision, event.URL).Scan(&eventID)
		if errors.Is(e, pgx.ErrNoRows) {
			continue
		}
		if e != nil {
			return 0, e
		}
		status := "held"
		if notify {
			status = "pending"
		}
		if event.Kind == "episode" && event.Season > 0 && event.Number != nil && *event.Number == 1 {
			status = "coalesced"
		}
		if _, e = tx.Exec(ctx, "INSERT INTO notification_outbox(event_id,status,next_attempt) VALUES($1,$2,$3)", eventID, status, now); e != nil {
			return 0, e
		}
		count++
	}
	_, e = tx.Exec(ctx, "UPDATE release_follows SET initialized=true,baseline_at=$2,checked_at=$3,next_refresh_at=$3::timestamptz+interval '6 hours',last_error='' WHERE title_id=$1", follow.TitleID, state.BaselineAt, now)
	if e != nil {
		return 0, e
	}
	return count, tx.Commit(ctx)
}

type UpcomingEpisode struct {
	TitleID     string    `json:"title_id"`
	Title       string    `json:"title"`
	Category    string    `json:"category"`
	EpisodeID   string    `json:"episode_id"`
	Name        string    `json:"name"`
	Season      int       `json:"season"`
	Number      *int      `json:"number"`
	AirDate     string    `json:"air_date"`
	ScheduledAt time.Time `json:"scheduled_at"`
	Precision   string    `json:"precision"`
	URL         string    `json:"url"`
}
type ReleaseEvent struct {
	ID         int64     `json:"id"`
	TitleID    string    `json:"title_id"`
	Title      string    `json:"title"`
	Category   string    `json:"category"`
	EpisodeID  string    `json:"episode_id"`
	Kind       string    `json:"kind"`
	Name       string    `json:"name"`
	Season     int       `json:"season"`
	Number     *int      `json:"number"`
	ReportedAt time.Time `json:"reported_at"`
	Precision  string    `json:"precision"`
	URL        string    `json:"url"`
	Delivery   string    `json:"delivery"`
}
type ReleaseOverview struct {
	UnmatchedTitles []Follow          `json:"unmatched_titles"`
	Upcoming        []UpcomingEpisode `json:"upcoming"`
	Recent          []ReleaseEvent    `json:"recent"`
	Following       int               `json:"following"`
	Ready           int               `json:"ready"`
	Unmatched       int               `json:"unmatched"`
	UpcomingTotal   int               `json:"upcoming_total"`
	RecentTotal     int               `json:"recent_total"`
	Limit           int               `json:"limit"`
	Offset          int               `json:"offset"`
}

func (s *Store) ReleaseOverview(ctx context.Context, now time.Time, limit, offset int) (ReleaseOverview, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	v := ReleaseOverview{Upcoming: []UpcomingEpisode{}, Recent: []ReleaseEvent{}, Limit: limit, Offset: offset}
	e := s.pool.QueryRow(ctx, "SELECT count(*),count(*) FILTER(WHERE initialized),count(*) FILTER(WHERE provider_id='') FROM release_follows f JOIN library_titles t ON t.id=f.title_id WHERE f.enabled AND t.canonical_id IS NULL").Scan(&v.Following, &v.Ready, &v.Unmatched)
	if e != nil {
		return v, e
	}
	unmatchedRows, err := s.pool.Query(ctx, "SELECT "+followColumns+" FROM release_follows f JOIN library_titles t ON t.id=f.title_id WHERE t.canonical_id IS NULL AND f.enabled AND f.provider_id='' ORDER BY t.name LIMIT 50")
	if err != nil {
		return v, err
	}
	v.UnmatchedTitles = []Follow{}
	for unmatchedRows.Next() {
		f, err := readFollow(unmatchedRows)
		if err != nil {
			unmatchedRows.Close()
			return v, err
		}
		v.UnmatchedTitles = append(v.UnmatchedTitles, f)
	}
	err = unmatchedRows.Err()
	unmatchedRows.Close()
	if err != nil {
		return v, err
	}
	filter := ` FROM provider_episodes p JOIN release_follows f ON f.title_id=p.title_id AND f.provider_id=p.show_id JOIN library_titles t ON t.id=p.title_id WHERE t.canonical_id IS NULL AND f.enabled AND p.active AND p.scheduled_at>$1`
	if e = s.pool.QueryRow(ctx, "SELECT count(*)"+filter, now).Scan(&v.UpcomingTotal); e != nil {
		return v, e
	}
	rows, e := s.pool.Query(ctx, `SELECT t.id::text,t.name,t.category,p.episode_id,p.name,p.season,p.number,p.air_date,p.scheduled_at,p.precision,p.provider_url`+filter+` ORDER BY p.scheduled_at,t.id,p.episode_id LIMIT $2 OFFSET $3`, now, limit, offset)
	if e != nil {
		return v, e
	}
	for rows.Next() {
		var p UpcomingEpisode
		if e = rows.Scan(&p.TitleID, &p.Title, &p.Category, &p.EpisodeID, &p.Name, &p.Season, &p.Number, &p.AirDate, &p.ScheduledAt, &p.Precision, &p.URL); e != nil {
			rows.Close()
			return v, e
		}
		v.Upcoming = append(v.Upcoming, p)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return v, e
	}
	const recentProjection = `WITH ranked AS (
 SELECT e.*,i.canonical_id,COALESCE(o.status,'held') AS delivery,
 row_number() OVER(PARTITION BY i.canonical_id,e.show_id,e.episode_id,e.kind ORDER BY
 CASE COALESCE(o.status,'held') WHEN 'delivered' THEN 0 WHEN 'sending' THEN 1 WHEN 'pending' THEN 2 WHEN 'retry' THEN 3 WHEN 'held' THEN 4 WHEN 'coalesced' THEN 5 ELSE 6 END,e.created_at DESC,e.id DESC) AS priority
 FROM release_events e JOIN title_identity i ON i.id=e.title_id LEFT JOIN notification_outbox o ON o.event_id=e.id
 ) `
	if e = s.pool.QueryRow(ctx, recentProjection+`SELECT count(*) FROM ranked WHERE priority=1`).Scan(&v.RecentTotal); e != nil {
		return v, e
	}
	rows, e = s.pool.Query(ctx, recentProjection+`SELECT e.id,e.canonical_id::text,t.name,t.category,e.episode_id,e.kind,e.episode_name,e.season,e.number,e.reported_at,e.precision,e.provider_url,
 CASE WHEN EXISTS(SELECT 1 FROM release_delivery_receipts sent WHERE sent.show_id=e.show_id AND sent.episode_id=e.episode_id AND sent.kind=e.kind) THEN 'delivered' ELSE e.delivery END FROM ranked e JOIN library_titles t ON t.id=e.canonical_id WHERE e.priority=1 ORDER BY e.created_at DESC,e.id DESC LIMIT $1 OFFSET $2`, limit, offset)
	if e != nil {
		return v, e
	}
	defer rows.Close()
	for rows.Next() {
		var event ReleaseEvent
		if e = rows.Scan(&event.ID, &event.TitleID, &event.Title, &event.Category, &event.EpisodeID, &event.Kind, &event.Name, &event.Season, &event.Number, &event.ReportedAt, &event.Precision, &event.URL, &event.Delivery); e != nil {
			return v, e
		}
		v.Recent = append(v.Recent, event)
	}
	return v, rows.Err()
}
