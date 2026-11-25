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
		case "/library/metadata/ep":
			http.NotFound(w, r)
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

func TestJellyfinInventoryEmitsEachVersionOfAnEpisode(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("IncludeItemTypes") == "Episode" {
			w.Write([]byte(`{"TotalRecordCount":1,"Items":[{"Id":"ep","Path":"/show/hd.mkv","ParentIndexNumber":1,"IndexNumber":2,"IndexNumberEnd":3,"MediaSources":[{"Id":"hd","Path":"/show/hd.mkv","Size":1000,"MediaStreams":[{"Type":"Video","Codec":"h264","Width":1920,"Height":1080,"BitRate":8000000,"VideoRangeType":"DOVIWithHDR10"}]},{"Id":"uhd","Path":"/show/uhd.mkv","Size":2000,"MediaStreams":[{"Type":"Subtitle","Language":"pt-BR"}]}]}]}`))
			return
		}
		w.Write([]byte(`{"TotalRecordCount":1,"Items":[{"Id":"show","Name":"Show","Type":"Series"}]}`))
	}))
	defer api.Close()
	server := MediaServer{Kind: "jellyfin", BaseURL: api.URL, Token: "fixture"}
	var items []InventoryItem
	if err := server.Inventory(context.Background(), func(item InventoryItem) error {
		items = append(items, item)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("want two copy observations, got %d", len(items))
	}
	if items[0].Key == items[1].Key || len(items[0].Episodes) != 2 || len(items[1].Episodes) != 2 {
		t.Fatalf("version identity or episode coverage lost: %+v", items)
	}
	if tech := items[0].Technical; tech == nil || tech.HDR != "Dolby Vision / HDR10" || tech.VideoBitrateBPS == nil || *tech.VideoBitrateBPS != 8_000_000 {
		t.Fatalf("incorrect Jellyfin video details: %+v", tech)
	}
}

func TestPlexInventoryEmitsEachPartOfAMovie(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/library/sections":
			w.Write([]byte(`{"MediaContainer":{"Directory":[{"key":"1","type":"movie"}]}}`))
		case "/library/sections/1/all", "/library/metadata/film":
			w.Write([]byte(`{"MediaContainer":{"totalSize":1,"Metadata":[{"ratingKey":"film","title":"Film","type":"movie","Media":[{"id":1,"bitrate":9000,"Part":[{"id":10,"file":"/movies/part1.mkv","size":3000},{"id":11,"file":"/movies/part2.mkv","size":0,"Stream":[]}]}]}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()
	server := MediaServer{Kind: "plex", BaseURL: api.URL, Token: "fixture"}
	var items []InventoryItem
	if err := server.Inventory(context.Background(), func(item InventoryItem) error {
		items = append(items, item)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("want two part observations, got %d", len(items))
	}
	if items[0].Key == items[1].Key || items[1].Path != "/movies/part2.mkv" {
		t.Fatalf("part identity lost: %+v", items)
	}
	for index, item := range items {
		profile := item.Technical
		if profile == nil || profile.PartCount != 2 || profile.PartNumber != index+1 || profile.BitrateBPS != nil {
			t.Fatalf("incorrect part scope: %+v", profile)
		}
	}
	if size := items[1].Technical.SizeBytes; size == nil || *size != 0 {
		t.Fatalf("explicit zero size lost: %v", size)
	}
	if items[0].Technical.Audio != nil || items[0].Technical.Subtitles != nil || items[1].Technical.Audio == nil || items[1].Technical.Subtitles == nil {
		t.Fatalf("absent and explicitly empty tracks must differ: %+v and %+v", items[0].Technical, items[1].Technical)
	}
}

func TestPlexInventoryUsesItemDetailForMissingPartStreams(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/library/sections":
			w.Write([]byte(`{"MediaContainer":{"Directory":[{"key":"1","type":"movie"}]}}`))
		case "/library/sections/1/all":
			w.Write([]byte(`{"MediaContainer":{"totalSize":1,"Metadata":[{"ratingKey":"film","title":"Film","type":"movie","Media":[{"id":20,"bitrate":9000,"Part":[{"id":31,"file":"/movies/a.mkv","size":3000}]}]}]}}`))
		case "/library/metadata/film":
			w.Write([]byte(`{"MediaContainer":{"Metadata":[{"ratingKey":"film","type":"movie","editionTitle":"Director's Cut","Media":[{"id":20,"Part":[{"id":31,"Stream":[{"streamType":1,"bitrate":7500,"frameRate":"23.976","DOVIPresent":true},{"streamType":2,"languageTag":"pt-BR","channels":6,"bitrate":512},{"streamType":3,"codec":"srt","languageCode":"eng","forced":0}]}]}]}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()
	server := MediaServer{Kind: "plex", BaseURL: api.URL, Token: "fixture"}
	var got InventoryItem
	if err := server.Inventory(context.Background(), func(item InventoryItem) error {
		got = item
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got.Path != "/movies/a.mkv" || got.ItemKey != "film" || got.Key == "film" || got.Technical == nil {
		t.Fatalf("missing file identity or technical data: %+v", got)
	}
	profile := got.Technical
	if profile.BitrateBPS == nil || *profile.BitrateBPS != 9_000_000 || profile.SizeBytes == nil || *profile.SizeBytes != 3000 ||
		profile.VideoBitrateBPS == nil || *profile.VideoBitrateBPS != 7_500_000 || profile.FrameRate == nil || *profile.FrameRate != 23.976 ||
		profile.HDR != "Dolby Vision" || profile.Edition != "Director's Cut" {
		t.Fatalf("incorrect part details: %+v", profile)
	}
	if len(profile.Audio) != 1 || profile.Audio[0].Language != "pt-BR" || profile.Audio[0].BitrateBPS == nil ||
		*profile.Audio[0].BitrateBPS != 512_000 || len(profile.Subtitles) != 1 || profile.Subtitles[0].Language != "en" ||
		profile.Subtitles[0].Forced == nil || *profile.Subtitles[0].Forced || profile.Subtitles[0].Default != nil {
		t.Fatalf("incorrect audio or subtitle details: %+v %+v", profile.Audio, profile.Subtitles)
	}
}

func TestJellyfinInventoryCopyKeysSurviveReorderedVersions(t *testing.T) {
	reverse := false
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		first := `{"Id":"a","Path":"/movies/a.mkv","RunTimeTicks":123450000}`
		second := `{"Path":"/movies/b.mkv","Size":0,"MediaStreams":[]}`
		if reverse {
			first, second = second, `{"Id":"a","Path":"/movies/renamed-a.mkv","RunTimeTicks":123450000}`
		}
		w.Write([]byte(`{"TotalRecordCount":1,"Items":[{"Id":"film","Type":"Movie","MediaSources":[` + first + "," + second + `]}]}`))
	}))
	defer api.Close()
	server := MediaServer{Kind: "jellyfin", BaseURL: api.URL, Token: "fixture"}
	forward, backward := []InventoryItem{}, []InventoryItem{}
	if err := server.Inventory(context.Background(), func(i InventoryItem) error { forward = append(forward, i); return nil }); err != nil {
		t.Fatal(err)
	}
	reverse = true
	if err := server.Inventory(context.Background(), func(i InventoryItem) error { backward = append(backward, i); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(forward) != 2 || len(backward) != 2 || forward[0].Key != backward[1].Key || forward[1].Key != backward[0].Key ||
		forward[0].ItemKey != "film" || forward[0].Key == forward[1].Key {
		t.Fatalf("copy identities changed: %+v and %+v", forward, backward)
	}
	if tech := forward[0].Technical; tech == nil || tech.DurationMS == nil || *tech.DurationMS != 12345 || tech.VideoBitrateBPS != nil {
		t.Fatalf("incorrect duration or missing bitrate: %+v", tech)
	}
	if size := forward[1].Technical.SizeBytes; size == nil || *size != 0 {
		t.Fatalf("explicit zero size lost: %v", size)
	}
	if forward[0].Technical.Audio != nil || forward[0].Technical.Subtitles != nil || forward[1].Technical.Audio == nil || forward[1].Technical.Subtitles == nil {
		t.Fatalf("absent and explicitly empty tracks must differ: %+v and %+v", forward[0].Technical, forward[1].Technical)
	}
}

func TestInventoryWithTitlesReportsEmptySeries(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("IncludeItemTypes") == "Episode" {
			w.Write([]byte(`{"TotalRecordCount":0,"Items":[]}`))
			return
		}
		w.Write([]byte(`{"TotalRecordCount":1,"Items":[{"Id":"show","Name":"Show","Type":"Series","Path":"/shows/Show"}]}`))
	}))
	defer api.Close()
	server := MediaServer{Kind: "jellyfin", BaseURL: api.URL, Token: "fixture"}
	var titles []ServerItem
	if err := server.InventoryWithTitles(context.Background(), func(title ServerItem) error {
		titles = append(titles, title)
		return nil
	}, func(item InventoryItem) error {
		t.Fatalf("empty series published an episode: %+v", item)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(titles) != 1 || titles[0].ID != "show" || len(titles[0].Paths) != 1 || titles[0].Paths[0] != "/shows/Show" {
		t.Fatalf("empty series title unavailable: %+v", titles)
	}
}

func TestJellyfinInventoryEnumeratesAdditionalParts(t *testing.T) {
	for _, kind := range []string{"Movie", "Episode"} {
		t.Run(kind, func(t *testing.T) {
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("unexpected method %s", r.Method)
				}
				switch r.URL.Path {
				case "/Items":
					if kind == "Episode" && r.URL.Query().Get("IncludeItemTypes") != "Episode" {
						w.Write([]byte(`{"TotalRecordCount":1,"Items":[{"Id":"show","Name":"Show","Type":"Series"}]}`))
						return
					}
					w.Write([]byte(`{"TotalRecordCount":1,"Items":[{"Id":"film","Type":"` + kind + `","Path":"/media/disc1.mkv","ParentIndexNumber":1,"IndexNumber":2,"PartCount":3,"MediaSources":[{"Id":"other","Path":"/media/alternate.mkv","Size":500},{"Id":"film","ETag":"main-revision","Path":"/media/disc1.mkv","Size":1000,"MediaStreams":[{"Type":"Video","Width":1920}]}]}]}`))
				case "/Videos/film/AdditionalParts":
					w.Write([]byte(`{"TotalRecordCount":2,"Items":[{"Id":"disc2","Path":"/media/disc2.mkv","MediaSources":[{"Id":"disc2","Path":"/media/disc2.mkv","ETag":"second-revision","Size":2000,"MediaStreams":[{"Type":"Video","Width":1280}]}]},{"Id":"disc3","Path":"/media/disc3.mkv"}]}`))
				default:
					http.NotFound(w, r)
				}
			}))
			defer api.Close()
			server := MediaServer{Kind: "jellyfin", BaseURL: api.URL, Token: "fixture"}
			items := map[string]InventoryItem{}
			if err := server.Inventory(context.Background(), func(item InventoryItem) error {
				items[item.Path] = item
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if len(items) != 4 {
				t.Fatalf("want primary version parts and alternate version, got %v", items)
			}
			keys := map[string]bool{}
			for index, path := range []string{"/media/disc1.mkv", "/media/disc2.mkv", "/media/disc3.mkv"} {
				item := items[path]
				profile := item.Technical
				if item.ItemKey != "film" || item.Key == "" || keys[item.Key] || profile == nil ||
					profile.VersionID != "film" || profile.PartCount != 3 || profile.PartNumber != index+1 {
					t.Fatalf("incorrect part identity for %s: %+v (%+v)", path, item, profile)
				}
				keys[item.Key] = true
				if kind == "Episode" && (!item.SeasonKnown || item.Season != 1 || len(item.Episodes) != 1 || item.Episodes[0] != 2) {
					t.Fatalf("additional part lost episode coverage: %+v", item)
				}
			}
			if item := items["/media/disc2.mkv"]; item.Revision != "second-revision" || item.Technical.PartID != "disc2" ||
				item.Technical.Width == nil || *item.Technical.Width != 1280 || item.Technical.SizeBytes == nil || *item.Technical.SizeBytes != 2000 {
				t.Fatalf("additional part used wrong file metadata: %+v (%+v)", item, item.Technical)
			}
			if item := items["/media/disc3.mkv"]; item.Revision != "" || item.Technical.Width != nil || item.Technical.SizeBytes != nil || item.Technical.Audio != nil {
				t.Fatalf("part inherited unreported metadata: %+v (%+v)", item, item.Technical)
			}
			if item := items["/media/alternate.mkv"]; item.Technical == nil || item.Technical.PartCount != 0 || item.Technical.VersionID != "other" {
				t.Fatalf("alternate incorrectly labelled as a primary version part: %+v", item.Technical)
			}
		})
	}
}

func TestJellyfinInventoryRejectsIncompleteAdditionalParts(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/Items" {
			w.Write([]byte(`{"TotalRecordCount":1,"Items":[{"Id":"film","Type":"Movie","Path":"/media/disc1.mkv","PartCount":2}]}`))
			return
		}
		w.Write([]byte(`{"TotalRecordCount":0,"Items":[]}`))
	}))
	defer api.Close()
	server := MediaServer{Kind: "jellyfin", BaseURL: api.URL, Token: "fixture"}
	if err := server.Inventory(context.Background(), func(InventoryItem) error { return nil }); err == nil {
		t.Fatal("incomplete split inventory was marked as complete")
	}
}
