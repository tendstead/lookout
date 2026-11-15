package store

import (
	"context"
	"github.com/tendstead/lookout/internal/metadata"
	"testing"
	"time"
)

func TestSeasonGapsUseCurrentScheduleAndObservedNumbering(t *testing.T) {
	s := testStore(t)
	id := releaseTitle(t, s)
	ctx := context.Background()
	now := time.Now().UTC()
	// A numbered ZIP member and a loose copy represent the same local episode.
	if _, e := s.pool.Exec(ctx, `UPDATE library_titles SET category='series' WHERE id=$1;
 `, id); e != nil {
		t.Fatal(e)
	}
	if _, e := s.pool.Exec(ctx, `UPDATE library_files SET season=1,episodes='{1}',numbering='season_episode',storage='zip',archive_member='Show.S01E01.mkv' WHERE title_id=$1`, id); e != nil {
		t.Fatal(e)
	}
	if _, e := s.pool.Exec(ctx, `INSERT INTO library_files(source_id,title_id,relative_path,size,modified_at,season,episodes,numbering,last_scan,observation) SELECT source_id,title_id,relative_path||'.copy',size,modified_at,season,episodes,numbering,last_scan,observation FROM library_files WHERE title_id=$1`, id); e != nil {
		t.Fatal(e)
	}
	if _, e := s.pool.Exec(ctx, `UPDATE library_sources SET status='ready',last_success=$1`, now); e != nil {
		t.Fatal(e)
	}
	if e := s.SetFollow(ctx, id, true, now); e != nil {
		t.Fatal(e)
	}
	n1, n2 := 1, 2
	past := now.Add(-time.Hour).Format(time.RFC3339)
	episodes := []metadata.Episode{{ID: "one", Season: 1, Number: &n1, AirStamp: past}, {ID: "two", Season: 1, Number: &n2, AirStamp: past}, {ID: "three", Season: 2, Number: &n1, AirStamp: past}, {ID: "future", Season: 3, Number: &n1, AirStamp: now.Add(time.Hour).Format(time.RFC3339)}}
	f, e := s.GetFollow(ctx, id)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.SaveEpisodeSnapshot(ctx, f, episodes, now, false); e != nil {
		t.Fatal(e)
	}
	q := TitleQuery{Limit: 1, Search: "Anime"}
	page, e := s.SeasonGaps(ctx, q, "missing", now)
	if e != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].Season != 2 || page.Items[0].LatestAired != 2 || page.Missing != 1 || page.Partial != 1 || page.Review != 0 {
		t.Fatalf("missing page: %+v %v", page, e)
	}
	page, e = s.SeasonGaps(ctx, q, "partial", now)
	if e != nil || page.Total != 1 || page.Items[0].Present != 1 || page.Items[0].Aired != 2 || len(page.Items[0].MissingEpisodes) != 1 || page.Items[0].MissingEpisodes[0] != 2 {
		t.Fatalf("partial page: %+v %v", page, e)
	}
	page, e = s.SeasonGaps(ctx, TitleQuery{Limit: 1, Offset: 1}, "missing", now)
	if e != nil || page.Total != 1 || len(page.Items) != 0 || page.Missing != 1 {
		t.Fatalf("pagination totals: %+v %v", page, e)
	}
	if _, e = s.pool.Exec(ctx, `UPDATE library_sources SET last_success=$1`, now.Add(-4*time.Hour)); e != nil {
		t.Fatal(e)
	}
	page, e = s.SeasonGaps(ctx, q, "review", now)
	if e != nil || page.Review != 1 || len(page.Items[0].Reasons) != 1 || page.Items[0].Reasons[0] != "inventory_unavailable" {
		t.Fatalf("stale source asserted gaps: %+v %v", page, e)
	}
	if _, e = s.pool.Exec(ctx, `UPDATE library_sources SET last_success=$1`, now); e != nil {
		t.Fatal(e)
	}
	// Unknown numbering might contain the missing season; suppress definitive gaps.
	if _, e = s.pool.Exec(ctx, `UPDATE library_files SET numbering='unknown' WHERE title_id=$1`, id); e != nil {
		t.Fatal(e)
	}
	page, e = s.SeasonGaps(ctx, q, "review", now)
	if e != nil || page.Review != 1 || page.Missing != 0 || page.Partial != 0 || len(page.Items[0].Reasons) == 0 {
		t.Fatalf("review page: %+v %v", page, e)
	}
	// A new metadata identity must not reuse the old show's cached episodes.
	if _, e = s.pool.Exec(ctx, `UPDATE library_titles SET provider_ids='{"tvmaze":"10"}' WHERE id=$1`, id); e != nil {
		t.Fatal(e)
	}
	page, e = s.SeasonGaps(ctx, q, "review", now)
	if e != nil || page.Items[0].LatestAired != 0 {
		t.Fatalf("old schedule reused: %+v %v", page, e)
	}
	page, e = s.SeasonGaps(ctx, TitleQuery{Limit: 1, Search: "does not match"}, "missing", now)
	if e != nil || page.Total != 0 || page.Review != 0 {
		t.Fatal("search did not filter", page, e)
	}
}
