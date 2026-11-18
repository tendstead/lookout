package store

import (
	"context"
	"github.com/tendstead/lookout/internal/library"
	"github.com/tendstead/lookout/internal/metadata"
	"sync"
	"testing"
	"time"
)

func TestSourceIdentityCannotMoveHostsOrRoots(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if e := s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	for _, id := range []string{"a", "b"} {
		s.EnsureServer(ctx, id, id)
	}
	root := library.Root{ID: "films", ServerID: "a", Path: "/films", Category: "movie"}
	if e := s.EnsureSource(ctx, root); e != nil {
		t.Fatal(e)
	}
	for _, change := range []library.Root{{ID: "films", ServerID: "b", Path: "/films", Category: "movie"}, {ID: "films", ServerID: "a", Path: "/other", Category: "movie"}, {ID: "films", ServerID: "a", Path: "/films", Category: "series"}} {
		if e := s.EnsureSource(ctx, change); e == nil {
			t.Fatal("source reassigned", change)
		}
	}
}
func TestManualMergeChecksWholeClusterAndMovesCopies(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if e := s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	a := identityFixture(t, s, "a", "A", "")
	b := identityFixture(t, s, "b", "B", "ttB")
	c := identityFixture(t, s, "c", "C", "ttC")
	s.pool.Exec(ctx, `UPDATE library_titles SET provider_ids=provider_ids||'{"tmdb":"1"}'::jsonb WHERE id::text=ANY($1)`, []string{a, b})
	if e := s.ReconcileIdentities(ctx); e != nil {
		t.Fatal(e)
	}
	if e := s.SetIdentity(ctx, c, a); e == nil {
		t.Fatal("target member conflict bypassed")
	}
	s.pool.Exec(ctx, `UPDATE library_titles SET provider_ids='{}' WHERE id::text=$1`, c)
	if e := s.SetIdentity(ctx, a, c); e != nil {
		t.Fatal(e)
	}
	p, e := s.ListTitles(ctx, TitleQuery{})
	if e != nil || p.Total != 1 || len(p.Items[0].Servers) != 3 {
		t.Fatal("cluster copies detached", p, e)
	}
}
func TestWantedUsesMemberIdentities(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if e := s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	a := identityFixture(t, s, "a", "Old name", "")
	b := identityFixture(t, s, "b", "New name", "tt1")
	s.pool.Exec(ctx, `UPDATE library_titles SET provider_ids=provider_ids||'{"tmdb":"1"}'::jsonb WHERE id::text=ANY($1)`, []string{a, b})
	s.ReconcileIdentities(ctx)
	v, e := s.AddWanted(ctx, Wanted{Name: "Different name", Category: "movie", ProviderIDs: map[string]string{"imdb": "tt1"}})
	if e != nil || v.Status != "available" {
		t.Fatal("member identity ignored", v, e)
	}
}
func TestMergedFollowPreservesBaselineAndPendingDelivery(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if e := s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	a := identityFixture(t, s, "a", "A", "tt1")
	b := identityFixture(t, s, "b", "B", "tt1")
	s.pool.Exec(ctx, `UPDATE library_titles SET category='series',provider_ids=provider_ids||'{"tvmaze":"9"}'::jsonb`)
	now := time.Now().UTC()
	number := 2
	episode := []metadata.Episode{{ID: "ep", Season: 1, Number: &number, AirStamp: now.Add(time.Hour).Format(time.RFC3339)}}
	for _, id := range []string{a, b} {
		if e := s.SetFollow(ctx, id, true, now); e != nil {
			t.Fatal(e)
		}
		f, _ := s.GetFollow(ctx, id)
		s.SaveEpisodeSnapshot(ctx, f, nil, now, true)
		f, _ = s.GetFollow(ctx, id)
		if _, e := s.SaveEpisodeSnapshot(ctx, f, episode, now.Add(2*time.Hour), true); e != nil {
			t.Fatal(e)
		}
	}
	if e := s.SetFollow(ctx, a, false, now); e != nil {
		t.Fatal(e)
	}
	if e := s.SetIdentity(ctx, b, a); e != nil {
		t.Fatal(e)
	}
	f, e := s.GetFollow(ctx, a)
	if e != nil || !f.Enabled || !f.Initialized {
		t.Fatal("follow state lost", f, e)
	}
	overview, err := s.ReleaseOverview(ctx, now.Add(3*time.Hour), 50, 0)
	if err != nil || overview.RecentTotal != 1 || len(overview.Recent) != 1 || overview.Recent[0].Delivery != "pending" {
		t.Fatal("merged pending history hidden", overview, err)
	}
	d, e := s.ClaimNotification(ctx, now.Add(3*time.Hour))
	if e != nil || d == nil {
		t.Fatal("pending child hidden", d, e)
	}
	if e = s.MarkNotification(ctx, d.ID, true, time.Minute, now.Add(3*time.Hour)); e != nil {
		t.Fatal(e)
	}
	if e = s.SetIdentity(ctx, b, ""); e != nil {
		t.Fatal(e)
	}
	// Re-enabling either copy must not resurrect a delivered provider event.
	for _, id := range []string{a, b} {
		s.SetFollow(ctx, id, true, now.Add(4*time.Hour))
	}
	d, e = s.ClaimNotification(ctx, now.Add(5*time.Hour))
	if e != nil || d != nil {
		t.Fatal("historical delivery revived", d, e)
	}
}
func TestConcurrentMergesRemainFlat(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if e := s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	a := identityFixture(t, s, "a", "A", "")
	b := identityFixture(t, s, "b", "B", "")
	c := identityFixture(t, s, "c", "C", "")
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	tx.Exec(ctx, `SELECT pg_advisory_xact_lock(726431210)`)
	var wg sync.WaitGroup
	for _, pair := range [][2]string{{a, b}, {b, c}} {
		wg.Go(func() {
			if e := s.SetIdentity(ctx, pair[0], pair[1]); e != nil {
				t.Error(e)
			}
		})
	}
	time.Sleep(30 * time.Millisecond)
	tx.Commit(ctx)
	wg.Wait()
	p, e := s.ListTitles(ctx, TitleQuery{})
	if e != nil || p.Total != 1 {
		t.Fatal("concurrent merge fragmented", p, e)
	}
	for _, id := range []string{a, b, c} {
		if _, e = s.GetTitle(ctx, id); e != nil {
			t.Fatal("identity chain", id, e)
		}
	}
}

func TestWantedBusyDoesNotOccupyWaitingPoolConnections(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if e := s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(726431211)`); e != nil {
		t.Fatal(e)
	}
	request, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	_, e = s.AddWanted(request, Wanted{Name: "Film", Category: "movie"})
	if e == nil || request.Err() != nil {
		t.Fatal("wanted writer waited on the shared pool", e)
	}
}
