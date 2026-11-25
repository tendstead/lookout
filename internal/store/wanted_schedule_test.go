// SPDX-License-Identifier: Elastic-2.0
package store

import (
	"context"
	"testing"
	"time"

	"github.com/tendstead/lookout/internal/metadata"
)

func TestWantedScheduleAirsWithoutLibraryTitle(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	season, finale := 1, 9
	entry, err := s.AddWanted(ctx, Wanted{
		Name: "Severance", Category: "series", Season: &season, AirAlerts: true,
		ProviderIDs: map[string]string{"tvmaze": "44933"},
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	air := now.Add(time.Hour)
	episode := metadata.Episode{ID: "2302265", Name: "The We We Are", Season: season, Number: &finale, AirStamp: air.Format(time.RFC3339)}
	if err = s.SaveWantedSchedule(ctx, "44933", []metadata.Episode{episode}, []metadata.TrackingSeason{{Number: 1, EpisodeOrder: 9, EndDate: air.Format(time.DateOnly)}}, now); err != nil {
		t.Fatal(err)
	}
	if count, err := s.QueueWantedAirAlerts(ctx, now, true); err != nil || count != 0 {
		t.Fatalf("before air time: %d, %v", count, err)
	}
	overview, err := s.ReleaseOverview(ctx, now, 10, 0)
	if err != nil || overview.UpcomingTotal != 1 || len(overview.Upcoming) != 1 || overview.Upcoming[0].TrackedID != entry.ID {
		t.Fatalf("upcoming: %+v, %v", overview, err)
	}
	after := air.Add(time.Minute)
	if count, err := s.QueueWantedAirAlerts(ctx, after, true); err != nil || count != 1 {
		t.Fatalf("after air time: %d, %v", count, err)
	}
	overview, err = s.ReleaseOverview(ctx, after, 10, 0)
	if err != nil || overview.UpcomingTotal != 0 || overview.RecentTotal != 1 || len(overview.Recent) != 1 || overview.Recent[0].TrackedID != entry.ID {
		t.Fatalf("recent: %+v, %v", overview, err)
	}
	delivery, err := s.ClaimNotification(ctx, after)
	if err != nil || delivery == nil || !delivery.TrackedAir || delivery.Name != episode.Name || delivery.Kind != "episode" {
		t.Fatalf("aired notification: %+v, %v", delivery, err)
	}
	if err = s.MarkTrackedAirNotification(ctx, delivery.ID, true, 0, after); err != nil {
		t.Fatal(err)
	}
	if count, err := s.QueueWantedAirAlerts(ctx, after, true); err != nil || count != 0 {
		t.Fatalf("duplicate air event: %d, %v", count, err)
	}
	if delivery, err := s.ClaimNotification(ctx, after); err != nil || delivery != nil {
		t.Fatalf("duplicate aired notification: %+v, %v", delivery, err)
	}
}

func TestNativeAndWantedScheduleShareAirDelivery(t *testing.T) {
	s := testStore(t)
	titleID := releaseTitle(t, s)
	ctx := context.Background()
	if _, err := s.AddWanted(ctx, Wanted{
		Name: "Anime Show", Category: "series", AirAlerts: true,
		ProviderIDs: map[string]string{"tvmaze": "9"},
	}); err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC().Truncate(time.Second)
	if err := s.SetFollow(ctx, titleID, true, base); err != nil {
		t.Fatal(err)
	}
	follow, err := s.GetFollow(ctx, titleID)
	if err != nil {
		t.Fatal(err)
	}
	ep := 9
	episodes := []metadata.Episode{{ID: "finale", Name: "Finale", Season: 1, Number: &ep, AirStamp: base.Add(time.Hour).Format(time.RFC3339)}}
	if _, err = s.SaveEpisodeSnapshot(ctx, follow, episodes, base, true); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveWantedSchedule(ctx, "9", episodes, nil, base); err != nil {
		t.Fatal(err)
	}
	overview, err := s.ReleaseOverview(ctx, base, 10, 0)
	if err != nil || overview.UpcomingTotal != 1 || len(overview.Upcoming) != 1 || overview.Upcoming[0].TitleID != titleID || overview.Upcoming[0].TrackedID == "" {
		t.Fatalf("merged upcoming: %+v, %v", overview, err)
	}
	after := base.Add(2 * time.Hour)
	if _, err = s.QueueWantedAirAlerts(ctx, after, true); err != nil {
		t.Fatal(err)
	}
	delivery, err := s.ClaimNotification(ctx, after)
	if err != nil || delivery == nil || !delivery.TrackedAir {
		t.Fatalf("wanted claim: %+v, %v", delivery, err)
	}
	if _, err = s.SaveEpisodeSnapshot(ctx, follow, episodes, after, true); err != nil {
		t.Fatal(err)
	}
	if duplicate, err := s.ClaimNotification(ctx, after); err != nil || duplicate != nil {
		t.Fatalf("duplicate while first delivery is in progress: %+v, %v", duplicate, err)
	}
	if err = s.MarkTrackedAirNotification(ctx, delivery.ID, true, 0, after); err != nil {
		t.Fatal(err)
	}
	if duplicate, err := s.ClaimNotification(ctx, after); err != nil || duplicate != nil {
		t.Fatalf("duplicate after delivery: %+v, %v", duplicate, err)
	}
	overview, err = s.ReleaseOverview(ctx, after, 10, 0)
	if err != nil || overview.RecentTotal != 1 || len(overview.Recent) != 1 || overview.Recent[0].Delivery != "delivered" {
		t.Fatalf("merged recent: %+v, %v", overview, err)
	}
}

func TestNativeReleaseAvailabilityIsEpisodeSpecific(t *testing.T) {
	s := testStore(t)
	id := releaseTitle(t, s)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := s.SetFollow(ctx, id, true, now); err != nil {
		t.Fatal(err)
	}
	follow, err := s.GetFollow(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	available, missing := 2, 3
	air := now.Add(time.Hour).Format(time.RFC3339)
	episodes := []metadata.Episode{
		{ID: "has-file", Name: "Episode 2", Season: 1, Number: &available, AirStamp: air},
		{ID: "has-no-file", Name: "Episode 3", Season: 1, Number: &missing, AirStamp: air},
	}
	if _, err = s.SaveEpisodeSnapshot(ctx, follow, episodes, now, false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, "UPDATE library_files SET season=1, episodes=ARRAY[2], numbering='season_episode'"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, "UPDATE library_sources SET status='ready',last_success=$1", now); err != nil {
		t.Fatal(err)
	}
	check := func(expected map[string]string) {
		t.Helper()
		overview, err := s.ReleaseOverview(ctx, now, 10, 0)
		if err != nil || len(overview.Upcoming) != 2 {
			t.Fatalf("upcoming: %+v, %v", overview, err)
		}
		for _, episode := range overview.Upcoming {
			if episode.Availability != expected[episode.EpisodeID] {
				t.Errorf("%s availability = %q, want %q", episode.EpisodeID, episode.Availability, expected[episode.EpisodeID])
			}
		}
	}
	check(map[string]string{"has-file": "available", "has-no-file": "missing"})
	if _, err = s.pool.Exec(ctx, "UPDATE library_sources SET last_success=$1", now.Add(-48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	check(map[string]string{"has-file": "uncertain", "has-no-file": "uncertain"})
}
