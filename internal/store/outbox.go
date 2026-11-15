// SPDX-License-Identifier: Elastic-2.0
package store

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

type Delivery struct {
	ID       int64
	Title    string
	Name     string
	Kind     string
	Season   int
	Number   *int
	URL      string
	Attempts int
}

func (s *Store) ClaimNotification(ctx context.Context, now time.Time) (*Delivery, error) {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	var d Delivery
	e = tx.QueryRow(ctx, `SELECT o.id,e.title_name,e.episode_name,e.kind,e.season,e.number,e.provider_url,o.attempts FROM notification_outbox o JOIN release_events e ON e.id=o.event_id JOIN release_follows f ON f.title_id=e.title_id AND f.provider_id=e.show_id WHERE f.enabled AND (o.status IN('pending','retry') OR o.status='sending' AND o.claimed_until<$1) AND o.next_attempt<=$1 ORDER BY o.id FOR UPDATE OF o SKIP LOCKED LIMIT 1`, now).Scan(&d.ID, &d.Title, &d.Name, &d.Kind, &d.Season, &d.Number, &d.URL, &d.Attempts)
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
	if e = tx.Commit(ctx); e != nil {
		return nil, e
	}
	return &d, nil
}
func (s *Store) MarkNotification(ctx context.Context, id int64, sent bool, retryAfter time.Duration, now time.Time) error {
	if sent {
		_, e := s.pool.Exec(ctx, "UPDATE notification_outbox SET status='delivered',sent_at=$2,claimed_until=NULL,last_error='' WHERE id=$1", id, now)
		return e
	}
	if retryAfter < time.Minute {
		retryAfter = time.Minute
	}
	_, e := s.pool.Exec(ctx, `UPDATE notification_outbox SET status=CASE WHEN attempts>=8 THEN 'failed' ELSE 'retry' END,next_attempt=$2,claimed_until=NULL,last_error='Telegram delivery failed; credentials are not included in logs' WHERE id=$1`, id, now.Add(retryAfter))
	return e
}
