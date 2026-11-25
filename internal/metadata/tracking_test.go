// SPDX-License-Identifier: Elastic-2.0
package metadata

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type trackingRoundTrip func(*http.Request) (*http.Response, error)

func (f trackingRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestResolveTrackingLinks(t *testing.T) {
	const show = `{"id":44933,"name":"Severance","premiered":"2022-02-18","externals":{"thetvdb":371980}}`
	client := &http.Client{Transport: trackingRoundTrip(func(r *http.Request) (*http.Response, error) {
		responses := map[string]string{
			"api.tvmaze.com/search/shows?q=severance":                                                        `[{"show":` + show + `}]`,
			"api.tvmaze.com/shows/44933":                                                                     show,
			"api.tvmaze.com/lookup/shows?thetvdb=371980":                                                     show,
			"api.tvmaze.com/episodes/2302265?embed=show":                                                     `{"id":2302265,"season":1,"number":9,"_embedded":{"show":` + show + `}}`,
			"api.tvmaze.com/seasons/102615/episodes?embed=show":                                              `[{"id":8,"season":1,"number":1,"_embedded":{"show":` + show + `}}]`,
			"www.youtube.com/oembed?format=json&url=https%3A%2F%2Fwww.youtube.com%2Fwatch%3Fv%3DYE7VzlLtp-4": `{"title":"Big Buck Bunny"}`,
		}
		body := responses[r.URL.Host+r.URL.RequestURI()]
		if body == "" {
			t.Fatalf("unexpected URL: %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	p := TVmaze{BaseURL: "https://api.tvmaze.com", HTTP: client}
	tests := []struct {
		link, id, name  string
		season, episode int
	}{
		{"https://thetvdb.com/index.php/series/severance", "44933", "Severance", 0, 0},
		{"https://thetvdb.com/series/371980", "44933", "Severance", 0, 0},
		{"https://www.tvmaze.com/shows/44933/severance", "44933", "Severance", 0, 0},
		{"https://www.tvmaze.com/episodes/2302265/severance-1x09-the-we-we-are", "44933", "Severance", 1, 9},
		{"https://www.tvmaze.com/seasons/102615/severance-season-1", "44933", "Severance", 1, 0},
		{"https://youtu.be/YE7VzlLtp-4?si=discard", "YE7VzlLtp-4", "Big Buck Bunny", 0, 0},
	}
	for _, test := range tests {
		t.Run(test.link, func(t *testing.T) {
			v, err := p.ResolveTracking(context.Background(), test.link)
			if err != nil || v.Title.ID != test.id || v.Title.Name != test.name || v.SourceURL == "" {
				t.Fatalf("resolution: %+v, %v", v, err)
			}
			if test.season != 0 && (v.Season == nil || *v.Season != test.season) || test.season == 0 && v.Season != nil {
				t.Fatalf("season = %v, expected %d", v.Season, test.season)
			}
			if test.episode != 0 && (v.Episode == nil || *v.Episode != test.episode) || test.episode == 0 && v.Episode != nil {
				t.Fatalf("episode = %v, expected %d", v.Episode, test.episode)
			}
		})
	}
}

func TestTrackingUnsafeAndUnavailableLinks(t *testing.T) {
	p := TVmaze{
		BaseURL: "https://api.tvmaze.com",
		HTTP: &http.Client{Transport: trackingRoundTrip(func(r *http.Request) (*http.Response, error) {
			if r.URL.Host != "www.youtube.com" {
				t.Fatalf("unexpected request to %s", r.URL.Host)
			}
			return nil, errors.New("offline")
		})},
	}
	for _, link := range []string{
		"https://example.org/unknown", "https://www.youtube.com/watch?v=YE7VzlLtp-4",
	} {
		v, err := p.ResolveTracking(context.Background(), link)
		if err == nil || v.SourceURL != link || v.Title.Name != "" {
			t.Fatalf("expected manual fallback for %s: %+v, %v", link, v, err)
		}
	}
	for _, link := range []string{
		"javascript:alert(1)", "https://user@tvmaze.com/shows/44933", "http://127.0.0.1/",
		"https://localhost/example.org", "https://tvmaze.com:443/shows/44933",
	} {
		v, err := p.ResolveTracking(context.Background(), link)
		if err == nil || v.SourceURL != "" {
			t.Fatalf("accepted invalid link %s: %+v, %v", link, v, err)
		}
	}
}

func TestTrackingSeasonCompletionMetadata(t *testing.T) {
	p := TVmaze{
		BaseURL: "https://api.tvmaze.com",
		HTTP: &http.Client{Transport: trackingRoundTrip(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() != "https://api.tvmaze.com/shows/44933/seasons" {
				t.Fatalf("unexpected URL: %s", r.URL)
			}
			const response = `[{"number":1,"name":"Example season","episodeOrder":9,"endDate":"2022-04-08"},{"number":3,"episodeOrder":null,"endDate":null}]`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response))}, nil
		})},
	}
	seasons, err := p.Seasons(context.Background(), "44933")
	if err != nil || len(seasons) != 2 ||
		seasons[0] != (TrackingSeason{Number: 1, Name: "Example season", EpisodeOrder: 9, EndDate: "2022-04-08"}) ||
		seasons[1] != (TrackingSeason{Number: 3}) {
		t.Fatalf("season metadata: %+v, %v", seasons, err)
	}
}

func TestTrackingTVDBSlugDoesNotChooseBetweenRemakes(t *testing.T) {
	p := TVmaze{
		BaseURL: "https://api.tvmaze.com",
		HTTP: &http.Client{Transport: trackingRoundTrip(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() != "https://api.tvmaze.com/search/shows?q=another+show" {
				t.Fatalf("unexpected URL: %s", r.URL)
			}
			const response = `[{"show":{"id":5,"name":"Another Show","externals":{"thetvdb":11}}},{"show":{"id":6,"name":"Another Show","externals":{"thetvdb":12}}}]`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response))}, nil
		})},
	}
	const link = "https://thetvdb.com/series/another-show"
	result, err := p.ResolveTracking(context.Background(), link)
	if err == nil || result.SourceURL != link || result.Title.ID != "" {
		t.Fatalf("ambiguous result: %+v, %v", result, err)
	}
}
