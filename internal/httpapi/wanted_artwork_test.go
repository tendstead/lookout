// SPDX-License-Identifier: Elastic-2.0
package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tendstead/lookout/internal/metadata"
	"github.com/tendstead/lookout/internal/store"
)

func TestWantedArtworkPreferenceAndServerScope(t *testing.T) {
	o := apiOptions{metadata: &metadataFixture{}, wantedPublic: true, wantedTV: &metadata.TVmaze{}}
	for _, test := range []struct {
		wanted store.Wanted
		url    string
	}{
		{store.Wanted{ArtworkTitleID: "catalogue", ProviderIDs: map[string]string{"tvmaze": "44933"}}, "/api/v1/titles/catalogue/artwork"},
		{store.Wanted{ProviderIDs: map[string]string{"tvmaze": "44933"}}, "/api/v1/wanted/artwork/tvmaze/44933"},
		{store.Wanted{SourceURL: "https://youtu.be/YE7VzlLtp-4"}, "/api/v1/wanted/artwork/youtube/YE7VzlLtp-4"},
		{store.Wanted{SourceURL: "https://example.org/film", ArtworkURL: "https://example.org/forged.jpg"}, ""},
		{store.Wanted{ServerID: "local", ProviderIDs: map[string]string{"tvmaze": "44933"}}, ""},
		{store.Wanted{ServerID: "local", ArtworkTitleID: "catalogue"}, "/api/v1/titles/catalogue/artwork"},
	} {
		o.populateWantedArtwork(&test.wanted)
		if test.wanted.ArtworkURL != test.url {
			t.Errorf("artwork %q; want %q", test.wanted.ArtworkURL, test.url)
		}
	}
}

type artworkRoundTrip func(*http.Request) (*http.Response, error)

func (f artworkRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestWantedArtworkRouteSupportsPublicMetadataAndCaching(t *testing.T) {
	const jpeg = "\xff\xd8\xff\xe0test image"
	var requests int
	tv := &metadata.TVmaze{HTTP: &http.Client{Transport: artworkRoundTrip(func(r *http.Request) (*http.Response, error) {
		requests++
		if r.URL.String() != "https://i.ytimg.com/vi/YE7VzlLtp-4/hqdefault.jpg" {
			t.Fatalf("unexpected image request %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(jpeg))}, nil
	})}}
	const path = "/api/v1/wanted/artwork/youtube/YE7VzlLtp-4"
	disabled := NewHandler(nil, nil, WithWantedTracking(trackingPreviewStore{}, tv, false, false))
	record := httptest.NewRecorder()
	disabled.ServeHTTP(record, httptest.NewRequest(http.MethodGet, path, nil))
	if record.Code != 404 || requests != 0 {
		t.Fatalf("public metadata disabled: %d, %d requests", record.Code, requests)
	}
	enabled := NewHandler(nil, nil, WithWantedTracking(trackingPreviewStore{}, tv, true, false))
	record = httptest.NewRecorder()
	enabled.ServeHTTP(record, httptest.NewRequest(http.MethodGet, path, nil))
	if record.Code != 200 || record.Body.String() != jpeg || record.Header().Get("Content-Type") != "image/jpeg" || record.Header().Get("ETag") == "" {
		t.Fatalf("image response: %d, %q, %s", record.Code, record.Body.String(), record.Header())
	}
	request := httptest.NewRequest(http.MethodHead, path, nil)
	request.Header.Set("If-None-Match", record.Header().Get("ETag"))
	record = httptest.NewRecorder()
	enabled.ServeHTTP(record, request)
	if record.Code != 304 || record.Body.Len() != 0 {
		t.Fatalf("conditional image HEAD: %d, %q", record.Code, record.Body.String())
	}
}
