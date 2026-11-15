package store

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/tendstead/lookout/internal/library"
	"github.com/tendstead/lookout/internal/metadata"
	"strings"
	"testing"
	"time"
)

func TestConnectionSecretsNotExposed(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if e := s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	key := bytes.Repeat([]byte{1}, 32)
	connection, e := s.SaveConnection(ctx, metadata.MediaServer{Kind: "jellyfin", BaseURL: "http://media.local:8096", Token: "private-test-token", Mappings: []metadata.PathMapping{}}, key)
	if e != nil {
		t.Fatal(e)
	}
	list, e := s.Connections(ctx)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(list)
	if strings.Contains(string(raw), "private-test-token") {
		t.Fatal("token exposed")
	}
	secret, e := s.ConnectionConfig(ctx, connection.ID, key)
	if e != nil || secret.Token != "private-test-token" {
		t.Fatal("credential lookup failed")
	}
	if _, e = s.ConnectionConfig(ctx, connection.ID, bytes.Repeat([]byte{2}, 32)); e == nil {
		t.Fatal("wrong encryption key accepted")
	}
}
func TestMetadataRequiresStablePathAndPreservesManualMatch(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if e := s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	root := library.Root{ID: "metadata", Label: "Series", Path: "/series", Category: "series"}
	if e := s.EnsureSource(ctx, root); e != nil {
		t.Fatal(e)
	}
	scan, e := s.StartScan(ctx, root.ID)
	if e != nil {
		t.Fatal(e)
	}
	f := library.FileObservation{SourceID: root.ID, RelativePath: "Show/file.mkv", Group: "Show", Category: "series", Title: library.ParseTitle("Show [tvdbid-123]")}
	if e = s.Observe(ctx, scan, f); e != nil {
		t.Fatal(e)
	}
	ids, e := s.MetadataTargets(ctx, root.ID, "Show", "series")
	if e != nil || len(ids) != 1 {
		t.Fatal(e, ids)
	}
	missing, e := s.MetadataTargets(ctx, root.ID, "Wrong", "series")
	if e != nil || len(missing) != 0 {
		t.Fatal("matched by title guess")
	}
	candidate := metadata.Candidate{Provider: "tvmaze", ID: "9", Name: "Wrong Show", ProviderIDs: map[string]string{"tvdb": "999", "tvmaze": "9"}}
	if e = s.ApplyMetadata(ctx, ids[0], MetadataUpdate{Candidate: candidate}); e != ErrMetadataConflict {
		t.Fatal("conflicting IDs merged", e)
	}
	candidate.Name = "Chosen Show"
	if e = s.ApplyMetadata(ctx, ids[0], MetadataUpdate{Candidate: candidate, Manual: true}); e != nil {
		t.Fatal(e)
	}
	candidate.Name = "Auto replacement"
	if e = s.ApplyMetadata(ctx, ids[0], MetadataUpdate{Candidate: candidate}); e != nil {
		t.Fatal(e)
	}
	title, e := s.GetTitle(ctx, ids[0])
	if e != nil || title.Name != "Chosen Show" || title.ProviderIDs["tvdb"] != "999" {
		t.Fatal("manual match overwritten", title, e)
	}
}
func TestManualMatchResetsArtworkAndReleaseBaseline(t *testing.T) {
	s := testStore(t)
	id := releaseTitle(t, s)
	ctx := context.Background()
	first := metadata.Candidate{Provider: "tvmaze", ID: "9", Name: "First", ProviderIDs: map[string]string{"tvmaze": "9"}, ImageURL: "https://static.tvmaze.com/uploads/images/first.jpg"}
	if e := s.ApplyMetadata(ctx, id, MetadataUpdate{Candidate: first, Manual: true}); e != nil {
		t.Fatal(e)
	}
	if e := s.CacheArtwork(ctx, id, "image/png", []byte{1, 2, 3}); e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC()
	if e := s.SetFollow(ctx, id, true, now); e != nil {
		t.Fatal(e)
	}
	f, _ := s.GetFollow(ctx, id)
	if _, e := s.SaveEpisodeSnapshot(ctx, f, nil, now, false); e != nil {
		t.Fatal(e)
	}
	next := first
	next.ID = "10"
	next.ProviderIDs = map[string]string{"tvmaze": "10"}
	next.ImageURL = "https://static.tvmaze.com/uploads/images/next.jpg"
	if e := s.ApplyMetadata(ctx, id, MetadataUpdate{Candidate: next, Manual: true}); e != nil {
		t.Fatal(e)
	}
	if _, _, e := s.CachedArtwork(ctx, id); e == nil {
		t.Fatal("old artwork retained after manual correction")
	}
	f, _ = s.GetFollow(ctx, id)
	if f.ProviderID != "10" || f.Initialized {
		t.Fatal("old show's schedule retained", f)
	}
}
