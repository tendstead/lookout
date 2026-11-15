// SPDX-License-Identifier: Elastic-2.0
package releases

import (
	"github.com/tendstead/lookout/internal/metadata"
	"sort"
	"time"
)

type LocalNumbering struct {
	Season                 int
	Episodes               []int
	Numbering, Observation string
}
type SeasonInventory struct {
	ScheduleReady, InventoryReady, MatchReview, Anime bool
	Local                                             []LocalNumbering
	Episodes                                          []metadata.Episode
}
type SeasonDifference struct {
	Season          int    `json:"season"`
	Kind            string `json:"kind"`
	Aired           int    `json:"aired"`
	Present         int    `json:"present"`
	MissingEpisodes []int  `json:"missing_episodes"`
}
type SeasonComparison struct {
	LatestAired  int
	LocalSeasons []int
	Gaps         []SeasonDifference
	Reasons      []string
}

func CompareSeasons(in SeasonInventory, now time.Time) SeasonComparison {
	out := SeasonComparison{LocalSeasons: []int{}, Gaps: []SeasonDifference{}, Reasons: []string{}}
	reasons := map[string]bool{}
	if !in.ScheduleReady {
		reasons["schedule_unavailable"] = true
	}
	if !in.InventoryReady {
		reasons["inventory_unavailable"] = true
	}
	if in.MatchReview {
		reasons["metadata_conflict"] = true
	}
	if in.Anime {
		reasons["anime_ordering"] = true
	}
	local := map[int]map[int]bool{}
	for _, f := range in.Local {
		if f.Observation != "observed" && f.Observation != "stable" {
			reasons["unconfirmed_files"] = true
			continue
		}
		if f.Numbering != "season_episode" || f.Season < 0 || len(f.Episodes) == 0 {
			reasons["unknown_numbering"] = true
			continue
		}
		if f.Season == 0 {
			continue
		}
		for _, n := range f.Episodes {
			if n < 1 {
				reasons["unknown_numbering"] = true
				continue
			}
			if local[f.Season] == nil {
				local[f.Season] = map[int]bool{}
			}
			local[f.Season][n] = true
		}
	}
	for season := range local {
		out.LocalSeasons = append(out.LocalSeasons, season)
	}
	sort.Ints(out.LocalSeasons)
	aired := map[int]map[int]bool{}
	for _, e := range in.Episodes {
		stamp, _, known := ReportedTime(e)
		if !known || stamp.After(now) || e.Season < 1 || e.Number == nil || *e.Number < 1 {
			continue
		}
		if aired[e.Season] == nil {
			aired[e.Season] = map[int]bool{}
		}
		aired[e.Season][*e.Number] = true
		if e.Season > out.LatestAired {
			out.LatestAired = e.Season
		}
	}
	for r := range reasons {
		out.Reasons = append(out.Reasons, r)
	}
	sort.Strings(out.Reasons)
	if len(out.Reasons) > 0 {
		return out
	}
	seasons := []int{}
	for season := range aired {
		seasons = append(seasons, season)
	}
	sort.Ints(seasons)
	for _, season := range seasons {
		gap := SeasonDifference{Season: season, Aired: len(aired[season]), MissingEpisodes: []int{}}
		for number := range aired[season] {
			if local[season][number] {
				gap.Present++
			} else {
				gap.MissingEpisodes = append(gap.MissingEpisodes, number)
			}
		}
		if len(gap.MissingEpisodes) == 0 {
			continue
		}
		sort.Ints(gap.MissingEpisodes)
		gap.Kind = "partial"
		if gap.Present == 0 {
			gap.Kind = "missing"
		}
		out.Gaps = append(out.Gaps, gap)
	}
	return out
}
