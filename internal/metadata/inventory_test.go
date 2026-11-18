package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJellyfinInventoryIncludesActualEpisodes(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Error("mutating request")
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("IncludeItemTypes") == "Episode" {
			w.Write([]byte(`{"TotalRecordCount":1,"Items":[{"Id":"ep","Name":"Pilot","Path":"/shows/Show/S01E02.mkv","ParentIndexNumber":1,"IndexNumber":2}]}`))
			return
		}
		w.Write([]byte(`{"TotalRecordCount":1,"Items":[{"Id":"show","Name":"Show","Type":"Series","ProviderIds":{"Tvdb":"7"}}]}`))
	}))
	defer api.Close()
	p := MediaServer{Kind: "jellyfin", BaseURL: api.URL, Token: "fixture"}
	items := []InventoryItem{}
	if e := p.Inventory(context.Background(), func(i InventoryItem) error { items = append(items, i); return nil }); e != nil {
		t.Fatal(e)
	}
	if len(items) != 1 || items[0].Title.ID != "show" || items[0].Season != 1 || len(items[0].Episodes) != 1 || items[0].Episodes[0] != 2 {
		t.Fatal(items)
	}
}
func TestPlexInventoryIncludesActualEpisodes(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/library/sections":
			w.Write([]byte(`{"MediaContainer":{"Directory":[{"key":"1","type":"show"}]}}`))
		case "/library/sections/1/all":
			w.Write([]byte(`{"MediaContainer":{"totalSize":1,"Metadata":[{"ratingKey":"show","title":"Show","type":"show"}]}}`))
		case "/library/metadata/show/allLeaves":
			w.Write([]byte(`{"MediaContainer":{"totalSize":1,"Metadata":[{"ratingKey":"ep","parentIndex":2,"index":3,"Media":[{"Part":[{"file":"/show/s2e3.mkv"}]}]}]}}`))
		default:
			t.Error(r.URL.Path)
			http.Error(w, "wrong path", 404)
		}
	}))
	defer api.Close()
	p := MediaServer{Kind: "plex", BaseURL: api.URL, Token: "fixture"}
	var got InventoryItem
	if e := p.Inventory(context.Background(), func(i InventoryItem) error { got = i; return nil }); e != nil {
		t.Fatal(e)
	}
	if got.Season != 2 || len(got.Episodes) != 1 || got.Episodes[0] != 3 {
		t.Fatal(got)
	}
}

func TestMissingSeasonIsNotConfirmedSeasonZero(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("IncludeItemTypes") == "Episode" {
			w.Write([]byte(`{"TotalRecordCount":1,"Items":[{"Id":"ep","IndexNumber":1}]}`))
			return
		}
		w.Write([]byte(`{"TotalRecordCount":1,"Items":[{"Id":"show","Name":"Show","Type":"Series"}]}`))
	}))
	defer api.Close()
	p := MediaServer{Kind: "jellyfin", BaseURL: api.URL, Token: "fixture"}
	if e := p.Inventory(context.Background(), func(v InventoryItem) error {
		if v.SeasonKnown {
			t.Fatal("missing season confirmed")
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
}
