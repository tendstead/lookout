// SPDX-License-Identifier: Elastic-2.0
package metadata

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestTrackingArtworkUsesFixedProvidersAndSniffsImages(t *testing.T) {
	const jpeg = "\xff\xd8\xff\xe0example image"
	p := TVmaze{BaseURL: "https://api.tvmaze.com", HTTP: &http.Client{
		Transport: trackingRoundTrip(func(r *http.Request) (*http.Response, error) {
			var body string
			switch r.URL.String() {
			case "https://api.tvmaze.com/shows/44933":
				body = `{"id":44933,"name":"Severance","image":{"medium":"https://static.tvmaze.com/uploads/images/medium_portrait/1/2.jpg"}}`
			case "https://static.tvmaze.com/uploads/images/medium_portrait/1/2.jpg", "https://i.ytimg.com/vi/YE7VzlLtp-4/hqdefault.jpg":
				body = jpeg
			case "https://i.ytimg.com/vi/11111111111/hqdefault.jpg":
				body = "<svg></svg>"
			default:
				t.Fatalf("unexpected request to %s", r.URL)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
		}),
	}}
	for provider, id := range map[string]string{"tvmaze": "44933", "youtube": "YE7VzlLtp-4"} {
		data, mime, err := p.TrackingArtwork(context.Background(), provider, id)
		if err != nil || string(data) != jpeg || mime != "image/jpeg" {
			t.Fatalf("%s image: %q %q %v", provider, data, mime, err)
		}
	}
	for _, v := range [][2]string{{"youtube", "11111111111"}, {"youtube", "../hq.jpg"}, {"tvmaze", "44933/art"}, {"other", "44933"}} {
		if _, _, err := p.TrackingArtwork(context.Background(), v[0], v[1]); err == nil {
			t.Fatalf("accepted invalid image %v", v)
		}
	}
}

func TestTrackingArtworkRejectsOffProviderRedirect(t *testing.T) {
	requests := 0
	p := TVmaze{HTTP: &http.Client{Transport: trackingRoundTrip(func(*http.Request) (*http.Response, error) {
		requests++
		return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"https://example.org/picture.jpg"}}, Body: http.NoBody}, nil
	})}}
	if _, _, err := p.TrackingArtwork(context.Background(), "youtube", "YE7VzlLtp-4"); err == nil || requests != 1 {
		t.Fatalf("redirect requests=%d, error=%v", requests, err)
	}
}
