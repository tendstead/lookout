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

func TestTrackingSeveranceArrivalAcrossServers(t *testing.T) {
	for _, mode := range []string{"episode", "season"} {
		t.Run(mode, func(t *testing.T) {
			testTrackingSeveranceArrival(t, mode)
		})
	}
}

func testTrackingSeveranceArrival(t *testing.T, mode string) {
	t.Helper()
	s := testStore(t)
	ctx := context.Background()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{"tvdb": "371980", "tvmaze": "44933"}
	observe := func(server string, numbers ...int) {
		t.Helper()
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
		for _, n := range numbers {
			if err := s.Observe(ctx, scan, library.FileObservation{
				SourceID: server, Group: "Severance", RelativePath: fmt.Sprintf("s01e%02d.mkv", n),
				Category: "series", Title: library.Metadata{Title: "Severance", ProviderIDs: ids},
				Season: 1, Episodes: []int{n}, Numbering: "season_episode",
			}); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.FinishScan(ctx, scan, server, library.ScanStats{Media: len(numbers)}, nil); err != nil {
			t.Fatal(err)
		}
	}
	observe("first", 1, 2, 3, 4)
	observe("second", 5, 6, 7, 8)
	season := 1
	var episodes []metadata.Episode
	for number := 1; number <= 9; number++ {
		episode := metadata.Episode{ID: fmt.Sprint(number), Name: fmt.Sprintf("Episode %d", number), Season: season, Number: &number}
		episodes = append(episodes, episode)
	}
	episodes[8].Name = "The We We Are"
	seasonMetadata := []metadata.TrackingSeason{{Number: season, EpisodeOrder: 9, EndDate: "2022-04-08"}}
	if err := s.SaveWantedSchedule(ctx, "44933", episodes, seasonMetadata, time.Now()); err != nil {
		t.Fatal(err)
	}
	draft := Wanted{Name: "Severance", Category: "series", ProviderIDs: ids, Season: &season, AlertMode: mode}
	if err := s.PreviewWanted(ctx, &draft, nil); err != nil {
		t.Fatal(err)
	}
	if len(draft.Episodes) != 9 || len(draft.Servers) != 2 || len(draft.Episodes[8].Servers) != 0 {
		t.Fatalf("unexpected Severance preview: %+v", draft)
	}
	for _, episode := range draft.Episodes[:8] {
		if len(episode.Servers) != 1 {
			t.Fatalf("expected one server for %+v", episode)
		}
	}
	if _, err := s.AddWanted(ctx, draft); err != nil {
		t.Fatal(err)
	}
	assertDelivery := func(episode string) {
		t.Helper()
		if err := s.RefreshWanted(ctx, true); err != nil {
			t.Fatal(err)
		}
		now := time.Now()
		d, err := s.ClaimWantedNotification(ctx, now)
		if err != nil || episode == "" && d != nil || episode != "" && (d == nil || d.Name != episode) {
			t.Fatalf("expected %q, got %+v, %v", episode, d, err)
		}
		if d != nil {
			if err := s.MarkWantedNotification(ctx, d.ID, true, 0, now, d.WantedIDs...); err != nil {
				t.Fatal(err)
			}
		}
	}
	assertDelivery("")
	observe("second", 5, 6, 7, 8, 9)
	expected := "S01E09"
	if mode == "season" {
		expected = "Season 1"
	}
	assertDelivery(expected)
	observe("first", 1, 2, 3, 4, 9)
	assertDelivery("")
}

func TestTrackingTakesOverAirAlertSettingFromLegacyFollow(t *testing.T) {
	s := testStore(t)
	id := releaseTitle(t, s)
	ctx := context.Background()
	if err := s.SetFollow(ctx, id, true, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddWanted(ctx, Wanted{Name: "Anime Show", Category: "series", LocalTitleID: id}); err != nil {
		t.Fatal(err)
	}
	follow, err := s.GetFollow(ctx, id)
	if err != nil || follow.Enabled {
		t.Fatalf("legacy follow still controls air alerts: %+v %v", follow, err)
	}
}

func TestTrackingScopeCancelsArrivalsOutsideNewEpisode(t *testing.T) {
	s := testStore(t)
	_ = releaseTitle(t, s)
	ctx := context.Background()
	entry, err := s.AddWanted(ctx, Wanted{Name: "Anime Show", Category: "series"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `INSERT INTO wanted_arrivals(wanted_id,server_id,content_key,description,status)
		VALUES($1,'legacy','1:1','S01E01','pending'),($1,'legacy','1:2','S01E02','retry')`, entry.ID); err != nil {
		t.Fatal(err)
	}
	season, episode := 1, 2
	entry.Season, entry.Episode = &season, &episode
	if _, err = s.UpdateWanted(ctx, entry); err != nil {
		t.Fatal(err)
	}
	delivery, err := s.ClaimWantedNotification(ctx, time.Now())
	if err != nil || delivery == nil || delivery.Name != "S01E02" {
		t.Fatalf("expected only the episode still in scope: %+v %v", delivery, err)
	}
}

func TestWantedExplicitLocalIdentityDoesNotBorrowSameName(t *testing.T) {
	entry := Wanted{Name: "Severance", Category: "series", LocalTitleID: "first"}
	if match, _ := wantedMatches(entry, Title{ID: "first", Name: entry.Name, Category: entry.Category}); !match {
		t.Fatal("the selected local title did not match")
	}
	if match, _ := wantedMatches(entry, Title{ID: "second", Name: entry.Name, Category: entry.Category}); match {
		t.Fatal("a different same-name title matched an explicitly selected identity")
	}
}

func TestTrackingReplacingIdentityClearsQueuedOldArrivals(t *testing.T) {
	s := testStore(t)
	_ = releaseTitle(t, s)
	ctx := context.Background()
	entry, err := s.AddWanted(ctx, Wanted{Name: "Original", Category: "series"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `INSERT INTO wanted_arrivals(wanted_id,server_id,content_key,description,status)
		VALUES($1,'legacy','1:1','S01E01','pending')`, entry.ID); err != nil {
		t.Fatal(err)
	}
	entry.Name = "Replacement"
	if _, err = s.UpdateWanted(ctx, entry); err != nil {
		t.Fatal(err)
	}
	if delivery, err := s.ClaimWantedNotification(ctx, time.Now()); err != nil || delivery != nil {
		t.Fatalf("replacement inherited an obsolete alert: %+v %v", delivery, err)
	}
}
