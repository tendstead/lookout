package store

import (
	"context"
	"github.com/tendstead/lookout/internal/library"
	"testing"
)

func TestServersPreserveNamesAndSeparatePaths(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if e := s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	for _, id := range []string{"a", "b"} {
		if e := s.EnsureServer(ctx, id, id); e != nil {
			t.Fatal(e)
		}
		if e := s.EnsureSource(ctx, library.Root{ID: id, ServerID: id, Path: "/films", Category: "movie"}); e != nil {
			t.Fatal(e)
		}
	}
	if e := s.SaveServer(ctx, Server{ID: "a", Name: "Family NAS", ScanHours: 6, StaleHours: 24}); e != nil {
		t.Fatal(e)
	}
	if e := s.EnsureServer(ctx, "a", "overwrite"); e != nil {
		t.Fatal(e)
	}
	rows, e := s.ListServers(ctx)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, v := range rows {
		if v.ID == "a" {
			found = v.Name == "Family NAS" && v.ScanHours == 6 && v.StaleHours == 24
		}
	}
	if !found {
		t.Fatal(rows)
	}
	if e = s.SaveServer(ctx, Server{ID: "a", Name: "Bad", ScanHours: 24, StaleHours: 6}); e == nil {
		t.Fatal("invalid freshness accepted")
	}
}
