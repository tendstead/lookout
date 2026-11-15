package store

import (
	"context"
	"fmt"
	"github.com/tendstead/lookout/internal/library"
	"testing"
	"time"
)

func TestWantedBaselineServerArrivalAndStaleness(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if e := s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	identityFixture(t, s, "a", "Film", "tt1")
	w, e := s.AddWanted(ctx, Wanted{Name: "Film", Category: "movie", ProviderIDs: map[string]string{"imdb": "tt1"}})
	if e != nil {
		t.Fatal(e)
	}
	if e = s.RefreshWanted(ctx, true); e != nil {
		t.Fatal(e)
	}
	d, e := s.ClaimWantedNotification(ctx, time.Now())
	if e != nil || d != nil {
		t.Fatal("historical arrival", d, e)
	}
	identityFixture(t, s, "b", "Film", "tt1")
	if e = s.RefreshWanted(ctx, true); e != nil {
		t.Fatal(e)
	}
	d, e = s.ClaimWantedNotification(ctx, time.Now())
	if e != nil || d == nil || d.ServerName != "b" || d.Title != "Film" {
		t.Fatal(d, e)
	}
	if e = s.MarkWantedNotification(ctx, d.ID, true, time.Minute, time.Now()); e != nil {
		t.Fatal(e)
	}
	if e = s.RefreshWanted(ctx, true); e != nil {
		t.Fatal(e)
	}
	d, e = s.ClaimWantedNotification(ctx, time.Now())
	if e != nil || d != nil {
		t.Fatal("duplicate", d, e)
	}
	s.pool.Exec(ctx, `UPDATE library_sources SET last_success=now()-interval '2 days'`)
	items, e := s.ListWanted(ctx)
	if e != nil || len(items) != 1 || items[0].ID != w.ID || items[0].Status != "uncertain" {
		t.Fatal(items, e)
	}
}
func TestWantedAbsentAndEpisodeTargets(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if e := s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	s.EnsureSource(ctx, library.Root{ID: "shows", Path: "/shows", Category: "series"})
	scan, _ := s.StartScan(ctx, "shows")
	season, episode := 2, 3
	s.Observe(ctx, scan, library.FileObservation{SourceID: "shows", Group: "Show", RelativePath: "s2e3", Category: "series", Title: library.Metadata{Title: "Show", ProviderIDs: map[string]string{"tvdb": "5"}}, Season: season, Episodes: []int{episode}, Numbering: "season_episode"})
	s.FinishScan(ctx, scan, "shows", library.ScanStats{Media: 1}, nil)
	for _, n := range []int{3, 4} {
		v, e := s.AddWanted(ctx, Wanted{Name: "Show", Category: "series", ProviderIDs: map[string]string{"tvdb": "5"}, Season: &season, Episode: &n})
		if e != nil {
			t.Fatal(e)
		}
		_ = v
	}
	items, e := s.ListWanted(ctx)
	if e != nil {
		t.Fatal(e)
	}
	statuses := map[int]string{}
	for _, v := range items {
		statuses[*v.Episode] = v.Status
	}
	if statuses[3] != "available" || statuses[4] != "missing" {
		t.Fatal(items)
	}
}

func TestWantedEpisodeArrivalsAreCoalescedPerServer(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if e := s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	s.EnsureSource(ctx, library.Root{ID: "shows", Path: "/shows", Category: "series"})
	_, e := s.AddWanted(ctx, Wanted{Name: "Show", Category: "series", ProviderIDs: map[string]string{"tvdb": "5"}})
	if e != nil {
		t.Fatal(e)
	}
	scan, _ := s.StartScan(ctx, "shows")
	for _, ep := range []int{1, 2} {
		if e = s.Observe(ctx, scan, library.FileObservation{SourceID: "shows", Group: "Show", RelativePath: fmt.Sprint(ep), Category: "series", Title: library.Metadata{Title: "Show", ProviderIDs: map[string]string{"tvdb": "5"}}, Season: 1, Episodes: []int{ep}, Numbering: "season_episode"}); e != nil {
			t.Fatal(e)
		}
	}
	if e = s.FinishScan(ctx, scan, "shows", library.ScanStats{Media: 2}, nil); e != nil {
		t.Fatal(e)
	}
	if e = s.RefreshWanted(ctx, true); e != nil {
		t.Fatal(e)
	}
	d, e := s.ClaimWantedNotification(ctx, time.Now())
	if e != nil || d == nil || len(d.WantedIDs) != 2 {
		t.Fatal(d, e)
	}
	if e = s.MarkWantedNotification(ctx, d.ID, true, time.Minute, time.Now(), d.WantedIDs...); e != nil {
		t.Fatal(e)
	}
	d, e = s.ClaimWantedNotification(ctx, time.Now())
	if e != nil || d != nil {
		t.Fatal("arrival flood", d, e)
	}
}
