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
	Search, Category string
	Limit, Offset    int
}
type Title struct {
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
	Files []MediaFile `json:"files"`
}

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
	filter := ` WHERE ($1='' OR t.name ILIKE '%'||$1||'%' OR t.group_key ILIKE '%'||$1||'%') AND ($2='' OR t.category=$2)`
	if e := s.pool.QueryRow(ctx, "SELECT count(*) FROM library_titles t"+filter, q.Search, q.Category).Scan(&page.Total); e != nil {
		return page, e
	}
	rows, e := s.pool.Query(ctx, `SELECT t.id::text,t.name,t.category,t.year,t.overview,t.provider_ids,t.provenance,t.match_status,(SELECT count(*) FROM library_files f WHERE f.title_id=t.id),s.label,(t.artwork_path<>'' OR t.artwork_url<>''),t.metadata_url FROM library_titles t JOIN library_sources s ON s.id=t.source_id`+filter+` ORDER BY lower(t.name),t.id LIMIT $3 OFFSET $4`, q.Search, q.Category, q.Limit, q.Offset)
	if e != nil {
		return page, e
	}
	defer rows.Close()
	for rows.Next() {
		v, e := readTitle(rows)
		if e != nil {
			return page, e
		}
		page.Items = append(page.Items, v)
	}
	return page, rows.Err()
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
	var out TitleDetail
	v, e := readTitle(s.pool.QueryRow(ctx, `SELECT t.id::text,t.name,t.category,t.year,t.overview,t.provider_ids,t.provenance,t.match_status,(SELECT count(*) FROM library_files f WHERE f.title_id=t.id),s.label,(t.artwork_path<>'' OR t.artwork_url<>''),t.metadata_url FROM library_titles t JOIN library_sources s ON s.id=t.source_id WHERE t.id::text=$1`, id))
	if errors.Is(e, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	if e != nil {
		return out, e
	}
	out.Title = v
	out.Files = []MediaFile{}
	rows, e := s.pool.Query(ctx, `SELECT id,relative_path,size,modified_at,season,episodes,numbering,episode_title,observation,storage,archive_member FROM library_files WHERE title_id=$1 ORDER BY season NULLS LAST,relative_path`, v.ID)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var f MediaFile
		if e = rows.Scan(&f.ID, &f.RelativePath, &f.Size, &f.Modified, &f.Season, &f.Episodes, &f.Numbering, &f.EpisodeTitle, &f.Observation, &f.Storage, &f.ArchiveMember); e != nil {
			return out, e
		}
		out.Files = append(out.Files, f)
	}
	return out, rows.Err()
}
