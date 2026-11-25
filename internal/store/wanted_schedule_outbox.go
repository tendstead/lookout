// SPDX-License-Identifier: Elastic-2.0
package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// The caller holds the release notification advisory lock for both outboxes.
func claimTrackedAir(ctx context.Context, tx pgx.Tx, now time.Time) (*Delivery, error) {
	delivery := &Delivery{TrackedAir: true}
	err := tx.QueryRow(ctx, `SELECT a.id,w.name,a.name,a.kind,a.season,a.number,a.provider_url,a.attempts,a.show_id,a.episode_id
 FROM wanted_air_outbox a JOIN wanted_entries w ON w.id=a.wanted_id
 WHERE w.air_alerts AND w.server_id IS NULL AND NOT w.paused AND w.provider_ids->>'tvmaze'=a.show_id AND a.aired_at>w.release_baseline_at
 AND (w.season IS NULL OR w.season=a.season) AND (w.episode IS NULL OR w.episode=a.number)
 AND (a.status IN('pending','retry') OR a.status='sending' AND a.claimed_until<$1) AND a.next_attempt<=$1
 AND NOT EXISTS(SELECT 1 FROM release_delivery_receipts r WHERE (r.show_id,r.episode_id,r.kind)=(a.show_id,a.episode_id,a.kind))
 AND NOT EXISTS(SELECT 1 FROM notification_outbox o JOIN release_events e ON e.id=o.event_id
     WHERE (e.show_id,e.episode_id,e.kind)=(a.show_id,a.episode_id,a.kind) AND o.status='sending' AND o.claimed_until>=$1)
 AND NOT EXISTS(SELECT 1 FROM wanted_air_outbox other WHERE (other.show_id,other.episode_id,other.kind)=(a.show_id,a.episode_id,a.kind)
     AND other.status='sending' AND other.claimed_until>=$1)
 ORDER BY a.id FOR UPDATE OF a,w SKIP LOCKED LIMIT 1`, now).
		Scan(&delivery.ID, &delivery.Title, &delivery.Name, &delivery.Kind, &delivery.Season, &delivery.Number,
			&delivery.URL, &delivery.Attempts, &delivery.ProviderID, &delivery.EpisodeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE wanted_air_outbox SET status='sending',
 claimed_until=$2::timestamptz+interval '2 minutes',attempts=attempts+1 WHERE id=$1`, delivery.ID, now); err != nil {
		return nil, err
	}
	return delivery, nil
}

func (s *Store) MarkTrackedAirNotification(ctx context.Context, id int64, sent bool, retry time.Duration, now time.Time) error {
	if !sent {
		_, err := s.pool.Exec(ctx, `UPDATE wanted_air_outbox
 SET status=CASE WHEN attempts>=8 THEN 'failed' ELSE 'retry' END,next_attempt=$2,claimed_until=NULL
 WHERE id=$1 AND status='sending'`, id, now.Add(max(time.Minute, retry)))
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO release_delivery_receipts(show_id,episode_id,kind,sent_at)
 SELECT show_id,episode_id,kind,$2 FROM wanted_air_outbox WHERE id=$1 ON CONFLICT DO NOTHING`, id, now); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE wanted_air_outbox SET status='delivered',sent_at=$2,claimed_until=NULL WHERE id=$1`, id, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
