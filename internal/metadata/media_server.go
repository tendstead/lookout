// SPDX-License-Identifier: Elastic-2.0
package metadata

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type ServerItem struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Kind        string            `json:"kind"`
	Year        int               `json:"year"`
	Overview    string            `json:"overview"`
	ProviderIDs map[string]string `json:"provider_ids"`
	Paths       []string          `json:"paths"`
	ImagePath   string            `json:"image_path"`
}
type MediaServer struct {
	Kind     string        `json:"kind"`
	BaseURL  string        `json:"url"`
	Token    string        `json:"token"`
	Mappings []PathMapping `json:"path_mappings"`
	PageSize int           `json:"-"`
	HTTP     *http.Client  `json:"-"`
}

func (p *MediaServer) client() *http.Client {
	if p.HTTP != nil {
		return p.HTTP
	}
	return &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 2 || req.URL.Host != via[0].URL.Host || req.URL.Scheme != via[0].URL.Scheme {
			return errors.New("cross-origin redirect rejected")
		}
		return nil
	}}
}
func (p *MediaServer) headers() map[string]string {
	if p.Kind == "jellyfin" {
		return map[string]string{"Authorization": `MediaBrowser Client="Lookout", Device="Lookout", DeviceId="lookout-local", Version="0.1.0", Token="` + strings.TrimSpace(p.Token) + `"`}
	}
	return map[string]string{"X-Plex-Token": strings.TrimSpace(p.Token), "X-Plex-Client-Identifier": "tendstead-lookout", "X-Plex-Product": "Tendstead Lookout"}
}
func (p *MediaServer) Validate() error {
	base, e := validateBaseURL(p.BaseURL)
	if e != nil {
		return e
	}
	p.BaseURL = base
	if p.Kind != "jellyfin" && p.Kind != "plex" {
		return errors.New("choose Jellyfin or Plex")
	}
	if strings.TrimSpace(p.Token) == "" || strings.ContainsAny(p.Token, "\r\n\"") {
		return errors.New("invalid media-server token")
	}
	return nil
}
func (p *MediaServer) Titles(ctx context.Context, emit func(ServerItem) error) error {
	if e := p.Validate(); e != nil {
		return e
	}
	if p.Kind == "jellyfin" {
		return p.jellyfin(ctx, emit)
	}
	return p.plex(ctx, emit)
}
func (p *MediaServer) Verify(ctx context.Context) error {
	if e := p.Validate(); e != nil {
		return e
	}
	endpoint := "/System/Info"
	if p.Kind == "plex" {
		endpoint = "/library/sections"
	}
	var v map[string]any
	return readJSON(ctx, p.client(), p.BaseURL+endpoint, p.headers(), &v)
}
func normalizedIDs(ids map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range ids {
		switch strings.ToLower(key) {
		case "tvdb", "tmdb", "imdb", "anidb", "tvmaze":
			if value != "" {
				out[strings.ToLower(key)] = value
			}
		}
	}
	return out
}
func (p *MediaServer) jellyfin(ctx context.Context, emit func(ServerItem) error) error {
	limit := p.PageSize
	if limit < 1 || limit > 200 {
		limit = 200
	}
	seen := map[string]bool{}
	for offset := 0; offset < 100000; {
		q := url.Values{"Recursive": {"true"}, "IncludeItemTypes": {"Movie,Series"}, "Fields": {"ProviderIds,Path,Overview,Genres"}, "StartIndex": {strconv.Itoa(offset)}, "Limit": {strconv.Itoa(limit)}}
		var response struct {
			Total int `json:"TotalRecordCount"`
			Items []struct {
				ID           string            `json:"Id"`
				Name         string            `json:"Name"`
				Type         string            `json:"Type"`
				Path         string            `json:"Path"`
				Year         int               `json:"ProductionYear"`
				Overview     string            `json:"Overview"`
				IDs          map[string]string `json:"ProviderIds"`
				Images       map[string]string `json:"ImageTags"`
				MediaSources []struct {
					Path string `json:"Path"`
				} `json:"MediaSources"`
			} `json:"Items"`
		}
		if e := readJSON(ctx, p.client(), p.BaseURL+"/Items?"+q.Encode(), p.headers(), &response); e != nil {
			return e
		}
		if len(response.Items) == 0 {
			if offset < response.Total {
				return errors.New("incomplete media-server page")
			}
			return nil
		}
		for _, i := range response.Items {
			if seen[i.ID] {
				return errors.New("media server repeated a page")
			}
			seen[i.ID] = true
			paths := []string{}
			if i.Path != "" {
				paths = append(paths, i.Path)
			}
			for _, source := range i.MediaSources {
				if source.Path != "" && source.Path != i.Path {
					paths = append(paths, source.Path)
				}
			}
			image := ""
			if i.Images["Primary"] != "" {
				image = "/Items/" + url.PathEscape(i.ID) + "/Images/Primary?maxWidth=300&quality=85"
			}
			kind := "movie"
			if i.Type == "Series" {
				kind = "series"
			}
			if e := emit(ServerItem{ID: i.ID, Name: i.Name, Kind: kind, Year: i.Year, Overview: i.Overview, ProviderIDs: normalizedIDs(i.IDs), Paths: paths, ImagePath: image}); e != nil {
				return e
			}
		}
		offset += len(response.Items)
		if offset >= response.Total {
			return nil
		}
	}
	return errors.New("media-server item limit reached")
}
func (p *MediaServer) plex(ctx context.Context, emit func(ServerItem) error) error {
	var sections struct {
		Container struct {
			Directories []struct {
				Key  string `json:"key"`
				Type string `json:"type"`
			} `json:"Directory"`
		} `json:"MediaContainer"`
	}
	if e := readJSON(ctx, p.client(), p.BaseURL+"/library/sections", p.headers(), &sections); e != nil {
		return e
	}
	for _, section := range sections.Container.Directories {
		if section.Type != "movie" && section.Type != "show" {
			continue
		}
		for offset := 0; offset < 100000; {
			q := url.Values{"X-Plex-Container-Start": {strconv.Itoa(offset)}, "X-Plex-Container-Size": {"200"}, "includeGuids": {"1"}}
			var response struct {
				Container struct {
					Total int `json:"totalSize"`
					Size  int `json:"size"`
					Items []struct {
						ID      string `json:"ratingKey"`
						Name    string `json:"title"`
						Type    string `json:"type"`
						Year    int    `json:"year"`
						Summary string `json:"summary"`
						Thumb   string `json:"thumb"`
						GUIDs   []struct {
							ID string `json:"id"`
						} `json:"Guid"`
						Locations []struct {
							Path string `json:"path"`
						} `json:"Location"`
						Media []struct {
							Parts []struct {
								File string `json:"file"`
							} `json:"Part"`
						} `json:"Media"`
					} `json:"Metadata"`
				} `json:"MediaContainer"`
			}
			if e := readJSON(ctx, p.client(), p.BaseURL+"/library/sections/"+url.PathEscape(section.Key)+"/all?"+q.Encode(), p.headers(), &response); e != nil {
				return e
			}
			if len(response.Container.Items) == 0 {
				if offset < response.Container.Total {
					return errors.New("incomplete Plex page")
				}
				break
			}
			for _, i := range response.Container.Items {
				ids := map[string]string{}
				for _, g := range i.GUIDs {
					parts := strings.SplitN(g.ID, "://", 2)
					if len(parts) != 2 {
						continue
					}
					switch parts[0] {
					case "tvdb", "tmdb", "imdb":
						ids[parts[0]] = strings.Split(parts[1], "?")[0]
					}
				}
				paths := []string{}
				for _, loc := range i.Locations {
					paths = append(paths, loc.Path)
				}
				for _, media := range i.Media {
					for _, part := range media.Parts {
						if part.File != "" {
							paths = append(paths, part.File)
						}
					}
				}
				kind := "movie"
				if i.Type == "show" {
					kind = "series"
				}
				image := ""
				if strings.HasPrefix(i.Thumb, "/library/metadata/") {
					image = i.Thumb
				}
				if e := emit(ServerItem{ID: i.ID, Name: i.Name, Kind: kind, Year: i.Year, Overview: i.Summary, ProviderIDs: ids, Paths: paths, ImagePath: image}); e != nil {
					return e
				}
			}
			offset += len(response.Container.Items)
			if offset >= response.Container.Total {
				break
			}
		}
	}
	return nil
}
