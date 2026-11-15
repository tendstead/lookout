package store

import (
	"context"
	"github.com/tendstead/lookout/internal/library"
	"testing"
	"time"
)

func TestCatalogueScanIdempotence(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if e := s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	root := library.Root{ID: "test-series", Label: "Series", Path: "/media/series", Category: "series"}
	if e := s.EnsureSource(ctx, root); e != nil {
		t.Fatal(e)
	}
	file := library.FileObservation{SourceID: root.ID, RelativePath: "Show/Season 1/Show.S01E02.mkv", Group: "Show", Category: "series", Size: 100, Modified: time.Unix(10, 0), Title: library.ParseTitle("Show [tvdbid-123]"), Season: 1, Episodes: []int{2}, Numbering: "season_episode"}
	for range 2 {
		scan, e := s.StartScan(ctx, root.ID)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.Observe(ctx, scan, file); e != nil {
			t.Fatal(e)
		}
		if e = s.FinishScan(ctx, scan, root.ID, library.ScanStats{Media: 1}, nil); e != nil {
			t.Fatal(e)
		}
	}
	page, e := s.ListTitles(ctx, TitleQuery{Limit: 20})
	if e != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].FileCount != 1 {
		t.Fatalf("%#v %v", page, e)
	}
	detail, e := s.GetTitle(ctx, page.Items[0].ID)
	if e != nil || len(detail.Files) != 1 || detail.Files[0].Numbering != "season_episode" {
		t.Fatalf("%#v %v", detail, e)
	}
}
func TestFailedAndEmptyScansPreserveInventory(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if e := s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	root := library.Root{ID: "test-preserve", Label: "Films", Path: "/media/films", Category: "movie"}
	if e := s.EnsureSource(ctx, root); e != nil {
		t.Fatal(e)
	}
	id, e := s.StartScan(ctx, root.ID)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Observe(ctx, id, library.FileObservation{SourceID: root.ID, RelativePath: "Film/Film.mkv", Group: "Film", Category: "movie", Size: 100, Modified: time.Now(), Title: library.ParseTitle("Film")}); e != nil {
		t.Fatal(e)
	}
	if e = s.FinishScan(ctx, id, root.ID, library.ScanStats{Media: 1}, nil); e != nil {
		t.Fatal(e)
	}
	for range 2 {
		id, e = s.StartScan(ctx, root.ID)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.FinishScan(ctx, id, root.ID, library.ScanStats{}, nil); e != nil {
			t.Fatal(e)
		}
	}
	page, e := s.ListTitles(ctx, TitleQuery{Category: "movie", Limit: 20})
	if e != nil || page.Total != 1 {
		t.Fatalf("inventory lost %#v %v", page, e)
	}
	sources, e := s.ListSources(ctx)
	if e != nil {
		t.Fatal(e)
	}
	for _, source := range sources {
		if source.ID == root.ID && source.Status != "review" {
			t.Fatalf("empty scan not quarantined: %#v", source)
		}
	}
}
func TestCataloguePaginationAndSearch(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if e := s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	root := library.Root{ID: "test-pages", Label: "Anime", Path: "/media/anime", Category: "anime_series"}
	if e := s.EnsureSource(ctx, root); e != nil {
		t.Fatal(e)
	}
	scan, e := s.StartScan(ctx, root.ID)
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"Anime Alpha", "Anime Beta", "Anime Gamma"} {
		e = s.Observe(ctx, scan, library.FileObservation{SourceID: root.ID, RelativePath: name + "/01.mkv", Group: name, Category: root.Category, Size: 10, Modified: time.Now(), Title: library.ParseTitle(name)})
		if e != nil {
			t.Fatal(e)
		}
	}
	page, e := s.ListTitles(ctx, TitleQuery{Search: "Anime", Category: "anime_series", Limit: 2, Offset: 1})
	if e != nil || page.Total != 3 || len(page.Items) != 2 {
		t.Fatalf("%#v %v", page, e)
	}
}
func TestGroupingCorrectionRetainsFiles(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if e := s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	root := library.Root{ID: "grouping", Label: "Films", Path: "/films", Category: "movie"}
	if e := s.EnsureSource(ctx, root); e != nil {
		t.Fatal(e)
	}
	for _, group := range []string{"Collection", "Collection/Film"} {
		scan, e := s.StartScan(ctx, root.ID)
		if e != nil {
			t.Fatal(e)
		}
		f := library.FileObservation{SourceID: root.ID, RelativePath: "Collection/Film/file.mkv", Group: group, Category: "movie", Size: 100, Modified: time.Now(), Title: library.ParseTitle(group)}
		if e = s.Observe(ctx, scan, f); e != nil {
			t.Fatal(e)
		}
		if e = s.FinishScan(ctx, scan, root.ID, library.ScanStats{Media: 1}, nil); e != nil {
			t.Fatal(e)
		}
	}
	page, e := s.ListTitles(ctx, TitleQuery{})
	if e != nil || page.Total != 1 || page.Items[0].FileCount != 1 {
		t.Fatalf("empty generated collection title retained: %#v %v", page, e)
	}
}
func TestArchiveMembersRemainDistinctAndIdempotent(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if e := s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	root := library.Root{ID: "archive", Path: "/series", Label: "Series", Category: "series"}
	if e := s.EnsureSource(ctx, root); e != nil {
		t.Fatal(e)
	}
	for range 2 {
		scan, e := s.StartScan(ctx, root.ID)
		if e != nil {
			t.Fatal(e)
		}
		for _, member := range []string{"Season 1/Show.S01E01.mkv", "Season 1/Show.S01E02.mkv"} {
			f := library.FileObservation{SourceID: root.ID, RelativePath: "Show/season.zip", ArchiveMember: member, Storage: "zip", Group: "Show", Category: "series", Title: library.ParseTitle("Show"), Size: 100, Modified: time.Now()}
			if e = s.Observe(ctx, scan, f); e != nil {
				t.Fatal(e)
			}
		}
		if e = s.FinishScan(ctx, scan, root.ID, library.ScanStats{ArchivedMedia: 2, Archives: 1}, nil); e != nil {
			t.Fatal(e)
		}
	}
	page, e := s.ListTitles(ctx, TitleQuery{})
	if e != nil || page.Items[0].FileCount != 2 {
		t.Fatal("archive identities collapsed", page, e)
	}
	detail, e := s.GetTitle(ctx, page.Items[0].ID)
	if e != nil || len(detail.Files) != 2 || detail.Files[0].Storage != "zip" || detail.Files[0].ArchiveMember == detail.Files[1].ArchiveMember {
		t.Fatal(detail, e)
	}
}
