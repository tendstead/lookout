package store

import (
	"context"
	"fmt"
	"github.com/tendstead/lookout/internal/library"
	"github.com/tendstead/lookout/internal/metadata"
	"testing"
	"time"
)

func identityFixture(t *testing.T, s *Store, server, name, imdb string) string {
	t.Helper()
	ctx := context.Background()
	if e := s.EnsureServer(ctx, server, server); e != nil {
		t.Fatal(e)
	}
	if e := s.EnsureSource(ctx, library.Root{ID: server, ServerID: server, Label: server, Path: "/films", Category: "movie"}); e != nil {
		t.Fatal(e)
	}
	scan, e := s.StartScan(ctx, server)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Observe(ctx, scan, library.FileObservation{SourceID: server, Group: name, RelativePath: name + ".mkv", Category: "movie", Modified: time.Now(), Title: library.Metadata{Title: name, ProviderIDs: map[string]string{"imdb": imdb}, Provenance: "nfo"}}); e != nil {
		t.Fatal(e)
	}
	if e = s.FinishScan(ctx, scan, server, library.ScanStats{Media: 1}, nil); e != nil {
		t.Fatal(e)
	}
	var id string
	if e = s.pool.QueryRow(ctx, `SELECT id::text FROM library_titles WHERE source_id=$1`, server).Scan(&id); e != nil {
		t.Fatal(e)
	}
	return id
}
func TestSharedCatalogueAvailabilityAndReversibleIdentity(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if e := s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	a := identityFixture(t, s, "a", "Film", "tt1")
	b := identityFixture(t, s, "b", "Different filename", "tt1")
	if e := s.ReconcileIdentities(ctx); e != nil {
		t.Fatal(e)
	}
	p, e := s.ListTitles(ctx, TitleQuery{})
	if e != nil || p.Total != 1 || len(p.Items[0].Servers) != 2 {
		t.Fatalf("not unified: %#v %v", p, e)
	}
	d, e := s.GetTitle(ctx, a)
	if e != nil || len(d.Files) != 2 || len(d.Servers) != 2 {
		t.Fatal(d, e)
	}
	p, e = s.ListTitles(ctx, TitleQuery{ServerID: "b"})
	if e != nil || p.Total != 1 {
		t.Fatal(p, e)
	}
	if e = s.SetIdentity(ctx, b, ""); e != nil {
		t.Fatal(e)
	}
	if e = s.ReconcileIdentities(ctx); e != nil {
		t.Fatal(e)
	}
	p, e = s.ListTitles(ctx, TitleQuery{})
	if e != nil || p.Total != 2 {
		t.Fatal("manual split lost", p, e)
	}
	if e = s.SetIdentity(ctx, b, a); e != nil {
		t.Fatal(e)
	}
	p, e = s.ListTitles(ctx, TitleQuery{})
	if e != nil || p.Total != 1 {
		t.Fatal("manual merge failed", p, e)
	}
}
func TestConflictingIDsDoNotMerge(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if e := s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	identityFixture(t, s, "a", "Film", "tt1")
	identityFixture(t, s, "b", "Film", "tt2")
	s.pool.Exec(ctx, `UPDATE library_titles SET provider_ids=provider_ids||'{"tmdb":"1"}'::jsonb`)
	if e := s.ReconcileIdentities(ctx); e != nil {
		t.Fatal(e)
	}
	p, e := s.ListTitles(ctx, TitleQuery{})
	if e != nil || p.Total != 2 {
		t.Fatal("conflicting identities merged", p, e)
	}
}

func TestCombinedEpisodesAndCanonicalFollows(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if e := s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	ids := []string{}
	for i, host := range []string{"a", "b"} {
		s.EnsureServer(ctx, host, host)
		s.EnsureSource(ctx, library.Root{ID: host, ServerID: host, Path: "/shows", Category: "series"})
		scan, _ := s.StartScan(ctx, host)
		if e := s.Observe(ctx, scan, library.FileObservation{SourceID: host, Group: "Show", RelativePath: host, Category: "series", Title: library.Metadata{Title: "Show", ProviderIDs: map[string]string{"tvmaze": "9"}}, Season: 1, Episodes: []int{i + 1}, Numbering: "season_episode"}); e != nil {
			t.Fatal(e)
		}
		s.FinishScan(ctx, scan, host, library.ScanStats{Media: 1}, nil)
		var id string
		s.pool.QueryRow(ctx, `SELECT id::text FROM library_titles WHERE source_id=$1`, host).Scan(&id)
		ids = append(ids, id)
	}
	now := time.Now()
	for _, id := range ids {
		if e := s.SetFollow(ctx, id, true, now); e != nil {
			t.Fatal(e)
		}
	}
	if e := s.ReconcileIdentities(ctx); e != nil {
		t.Fatal(e)
	}
	f, e := s.GetFollow(ctx, ids[1])
	if e != nil {
		t.Fatal(e)
	}
	if f.TitleID != ids[0] {
		t.Fatal("follow not canonical", f)
	}
	episodes := []metadata.Episode{}
	for n := 1; n <= 3; n++ {
		number := n
		episodes = append(episodes, metadata.Episode{ID: fmt.Sprint(n), Season: 1, Number: &number, AirStamp: now.Add(-time.Hour).Format(time.RFC3339)})
	}
	if _, e = s.SaveEpisodeSnapshot(ctx, f, episodes, now, false); e != nil {
		t.Fatal(e)
	}
	p, e := s.SeasonGaps(ctx, TitleQuery{}, "partial", now)
	if e != nil || p.Total != 1 || p.Items[0].Present != 2 {
		t.Fatal("combined coverage", p, e)
	}
	p, e = s.SeasonGaps(ctx, TitleQuery{ServerID: "a"}, "partial", now)
	if e != nil || p.Total != 1 || p.Items[0].Present != 1 {
		t.Fatal("selected server coverage", p, e)
	}
	s.pool.Exec(ctx, `UPDATE library_sources SET last_success=now()-interval '2 days' WHERE server_id='b'`)
	p, e = s.SeasonGaps(ctx, TitleQuery{}, "review", now)
	if e != nil || p.Review != 1 {
		t.Fatal("stale combined coverage", p, e)
	}
	follows, e := s.ListFollows(ctx)
	if e != nil || len(follows) != 1 {
		t.Fatal("duplicate follows", follows, e)
	}
}
