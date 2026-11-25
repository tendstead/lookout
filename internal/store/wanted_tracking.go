// SPDX-License-Identifier: Elastic-2.0
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/tendstead/lookout/internal/metadata"
)

// PreviewWanted pairs the requested metadata with confirmed server observations.
func (s *Store) PreviewWanted(ctx context.Context, v *Wanted, episodes []metadata.Episode) error {
	units, err := s.wantedEvidence(ctx, v)
	if err != nil {
		return err
	}
	if episodes == nil {
		episodes, err = s.WantedEpisodes(ctx, *v)
		if err != nil {
			return err
		}
	}
	servers := make(map[string]string, len(v.Servers))
	for _, server := range v.Servers {
		servers[server.ServerID] = server.Name
	}
	byKey := make(map[string][]string)
	for _, unit := range units {
		byKey[unit.key] = append(byKey[unit.key], servers[unit.server])
	}
	v.Episodes = []WantedEpisode{}
	for _, episode := range episodes {
		if (v.Season != nil && episode.Season != *v.Season) ||
			(v.Episode != nil && (episode.Number == nil || *episode.Number != *v.Episode)) {
			continue
		}
		available := []string{}
		if episode.Number != nil {
			available = append(available, byKey[fmt.Sprintf("%d:%d", episode.Season, *episode.Number)]...)
		}
		sort.Strings(available)
		v.Episodes = append(v.Episodes, WantedEpisode{Episode: episode, Servers: available})
	}
	if v.AlertMode == "season" {
		ends, err := s.WantedSeasonEnds(ctx, *v)
		if err != nil {
			return err
		}
		v.SeasonEndKnown = v.ExpectedEpisodes != nil
		for number, season := range ends {
			if (v.Season == nil || number == *v.Season) && season.EndDate != "" && season.EpisodeOrder > 0 {
				v.SeasonEndKnown = true
			}
		}
	}
	return nil
}

func (s *Store) FindTrackedTitle(ctx context.Context, target Wanted) (string, error) {
	items, err := s.wantedEntries(ctx)
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if item.ServerID != target.ServerID || !sameKind(target.Category, item.Category) {
			continue
		}
		if target.LocalTitleID != "" && target.LocalTitleID == item.LocalTitleID ||
			target.SourceURL != "" && target.SourceURL == item.SourceURL {
			return item.ID, nil
		}
		shared, conflict := false, false
		for provider, id := range target.ProviderIDs {
			if other := item.ProviderIDs[provider]; other != "" {
				shared = shared || other == id
				conflict = conflict || other != id
			}
		}
		if !conflict && (shared || len(target.ProviderIDs) == 0 && len(item.ProviderIDs) == 0 &&
			strings.EqualFold(target.Name, item.Name) && (target.Year == nil || item.Year == nil || *target.Year == *item.Year)) {
			return item.ID, nil
		}
	}
	return "", nil
}

func (s *Store) UpdateWanted(ctx context.Context, v Wanted) (Wanted, error) {
	if err := validateWanted(&v); err != nil {
		return v, err
	}
	raw, err := json.Marshal(v.ProviderIDs)
	if err != nil {
		return v, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return v, err
	}
	defer tx.Rollback(ctx)
	var locked bool
	if err = tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(726431211)`).Scan(&locked); err != nil {
		return v, err
	}
	if !locked {
		return v, fmt.Errorf("tracking refresh in progress")
	}
	var previous Wanted
	var previousIDs []byte
	err = tx.QueryRow(ctx, `SELECT name,category,year,local_title_id,provider_ids,source_url,COALESCE(server_id,''),paused FROM wanted_entries WHERE id::text=$1 FOR UPDATE`, v.ID).
		Scan(&previous.Name, &previous.Category, &previous.Year, &previous.LocalTitleID, &previousIDs, &previous.SourceURL, &previous.ServerID, &previous.Paused)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, ErrNotFound
	}
	if err != nil {
		return v, err
	}
	if err = json.Unmarshal(previousIDs, &previous.ProviderIDs); err != nil {
		return v, err
	}
	identityChanged := wantedIdentityChanged(previous, v) || previous.ServerID != v.ServerID
	pauseChanged := previous.Paused != v.Paused
	tag, err := tx.Exec(ctx, `
        UPDATE wanted_entries
        SET name=$2, category=$3, year=$4, provider_ids=$5, season=$6, episode=$7,
            source_url=$8, alert_mode=$9, air_alerts=$10, local_title_id=$11, expected_episodes=$12,
            server_id=NULLIF($14,''), paused=$15,
            release_baseline_at=CASE WHEN (NOT air_alerts AND $10) OR provider_ids IS DISTINCT FROM $5::jsonb
                OR season IS DISTINCT FROM $6 OR episode IS DISTINCT FROM $7 OR $13 OR paused<>$15 THEN now() ELSE release_baseline_at END
        WHERE id::text=$1`,
		v.ID, v.Name, v.Category, v.Year, raw, v.Season, v.Episode,
		v.SourceURL, v.AlertMode, v.AirAlerts, v.LocalTitleID, v.ExpectedEpisodes, identityChanged, v.ServerID, v.Paused)
	if err != nil {
		return v, err
	}
	if tag.RowsAffected() == 0 {
		return v, ErrNotFound
	}
	if identityChanged {
		if _, err = tx.Exec(ctx, `DELETE FROM wanted_arrivals WHERE wanted_id::text=$1`, v.ID); err != nil {
			return v, err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM wanted_air_outbox WHERE wanted_id::text=$1`, v.ID); err != nil {
			return v, err
		}
	} else if pauseChanged {
		if _, err = tx.Exec(ctx, `UPDATE wanted_arrivals SET status='baseline',claimed_until=NULL WHERE wanted_id::text=$1 AND status<>'delivered'`, v.ID); err != nil {
			return v, err
		}
		if _, err = tx.Exec(ctx, `UPDATE wanted_air_outbox SET status='cancelled',claimed_until=NULL
            WHERE wanted_id::text=$1 AND status IN ('pending','retry','sending')`, v.ID); err != nil {
			return v, err
		}
	} else if _, err = tx.Exec(ctx, `UPDATE wanted_arrivals SET status='baseline' WHERE wanted_id::text=$1
        AND status IN ('pending','retry') AND (
            ($2='season' AND content_key NOT LIKE 'complete:%') OR ($2='episode' AND content_key LIKE 'complete:%')
            OR $2='none' OR ($3::int IS NOT NULL AND CASE WHEN content_key LIKE 'complete:%'
                THEN split_part(content_key, ':', 2) <> $3::int::text
                ELSE split_part(content_key, ':', 1) <> $3::int::text END)
            OR ($4::int IS NOT NULL AND split_part(content_key, ':', 2) <> $4::int::text))`,
		v.ID, v.AlertMode, v.Season, v.Episode); err != nil {
		return v, err
	}
	units, err := s.wantedEvidence(ctx, &v, true)
	if err != nil {
		return v, err
	}
	for _, unit := range units {
		if _, err = tx.Exec(ctx, `INSERT INTO wanted_arrivals(wanted_id,server_id,content_key,description,status)
            VALUES($1,$2,$3,$4,'baseline') ON CONFLICT DO NOTHING`, v.ID, unit.server, unit.key, unit.description); err != nil {
			return v, err
		}
	}
	if err = transferWantedAirAlerts(ctx, tx, v); err != nil {
		return v, err
	}
	if err = s.recordSeasonCompletion(ctx, tx, v, units, "baseline"); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

// The Watchlist owns airing alerts once a library follow is edited there.
func transferWantedAirAlerts(ctx context.Context, tx pgx.Tx, v Wanted) error {
	if v.LocalTitleID == "" || v.Category != "series" || v.ServerID != "" {
		return nil
	}
	var id string
	err := tx.QueryRow(ctx, `UPDATE release_follows SET enabled=false
        WHERE enabled AND title_id=(SELECT canonical_id FROM title_identity WHERE id::text=$1)
        RETURNING title_id::text`, v.LocalTitleID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE notification_outbox o SET status='cancelled' FROM release_events e
        WHERE o.event_id=e.id AND e.title_id IN (SELECT id FROM title_identity WHERE canonical_id::text=$1)
        AND o.status IN ('pending','retry')`, id)
	return err
}

func wantedIdentityChanged(previous, current Wanted) bool {
	if previous.Category != current.Category {
		return true
	}
	shared := false
	for provider, id := range previous.ProviderIDs {
		if nextID := current.ProviderIDs[provider]; nextID != "" {
			if nextID != id {
				return true
			}
			shared = true
		}
	}
	if shared || previous.LocalTitleID != "" && previous.LocalTitleID == current.LocalTitleID {
		return false
	}
	if previous.LocalTitleID != "" && current.LocalTitleID != "" {
		return true
	}
	return !strings.EqualFold(previous.Name, current.Name) ||
		previous.Year != nil && current.Year != nil && *previous.Year != *current.Year ||
		previous.SourceURL != "" && current.SourceURL != "" && previous.SourceURL != current.SourceURL
}

func (s *Store) recordSeasonCompletion(ctx context.Context, tx pgx.Tx, v Wanted, units []wantedUnit, status string) error {
	if v.AlertMode != "season" || len(units) == 0 {
		return nil
	}
	episodes, err := s.WantedEpisodes(ctx, v)
	if err != nil {
		return err
	}
	ends, err := s.WantedSeasonEnds(ctx, v)
	if err != nil {
		return err
	}
	present := make(map[string]string)
	for _, unit := range units {
		if present[unit.key] == "" || unit.server < present[unit.key] {
			present[unit.key] = unit.server
		}
	}
	now := time.Now().UTC()
	expected := make(map[int]map[int]bool)
	for _, episode := range episodes {
		season, ok := ends[episode.Season]
		if !ok || episode.Season <= 0 || episode.Number == nil || *episode.Number <= 0 ||
			(v.Season != nil && *v.Season != episode.Season) {
			continue
		}
		end, e := time.Parse(time.DateOnly, season.EndDate)
		// Date-only metadata is elapsed after that calendar day.
		if e != nil || now.Before(end.AddDate(0, 0, 1)) || season.EpisodeOrder <= 0 {
			continue
		}
		if expected[episode.Season] == nil {
			expected[episode.Season] = make(map[int]bool)
		}
		expected[episode.Season][*episode.Number] = true
	}
	if v.ExpectedEpisodes != nil {
		manual := make(map[int]bool, *v.ExpectedEpisodes)
		for number := 1; number <= *v.ExpectedEpisodes; number++ {
			manual[number] = true
		}
		expected[*v.Season] = manual
	}
	for season, numbers := range expected {
		if v.ExpectedEpisodes == nil && len(numbers) != ends[season].EpisodeOrder {
			continue
		}
		server := ""
		for number := range numbers {
			id := present[fmt.Sprintf("%d:%d", season, number)]
			if id == "" {
				server = ""
				break
			}
			if server == "" || id < server {
				server = id
			}
		}
		if server == "" {
			continue
		}
		completionStatus := status
		if status != "baseline" {
			// If metadata arrives after creation, a season whose episodes
			// were all present at creation remains a baseline.
			keys := make([]string, 0, len(numbers))
			for number := range numbers {
				keys = append(keys, fmt.Sprintf("%d:%d", season, number))
			}
			var arrived bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM wanted_arrivals
                WHERE wanted_id=$1 AND content_key=ANY($2) AND status<>'baseline')`, v.ID, keys).Scan(&arrived); err != nil {
				return err
			}
			if !arrived {
				completionStatus = "baseline"
			}
		}
		key := fmt.Sprintf("complete:%d", season)
		_, err = tx.Exec(ctx, `INSERT INTO wanted_arrivals(wanted_id,server_id,content_key,description,status)
            SELECT $1,$2,$3,$4,$5 WHERE NOT EXISTS (SELECT 1 FROM wanted_arrivals WHERE wanted_id=$1 AND content_key=$3)
            ON CONFLICT DO NOTHING`, v.ID, server, key, fmt.Sprintf("Season %d", season), completionStatus)
		if err != nil {
			return err
		}
	}
	return nil
}
