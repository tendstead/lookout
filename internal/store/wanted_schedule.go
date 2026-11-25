// SPDX-License-Identifier: Elastic-2.0
package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/tendstead/lookout/internal/metadata"
	"github.com/tendstead/lookout/internal/releases"
)

func (s *Store) DueWantedSchedules(ctx context.Context, now time.Time, force bool) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT w.provider_ids->>'tvmaze' AS provider
 FROM wanted_entries w LEFT JOIN wanted_schedules s ON s.show_id=w.provider_ids->>'tvmaze'
 WHERE w.category='series' AND w.server_id IS NULL AND NOT w.paused AND COALESCE(w.provider_ids->>'tvmaze','')<>'' AND
 ($2 OR s.next_refresh_at IS NULL OR s.next_refresh_at<=$1)
 ORDER BY provider`, now, force)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func (s *Store) WantedScheduleFailure(ctx context.Context, provider string, now time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO wanted_schedules(show_id,next_refresh_at,last_error)
 VALUES($1,$2::timestamptz+interval '30 minutes',true)
 ON CONFLICT(show_id) DO UPDATE SET next_refresh_at=excluded.next_refresh_at,last_error=true`, provider, now)
	return err
}

func (s *Store) SaveWantedSchedule(ctx context.Context, provider string, episodes []metadata.Episode, seasons []metadata.TrackingSeason, now time.Time) error {
	if provider == "" || len(episodes) > 10000 || len(seasons) > 1000 {
		return errors.New("invalid wanted schedule")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO wanted_schedules(show_id,checked_at,next_refresh_at) VALUES($1,$2,$2::timestamptz+interval '6 hours')
 ON CONFLICT(show_id) DO UPDATE SET checked_at=excluded.checked_at,next_refresh_at=excluded.next_refresh_at,last_error=false`, provider, now); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "DELETE FROM wanted_provider_episodes WHERE show_id=$1", provider); err != nil {
		return err
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
		batch.Queue(`INSERT INTO wanted_provider_episodes(show_id,episode_id,name,season,number,air_date,air_stamp,scheduled_at,precision,provider_url)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT DO NOTHING`, provider, episode.ID, episode.Name,
			episode.Season, episode.Number, episode.AirDate, episode.AirStamp, scheduled, precision, episode.URL)
	}
	if seasons != nil {
		batch.Queue("DELETE FROM wanted_provider_seasons WHERE show_id=$1", provider)
		for _, season := range seasons {
			batch.Queue(`INSERT INTO wanted_provider_seasons(show_id,number,episode_order,end_date) VALUES($1,$2,$3,$4)
 ON CONFLICT DO NOTHING`, provider, season.Number, season.EpisodeOrder, season.EndDate)
		}
	}
	if err = tx.SendBatch(ctx, batch).Close(); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) WantedEpisodes(ctx context.Context, v Wanted) ([]metadata.Episode, error) {
	rows, err := s.pool.Query(ctx, `SELECT episode_id,name,season,number,air_date,air_stamp,provider_url
 FROM wanted_provider_episodes
 WHERE show_id=$1 AND ($2::int IS NULL OR season=$2) AND ($3::int IS NULL OR number=$3)
 ORDER BY season,number NULLS LAST,episode_id`, v.ProviderIDs["tvmaze"], v.Season, v.Episode)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (metadata.Episode, error) {
		var episode metadata.Episode
		err := row.Scan(&episode.ID, &episode.Name, &episode.Season, &episode.Number, &episode.AirDate, &episode.AirStamp, &episode.URL)
		return episode, err
	})
}

func (s *Store) WantedSeasonEnds(ctx context.Context, v Wanted) (map[int]metadata.TrackingSeason, error) {
	rows, err := s.pool.Query(ctx, `SELECT number,episode_order,end_date FROM wanted_provider_seasons
 WHERE show_id=$1 AND ($2::int IS NULL OR number=$2)`, v.ProviderIDs["tvmaze"], v.Season)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seasons := map[int]metadata.TrackingSeason{}
	for rows.Next() {
		var season metadata.TrackingSeason
		if err = rows.Scan(&season.Number, &season.EpisodeOrder, &season.EndDate); err != nil {
			return nil, err
		}
		seasons[season.Number] = season
	}
	return seasons, rows.Err()
}

func (s *Store) QueueWantedAirAlerts(ctx context.Context, now time.Time, notify bool) (int, error) {
	status := "held"
	if notify {
		status = "pending"
	}
	tag, err := s.pool.Exec(ctx, `INSERT INTO wanted_air_outbox(wanted_id,show_id,episode_id,name,season,number,provider_url,kind,aired_at,status,next_attempt)
 SELECT w.id,p.show_id,p.episode_id,p.name,p.season,p.number,p.provider_url,CASE WHEN p.season>0 AND p.number=1 THEN 'season' ELSE 'episode' END,p.scheduled_at,$2,$1
 FROM wanted_entries w JOIN wanted_schedules s ON s.show_id=w.provider_ids->>'tvmaze'
 JOIN wanted_provider_episodes p ON p.show_id=s.show_id
 WHERE w.category='series' AND w.server_id IS NULL AND NOT w.paused AND w.air_alerts AND NOT s.last_error AND
 (w.season IS NULL OR w.season=p.season) AND (w.episode IS NULL OR w.episode=p.number)
 AND p.scheduled_at>w.release_baseline_at AND p.scheduled_at<=$1
 ON CONFLICT DO NOTHING`, now, status)
	return int(tag.RowsAffected()), err
}
