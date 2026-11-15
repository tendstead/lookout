// SPDX-License-Identifier: Elastic-2.0
package metadata

import (
	"context"
	"errors"
	"net/url"
	"strconv"
)

type InventoryItem struct {
	Title     ServerItem
	Key, Path string
	Season    int
	Episodes  []int
}

// Inventory reports real movie/episode entries, never inferring that a series
// record means all of its seasons exist. Metadata and media bodies are not read.
func (p *MediaServer) Inventory(ctx context.Context, emit func(InventoryItem) error) error {
	return p.Titles(ctx, func(t ServerItem) error {
		if t.Kind == "movie" {
			path := ""
			if len(t.Paths) > 0 {
				path = t.Paths[0]
			}
			return emit(InventoryItem{Title: t, Key: t.ID, Path: path})
		}
		seen := map[string]bool{}
		for offset := 0; offset < 100000; {
			type entry struct {
				ID          string
				Path        string
				Season      int
				Number, End *int
			}
			entries := []entry{}
			total := 0
			if p.Kind == "jellyfin" {
				q := url.Values{"ParentId": {t.ID}, "Recursive": {"true"}, "IncludeItemTypes": {"Episode"}, "Fields": {"Path"}, "StartIndex": {strconv.Itoa(offset)}, "Limit": {"200"}}
				var response struct {
					Total int `json:"TotalRecordCount"`
					Items []struct {
						ID     string `json:"Id"`
						Path   string `json:"Path"`
						Season int    `json:"ParentIndexNumber"`
						Number *int   `json:"IndexNumber"`
						End    *int   `json:"IndexNumberEnd"`
					} `json:"Items"`
				}
				if e := readJSON(ctx, p.client(), p.BaseURL+"/Items?"+q.Encode(), p.headers(), &response); e != nil {
					return e
				}
				total = response.Total
				for _, v := range response.Items {
					entries = append(entries, entry{v.ID, v.Path, v.Season, v.Number, v.End})
				}
			} else {
				q := url.Values{"X-Plex-Container-Start": {strconv.Itoa(offset)}, "X-Plex-Container-Size": {"200"}}
				var response struct {
					Container struct {
						Total int `json:"totalSize"`
						Items []struct {
							ID     string `json:"ratingKey"`
							Season int    `json:"parentIndex"`
							Number *int   `json:"index"`
							Media  []struct {
								Parts []struct {
									File string `json:"file"`
								} `json:"Part"`
							} `json:"Media"`
						} `json:"Metadata"`
					} `json:"MediaContainer"`
				}
				if e := readJSON(ctx, p.client(), p.BaseURL+"/library/metadata/"+url.PathEscape(t.ID)+"/allLeaves?"+q.Encode(), p.headers(), &response); e != nil {
					return e
				}
				total = response.Container.Total
				for _, v := range response.Container.Items {
					path := ""
					for _, m := range v.Media {
						for _, part := range m.Parts {
							if path == "" {
								path = part.File
							}
						}
					}
					entries = append(entries, entry{ID: v.ID, Path: path, Season: v.Season, Number: v.Number})
				}
			}
			if len(entries) == 0 {
				if offset < total {
					return errors.New("incomplete episode inventory")
				}
				return nil
			}
			for _, v := range entries {
				if v.ID == "" || seen[v.ID] {
					return errors.New("invalid or repeated episode page")
				}
				seen[v.ID] = true
				numbers := []int{}
				if v.Number != nil && *v.Number > 0 {
					end := *v.Number
					if v.End != nil && *v.End >= end && *v.End-end < 100 {
						end = *v.End
					}
					for n := *v.Number; n <= end; n++ {
						numbers = append(numbers, n)
					}
				}
				if e := emit(InventoryItem{Title: t, Key: v.ID, Path: v.Path, Season: v.Season, Episodes: numbers}); e != nil {
					return e
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
