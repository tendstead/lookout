package store

import (
	"context"
	"github.com/tendstead/lookout/internal/library"
	"github.com/tendstead/lookout/internal/metadata"
	"testing"
	"time"
)

func releaseTitle(t *testing.T, s *Store) string {
	t.Helper()
	ctx := context.Background()
	if e := s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	root := library.Root{ID: "releases", Label: "Anime", Path: "/anime", Category: "anime_series"}
	if e := s.EnsureSource(ctx, root); e != nil {
		t.Fatal(e)
	}
	scan, e := s.StartScan(ctx, root.ID)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Observe(ctx, scan, library.FileObservation{SourceID: root.ID, RelativePath: "Show/file.mkv", Group: "Show", Category: "anime_series", Title: library.Metadata{Title: "Anime Show", ProviderIDs: map[string]string{"tvmaze": "9"}, Provenance: "filename"}}); e != nil {
		t.Fatal(e)
	}
	page, e := s.ListTitles(ctx, TitleQuery{})
	if e != nil || len(page.Items) != 1 {
		t.Fatal(e)
	}
	return page.Items[0].ID
}
func TestReleaseSnapshotBaselineRestartAndNotificationCoalescing(t *testing.T) {
	s := testStore(t)
	id := releaseTitle(t, s)
	ctx := context.Background()
	base := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	if e := s.SetFollow(ctx, id, true, base); e != nil {
		t.Fatal(e)
	}
	first := 1
	episodes := []metadata.Episode{{ID: "future", Name: "Season premiere", Season: 2, Number: &first, AirStamp: base.Add(time.Hour).Format(time.RFC3339)}}
	f, e := s.GetFollow(ctx, id)
	if e != nil {
		t.Fatal(e)
	}
	if n, e := s.SaveEpisodeSnapshot(ctx, f, episodes, base, true); e != nil || n != 0 {
		t.Fatal("baseline announced history", n, e)
	}
	f, _ = s.GetFollow(ctx, id)
	if n, e := s.SaveEpisodeSnapshot(ctx, f, episodes, base.Add(2*time.Hour), true); e != nil || n != 2 {
		t.Fatal("premiere not recorded", n, e)
	}
	f, _ = s.GetFollow(ctx, id)
	if n, e := s.SaveEpisodeSnapshot(ctx, f, episodes, base.Add(3*time.Hour), true); e != nil || n != 0 {
		t.Fatal("restart duplicated event", n, e)
	}
	var count int
	if e = s.pool.QueryRow(ctx, "SELECT count(*) FROM notification_outbox WHERE status='pending'").Scan(&count); e != nil || count != 1 {
		t.Fatal("premiere not coalesced", count, e)
	}
	delivery, e := s.ClaimNotification(ctx, base.Add(3*time.Hour))
	if e != nil || delivery == nil || delivery.Kind != "season" {
		t.Fatal(delivery, e)
	}
	if e = s.MarkNotification(ctx, delivery.ID, true, 0, base.Add(3*time.Hour)); e != nil {
		t.Fatal(e)
	}
	if next, e := s.ClaimNotification(ctx, base.Add(4*time.Hour)); e != nil || next != nil {
		t.Fatal("delivered event reclaimed", next, e)
	}
}
func TestProviderChangeResetsBaselineAndUnfollowCancelsPending(t *testing.T) {
	s := testStore(t)
	id := releaseTitle(t, s)
	ctx := context.Background()
	base := time.Now().UTC()
	if e := s.SetFollow(ctx, id, true, base); e != nil {
		t.Fatal(e)
	}
	f, _ := s.GetFollow(ctx, id)
	_, e := s.SaveEpisodeSnapshot(ctx, f, nil, base, true)
	if e != nil {
		t.Fatal(e)
	}
	n := 2
	episode := metadata.Episode{ID: "new", Season: 1, Number: &n, AirStamp: base.Add(time.Minute).Format(time.RFC3339)}
	f, _ = s.GetFollow(ctx, id)
	if _, e = s.SaveEpisodeSnapshot(ctx, f, []metadata.Episode{episode}, base.Add(2*time.Minute), true); e != nil {
		t.Fatal(e)
	}
	if e = s.SetFollow(ctx, id, false, base.Add(3*time.Minute)); e != nil {
		t.Fatal(e)
	}
	if d, e := s.ClaimNotification(ctx, base.Add(4*time.Minute)); e != nil || d != nil {
		t.Fatal("unfollowed notification delivered", d, e)
	}
	if _, e = s.pool.Exec(ctx, `UPDATE library_titles SET provider_ids='{"tvmaze":"10"}' WHERE id=$1`, id); e != nil {
		t.Fatal(e)
	}
	if e = s.SetFollow(ctx, id, true, base.Add(5*time.Minute)); e != nil {
		t.Fatal(e)
	}
	f, _ = s.GetFollow(ctx, id)
	if f.Initialized || f.ProviderID != "10" {
		t.Fatal("provider change retained old baseline", f)
	}
}
