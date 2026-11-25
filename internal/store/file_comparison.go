// SPDX-License-Identifier: Elastic-2.0
package store

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/tendstead/lookout/internal/library"
)

type FileDifference struct {
	TitleID    string      `json:"title_id"`
	TitleName  string      `json:"title_name"`
	HasArtwork bool        `json:"has_artwork"`
	Season     *int        `json:"season,omitempty"`
	Episode    *int        `json:"episode,omitempty"`
	Files      []MediaFile `json:"files"`
}

type FileDifferencePage struct {
	Items   []FileDifference `json:"items"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
	HasMore bool             `json:"has_more"`
}

func (s *Store) ListFileDifferences(ctx context.Context, limit, offset int) (FileDifferencePage, error) {
	page := FileDifferencePage{Items: []FileDifference{}, Limit: limit, Offset: offset}
	rows, err := s.pool.Query(ctx, `
 WITH candidate_titles AS (
   SELECT i.canonical_id
   FROM library_files f
   JOIN title_identity i ON i.id=f.title_id
   JOIN library_sources ls ON ls.id=f.source_id
   WHERE f.observation IN ('observed','stable')
   GROUP BY i.canonical_id HAVING count(DISTINCT ls.server_id)>1
 )
 SELECT t.id::text,t.name,t.category,(t.artwork_path<>'' OR t.artwork_url<>''),`+mediaFileColumns+`
 FROM candidate_titles ct
 JOIN library_titles t ON t.id=ct.canonical_id
 JOIN title_identity i ON i.canonical_id=t.id
 JOIN library_files f ON f.title_id=i.id
 JOIN library_sources ls ON ls.id=f.source_id
 JOIN library_servers v ON v.id=ls.server_id
 WHERE f.observation IN ('observed','stable')
 ORDER BY lower(t.name),t.id,v.name,f.relative_path,f.id`)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	var titleID, titleName, category string
	var hasArtwork bool
	var files []MediaFile
	collect := func() {
		for _, group := range fileGroups(titleID, titleName, category, files) {
			if !hasFileDifference(group.Files) {
				continue
			}
			if offset > 0 {
				offset--
				continue
			}
			if len(page.Items) == limit {
				page.HasMore = true
				return
			}
			group.HasArtwork = hasArtwork
			page.Items = append(page.Items, group)
		}
	}
	for rows.Next() {
		var id, name, kind string
		var artwork bool
		file, err := readMediaFile(rows, &id, &name, &kind, &artwork)
		if err != nil {
			return page, err
		}
		if id != titleID {
			collect()
			if page.HasMore {
				return page, nil
			}
			titleID, titleName, category, hasArtwork, files = id, name, kind, artwork, nil
		}
		files = append(files, file)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	collect()
	return page, nil
}

func fileGroups(id, name, category string, files []MediaFile) []FileDifference {
	if len(files) < 2 {
		return nil
	}
	if category == "movie" || category == "anime_movie" {
		return []FileDifference{{TitleID: id, TitleName: name, Files: files}}
	}
	type episodeKey struct{ season, episode int }
	groups := map[episodeKey][]MediaFile{}
	for _, file := range files {
		if file.Season == nil || *file.Season < 0 || (file.Numbering != "season_episode" && file.Numbering != "nfo") {
			continue
		}
		seen := make(map[int]bool, len(file.Episodes))
		for _, episode := range file.Episodes {
			if episode > 0 && !seen[episode] {
				key := episodeKey{*file.Season, episode}
				groups[key] = append(groups[key], file)
				seen[episode] = true
			}
		}
	}
	keys := slices.SortedFunc(maps.Keys(groups), func(a, b episodeKey) int {
		if a.season != b.season {
			return a.season - b.season
		}
		return a.episode - b.episode
	})
	result := make([]FileDifference, 0, len(keys))
	for _, key := range keys {
		files := groups[key]
		if len(files) >= 2 {
			result = append(result, FileDifference{TitleID: id, TitleName: name, Season: &key.season, Episode: &key.episode, Files: files})
		}
	}
	return result
}

func hasFileDifference(files []MediaFile) bool {
	for i, file := range files {
		for _, other := range files[i+1:] {
			if file.ServerID != other.ServerID && filesDiffer(file, other) {
				return true
			}
		}
	}
	return false
}

func filesDiffer(a, b MediaFile) bool {
	if knownDifference(a.FileSize, b.FileSize) {
		return true
	}
	x, y := a.Technical, b.Technical
	if x == nil || y == nil {
		return false
	}
	return knownDifference(x.Width, y.Width) || knownDifference(x.Height, y.Height) ||
		knownDifference(x.DurationMS, y.DurationMS) || knownDifference(x.BitrateBPS, y.BitrateBPS) ||
		knownDifference(x.VideoBitrateBPS, y.VideoBitrateBPS) || knownDifference(x.FrameRate, y.FrameRate) ||
		knownDifference(x.BitDepth, y.BitDepth) ||
		textDifference(x.Container, y.Container) || textDifference(x.VideoCodec, y.VideoCodec) ||
		textDifference(x.VideoProfile, y.VideoProfile) || textDifference(x.HDR, y.HDR) ||
		textDifference(x.Edition, y.Edition) ||
		tracksDiffer(audioFields(x.Audio), audioFields(y.Audio)) ||
		tracksDiffer(subtitleFields(x.Subtitles), subtitleFields(y.Subtitles))
}

func knownDifference[T comparable](a, b *T) bool {
	return a != nil && b != nil && *a != *b
}

func normalizedText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "unknown" || s == "-" {
		return ""
	}
	return s
}

func textDifference(a, b string) bool {
	a, b = normalizedText(a), normalizedText(b)
	return a != "" && b != "" && a != b
}

type trackFields struct {
	language string
	fields   []string
}

func audioFields(tracks []library.AudioTrack) []trackFields {
	if tracks == nil {
		return nil
	}
	out := make([]trackFields, 0, len(tracks))
	for _, track := range tracks {
		out = append(out, trackFields{normalizedText(library.NormalizeLanguage(track.Language)), []string{
			normalizedText(track.Codec), knownField(track.Channels), knownField(track.BitrateBPS),
		}})
	}
	return out
}

func subtitleFields(tracks []library.SubtitleTrack) []trackFields {
	if tracks == nil {
		return nil
	}
	out := make([]trackFields, 0, len(tracks))
	for _, track := range tracks {
		out = append(out, trackFields{normalizedText(library.NormalizeLanguage(track.Language)), []string{
			normalizedText(track.Codec), knownField(track.Forced), knownField(track.Default), knownField(track.HearingImpaired),
		}})
	}
	return out
}

func knownField[T any](p *T) string {
	if p == nil {
		return ""
	}
	return fmt.Sprint(*p)
}

func tracksDiffer(a, b []trackFields) bool {
	if a == nil || b == nil {
		return false
	}
	left, right := groupTracks(a), groupTracks(b)
	if _, leftUnknown := left[""]; !leftUnknown {
		if _, rightUnknown := right[""]; !rightUnknown && !maps.EqualFunc(left, right, func(_, _ [][]string) bool { return true }) {
			return true
		}
	}
	for language, fields := range left {
		other := right[language]
		if len(other) == 0 || (language == "" && (len(fields) != 1 || len(other) != 1)) {
			continue
		}
		var comparableFields []int
		for index := range fields[0] {
			unknown := func(track []string) bool { return track[index] == "" }
			if !slices.ContainsFunc(fields, unknown) && !slices.ContainsFunc(other, unknown) {
				comparableFields = append(comparableFields, index)
			}
		}
		if len(comparableFields) > 0 && !maps.Equal(trackValues(fields, comparableFields), trackValues(other, comparableFields)) {
			return true
		}
	}
	return false
}

func groupTracks(tracks []trackFields) map[string][][]string {
	groups := make(map[string][][]string)
	for _, track := range tracks {
		groups[track.language] = append(groups[track.language], track.fields)
	}
	return groups
}

func trackValues(tracks [][]string, indices []int) map[string]struct{} {
	values := make(map[string]struct{}, len(tracks))
	for _, track := range tracks {
		fields := make([]string, 0, len(indices))
		for _, index := range indices {
			fields = append(fields, track[index])
		}
		tuple, _ := json.Marshal(fields)
		values[string(tuple)] = struct{}{}
	}
	return values
}

func fileRevision(file MediaFile) string {
	if file.Adapter != "jellyfin" && file.Adapter != "plex" {
		return fmt.Sprintf("%d:%d", file.Size, file.Modified.UnixMicro())
	}
	var profile library.TechnicalProfile
	if file.Technical != nil {
		profile = *file.Technical
		profile.ObservedAt = time.Time{}
		profile.Audio = sortedTracks(profile.Audio)
		profile.Subtitles = sortedTracks(profile.Subtitles)
	}
	data, _ := json.Marshal(struct {
		ProviderRevision string
		Profile          library.TechnicalProfile
	}{file.providerRevision, profile})
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func sortedTracks[T any](tracks []T) []T {
	ordered := slices.Clone(tracks)
	slices.SortFunc(ordered, func(a, b T) int {
		left, _ := json.Marshal(a)
		right, _ := json.Marshal(b)
		return strings.Compare(string(left), string(right))
	})
	return ordered
}
