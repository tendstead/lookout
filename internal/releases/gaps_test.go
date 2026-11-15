package releases

import (
	"github.com/tendstead/lookout/internal/metadata"
	"reflect"
	"testing"
	"time"
)

func TestSeasonGapsCompareAiredNumbersInsteadOfCounts(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	episode := func(id string, season, number int, stamp string) metadata.Episode {
		return metadata.Episode{ID: id, Season: season, Number: &number, AirStamp: stamp}
	}
	past := now.Add(-time.Hour).Format(time.RFC3339)
	in := SeasonInventory{ScheduleReady: true, InventoryReady: true, Local: []LocalNumbering{{Season: 1, Episodes: []int{1, 2}, Numbering: "season_episode", Observation: "observed"}, {Season: 2, Episodes: []int{1, 1, 99}, Numbering: "season_episode", Observation: "stable"}}, Episodes: []metadata.Episode{
		episode("a", 1, 1, past), episode("b", 1, 2, past), episode("c", 2, 1, past), episode("d", 2, 2, past), episode("duplicate", 2, 2, past), episode("e", 3, 1, past), episode("future", 4, 1, now.Add(time.Hour).Format(time.RFC3339)), episode("special", 0, 1, past), episode("unknown", 5, 1, "")}}
	got := CompareSeasons(in, now)
	if len(got.Reasons) != 0 || got.LatestAired != 3 || !reflect.DeepEqual(got.LocalSeasons, []int{1, 2}) {
		t.Fatalf("context: %+v", got)
	}
	if len(got.Gaps) != 2 || got.Gaps[0].Season != 2 || got.Gaps[0].Kind != "partial" || got.Gaps[0].Present != 1 || got.Gaps[0].Aired != 2 || !reflect.DeepEqual(got.Gaps[0].MissingEpisodes, []int{2}) || got.Gaps[1].Season != 3 || got.Gaps[1].Kind != "missing" {
		t.Fatalf("gaps: %+v", got.Gaps)
	}
}
func TestSeasonGapsNeverGuessUncertainInventory(t *testing.T) {
	now := time.Now().UTC()
	n := 1
	base := SeasonInventory{ScheduleReady: true, InventoryReady: true, Episodes: []metadata.Episode{{ID: "a", Season: 2, Number: &n, AirStamp: now.Add(-time.Hour).Format(time.RFC3339)}}}
	tests := []struct {
		name, reason string
		modify       func(*SeasonInventory)
	}{
		{"anime", "anime_ordering", func(i *SeasonInventory) { i.Anime = true }},
		{"no schedule", "schedule_unavailable", func(i *SeasonInventory) { i.ScheduleReady = false }},
		{"source failure", "inventory_unavailable", func(i *SeasonInventory) { i.InventoryReady = false }},
		{"conflict", "metadata_conflict", func(i *SeasonInventory) { i.MatchReview = true }},
		{"unknown file", "unknown_numbering", func(i *SeasonInventory) { i.Local = []LocalNumbering{{Numbering: "unknown", Observation: "observed"}} }},
		{"missing file", "unconfirmed_files", func(i *SeasonInventory) {
			i.Local = []LocalNumbering{{Season: 1, Episodes: []int{1}, Numbering: "season_episode", Observation: "missing"}}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := base
			tc.modify(&in)
			got := CompareSeasons(in, now)
			found := false
			for _, r := range got.Reasons {
				if r == tc.reason {
					found = true
				}
			}
			if !found || len(got.Gaps) != 0 {
				t.Fatalf("uncertainty discarded: %+v", got)
			}
		})
	}
}
func TestSeasonGapsDateOnlyAndCompleteSeason(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	n := 1
	in := SeasonInventory{ScheduleReady: true, InventoryReady: true, Local: []LocalNumbering{{Season: 1, Episodes: []int{1}, Numbering: "season_episode", Observation: "observed"}}, Episodes: []metadata.Episode{{ID: "a", Season: 1, Number: &n, AirDate: "2026-11-14"}, {ID: "b", Season: 2, Number: &n, AirDate: "2026-11-15"}}}
	got := CompareSeasons(in, now)
	if len(got.Gaps) != 0 || got.LatestAired != 1 {
		t.Fatalf("future date included: %+v", got)
	}
	got = CompareSeasons(in, now.Add(12*time.Hour))
	if len(got.Gaps) != 1 || got.Gaps[0].Season != 2 {
		t.Fatalf("finished date ignored: %+v", got)
	}
}
