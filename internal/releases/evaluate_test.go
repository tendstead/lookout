package releases

import (
	"github.com/tendstead/lookout/internal/metadata"
	"testing"
	"time"
)

func number(n int) *int { return &n }
func TestReleaseBaselineThenNewEpisodeAndSeason(t *testing.T) {
	base := time.Date(2026, 11, 14, 20, 0, 0, 0, time.UTC)
	episodes := []metadata.Episode{{ID: "old", Name: "Old", Season: 1, Number: number(1), AirStamp: base.Add(-time.Hour).Format(time.RFC3339)}, {ID: "new", Name: "New season", Season: 2, Number: number(1), AirStamp: base.Add(time.Hour).Format(time.RFC3339)}}
	state, events := Evaluate(State{}, episodes, base)
	if len(events) != 0 || !state.Initialized {
		t.Fatal("baseline spam")
	}
	state, events = Evaluate(state, episodes, base.Add(2*time.Hour))
	if len(events) != 2 || events[0].EpisodeID != "new" {
		t.Fatalf("missing episode/season: %#v", events)
	}
	_, events = Evaluate(state, episodes, base.Add(3*time.Hour))
	if len(events) != 0 {
		t.Fatal("duplicate poll spam")
	}
}
func TestAnimeUnknownDatesSpecialsAndCorrections(t *testing.T) {
	base := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	state, _ := Evaluate(State{}, nil, base)
	episodes := []metadata.Episode{{ID: "anime", Name: "Anime premiere", Season: 2, Number: number(1), AirStamp: base.Add(time.Hour).Format(time.RFC3339)}, {ID: "special", Name: "OVA", Season: 0, Number: nil, AirStamp: base.Add(time.Hour).Format(time.RFC3339)}, {ID: "unknown", Name: "Unknown date", Season: 3, Number: number(1)}, {ID: "late-old", Name: "Old metadata arrived late", Season: 1, Number: number(9), AirStamp: base.Add(-time.Hour).Format(time.RFC3339)}}
	state, events := Evaluate(state, episodes, base.Add(2*time.Hour))
	if len(events) != 3 {
		t.Fatalf("wrong regular/special event count: %#v", events)
	}
	episodes[0].AirStamp = base.Add(3 * time.Hour).Format(time.RFC3339)
	state, events = Evaluate(state, episodes, base.Add(2*time.Hour))
	if len(events) != 0 {
		t.Fatal("date correction repeated event")
	}
	_, events = Evaluate(state, episodes, base.Add(4*time.Hour))
	if len(events) != 0 {
		t.Fatal("corrected date re-announced same premiere")
	}
}
func TestLateNewEpisodeAndDateOnly(t *testing.T) {
	base := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	state, _ := Evaluate(State{}, nil, base)
	episodes := []metadata.Episode{{ID: "late", Season: 1, Number: number(2), AirStamp: base.Add(time.Hour).Format(time.RFC3339)}, {ID: "date", Season: 1, Number: number(3), AirDate: "2026-11-15"}}
	state, events := Evaluate(state, episodes, base.Add(2*time.Hour))
	if len(events) != 1 {
		t.Fatal("late post-baseline release missing")
	}
	_, events = Evaluate(state, episodes, time.Date(2026, 11, 15, 1, 0, 0, 0, time.UTC))
	if len(events) != 0 {
		t.Fatal("date-only episode declared aired before day completed")
	}
	_, events = Evaluate(state, episodes, time.Date(2026, 11, 16, 1, 0, 0, 0, time.UTC))
	if len(events) != 1 || events[0].Precision != "date" {
		t.Fatalf("date-only release missing: %#v", events)
	}
}
