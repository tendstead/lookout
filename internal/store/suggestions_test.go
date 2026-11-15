package store

import (
	"context"
	"github.com/tendstead/lookout/internal/library"
	"testing"
)

func TestLocalSuggestionsRequireAvailableFileEvidence(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if e := s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	root := library.Root{ID: "suggestions", Label: "Library", Path: "/library", Category: "movie"}
	if e := s.EnsureSource(ctx, root); e != nil {
		t.Fatal(e)
	}
	scan, e := s.StartScan(ctx, root.ID)
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"Local available", "Local missing", "Local uncertain"} {
		if e = s.Observe(ctx, scan, library.FileObservation{SourceID: root.ID, RelativePath: name + "/file.mkv", Group: name, Category: "movie", Title: library.ParseTitle(name)}); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = s.pool.Exec(ctx, `UPDATE library_files f SET observation=CASE WHEN t.name='Local missing' THEN 'missing' ELSE 'unconfirmed' END FROM library_titles t WHERE f.title_id=t.id AND t.name<>'Local available'`); e != nil {
		t.Fatal(e)
	}
	page, e := s.ListTitles(ctx, TitleQuery{Search: "Local", Limit: 8, LocalOnly: true})
	if e != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].Name != "Local available" {
		t.Fatalf("unavailable title suggested: %+v %v", page, e)
	}
	all, e := s.ListTitles(ctx, TitleQuery{Search: "Local", Limit: 8})
	if e != nil || all.Total != 3 {
		t.Fatal("ordinary retained inventory changed", all, e)
	}
	// A second confirmed copy makes a retained title eligible again.
	if _, e = s.pool.Exec(ctx, `UPDATE library_files f SET observation='stable' FROM library_titles t WHERE f.title_id=t.id AND t.name='Local missing'`); e != nil {
		t.Fatal(e)
	}
	page, e = s.ListTitles(ctx, TitleQuery{Search: "Local", Category: "movie", Limit: 8, LocalOnly: true})
	if e != nil || page.Total != 2 {
		t.Fatal(page, e)
	}
	page, e = s.ListTitles(ctx, TitleQuery{Search: "Local", Category: "series", Limit: 8, LocalOnly: true})
	if e != nil || page.Total != 0 {
		t.Fatal("category not applied", page, e)
	}
}
