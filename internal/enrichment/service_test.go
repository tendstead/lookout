package enrichment

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tendstead/lookout/internal/config"
	"github.com/tendstead/lookout/internal/library"
	"github.com/tendstead/lookout/internal/metadata"
	"github.com/tendstead/lookout/internal/store"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("run make service-test")
	}
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, raw)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(admin.Close)
	var random [8]byte
	_, _ = rand.Read(random[:])
	schema := "notify_" + hex.EncodeToString(random[:])
	if _, e = admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _, _ = admin.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE") })
	u, e := url.Parse(raw)
	if e != nil {
		t.Fatal(e)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	s, e := store.Open(ctx, config.Config{DatabaseURL: u.String()})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(s.Close)
	if e = s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	return s
}
func TestOfficialRefreshDoesNotSkipRenamedTitles(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	root := library.Root{ID: "series", Label: "Series", Path: "/series", Category: "series"}
	if e := s.EnsureSource(ctx, root); e != nil {
		t.Fatal(e)
	}
	scan, e := s.StartScan(ctx, root.ID)
	if e != nil {
		t.Fatal(e)
	}
	for i := 1; i <= 101; i++ {
		name := fmt.Sprintf("A%03d", i)
		e = s.Observe(ctx, scan, library.FileObservation{SourceID: root.ID, RelativePath: name + "/file.mkv", Group: name, Category: "series", Title: library.Metadata{Title: name, Provenance: "filename", ProviderIDs: map[string]string{"tvmaze": fmt.Sprint(i)}}})
		if e != nil {
			t.Fatal(e)
		}
	}
	calls := map[string]int{}
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/shows/")
		calls[id]++
		var n int
		fmt.Sscan(id, &n)
		json.NewEncoder(w).Encode(map[string]any{"id": n, "name": "Z" + id})
	}))
	defer mock.Close()
	service := New(s, nil)
	service.AllowPublic = true
	service.TV = &metadata.TVmaze{BaseURL: mock.URL, HTTP: mock.Client()}
	report, e := service.RefreshOfficial(ctx)
	if e != nil || report.Matched != 101 || len(calls) != 101 {
		t.Fatalf("missed or repeated titles: report=%+v unique=%d error=%v", report, len(calls), e)
	}
	for id, n := range calls {
		if n != 1 {
			t.Fatalf("title %s looked up %d times", id, n)
		}
	}
}
func TestDisabledPublicMetadataBlocksUncachedArtworkButServesCache(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	root := library.Root{ID: "series", Label: "Series", Path: "/series", Category: "series"}
	if e := s.EnsureSource(ctx, root); e != nil {
		t.Fatal(e)
	}
	scan, e := s.StartScan(ctx, root.ID)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Observe(ctx, scan, library.FileObservation{SourceID: root.ID, RelativePath: "Show/file.mkv", Group: "Show", Category: "series", Title: library.ParseTitle("Show")}); e != nil {
		t.Fatal(e)
	}
	page, e := s.ListTitles(ctx, store.TitleQuery{})
	if e != nil {
		t.Fatal(e)
	}
	id := page.Items[0].ID
	if e = s.ApplyMetadata(ctx, id, store.MetadataUpdate{Candidate: metadata.Candidate{Provider: "tvmaze", ID: "9", Name: "Show", ProviderIDs: map[string]string{"tvmaze": "9"}, ImageURL: "http://127.0.0.1:1/image"}}); e != nil {
		t.Fatal(e)
	}
	service := New(s, nil)
	if _, _, e = service.Image(ctx, id); e == nil || e.Error() != "public metadata lookups are disabled" {
		t.Fatalf("expected consent gate before public fetch, got %v", e)
	}
	if e = s.CacheArtwork(ctx, id, "image/jpeg", []byte("fixture")); e != nil {
		t.Fatal(e)
	}
	b, m, e := service.Image(ctx, id)
	if e != nil || string(b) != "fixture" || m != "image/jpeg" {
		t.Fatal("disabled service did not serve local cache", e)
	}
}
