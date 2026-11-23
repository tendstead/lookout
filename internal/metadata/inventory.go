// SPDX-License-Identifier: Elastic-2.0
package metadata

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/tendstead/lookout/internal/library"
)

type InventoryItem struct {
	Title                        ServerItem
	Key, ItemKey, Path, Revision string
	Technical                    *library.TechnicalProfile
	SeasonKnown                  bool
	Season                       int
	Episodes                     []int
}

const mediaInventoryPageSize = 100

type inventoryCopy struct {
	path, identityKind, identity, revision string
	profile                                *library.TechnicalProfile
}

func (i InventoryItem) emitCopies(copies []inventoryCopy, observedAt time.Time, emit func(InventoryItem) error) error {
	if len(copies) == 0 {
		return emit(i)
	}
	keys := make(map[string]bool, len(copies))
	for _, copy := range copies {
		item := i
		item.Path = copy.path
		item.Revision = copy.revision
		item.Technical = copy.profile
		if copy.profile != nil {
			copy.profile.ObservedAt = observedAt
		}
		if copy.identity != "" {
			digest := sha256.Sum256([]byte(fmt.Sprintf("%q:%q:%q", i.ItemKey, copy.identityKind, copy.identity)))
			item.Key = fmt.Sprintf("copy:%x", digest)
		}
		if keys[item.Key] {
			continue
		}
		keys[item.Key] = true
		if err := emit(item); err != nil {
			return err
		}
	}
	return nil
}

type episodeInventoryEntry struct {
	id, path            string
	season, number, end *int
	copies              []inventoryCopy
}

// Inventory reports actual movie files, episode files and item-level availability.
func (p *MediaServer) Inventory(ctx context.Context, emit func(InventoryItem) error) error {
	return p.InventoryWithTitles(ctx, nil, emit)
}

// InventoryWithTitles emits each title before its copies, including series with
// no episodes. Pass nil for emitTitle to receive only inventory observations.
func (p *MediaServer) InventoryWithTitles(ctx context.Context, emitTitle func(ServerItem) error, emit func(InventoryItem) error) error {
	observedAt := time.Now().UTC()
	return p.titles(ctx, true, func(t ServerItem, copies []inventoryCopy) error {
		if t.ID == "" {
			return errors.New("media server item is missing an identifier")
		}
		if emitTitle != nil {
			if err := emitTitle(t); err != nil {
				return err
			}
		}
		if t.Kind == "movie" {
			item := InventoryItem{Title: t, Key: t.ID, ItemKey: t.ID}
			if len(t.Paths) > 0 {
				item.Path = t.Paths[0]
			}
			return item.emitCopies(copies, observedAt, emit)
		}
		seen := map[string]bool{}
		for offset := 0; offset < 100000; {
			entries, total, err := p.episodeInventoryPage(ctx, t.ID, offset)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				if offset < total {
					return errors.New("incomplete episode inventory")
				}
				return nil
			}
			for _, v := range entries {
				if v.id == "" || seen[v.id] {
					return errors.New("invalid or repeated episode page")
				}
				seen[v.id] = true
				numbers := []int{}
				if v.number != nil && *v.number > 0 {
					numbers = append(numbers, *v.number)
					if v.end != nil && *v.end > *v.number && *v.end-*v.number < 100 {
						for n := 1; n <= *v.end-*v.number; n++ {
							numbers = append(numbers, *v.number+n)
						}
					}
				}
				item := InventoryItem{Title: t, Key: v.id, ItemKey: v.id, Path: v.path, Episodes: numbers}
				if v.season != nil && *v.season >= 0 {
					item.Season = *v.season
					item.SeasonKnown = true
				}
				if err := item.emitCopies(v.copies, observedAt, emit); err != nil {
					return err
				}
			}
			offset += len(entries)
			if offset >= total {
				return nil
			}
		}
		return errors.New("episode inventory exceeds limit")
	})
}

func (p *MediaServer) episodeInventoryPage(ctx context.Context, seriesID string, offset int) ([]episodeInventoryEntry, int, error) {
	if p.Kind == "jellyfin" {
		q := url.Values{"ParentId": {seriesID}, "Recursive": {"true"}, "IncludeItemTypes": {"Episode"}, "Fields": {"Path,MediaSources,MediaStreams"}, "StartIndex": {strconv.Itoa(offset)}, "Limit": {strconv.Itoa(mediaInventoryPageSize)}}
		var page jellyfinItemPage
		if err := readJSON(ctx, p.client(), p.BaseURL+"/Items?"+q.Encode(), p.headers(), &page); err != nil {
			return nil, 0, err
		}
		entries := make([]episodeInventoryEntry, 0, len(page.Items))
		for _, item := range page.Items {
			copies, err := p.jellyfinCopies(ctx, item)
			if err != nil {
				return nil, 0, err
			}
			entries = append(entries, episodeInventoryEntry{id: item.ID, path: item.Path, season: item.Season, number: item.Number, end: item.End, copies: copies})
		}
		return entries, page.Total, nil
	}
	q := url.Values{"X-Plex-Container-Start": {strconv.Itoa(offset)}, "X-Plex-Container-Size": {"200"}}
	var page plexItemPage
	if err := readJSON(ctx, p.client(), p.BaseURL+"/library/metadata/"+url.PathEscape(seriesID)+"/allLeaves?"+q.Encode(), p.headers(), &page); err != nil {
		return nil, 0, err
	}
	p.fillPlexPartDetails(ctx, page.Container.Items)
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	entries := make([]episodeInventoryEntry, 0, len(page.Container.Items))
	for _, item := range page.Container.Items {
		entries = append(entries, episodeInventoryEntry{id: item.ID, season: item.Season, number: item.Number, copies: item.copies()})
	}
	return entries, page.Container.Total, nil
}

// SuggestedName is a best-effort display hint. It never replaces a user label.
func (p *MediaServer) SuggestedName(ctx context.Context) string {
	if p.Kind == "jellyfin" {
		var v struct {
			Name string `json:"ServerName"`
		}
		if readJSON(ctx, p.client(), p.BaseURL+"/System/Info", p.headers(), &v) == nil && v.Name != "" {
			return v.Name
		}
	}
	if p.Kind == "plex" {
		var v struct {
			Container struct {
				Name string `json:"friendlyName"`
			} `json:"MediaContainer"`
		}
		if readJSON(ctx, p.client(), p.BaseURL+"/", p.headers(), &v) == nil && v.Container.Name != "" {
			return v.Container.Name
		}
	}
	u, e := url.Parse(p.BaseURL)
	if e == nil {
		return u.Hostname()
	}
	return p.Kind
}

func firstKnown[T comparable](first, second T) T {
	var zero T
	if first != zero {
		return first
	}
	return second
}

func positive[T int | int64 | float64](value *T) *T {
	if value != nil && *value > 0 {
		return value
	}
	return nil
}

func nonnegative(value *int64) *int64 {
	if value != nil && *value >= 0 {
		return value
	}
	return nil
}
