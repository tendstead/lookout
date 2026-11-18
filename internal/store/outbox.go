// SPDX-License-Identifier: Elastic-2.0
package store

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

type Delivery struct {
	ProviderID string
	EpisodeID  string
	WantedIDs  []int64
	Wanted     bool
	ServerName string
	ID         int64
	Title      string
	Name       string
	Kind       string
	Season     int
	Number     *int
	URL        string
	Attempts   int
}

func (s *Store) ClaimNotification(ctx context.Context, now time.Time) (*Delivery, error) {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	var d Delivery
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(726431212)`); e != nil {
		return nil, e
	}
	e = tx.QueryRow(ctx, `SELECT o.id,e.title_name,e.episode_name,e.kind,e.season,e.number,e.provider_url,o.attempts,e.show_id,e.episode_id FROM notification_outbox o JOIN release_events e ON e.id=o.event_id JOIN title_identity i ON i.id=e.title_id JOIN release_follows f ON f.title_id=i.canonical_id AND f.provider_id=e.show_id WHERE f.enabled AND NOT EXISTS(SELECT 1 FROM release_delivery_receipts sent WHERE sent.show_id=e.show_id AND sent.episode_id=e.episode_id AND sent.kind=e.kind) AND (o.status IN('pending','retry') OR o.status='sending' AND o.claimed_until<$1) AND o.next_attempt<=$1 ORDER BY o.id FOR UPDATE OF o SKIP LOCKED LIMIT 1`, now).Scan(&d.ID, &d.Title, &d.Name, &d.Kind, &d.Season, &d.Number, &d.URL, &d.Attempts, &d.ProviderID, &d.EpisodeID)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	_, e = tx.Exec(ctx, "UPDATE notification_outbox SET status='sending',claimed_until=$2::timestamptz+interval '2 minutes',attempts=attempts+1 WHERE id=$1", d.ID, now)
	if e != nil {
		return nil, e
	}
	if _, e = tx.Exec(ctx, `UPDATE notification_outbox o SET status='coalesced' FROM release_events e WHERE o.event_id=e.id AND o.id<>$1 AND e.show_id=$2 AND e.episode_id=$3 AND e.kind=$4 AND o.status IN('pending','retry')`, d.ID, d.ProviderID, d.EpisodeID, d.Kind); e != nil {
		return nil, e
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, e
	}
	return &d, nil
}
func (s *Store) MarkNotification(ctx context.Context, id int64, sent bool, retryAfter time.Duration, now time.Time) error {
	if sent {
		tx, e := s.pool.Begin(ctx)
		if e != nil {
			return e
		}
		defer tx.Rollback(ctx)
		if _, e = tx.Exec(ctx, `INSERT INTO release_delivery_receipts(show_id,episode_id,kind,sent_at) SELECT e.show_id,e.episode_id,e.kind,$2 FROM release_events e JOIN notification_outbox o ON o.event_id=e.id WHERE o.id=$1 ON CONFLICT DO NOTHING`, id, now); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `UPDATE notification_outbox SET status='delivered',sent_at=$2,claimed_until=NULL,last_error='' WHERE id=$1`, id, now); e != nil {
			return e
		}
		return tx.Commit(ctx)
	}
	if retryAfter < time.Minute {
		retryAfter = time.Minute
	}
	_, e := s.pool.Exec(ctx, `UPDATE notification_outbox SET status=CASE WHEN attempts>=8 THEN 'failed' ELSE 'retry' END,next_attempt=$2,claimed_until=NULL,last_error='Telegram delivery failed; credentials are not included in logs' WHERE id=$1`, id, now.Add(retryAfter))
	return e
}
