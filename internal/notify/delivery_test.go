package notify

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
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
	"sync/atomic"
	"testing"
	"time"
)

func TestGenuineEventDeliveryOnceUsesMockServer(t *testing.T) {
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("run make service-test")
	}
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, raw)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	var random [8]byte
	_, _ = rand.Read(random[:])
	schema := "notify_" + hex.EncodeToString(random[:])
	if _, e = admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); e != nil {
		t.Fatal(e)
	}
	defer admin.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
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
	defer s.Close()
	if e = s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	root := library.Root{ID: "notify", Label: "Fixture", Path: "/fixture", Category: "anime_series"}
	if e = s.EnsureSource(ctx, root); e != nil {
		t.Fatal(e)
	}
	scan, e := s.StartScan(ctx, root.ID)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Observe(ctx, scan, library.FileObservation{SourceID: root.ID, RelativePath: "Show/file.mkv", Group: "Show", Category: root.Category, Title: library.Metadata{Title: "Fixture Anime", ProviderIDs: map[string]string{"tvmaze": "9"}, Provenance: "filename"}}); e != nil {
		t.Fatal(e)
	}
	page, e := s.ListTitles(ctx, store.TitleQuery{})
	if e != nil {
		t.Fatal(e)
	}
	id := page.Items[0].ID
	base := time.Now().UTC()
	if e = s.SetFollow(ctx, id, true, base); e != nil {
		t.Fatal(e)
	}
	f, _ := s.GetFollow(ctx, id)
	if _, e = s.SaveEpisodeSnapshot(ctx, f, nil, base, true); e != nil {
		t.Fatal(e)
	}
	n := 2
	f, _ = s.GetFollow(ctx, id)
	if _, e = s.SaveEpisodeSnapshot(ctx, f, []metadata.Episode{{ID: "future", Name: "Fixture new episode", Season: 1, Number: &n, AirStamp: base.Add(time.Hour).Format(time.RFC3339)}}, base.Add(2*time.Hour), true); e != nil {
		t.Fatal(e)
	}
	var calls atomic.Int32
	var lastMessage atomic.Value
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bot123:fixture/sendMessage" {
			t.Error("unexpected method")
		}
		var body struct {
			Text string `json:"text"`
		}
		if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
			t.Error(e)
		}
		lastMessage.Store(body.Text)
		calls.Add(1)
		w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	}))
	defer mock.Close()
	service := &Service{store: s, client: &Client{BaseURL: mock.URL, Token: "123:fixture", ChatID: "2", HTTP: mock.Client()}, status: Status{Configured: true, Verified: true, DeliveryEnabled: true}}
	for range 2 {
		if e = service.DeliverOne(ctx, base.Add(3*time.Hour)); e != nil {
			t.Fatal(e)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("duplicate delivery: %d", calls.Load())
	}
	// A genuine Wanted arrival names the logical server and uses the same mock.
	if e = s.SaveServer(ctx, store.Server{ID: "legacy", Name: "Family NAS", ScanHours: 6, StaleHours: 24}); e != nil {
		t.Fatal(e)
	}
	root = library.Root{ID: "wanted-films", Path: "/wanted-fixture", Category: "movie"}
	if e = s.EnsureSource(ctx, root); e != nil {
		t.Fatal(e)
	}
	if _, e = s.AddWanted(ctx, store.Wanted{Name: "Wanted Film", Category: "movie", ProviderIDs: map[string]string{"imdb": "tt123"}}); e != nil {
		t.Fatal(e)
	}
	scan, e = s.StartScan(ctx, root.ID)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Observe(ctx, scan, library.FileObservation{SourceID: root.ID, RelativePath: "wanted.mkv", Group: "wanted", Category: "movie", Title: library.Metadata{Title: "Wanted Film", ProviderIDs: map[string]string{"imdb": "tt123"}}}); e != nil {
		t.Fatal(e)
	}
	if e = s.FinishScan(ctx, scan, root.ID, library.ScanStats{Media: 1}, nil); e != nil {
		t.Fatal(e)
	}
	if e = s.RefreshWanted(ctx, true); e != nil {
		t.Fatal(e)
	}
	for range 2 {
		if e = service.DeliverOne(ctx, time.Now().Add(time.Hour)); e != nil {
			t.Fatal(e)
		}
	}
	message, _ := lastMessage.Load().(string)
	if calls.Load() != 2 || !strings.Contains(message, "Added on Family NAS") || !strings.Contains(message, "Wanted Film") {
		t.Fatalf("wrong Wanted notification: %q (%d)", message, calls.Load())
	}

}
