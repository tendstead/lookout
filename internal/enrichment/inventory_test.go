package enrichment

import (
	"context"
	"github.com/tendstead/lookout/internal/catalogue"
	"github.com/tendstead/lookout/internal/library"
	"github.com/tendstead/lookout/internal/metadata"
	"github.com/tendstead/lookout/internal/store"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestAPIOnlyInventoryDoesNotRequireSFTP(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	var fail atomic.Bool
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			http.Error(w, "offline", 503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/System/Info" {
			w.Write([]byte(`{"ServerName":"Family Jellyfin"}`))
			return
		}
		if r.URL.Query().Get("IncludeItemTypes") == "Episode" {
			w.Write([]byte(`{"TotalRecordCount":1,"Items":[{"Id":"ep","ParentIndexNumber":1,"IndexNumber":2}]}`))
			return
		}
		w.Write([]byte(`{"TotalRecordCount":2,"Items":[{"Id":"movie","Name":"Film","Type":"Movie","ProviderIds":{"Imdb":"tt1"}},{"Id":"show","Name":"Show","Type":"Series","ProviderIds":{"Tvdb":"7"}}]}`))
	}))
	defer api.Close()
	key := make([]byte, 32)
	service := New(s, key)
	c, e := service.SaveConnection(ctx, metadata.MediaServer{Kind: "jellyfin", BaseURL: api.URL, Token: "fixture"})
	if e != nil {
		t.Fatal(e)
	}
	for range 2 {
		if _, e = service.ImportConnection(ctx, c.ID); e != nil {
			t.Fatal(e)
		}
	}
	if service.Status().Running {
		t.Fatal("automatic import leaves worker busy")
	}
	page, e := s.ListTitles(ctx, store.TitleQuery{})
	if e != nil || page.Total != 2 {
		t.Fatal(page, e)
	}
	for _, title := range page.Items {
		if len(title.Servers) != 1 || title.Servers[0].Name != "Family Jellyfin" {
			t.Fatal(title)
		}
		d, e := s.GetTitle(ctx, title.ID)
		if e != nil || len(d.Files) != 1 {
			t.Fatal(d, e)
		}
		if title.Category == "series" && (d.Files[0].Season == nil || *d.Files[0].Season != 1 || d.Files[0].Episodes[0] != 2) {
			t.Fatal(d)
		}
	}
	fail.Store(true)
	if _, e = service.ImportConnection(ctx, c.ID); e == nil {
		t.Fatal("expected verification failure")
	}
	sources, e := s.ListSources(ctx)
	if e != nil {
		t.Fatal(e)
	}
	for _, source := range sources {
		if source.Status != "error" {
			t.Fatal("failed API still ready", source)
		}
	}

}

func TestAPISetupFailureCleansStartedScopes(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	service := New(s, make([]byte, 32))
	s.EnsureServer(ctx, "fixture", "Fixture")
	id := "fixture"
	source := inventorySource(id, "series")
	s.EnsureSource(ctx, library.Root{ID: source, ServerID: "fixture", Path: "/api/fixture/series", Category: "series"})
	s.SetSourceAdapter(ctx, source, "jellyfin")
	s.StartScan(ctx, source)
	c := metadata.MediaServer{Kind: "jellyfin", ServerID: "fixture"}
	if _, e := service.ImportInventory(ctx, id, c); e == nil {
		t.Fatal("expected second scope busy")
	}
	sources, e := s.ListSources(ctx)
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range sources {
		if v.ID == inventorySource(id, "movie") && v.Status == "scanning" {
			t.Fatal("started scope stranded")
		}
	}
}

func TestCatalogueStartupReconcilesExistingCopies(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	for _, id := range []string{"a", "b"} {
		s.EnsureSource(ctx, library.Root{ID: id, Path: "/" + id, Category: "movie"})
		scan, _ := s.StartScan(ctx, id)
		if e := s.Observe(ctx, scan, library.FileObservation{SourceID: id, Group: id, RelativePath: id, Category: "movie", Title: library.Metadata{Title: id, ProviderIDs: map[string]string{"imdb": "tt1"}}}); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := catalogue.New(ctx, s, catalogue.Config{}); e != nil {
		t.Fatal(e)
	}
	p, e := s.ListTitles(ctx, store.TitleQuery{})
	if e != nil || p.Total != 1 {
		t.Fatal("startup did not reconcile", p, e)
	}
}
func TestQueueAccountsForConcurrentAutomaticImports(t *testing.T) {
	s := New(nil, nil)
	s.AllowPublic = true
	s.active = map[string]bool{"fixture": true}
	if e := s.Queue("official"); e == nil {
		t.Fatal("queued metadata during automatic inventory")
	}
}
