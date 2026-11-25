// SPDX-License-Identifier: Elastic-2.0
package store

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/tendstead/lookout/internal/metadata"
	"github.com/tendstead/lookout/internal/releases"
	"sort"
	"strings"
	"time"
)

type SeasonGap struct {
	Seasons []releases.SeasonDifference `json:"seasons,omitempty"`
	releases.SeasonDifference
	TitleID      string     `json:"title_id"`
	Title        string     `json:"title"`
	Category     string     `json:"category"`
	LatestAired  int        `json:"latest_aired_season"`
	LocalSeasons []int      `json:"local_seasons"`
	Reasons      []string   `json:"reasons"`
	CheckedAt    *time.Time `json:"checked_at"`
	InventoryAt  *time.Time `json:"inventory_at"`
}
type SeasonGapPage struct {
	Items   []SeasonGap `json:"items"`
	Total   int         `json:"total"`
	Missing int         `json:"missing"`
	Partial int         `json:"partial"`
	Review  int         `json:"review"`
	Limit   int         `json:"limit"`
	Offset  int         `json:"offset"`
}
type seasonGapInput struct {
	row       SeasonGap
	inventory releases.SeasonInventory
}

// SeasonGaps compares one consistent snapshot of provider and inventory evidence.
// It does not depend on the release-event baseline or queue notifications.
func (s *Store) SeasonGaps(ctx context.Context, q TitleQuery, kind string, now time.Time) (SeasonGapPage, error) {
	if q.Limit < 1 || q.Limit > 100 {
		q.Limit = 48
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	out := SeasonGapPage{Items: []SeasonGap{}, Limit: q.Limit, Offset: q.Offset}
	tx, e := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if e != nil {
		return out, e
	}
	defer tx.Rollback(ctx)
	filter := ` t.canonical_id IS NULL AND t.category IN('series','anime_series') AND ($1='' OR t.name ILIKE '%'||$1||'%' OR t.group_key ILIKE '%'||$1||'%') AND ($2='' OR t.category=$2)`
	args := []any{q.Search, q.Category, q.ServerID}
	const nativeReady = `COALESCE(f.initialized AND f.provider_id=t.provider_ids->>'tvmaze' AND f.provider_id<>'',false)`
	const nativePreferred = `(` + nativeReady + ` AND (ws.checked_at IS NULL OR f.checked_at>=ws.checked_at))`
	const scheduleJoins = `
        LEFT JOIN release_follows f ON f.title_id=t.id
        LEFT JOIN wanted_schedules ws ON ws.show_id=t.provider_ids->>'tvmaze'`
	rows, e := tx.Query(ctx, `SELECT t.id::text,t.name,t.category,t.match_status='review',
        (`+nativeReady+` OR ws.checked_at IS NOT NULL),
        COALESCE((SELECT bool_and(xs.status IN('ready','scanning') AND xs.last_success IS NOT NULL
            AND xs.last_success>$4::timestamptz-make_interval(hours=>v.stale_hours))
            FROM library_sources xs JOIN library_servers v ON v.id=xs.server_id
            WHERE xs.category IN('series','anime_series') AND ($3='' OR xs.server_id=$3)),false),
        CASE WHEN `+nativePreferred+` THEN f.checked_at ELSE ws.checked_at END,
        (SELECT min(xs.last_success) FROM library_sources xs
            WHERE xs.category IN('series','anime_series') AND ($3='' OR xs.server_id=$3))
        FROM library_titles t `+scheduleJoins+` WHERE `+filter, append(args, now)...)
	if e != nil {
		return out, e
	}
	inputs := map[string]*seasonGapInput{}
	for rows.Next() {
		in := &seasonGapInput{}
		if e = rows.Scan(&in.row.TitleID, &in.row.Title, &in.row.Category, &in.inventory.MatchReview, &in.inventory.ScheduleReady, &in.inventory.InventoryReady, &in.row.CheckedAt, &in.row.InventoryAt); e != nil {
			rows.Close()
			return out, e
		}
		in.inventory.Anime = in.row.Category == "anime_series"
		inputs[in.row.TitleID] = in
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	// Aggregate episode numbers across copies and archive members before transfer.
	rows, e = tx.Query(ctx, `SELECT i.canonical_id::text,COALESCE(f.season,-1),f.numbering,f.observation,COALESCE(array_agg(DISTINCT n) FILTER(WHERE n IS NOT NULL),'{}') FROM library_files f JOIN title_identity i ON i.id=f.title_id JOIN library_titles t ON t.id=i.canonical_id JOIN library_sources fs ON fs.id=f.source_id LEFT JOIN LATERAL unnest(f.episodes) n ON true WHERE `+filter+` AND ($3='' OR fs.server_id=$3) GROUP BY i.canonical_id,f.season,f.numbering,f.observation`, args...)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var id string
		var f releases.LocalNumbering
		if e = rows.Scan(&id, &f.Season, &f.Numbering, &f.Observation, &f.Episodes); e != nil {
			rows.Close()
			return out, e
		}
		if in := inputs[id]; in != nil {
			in.inventory.Local = append(in.inventory.Local, f)
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	// scheduled_at already applies the shared conservative date-only policy.
	rows, e = tx.Query(ctx, `SELECT t.id::text,p.season,p.number,p.scheduled_at FROM library_titles t`+scheduleJoins+`
        JOIN LATERAL (
            SELECT native.season,native.number,native.scheduled_at FROM provider_episodes native
            WHERE `+nativePreferred+` AND native.title_id=t.id AND native.show_id=f.provider_id AND native.active
            UNION ALL
            SELECT tracked.season,tracked.number,tracked.scheduled_at FROM wanted_provider_episodes tracked
            WHERE NOT `+nativePreferred+` AND ws.checked_at IS NOT NULL AND tracked.show_id=ws.show_id
        ) p ON true WHERE `+filter+`
        AND ($3='' OR EXISTS(SELECT 1 FROM library_sources scope WHERE scope.server_id=$3 AND scope.category IN('series','anime_series')))
        AND p.scheduled_at<=$4 AND p.season>0 AND p.number>0`, append(args, now)...)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var id string
		var episode metadata.Episode
		var stamp time.Time
		if e = rows.Scan(&id, &episode.Season, &episode.Number, &stamp); e != nil {
			rows.Close()
			return out, e
		}
		episode.AirStamp = stamp.Format(time.RFC3339Nano)
		if in := inputs[id]; in != nil && in.inventory.ScheduleReady {
			in.inventory.Episodes = append(in.inventory.Episodes, episode)
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	all := []SeasonGap{}
	for _, in := range inputs {
		result := releases.CompareSeasons(in.inventory, now)
		in.row.LatestAired = result.LatestAired
		in.row.LocalSeasons = result.LocalSeasons
		in.row.Reasons = result.Reasons
		if len(result.Reasons) > 0 {
			in.row.Kind = "review"
			in.row.MissingEpisodes = []int{}
			all = append(all, in.row)
			out.Review++
			continue
		}
		for _, gap := range result.Gaps {
			row := in.row
			row.SeasonDifference = gap
			all = append(all, row)
			if gap.Kind == "missing" {
				out.Missing++
			} else {
				out.Partial++
			}
		}
	}
	sort.Slice(all, func(i, j int) bool {
		a, b := strings.ToLower(all[i].Title), strings.ToLower(all[j].Title)
		if a != b {
			return a < b
		}
		if all[i].TitleID != all[j].TitleID {
			return all[i].TitleID < all[j].TitleID
		}
		return all[i].Season < all[j].Season
	})
	visible := []SeasonGap{}
	for _, row := range all {
		if row.Kind == kind {
			visible = append(visible, row)
		}
	}
	if q.GroupByTitle && kind != "review" {
		grouped := []SeasonGap{}
		byID := map[string]int{}
		for _, row := range visible {
			index, exists := byID[row.TitleID]
			if !exists {
				index = len(grouped)
				byID[row.TitleID] = index
				first := row
				first.SeasonDifference = releases.SeasonDifference{Kind: kind, MissingEpisodes: []int{}}
				first.Seasons = []releases.SeasonDifference{}
				grouped = append(grouped, first)
			}
			grouped[index].Seasons = append(grouped[index].Seasons, row.SeasonDifference)
			grouped[index].Aired += row.Aired
			grouped[index].Present += row.Present
		}
		visible = grouped
	}
	for _, row := range visible {
		if row.Kind != kind {
			continue
		}
		index := out.Total
		out.Total++
		if index >= q.Offset && len(out.Items) < q.Limit {
			out.Items = append(out.Items, row)
		}
	}
	return out, tx.Commit(ctx)
}
