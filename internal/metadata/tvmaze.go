// SPDX-License-Identifier: Elastic-2.0
package metadata

import (
	"context"
	"errors"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type TVmaze struct {
	BaseURL       string
	HTTP          *http.Client
	Interval      time.Duration
	TrackingMedia func(context.Context, string) (Candidate, error)
	mu            sync.Mutex
	next          time.Time
}

func NewTVmaze() *TVmaze {
	return &TVmaze{BaseURL: "https://api.tvmaze.com", HTTP: &http.Client{Timeout: 15 * time.Second}, Interval: 600 * time.Millisecond}
}
func (p *TVmaze) get(ctx context.Context, endpoint string, out any) error {
	if p.Interval > 0 {
		p.mu.Lock()
		wait := time.Until(p.next)
		if wait < 0 {
			wait = 0
		}
		p.next = time.Now().Add(wait + p.Interval)
		p.mu.Unlock()
		if wait > 0 {
			timer := time.NewTimer(wait)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	return readJSON(ctx, p.HTTP, strings.TrimRight(p.BaseURL, "/")+endpoint, nil, out)
}

type mazeShow struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Premiered string `json:"premiered"`
	Summary   string `json:"summary"`
	URL       string `json:"url"`
	Image     *struct {
		Medium   string `json:"medium"`
		Original string `json:"original"`
	} `json:"image"`
	Externals struct {
		TVDB int    `json:"thetvdb"`
		IMDB string `json:"imdb"`
	} `json:"externals"`
}

var markup = regexp.MustCompile(`<[^>]*>`)

func (p mazeShow) candidate() Candidate {
	ids := map[string]string{"tvmaze": strconv.Itoa(p.ID)}
	if p.Externals.TVDB > 0 {
		ids["tvdb"] = strconv.Itoa(p.Externals.TVDB)
	}
	if p.Externals.IMDB != "" {
		ids["imdb"] = p.Externals.IMDB
	}
	year := 0
	if len(p.Premiered) >= 4 {
		year, _ = strconv.Atoi(p.Premiered[:4])
	}
	image := ""
	if p.Image != nil {
		image = p.Image.Medium
	}
	return Candidate{Provider: "tvmaze", ID: strconv.Itoa(p.ID), Name: p.Name, Year: year, Overview: strings.TrimSpace(html.UnescapeString(markup.ReplaceAllString(p.Summary, ""))), ProviderIDs: ids, URL: p.URL, ImageURL: image}
}
func (p *TVmaze) Search(ctx context.Context, q string) ([]Candidate, error) {
	var response []struct {
		Show mazeShow `json:"show"`
	}
	if e := p.get(ctx, "/search/shows?q="+url.QueryEscape(q), &response); e != nil {
		return nil, e
	}
	out := []Candidate{}
	for _, r := range response {
		if r.Show.ID > 0 {
			out = append(out, r.Show.candidate())
		}
	}
	return out, nil
}
func (p *TVmaze) Show(ctx context.Context, id string) (Candidate, error) {
	n, e := strconv.Atoi(id)
	if e != nil || n <= 0 {
		return Candidate{}, errors.New("invalid TVmaze identity")
	}
	var show mazeShow
	if e = p.get(ctx, "/shows/"+strconv.Itoa(n), &show); e != nil {
		return Candidate{}, e
	}
	return show.candidate(), nil
}
func (p *TVmaze) Lookup(ctx context.Context, ids map[string]string) (Candidate, error) {
	if id := ids["tvmaze"]; id != "" {
		return p.Show(ctx, id)
	}
	query := url.Values{}
	if id := ids["tvdb"]; id != "" {
		query.Set("thetvdb", id)
	} else if id := ids["imdb"]; id != "" {
		query.Set("imdb", id)
	} else {
		return Candidate{}, errors.New("no supported stable provider identity")
	}
	var show mazeShow
	if e := p.get(ctx, "/lookup/shows?"+query.Encode(), &show); e != nil {
		return Candidate{}, e
	}
	c := show.candidate()
	for key, id := range ids {
		if key == "tvdb" || key == "imdb" {
			if c.ProviderIDs[key] != id {
				return Candidate{}, errors.New("provider returned a conflicting identity")
			}
		}
	}
	return c, nil
}
func (p *TVmaze) Episodes(ctx context.Context, id string) ([]Episode, error) {
	n, e := strconv.Atoi(id)
	if e != nil || n <= 0 {
		return nil, errors.New("invalid TVmaze identity")
	}
	var response []struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Season   int    `json:"season"`
		Number   *int   `json:"number"`
		AirDate  string `json:"airdate"`
		AirStamp string `json:"airstamp"`
		URL      string `json:"url"`
	}
	if e = p.get(ctx, "/shows/"+strconv.Itoa(n)+"/episodes?specials=1", &response); e != nil {
		return nil, e
	}
	out := []Episode{}
	for _, v := range response {
		if v.ID > 0 {
			out = append(out, Episode{ID: strconv.Itoa(v.ID), Name: v.Name, Season: v.Season, Number: v.Number, AirDate: v.AirDate, AirStamp: v.AirStamp, URL: v.URL})
		}
	}
	return out, nil
}
