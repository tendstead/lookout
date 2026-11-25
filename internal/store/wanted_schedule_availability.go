// SPDX-License-Identifier: Elastic-2.0
package store

import "context"

type nativeReleaseEpisode struct {
	titleID string
	season  int
	number  int
}

func (s *Store) nativeReleaseAvailability(ctx context.Context, view *ReleaseOverview) (map[nativeReleaseEpisode]string, error) {
	availability := map[nativeReleaseEpisode]string{}
	var titles []string
	var seasons, numbers []int32
	request := func(trackedID, titleID string, season int, number *int) {
		if trackedID != "" || titleID == "" || number == nil {
			return
		}
		key := nativeReleaseEpisode{titleID, season, *number}
		if _, exists := availability[key]; exists {
			return
		}
		availability[key] = "uncertain"
		titles = append(titles, titleID)
		seasons = append(seasons, int32(season))
		numbers = append(numbers, int32(*number))
	}
	for _, episode := range view.Upcoming {
		request(episode.TrackedID, episode.TitleID, episode.Season, episode.Number)
	}
	for _, episode := range view.Recent {
		request(episode.TrackedID, episode.TitleID, episode.Season, episode.Number)
	}
	if len(titles) == 0 {
		return availability, nil
	}
	rows, err := s.pool.Query(ctx, `WITH scopes AS (
 SELECT v.id,bool_and(ls.status='ready' AND ls.last_success IS NOT NULL
      AND ls.last_success>now()-make_interval(hours=>v.stale_hours)) AS ready
 FROM library_servers v JOIN library_sources ls ON ls.server_id=v.id
 WHERE ls.category IN ('series','anime_series') GROUP BY v.id
 ), coverage AS (SELECT COALESCE(bool_and(ready),false) AS ready FROM scopes)
 SELECT r.title_id,r.season,r.number,CASE
 WHEN t.id IS NULL OR t.match_status='review' THEN 'uncertain'
 WHEN bool_or(scope.ready AND f.observation IN('observed','stable') AND f.numbering='season_episode'
      AND f.season=r.season AND r.number=ANY(f.episodes)) THEN 'available'
 WHEN NOT coverage.ready OR bool_or(f.observation IN('observed','stable','unconfirmed') AND (
      f.numbering<>'season_episode' OR f.season IS NULL OR cardinality(f.episodes)=0 OR
      f.observation='unconfirmed' AND f.season=r.season AND r.number=ANY(f.episodes)
 )) THEN 'uncertain'
 ELSE 'missing' END
 FROM unnest($1::text[],$2::integer[],$3::integer[]) r(title_id,season,number)
 CROSS JOIN coverage
 LEFT JOIN library_titles t ON t.id=r.title_id::uuid
 LEFT JOIN title_identity i ON i.canonical_id=t.id
 LEFT JOIN library_files f ON f.title_id=i.id AND (f.season=r.season OR f.season IS NULL)
 LEFT JOIN library_sources ls ON ls.id=f.source_id
 LEFT JOIN scopes scope ON scope.id=ls.server_id
 GROUP BY r.title_id,r.season,r.number,t.id,coverage.ready`, titles, seasons, numbers)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var key nativeReleaseEpisode
		var value string
		if err = rows.Scan(&key.titleID, &key.season, &key.number, &value); err != nil {
			return nil, err
		}
		availability[key] = value
	}
	return availability, rows.Err()
}
