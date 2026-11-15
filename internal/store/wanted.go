// SPDX-License-Identifier: Elastic-2.0
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"sort"
	"strings"
	"time"
)

type Wanted struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Category    string            `json:"category"`
	Year        *int              `json:"year,omitempty"`
	ProviderIDs map[string]string `json:"provider_ids"`
	Season      *int              `json:"season,omitempty"`
	Episode     *int              `json:"episode,omitempty"`
	Status      string            `json:"status"`
	Servers     []WantedServer    `json:"servers"`
}
type WantedServer struct {
	ServerID string   `json:"server_id"`
	Name     string   `json:"name"`
	Status   string   `json:"status"`
	Content  []string `json:"content"`
}
type wantedUnit struct{ server, key, description string }

func validateWanted(v *Wanted) error {
	v.Name = strings.TrimSpace(v.Name)
	if v.Name == "" || len(v.Name) > 300 || (v.Category != "movie" && v.Category != "series") || (v.Year != nil && (*v.Year < 1800 || *v.Year > 3000)) || (v.Season != nil && (*v.Season < 0 || *v.Season > 1000 || v.Category != "series")) || (v.Episode != nil && (v.Season == nil || *v.Episode < 1 || *v.Episode > 100000)) {
		return errors.New("invalid wanted target")
	}
	if v.ProviderIDs == nil {
		v.ProviderIDs = map[string]string{}
	}
	for k, id := range v.ProviderIDs {
		switch k {
		case "imdb", "tmdb", "tvdb", "tvmaze", "anidb":
		default:
			return errors.New("unsupported provider")
		}
		if id == "" || len(id) > 100 {
			return errors.New("invalid identity")
		}
	}
	return nil
}
func (s *Store) AddWanted(ctx context.Context, v Wanted) (Wanted, error) {
	if e := validateWanted(&v); e != nil {
		return v, e
	}
	raw, e := json.Marshal(v.ProviderIDs)
	if e != nil {
		return v, e
	}
	// Serialize creation/refresh so an already present copy is always baselined.
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return v, e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(726431211)`); e != nil {
		return v, e
	}
	if e = tx.QueryRow(ctx, `INSERT INTO wanted_entries(name,category,year,provider_ids,season,episode) VALUES($1,$2,$3,$4,$5,$6) RETURNING id::text`, v.Name, v.Category, v.Year, raw, v.Season, v.Episode).Scan(&v.ID); e != nil {
		return v, e
	}
	units, e := s.wantedEvidence(ctx, &v, true)
	if e != nil {
		return v, e
	}
	for _, u := range units {
		if _, e = tx.Exec(ctx, `INSERT INTO wanted_arrivals(wanted_id,server_id,content_key,description,status) VALUES($1,$2,$3,$4,'baseline') ON CONFLICT DO NOTHING`, v.ID, u.server, u.key, u.description); e != nil {
			return v, e
		}
	}
	return v, tx.Commit(ctx)
}
func (s *Store) DeleteWanted(ctx context.Context, id string) error {
	tag, e := s.pool.Exec(ctx, `DELETE FROM wanted_entries WHERE id::text=$1`, id)
	if e == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return e
}
func (s *Store) wantedEntries(ctx context.Context) ([]Wanted, error) {
	rows, e := s.pool.Query(ctx, `SELECT id::text,name,category,year,provider_ids,season,episode FROM wanted_entries ORDER BY lower(name),id`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Wanted{}
	for rows.Next() {
		var v Wanted
		var raw []byte
		if e = rows.Scan(&v.ID, &v.Name, &v.Category, &v.Year, &raw, &v.Season, &v.Episode); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(raw, &v.ProviderIDs); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) ListWanted(ctx context.Context) ([]Wanted, error) {
	items, e := s.wantedEntries(ctx)
	if e != nil {
		return nil, e
	}
	for i := range items {
		if _, e = s.wantedEvidence(ctx, &items[i]); e != nil {
			return nil, e
		}
	}
	return items, nil
}
func sameKind(category, kind string) bool {
	return (category == "series" || category == "anime_series") == (kind == "series")
}
func wantedMatches(w Wanted, t Title) (bool, bool) {
	if !sameKind(t.Category, w.Category) {
		return false, false
	}
	shared := false
	for k, v := range w.ProviderIDs {
		if id := t.ProviderIDs[k]; id != "" {
			if id != v {
				return false, false
			}
			shared = true
		}
	}
	if shared {
		return t.MatchStatus != "review", t.MatchStatus == "review"
	}
	name := strings.EqualFold(strings.Join(strings.Fields(w.Name), " "), strings.Join(strings.Fields(t.Name), " "))
	if !name {
		return false, false
	}
	if w.Year != nil && t.Year != nil && *w.Year != *t.Year {
		return false, false
	}
	// Known IDs on a request cannot be satisfied by a title lacking those IDs.
	if len(w.ProviderIDs) > 0 || w.Year != nil && t.Year == nil {
		return false, true
	}
	return t.MatchStatus != "review", t.MatchStatus == "review"
}
func (s *Store) wantedEvidence(ctx context.Context, w *Wanted, baseline ...bool) ([]wantedUnit, error) {
	type scope struct {
		v     WantedServer
		ready bool
	}
	scopes := map[string]*scope{}
	rows, e := s.pool.Query(ctx, `SELECT v.id,v.name,bool_and(ls.status='ready' AND ls.last_success IS NOT NULL AND ls.last_success>now()-make_interval(hours=>v.stale_hours)) FROM library_servers v JOIN library_sources ls ON ls.server_id=v.id WHERE ($1='series' AND ls.category IN('series','anime_series')) OR ($1='movie' AND ls.category IN('movie','anime_movie')) GROUP BY v.id ORDER BY v.name`, w.Category)
	if e != nil {
		return nil, e
	}
	for rows.Next() {
		var id, name string
		var ready bool
		if e = rows.Scan(&id, &name, &ready); e != nil {
			rows.Close()
			return nil, e
		}
		scopes[id] = &scope{WantedServer{ServerID: id, Name: name, Status: "missing", Content: []string{}}, ready}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	// Read all possible local identities; exact IDs can match different filenames.
	pageTitles := []Title{}
	rows, e = s.pool.Query(ctx, `SELECT t.id::text,t.name,t.category,t.year,t.provider_ids,t.match_status FROM library_titles t WHERE t.canonical_id IS NULL`)
	if e != nil {
		return nil, e
	}
	for rows.Next() {
		var v Title
		var ids []byte
		if e = rows.Scan(&v.ID, &v.Name, &v.Category, &v.Year, &ids, &v.MatchStatus); e != nil {
			rows.Close()
			return nil, e
		}
		if e = json.Unmarshal(ids, &v.ProviderIDs); e != nil {
			rows.Close()
			return nil, e
		}
		pageTitles = append(pageTitles, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	matches := []Title{}
	ambiguous := false
	for _, title := range pageTitles {
		yes, maybe := wantedMatches(*w, title)
		ambiguous = ambiguous || maybe
		if yes {
			matches = append(matches, title)
		}
	}
	if len(w.ProviderIDs) == 0 && len(matches) > 1 {
		ambiguous = true
		matches = nil
	}
	units := []wantedUnit{}
	seen := map[string]bool{}
	for _, title := range matches {
		d, err := s.GetTitle(ctx, title.ID)
		if err != nil {
			return nil, err
		}
		for _, f := range d.Files {
			scope := scopes[f.ServerID]
			if scope == nil {
				continue
			}
			if f.Observation == "unconfirmed" {
				ambiguous = true
			}
			if f.Observation != "observed" && f.Observation != "stable" {
				continue
			}
			if !scope.ready && !(len(baseline) > 0 && baseline[0]) {
				continue
			}
			keys := []wantedUnit{}
			if w.Category == "movie" {
				keys = append(keys, wantedUnit{f.ServerID, "movie", "Movie"})
			} else if f.Season != nil && len(f.Episodes) > 0 && f.Numbering == "season_episode" {
				for _, ep := range f.Episodes {
					if w.Season != nil && *w.Season != *f.Season || w.Episode != nil && *w.Episode != ep {
						continue
					}
					keys = append(keys, wantedUnit{f.ServerID, fmt.Sprintf("%d:%d", *f.Season, ep), fmt.Sprintf("S%02dE%02d", *f.Season, ep)})
				}
			} else if w.Season == nil {
				keys = append(keys, wantedUnit{f.ServerID, fmt.Sprintf("entry:%d", f.ID), "Episode (numbering unknown)"})
			} else {
				ambiguous = true
			}
			for _, u := range keys {
				key := u.server + ":" + u.key
				if seen[key] {
					continue
				}
				seen[key] = true
				units = append(units, u)
				scope.v.Content = append(scope.v.Content, u.description)
			}
		}
	}
	w.Status = "missing"
	w.Servers = []WantedServer{}
	for _, scope := range scopes {
		if len(scope.v.Content) > 0 {
			scope.v.Status = "available"
		} else if !scope.ready || ambiguous {
			scope.v.Status = "uncertain"
		}
		w.Servers = append(w.Servers, scope.v)
		if scope.v.Status == "available" {
			w.Status = "available"
		} else if scope.v.Status == "uncertain" && w.Status != "available" {
			w.Status = "uncertain"
		}
	}
	sort.Slice(w.Servers, func(i, j int) bool { return w.Servers[i].Name < w.Servers[j].Name })
	if len(scopes) == 0 {
		w.Status = "uncertain"
	}
	return units, nil
}
func (s *Store) RefreshWanted(ctx context.Context, notify bool) error {
	// One refresh transaction owns deduplication; existing observations are never
	// replayed merely because delivery is enabled later.
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(726431211)`); e != nil {
		return e
	}
	items, e := s.ListWanted(ctx)
	if e != nil {
		return e
	}
	status := "held"
	if notify {
		status = "pending"
	}
	for _, v := range items {
		units, e := s.wantedEvidence(ctx, &v)
		if e != nil {
			return e
		}
		for _, u := range units {
			if _, e = tx.Exec(ctx, `INSERT INTO wanted_arrivals(wanted_id,server_id,content_key,description,status) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, v.ID, u.server, u.key, u.description, status); e != nil {
				return e
			}
		}
	}
	return tx.Commit(ctx)
}
func (s *Store) ClaimWantedNotification(ctx context.Context, now time.Time) (*Delivery, error) {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	var d Delivery
	d.Wanted = true
	d.Kind = "wanted"
	var wanted, server string
	e = tx.QueryRow(ctx, `SELECT a.id,w.name,v.name,a.attempts,a.wanted_id::text,a.server_id FROM wanted_arrivals a JOIN wanted_entries w ON w.id=a.wanted_id JOIN library_servers v ON v.id=a.server_id WHERE (a.status IN('pending','retry') OR a.status='sending' AND a.claimed_until<$1) AND a.next_attempt<=$1 ORDER BY a.id FOR UPDATE OF a SKIP LOCKED LIMIT 1`, now).Scan(&d.ID, &d.Title, &d.ServerName, &d.Attempts, &wanted, &server)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	rows, e := tx.Query(ctx, `SELECT id,description FROM wanted_arrivals WHERE wanted_id::text=$1 AND server_id=$2 AND (status IN('pending','retry') OR status='sending' AND claimed_until<$3) AND next_attempt<=$3 ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 10000`, wanted, server, now)
	if e != nil {
		return nil, e
	}
	descriptions := []string{}
	for rows.Next() {
		var id int64
		var description string
		if e = rows.Scan(&id, &description); e != nil {
			rows.Close()
			return nil, e
		}
		d.WantedIDs = append(d.WantedIDs, id)
		if len(descriptions) < 12 {
			descriptions = append(descriptions, description)
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	d.Name = strings.Join(descriptions, ", ")
	if len(d.WantedIDs) > 12 {
		d.Name += fmt.Sprintf(" (+%d)", len(d.WantedIDs)-12)
	}
	if _, e = tx.Exec(ctx, `UPDATE wanted_arrivals SET status='sending',claimed_until=$2::timestamptz+interval '2 minutes',attempts=attempts+1 WHERE id=ANY($1)`, d.WantedIDs, now); e != nil {
		return nil, e
	}
	return &d, tx.Commit(ctx)
}
func (s *Store) MarkWantedNotification(ctx context.Context, id int64, sent bool, retry time.Duration, now time.Time, batch ...int64) error {
	if len(batch) == 0 {
		batch = []int64{id}
	}
	if sent {
		_, e := s.pool.Exec(ctx, `UPDATE wanted_arrivals SET status='delivered',sent_at=$2,claimed_until=NULL,last_error='' WHERE id=ANY($1)`, batch, now)
		return e
	}
	if retry < time.Minute {
		retry = time.Minute
	}
	_, e := s.pool.Exec(ctx, `UPDATE wanted_arrivals SET status=CASE WHEN attempts>=8 THEN 'failed' ELSE 'retry' END,next_attempt=$2,claimed_until=NULL,last_error='Telegram delivery failed' WHERE id=ANY($1)`, batch, now.Add(retry))
	return e
}
