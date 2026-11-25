// SPDX-License-Identifier: Elastic-2.0
package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/tendstead/lookout/internal/library"
	"github.com/tendstead/lookout/internal/metadata"
)

func observeWantedScope(t *testing.T, s *Store, server string, episodes ...int) {
	t.Helper()
	ctx := context.Background()
	if err := s.EnsureServer(ctx, server, server); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureSource(ctx, library.Root{ID: server, ServerID: server, Path: "/shows", Category: "series"}); err != nil {
		t.Fatal(err)
	}
	scan, err := s.StartScan(ctx, server)
	if err != nil {
		t.Fatal(err)
	}
	for _, episode := range episodes {
		err = s.Observe(ctx, scan, library.FileObservation{
			SourceID: server, Group: "Scope Show", RelativePath: fmt.Sprintf("s01e%02d.mkv", episode),
			Category: "series", Title: library.Metadata{Title: "Scope Show", ProviderIDs: map[string]string{"tvmaze": "49"}},
			Season: 1, Episodes: []int{episode}, Numbering: "season_episode",
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if err = s.FinishScan(ctx, scan, server, library.ScanStats{Media: len(episodes)}, nil); err != nil {
		t.Fatal(err)
	}
}

func TestWantedServerScopeIgnoresOtherServersAndPublicSchedule(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	observeWantedScope(t, s, "first")
	observeWantedScope(t, s, "second", 1)
	entry, err := s.AddWanted(ctx, Wanted{Name: "Scope Show", Category: "series", ServerID: "first",
		ProviderIDs: map[string]string{"tvmaze": "49"}, AirAlerts: true})
	if err != nil {
		t.Fatal(err)
	}
	if entry.AirAlerts || entry.Status != "missing" || len(entry.Servers) != 1 || entry.Servers[0].ServerID != "first" {
		t.Fatalf("unexpected scoped evidence: %+v", entry)
	}
	global := entry
	global.ServerID = ""
	if id, err := s.FindTrackedTitle(ctx, global); err != nil || id != "" {
		t.Fatalf("scoped entry matched global: %q, %v", id, err)
	}
	now := time.Now().UTC().Add(time.Minute)
	if providers, err := s.DueWantedSchedules(ctx, now, true); err != nil || len(providers) != 0 {
		t.Fatalf("public lookup due: %v, %v", providers, err)
	}
	n := 1
	ep := metadata.Episode{ID: "49", Season: 1, Number: &n, AirStamp: now.Add(time.Hour).Format(time.RFC3339)}
	if err = s.SaveWantedSchedule(ctx, "49", []metadata.Episode{ep}, nil, now); err != nil {
		t.Fatal(err)
	}
	if err = s.PreviewWanted(ctx, &entry, nil); err != nil || len(entry.Episodes) != 1 || len(entry.Episodes[0].Servers) != 0 {
		t.Fatalf("cached scoped metadata: %+v, %v", entry, err)
	}
	if view, err := s.ReleaseOverview(ctx, now, 10, 0); err != nil || view.UpcomingTotal != 0 {
		t.Fatalf("server-only schedule leaked into releases: %+v, %v", view, err)
	}
	observeWantedScope(t, s, "first", 1)
	if err = s.RefreshWanted(ctx, true); err != nil {
		t.Fatal(err)
	}
	if notification, err := s.ClaimWantedNotification(ctx, time.Now()); err != nil || notification == nil || notification.ServerName != "first" {
		t.Fatalf("expected arrival on first server only: %+v, %v", notification, err)
	}
}

func TestWantedPauseAndResumeBaseline(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	observeWantedScope(t, s, "first")
	entry, err := s.AddWanted(ctx, Wanted{Name: "Scope Show", Category: "series", AirAlerts: true,
		ProviderIDs: map[string]string{"tvmaze": "49"}})
	if err != nil {
		t.Fatal(err)
	}
	observeWantedScope(t, s, "first", 1)
	if err = s.RefreshWanted(ctx, true); err != nil {
		t.Fatal(err)
	}
	entry.Paused = true
	if entry, err = s.UpdateWanted(ctx, entry); err != nil {
		t.Fatal(err)
	}
	items, err := s.ListWanted(ctx)
	if err != nil || len(items) != 1 || !items[0].Paused || !items[0].AirAlerts || items[0].AlertMode != "episode" {
		t.Fatalf("pause did not preserve entry and alert settings: %+v, %v", items, err)
	}
	if providers, err := s.DueWantedSchedules(ctx, time.Now(), true); err != nil || len(providers) != 0 {
		t.Fatalf("paused entry has due public lookups: %v, %v", providers, err)
	}
	assertArrival := func(expected string) {
		t.Helper()
		if err := s.RefreshWanted(ctx, true); err != nil {
			t.Fatal(err)
		}
		item, err := s.ClaimWantedNotification(ctx, time.Now())
		if err != nil || (expected == "" && item != nil) || (expected != "" && (item == nil || item.Name != expected)) {
			t.Fatalf("wanted arrival %q: %+v, %v", expected, item, err)
		}
	}
	assertArrival("")
	observeWantedScope(t, s, "first", 1, 2)
	assertArrival("")
	entry.Paused = false
	if entry, err = s.UpdateWanted(ctx, entry); err != nil {
		t.Fatal(err)
	}
	assertArrival("")
	observeWantedScope(t, s, "first", 1, 2, 3)
	assertArrival("S01E03")
}
