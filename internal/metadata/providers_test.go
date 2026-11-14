package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTVmazeOfficialIdentityAndEpisodes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Fatal("credential sent to public provider")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/lookup/shows":
			if r.URL.Query().Get("thetvdb") != "123" {
				t.Error("wrong lookup")
			}
			w.Write([]byte(`{"id":9,"name":"Example Show","premiered":"2020-01-02","summary":"<p>Example &amp; description</p>","externals":{"thetvdb":123}}`))
		case "/shows/9/episodes":
			w.Write([]byte(`[{"id":10,"name":"Premiere","season":2,"number":1,"airstamp":"2026-11-15T20:00:00Z","airdate":"2026-11-15"},{"id":11,"name":"Special","season":0,"number":null,"airdate":""}]`))
		default:
			t.Error(r.URL.Path)
		}
	}))
	defer server.Close()
	p := TVmaze{BaseURL: server.URL, HTTP: server.Client()}
	c, e := p.Lookup(context.Background(), map[string]string{"tvdb": "123"})
	if e != nil || c.ID != "9" || c.ProviderIDs["tvdb"] != "123" || c.Overview != "Example & description" {
		t.Fatalf("%#v %v", c, e)
	}
	episodes, e := p.Episodes(context.Background(), "9")
	if e != nil || len(episodes) != 2 || episodes[1].Number != nil || episodes[1].AirDate != "" {
		t.Fatalf("%#v %v", episodes, e)
	}
}
func TestTVmazeAmbiguityAndFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "offline") {
			w.WriteHeader(429)
			return
		}
		w.Write([]byte(`[{"score":1,"show":{"id":1,"name":"Same Name","premiered":"2000-01-01"}},{"score":1,"show":{"id":2,"name":"Same Name","premiered":"2020-01-01"}}]`))
	}))
	defer server.Close()
	p := TVmaze{BaseURL: server.URL, HTTP: server.Client()}
	items, e := p.Search(context.Background(), "Same Name")
	if e != nil || len(items) != 2 {
		t.Fatal(e)
	}
	if _, ok := ExactCandidate(items, "Same Name", 0); ok {
		t.Fatal("ambiguous name auto matched")
	}
	if c, ok := ExactCandidate(items, "Same Name", 2020); !ok || c.ID != "2" {
		t.Fatal("year not respected")
	}
	if _, e = p.Search(context.Background(), "offline"); e == nil {
		t.Fatal("provider failure ignored")
	}
}
func TestJellyfinPagedReadOnlyMetadata(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Fatal("media server write")
		}
		if !strings.Contains(r.Header.Get("Authorization"), `Token="secret-test"`) {
			t.Fatal("missing auth header")
		}
		calls++
		if r.URL.Query().Get("StartIndex") == "0" {
			w.Write([]byte(`{"TotalRecordCount":2,"Items":[{"Id":"a","Name":"Show","Type":"Series","Path":"/data/Series/Show","ProviderIds":{"Tvdb":"123"},"Overview":"Text","ImageTags":{"Primary":"tag"}}]}`))
		} else {
			w.Write([]byte(`{"TotalRecordCount":2,"Items":[{"Id":"b","Name":"Film","Type":"Movie","Path":"/data/Movies/Film/film.mkv","ProviderIds":{"Tmdb":"456"}}]}`))
		}
	}))
	defer server.Close()
	p := MediaServer{Kind: "jellyfin", BaseURL: server.URL, Token: "secret-test", HTTP: server.Client(), PageSize: 1}
	var items []ServerItem
	e := p.Titles(context.Background(), func(i ServerItem) error { items = append(items, i); return nil })
	if e != nil || calls != 2 || len(items) != 2 || items[0].ProviderIDs["tvdb"] != "123" {
		t.Fatalf("%#v %v", items, e)
	}
}
func TestPlexMetadataReadOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.Header.Get("X-Plex-Token") != "secret-test" || r.Header.Get("Accept") != "application/json" {
			t.Fatal("bad Plex request")
		}
		if r.URL.Path == "/library/sections" {
			w.Write([]byte(`{"MediaContainer":{"Directory":[{"key":"1","type":"movie"}]}}`))
			return
		}
		w.Write([]byte(`{"MediaContainer":{"size":1,"totalSize":1,"Metadata":[{"ratingKey":"42","title":"Film","type":"movie","year":2020,"summary":"Text","Guid":[{"id":"tmdb://99"}],"Media":[{"Part":[{"file":"/data/Movies/Film/file.mkv"}]}]}]}}`))
	}))
	defer server.Close()
	p := MediaServer{Kind: "plex", BaseURL: server.URL, Token: "secret-test", HTTP: server.Client()}
	var items []ServerItem
	e := p.Titles(context.Background(), func(i ServerItem) error { items = append(items, i); return nil })
	if e != nil || len(items) != 1 || items[0].ProviderIDs["tmdb"] != "99" || items[0].Paths[0] != "/data/Movies/Film/file.mkv" {
		t.Fatalf("%#v %v", items, e)
	}
}
func TestExplicitPathMapping(t *testing.T) {
	m := []PathMapping{{ServerPrefix: "/data/Series", SourceID: "series", SourcePrefix: "/mnt/Movies/Serien"}}
	if root, rel, ok := MapPath("/data/Series/Show", m); !ok || root != "series" || rel != "Show" {
		t.Fatal(root, rel, ok)
	}
	for _, p := range []string{"/data/SeriesOther/Show", "/data/Series/../Secrets"} {
		if _, _, ok := MapPath(p, m); ok {
			t.Fatal("outside root matched")
		}
	}
}
func TestArtworkUsesHeadersAndRejectsOutsidePaths(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "secret-test") {
			t.Fatal("token leaked in image URL")
		}
		if !strings.Contains(r.Header.Get("Authorization"), "secret-test") {
			t.Fatal("missing image authorization")
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte{137, 80, 78, 71, 13, 10, 26, 10})
	}))
	defer server.Close()
	p := MediaServer{Kind: "jellyfin", BaseURL: server.URL, Token: "secret-test", HTTP: server.Client()}
	if _, _, e := p.Image(context.Background(), "/Items/id/Images/Primary"); e != nil {
		t.Fatal(e)
	}
	for _, v := range []string{"https://outside.invalid/steal", "//outside.invalid/steal", "/Items/../Secrets"} {
		if _, _, e := p.Image(context.Background(), v); e == nil {
			t.Fatal("unsafe artwork accepted")
		}
	}
}
