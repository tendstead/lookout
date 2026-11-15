// SPDX-License-Identifier: Elastic-2.0
package store

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

type TitleQuery struct {
	LocalOnly        bool
	GroupByTitle     bool
	Search, Category string
	ServerID         string
	Limit, Offset    int
}
type Availability struct {
	ServerID    string     `json:"server_id"`
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	LastSuccess *time.Time `json:"last_success"`
	Files       int64      `json:"files"`
	Seasons     []int      `json:"seasons"`
}
type Title struct {
	Servers     []Availability    `json:"servers"`
	HasArtwork  bool              `json:"has_artwork"`
	MetadataURL string            `json:"metadata_url,omitempty"`
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Category    string            `json:"category"`
	Year        *int              `json:"year,omitempty"`
	Overview    string            `json:"overview"`
	ProviderIDs map[string]string `json:"provider_ids"`
	Provenance  string            `json:"provenance"`
	MatchStatus string            `json:"match_status"`
	FileCount   int64             `json:"file_count"`
	SourceLabel string            `json:"source_label"`
}
type TitlePage struct {
	Items  []Title `json:"items"`
	Total  int64   `json:"total"`
	Limit  int     `json:"limit"`
	Offset int     `json:"offset"`
}
type MediaFile struct {
	Adapter       string    `json:"adapter"`
	ReportedPath  string    `json:"reported_path,omitempty"`
	ServerID      string    `json:"server_id"`
	ServerName    string    `json:"server_name"`
	SourceID      string    `json:"source_id"`
	Storage       string    `json:"storage"`
	ArchiveMember string    `json:"archive_member,omitempty"`
	ID            int64     `json:"id"`
	RelativePath  string    `json:"relative_path"`
	Size          int64     `json:"size"`
	Modified      time.Time `json:"modified"`
	Season        *int      `json:"season"`
	Episodes      []int     `json:"episodes"`
	Numbering     string    `json:"numbering"`
	EpisodeTitle  string    `json:"episode_title"`
	Observation   string    `json:"observation"`
}
type TitleDetail struct {
	Title
	Files   []MediaFile   `json:"files"`
	Members []TitleMember `json:"members"`
}

type TitleMember struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	SourceID string `json:"source_id"`
	ServerID string `json:"server_id"`
}

const titleColumns = `t.id::text,t.name,t.category,t.year,t.overview,t.provider_ids,t.provenance,t.match_status,
 (SELECT count(*) FROM library_files f JOIN title_identity i ON i.id=f.title_id WHERE i.canonical_id=t.id),s.label,
 (t.artwork_path<>'' OR t.artwork_url<>''),t.metadata_url`
const titleFrom = ` FROM library_titles t JOIN library_sources s ON s.id=t.source_id WHERE t.canonical_id IS NULL`

func (s *Store) ListTitles(ctx context.Context, q TitleQuery) (TitlePage, error) {
	if q.Limit <= 0 {
		q.Limit = 48
	}
	if q.Limit > 100 {
		q.Limit = 100
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	page := TitlePage{Items: []Title{}, Limit: q.Limit, Offset: q.Offset}
	filter := ` AND ($1='' OR t.name ILIKE '%'||$1||'%' OR EXISTS(SELECT 1 FROM library_titles member JOIN title_identity mi ON mi.id=member.id WHERE mi.canonical_id=t.id AND member.group_key ILIKE '%'||$1||'%')) AND ($2='' OR t.category=$2)
 AND (NOT $3 OR EXISTS(SELECT 1 FROM library_files f JOIN title_identity i ON i.id=f.title_id JOIN library_sources fs ON fs.id=f.source_id WHERE i.canonical_id=t.id AND f.observation IN('observed','stable') AND ($4='' OR fs.server_id=$4)))
 AND ($4='' OR EXISTS(SELECT 1 FROM library_titles member JOIN title_identity mi ON mi.id=member.id JOIN library_sources ms ON ms.id=member.source_id WHERE mi.canonical_id=t.id AND ms.server_id=$4))`
	args := []any{q.Search, q.Category, q.LocalOnly, q.ServerID}
	if e := s.pool.QueryRow(ctx, `SELECT count(*)`+titleFrom+filter, args...).Scan(&page.Total); e != nil {
		return page, e
	}
	rows, e := s.pool.Query(ctx, `SELECT `+titleColumns+titleFrom+filter+` ORDER BY lower(t.name),t.id LIMIT $5 OFFSET $6`, append(args, q.Limit, q.Offset)...)
	if e != nil {
		return page, e
	}
	for rows.Next() {
		v, e := readTitle(rows)
		if e != nil {
			rows.Close()
			return page, e
		}
		page.Items = append(page.Items, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return page, e
	}
	for i := range page.Items {
		page.Items[i].Servers, e = s.TitleAvailability(ctx, page.Items[i].ID, q.ServerID)
		if e != nil {
			return page, e
		}
	}
	return page, nil
}
func readTitle(row pgx.Row) (Title, error) {
	var v Title
	var ids []byte
	e := row.Scan(&v.ID, &v.Name, &v.Category, &v.Year, &v.Overview, &ids, &v.Provenance, &v.MatchStatus, &v.FileCount, &v.SourceLabel, &v.HasArtwork, &v.MetadataURL)
	if e == nil {
		e = json.Unmarshal(ids, &v.ProviderIDs)
	}
	return v, e
}

var ErrNotFound = errors.New("not found")

func (s *Store) GetTitle(ctx context.Context, id string) (TitleDetail, error) {
	out := TitleDetail{Files: []MediaFile{}, Members: []TitleMember{}}
	canonical, e := s.CanonicalID(ctx, id)
	if e != nil {
		return out, e
	}
	out.Title, e = readTitle(s.pool.QueryRow(ctx, `SELECT `+titleColumns+titleFrom+` AND t.id::text=$1`, canonical))
	if e != nil {
		return out, e
	}
	out.Servers, e = s.TitleAvailability(ctx, canonical, "")
	if e != nil {
		return out, e
	}
	rows, e := s.pool.Query(ctx, `SELECT f.id,f.relative_path,f.size,f.modified_at,f.season,f.episodes,f.numbering,f.episode_title,f.observation,f.storage,f.archive_member,ls.server_id,v.name,f.source_id,ls.adapter,f.reported_path FROM library_files f JOIN title_identity i ON i.id=f.title_id JOIN library_sources ls ON ls.id=f.source_id JOIN library_servers v ON v.id=ls.server_id WHERE i.canonical_id::text=$1 ORDER BY v.name,f.season NULLS LAST,f.relative_path`, canonical)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var f MediaFile
		if e = rows.Scan(&f.ID, &f.RelativePath, &f.Size, &f.Modified, &f.Season, &f.Episodes, &f.Numbering, &f.EpisodeTitle, &f.Observation, &f.Storage, &f.ArchiveMember, &f.ServerID, &f.ServerName, &f.SourceID, &f.Adapter, &f.ReportedPath); e != nil {
			rows.Close()
			return out, e
		}
		out.Files = append(out.Files, f)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	rows, e = s.pool.Query(ctx, `SELECT t.id::text,t.name,t.source_id,ls.server_id FROM library_titles t JOIN title_identity i ON i.id=t.id JOIN library_sources ls ON ls.id=t.source_id WHERE i.canonical_id::text=$1 ORDER BY t.created_at,t.id`, canonical)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var m TitleMember
		if e = rows.Scan(&m.ID, &m.Name, &m.SourceID, &m.ServerID); e != nil {
			return out, e
		}
		out.Members = append(out.Members, m)
	}
	return out, rows.Err()
}
func (s *Store) TitleAvailability(ctx context.Context, id, server string) ([]Availability, error) {
	rows, e := s.pool.Query(ctx, `SELECT v.id,v.name,
 CASE WHEN bool_or(ls.status IN('error','review','interrupted') OR ls.last_success IS NULL OR ls.last_success<now()-make_interval(hours=>v.stale_hours)) THEN 'uncertain'
 WHEN count(f.id) FILTER(WHERE f.observation IN('observed','stable'))>0 THEN 'available'
 WHEN count(f.id) FILTER(WHERE f.observation='unconfirmed')>0 THEN 'uncertain' ELSE 'missing' END,
 min(ls.last_success),count(DISTINCT f.id) FILTER(WHERE f.observation IN('observed','stable')),
 COALESCE(array_agg(DISTINCT f.season ORDER BY f.season) FILTER(WHERE f.season IS NOT NULL AND f.observation IN('observed','stable')),'{}')
 FROM library_titles t JOIN title_identity i ON i.id=t.id JOIN library_sources ls ON ls.id=t.source_id JOIN library_servers v ON v.id=ls.server_id LEFT JOIN library_files f ON f.title_id=t.id WHERE i.canonical_id::text=$1 AND ($2='' OR v.id=$2) GROUP BY v.id ORDER BY lower(v.name),v.id`, id, server)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Availability{}
	for rows.Next() {
		var v Availability
		if e = rows.Scan(&v.ServerID, &v.Name, &v.Status, &v.LastSuccess, &v.Files, &v.Seasons); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
