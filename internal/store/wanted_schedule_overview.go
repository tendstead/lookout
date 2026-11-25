// SPDX-License-Identifier: Elastic-2.0
package store

import (
	"context"
	"fmt"
	"time"
)

const trackedEpisodes = `WITH tracked_episodes AS (
 SELECT DISTINCT ON(p.show_id,p.episode_id) w.id::text AS tracked_id,w.local_title_id AS title_id,w.name AS title,w.category,p.*
 FROM wanted_entries w JOIN wanted_provider_episodes p ON p.show_id=w.provider_ids->>'tvmaze'
 WHERE w.category='series' AND w.server_id IS NULL AND NOT w.paused
 AND (w.season IS NULL OR p.season=w.season) AND (w.episode IS NULL OR p.number=w.episode)
 ORDER BY p.show_id,p.episode_id,(w.episode IS NULL),(w.season IS NULL),w.created_at,w.id
 )`

func (s *Store) wantedReleaseCounts(ctx context.Context, v *ReleaseOverview) error {
	var added, ready int
	err := s.pool.QueryRow(ctx, `WITH tracked AS (
 SELECT DISTINCT w.provider_ids->>'tvmaze' AS provider FROM wanted_entries w
 WHERE w.category='series' AND w.server_id IS NULL AND NOT w.paused AND COALESCE(w.provider_ids->>'tvmaze','')<>'' AND NOT EXISTS (
   SELECT 1 FROM release_follows f JOIN library_titles t ON t.id=f.title_id
   WHERE f.enabled AND t.canonical_id IS NULL AND f.provider_id=w.provider_ids->>'tvmaze'
 )
 ) SELECT count(*),count(s.checked_at) FROM tracked LEFT JOIN wanted_schedules s ON s.show_id=tracked.provider`).Scan(&added, &ready)
	v.Following += added
	v.Ready += ready
	return err
}

func (s *Store) readUpcomingReleases(ctx context.Context, v *ReleaseOverview, now time.Time) error {
	const projection = trackedEpisodes + `, native_upcoming AS (
 SELECT t.id::text AS title_id,COALESCE(w.tracked_id,'') AS tracked_id,t.name AS title,t.category,
 p.episode_id,p.name,p.season,p.number,p.air_date,p.scheduled_at,p.precision,p.provider_url,p.show_id
 FROM provider_episodes p JOIN release_follows f ON f.title_id=p.title_id AND f.provider_id=p.show_id
 JOIN library_titles t ON t.id=p.title_id LEFT JOIN tracked_episodes w ON (w.show_id,w.episode_id)=(p.show_id,p.episode_id)
 WHERE t.canonical_id IS NULL AND f.enabled AND p.active AND p.scheduled_at>$1
 ), upcoming AS (
 SELECT title_id,tracked_id,title,category,episode_id,name,season,number,air_date,scheduled_at,precision,provider_url FROM native_upcoming
 UNION ALL
 SELECT w.title_id,w.tracked_id,w.title,w.category,w.episode_id,w.name,w.season,w.number,w.air_date,w.scheduled_at,w.precision,w.provider_url
 FROM tracked_episodes w WHERE w.scheduled_at>$1 AND NOT EXISTS (
   SELECT 1 FROM native_upcoming n WHERE (w.show_id,w.episode_id)=(n.show_id,n.episode_id)
 )
 )`
	if err := s.pool.QueryRow(ctx, projection+`SELECT count(*) FROM upcoming`, now).Scan(&v.UpcomingTotal); err != nil {
		return err
	}
	rows, err := s.pool.Query(ctx, projection+`SELECT * FROM upcoming ORDER BY scheduled_at,title_id,tracked_id,episode_id LIMIT $2 OFFSET $3`, now, v.Limit, v.Offset)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var episode UpcomingEpisode
		if err = rows.Scan(&episode.TitleID, &episode.TrackedID, &episode.Title, &episode.Category, &episode.EpisodeID,
			&episode.Name, &episode.Season, &episode.Number, &episode.AirDate, &episode.ScheduledAt, &episode.Precision, &episode.URL); err != nil {
			return err
		}
		v.Upcoming = append(v.Upcoming, episode)
	}
	return rows.Err()
}

func (s *Store) readRecentReleases(ctx context.Context, v *ReleaseOverview, now time.Time) error {
	const projection = trackedEpisodes + `, ranked AS (
 SELECT e.*,i.canonical_id,COALESCE(o.status,'held') AS delivery,
 row_number() OVER(PARTITION BY i.canonical_id,e.show_id,e.episode_id,e.kind ORDER BY
 CASE COALESCE(o.status,'held') WHEN 'delivered' THEN 0 WHEN 'sending' THEN 1 WHEN 'pending' THEN 2 WHEN 'retry' THEN 3 WHEN 'held' THEN 4 WHEN 'coalesced' THEN 5 ELSE 6 END,e.created_at DESC,e.id DESC) AS priority
 FROM release_events e JOIN title_identity i ON i.id=e.title_id LEFT JOIN notification_outbox o ON o.event_id=e.id
 ), recent AS (
 SELECT e.id,e.canonical_id::text AS title_id,COALESCE(w.tracked_id,'') AS tracked_id,t.name AS title,t.category,e.episode_id,e.kind,e.episode_name,e.season,e.number,e.reported_at,e.precision,e.provider_url,e.delivery,e.created_at,e.show_id
 FROM ranked e JOIN library_titles t ON t.id=e.canonical_id
 LEFT JOIN tracked_episodes w ON (w.show_id,w.episode_id)=(e.show_id,e.episode_id)
 WHERE e.priority=1
 UNION ALL
 SELECT -row_number() OVER(ORDER BY w.show_id,w.episode_id),w.title_id,w.tracked_id,w.title,w.category,w.episode_id,
 COALESCE(a.kind,'episode'),w.name,w.season,w.number,w.scheduled_at,w.precision,w.provider_url,COALESCE(a.status,'held'),w.scheduled_at,w.show_id
 FROM tracked_episodes w LEFT JOIN wanted_air_outbox a ON a.wanted_id::text=w.tracked_id AND (a.show_id,a.episode_id)=(w.show_id,w.episode_id)
 WHERE w.scheduled_at<=$1 AND NOT EXISTS (
   SELECT 1 FROM ranked e WHERE e.priority=1 AND (w.show_id,w.episode_id)=(e.show_id,e.episode_id)
 )
 )`
	if err := s.pool.QueryRow(ctx, projection+`SELECT count(*) FROM recent`, now).Scan(&v.RecentTotal); err != nil {
		return err
	}
	rows, err := s.pool.Query(ctx, projection+`SELECT e.id,e.title_id,e.tracked_id,e.title,e.category,e.episode_id,e.kind,e.episode_name,e.season,e.number,e.reported_at,e.precision,e.provider_url,
 CASE WHEN EXISTS(SELECT 1 FROM release_delivery_receipts sent WHERE (sent.show_id,sent.episode_id,sent.kind)=(e.show_id,e.episode_id,e.kind)) THEN 'delivered' ELSE e.delivery END
 FROM recent e ORDER BY e.created_at DESC,e.id DESC LIMIT $2 OFFSET $3`, now, v.Limit, v.Offset)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var event ReleaseEvent
		if err = rows.Scan(&event.ID, &event.TitleID, &event.TrackedID, &event.Title, &event.Category, &event.EpisodeID,
			&event.Kind, &event.Name, &event.Season, &event.Number, &event.ReportedAt, &event.Precision, &event.URL, &event.Delivery); err != nil {
			return err
		}
		v.Recent = append(v.Recent, event)
	}
	return rows.Err()
}

func (s *Store) setReleaseAvailability(ctx context.Context, view *ReleaseOverview) error {
	type evidence struct {
		available map[string]bool
		uncertain bool
	}
	requested := map[string]*evidence{}
	for _, episode := range view.Upcoming {
		if episode.TrackedID != "" {
			requested[episode.TrackedID] = nil
		}
	}
	for _, event := range view.Recent {
		if event.TrackedID != "" {
			requested[event.TrackedID] = nil
		}
	}
	var items []Wanted
	if len(requested) != 0 {
		var err error
		items, err = s.wantedEntries(ctx)
		if err != nil {
			return err
		}
	}
	for _, item := range items {
		if _, present := requested[item.ID]; !present {
			continue
		}
		units, err := s.wantedEvidence(ctx, &item)
		if err != nil {
			return err
		}
		status := &evidence{available: map[string]bool{}, uncertain: item.Status == "uncertain"}
		for _, unit := range units {
			status.available[unit.key] = true
		}
		for _, server := range item.Servers {
			if server.Status == "uncertain" {
				status.uncertain = true
			}
		}
		requested[item.ID] = status
	}
	native, err := s.nativeReleaseAvailability(ctx, view)
	if err != nil {
		return err
	}
	availability := func(id, titleID string, season int, episode *int) string {
		if episode == nil {
			return "uncertain"
		}
		if id == "" {
			if value := native[nativeReleaseEpisode{titleID, season, *episode}]; value != "" {
				return value
			}
			return "uncertain"
		}
		status := requested[id]
		if status == nil {
			return "uncertain"
		}
		if status.available[fmt.Sprintf("%d:%d", season, *episode)] {
			return "available"
		}
		if status.uncertain {
			return "uncertain"
		}
		return "missing"
	}
	for i := range view.Upcoming {
		ep := &view.Upcoming[i]
		ep.Availability = availability(ep.TrackedID, ep.TitleID, ep.Season, ep.Number)
	}
	for i := range view.Recent {
		ep := &view.Recent[i]
		ep.Availability = availability(ep.TrackedID, ep.TitleID, ep.Season, ep.Number)
	}
	return nil
}
