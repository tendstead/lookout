package store

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/tendstead/lookout/internal/library"
	"github.com/tendstead/lookout/internal/metadata"
)

func TestConfiguredConnectionKeepsIdentityAndLeavesLegacyToken(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	key := bytes.Repeat([]byte{1}, 32)
	legacy, err := s.SaveConnection(ctx, metadata.MediaServer{Kind: "jellyfin", BaseURL: "https://old.example", Token: "old-token"}, key)
	if err != nil {
		t.Fatal(err)
	}
	c := metadata.MediaServer{ServerID: legacy.ServerID, Kind: "jellyfin", BaseURL: "https://old.example", Token: "toml-token"}
	id, err := s.ConfigureConnection(ctx, c)
	if err != nil || id != legacy.ID {
		t.Fatalf("migration should reuse connection ID: %v", err)
	}
	old, err := s.ConnectionConfig(ctx, id, key)
	if err != nil || old.Token != "old-token" {
		t.Fatalf("existing encrypted credential should remain intact: %v", err)
	}
	c.BaseURL = "https://new.example"
	id, err = s.ConfigureConnection(ctx, c)
	if err != nil || id != legacy.ID {
		t.Fatalf("changing URL should retain connection ID: %v", err)
	}
	if _, err := s.ConnectionConfig(ctx, id, key); err == nil {
		t.Fatal("a legacy token must not be carried to the changed URL")
	}
}

func TestConfiguredInventoryIgnoresRetiredRootStatus(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureServer(ctx, "home", "Home"); err != nil {
		t.Fatal(err)
	}
	old := library.Root{ID: "films", ServerID: "home", Path: "/old/films", Label: "Films", Category: "movie"}
	if err := s.EnsureSource(ctx, old); err != nil {
		t.Fatal(err)
	}
	failed, err := s.StartScan(ctx, old.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.FinishScan(ctx, failed, old.ID, library.ScanStats{}, errors.New("unavailable")); err != nil {
		t.Fatal(err)
	}
	id, err := s.ConfigureConnection(ctx, metadata.MediaServer{ServerID: "home", Kind: "plex", BaseURL: "https://media.example", Token: "configured"})
	if err != nil || id == "" {
		t.Fatalf("an old SFTP root should not prevent switching inventory types: %v", err)
	}
	current := library.Root{ID: "api-root", ServerID: "home", Path: "/api/media/films", Label: "Films", Category: "movie"}
	if err := s.EnsureSource(ctx, current); err != nil {
		t.Fatal(err)
	}
	started, err := s.StartScan(ctx, current.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.FinishScan(ctx, started, current.ID, library.ScanStats{}, nil); err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListServersForSources(ctx, []string{current.ID})
	if err != nil {
		t.Fatal(err)
	}
	for _, server := range rows {
		if server.ID == "home" {
			if server.Status != "ready" {
				t.Fatalf("retired filesystem rows must not affect active server status, got %s", server.Status)
			}
			return
		}
	}
	t.Fatal("active server is missing")
}
