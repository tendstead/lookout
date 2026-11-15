// SPDX-License-Identifier: Elastic-2.0
package releases

import (
	"github.com/tendstead/lookout/internal/metadata"
	"time"
)

type State struct {
	Initialized bool
	BaselineAt  time.Time
	Emitted     map[string]bool
}
type Event struct {
	Key        string    `json:"key"`
	EpisodeID  string    `json:"episode_id"`
	Kind       string    `json:"kind"`
	Name       string    `json:"name"`
	Season     int       `json:"season"`
	Number     *int      `json:"number"`
	ReportedAt time.Time `json:"reported_at"`
	Precision  string    `json:"precision"`
	URL        string    `json:"url"`
}

// ReportedTime is provider evidence of airing, never evidence of a local file.
// Date-only observations become due after that UTC date has finished.
func ReportedTime(e metadata.Episode) (time.Time, string, bool) {
	if stamp, err := time.Parse(time.RFC3339, e.AirStamp); err == nil {
		return stamp.UTC(), "time", true
	}
	if date, err := time.Parse("2006-01-02", e.AirDate); err == nil {
		return date.Add(24 * time.Hour), "date", true
	}
	return time.Time{}, "unknown", false
}
func Evaluate(previous State, episodes []metadata.Episode, now time.Time) (State, []Event) {
	state := previous
	state.Emitted = map[string]bool{}
	for k, v := range previous.Emitted {
		state.Emitted[k] = v
	}
	out := []Event{}
	if !previous.Initialized {
		state.Initialized = true
		state.BaselineAt = now.UTC()
		return state, out
	}
	for _, episode := range episodes {
		if episode.ID == "" {
			continue
		}
		stamp, precision, ok := ReportedTime(episode)
		if !ok || stamp.After(now) || !stamp.After(state.BaselineAt) {
			continue
		}
		kinds := []string{"episode"}
		if episode.Season > 0 && episode.Number != nil && *episode.Number == 1 {
			kinds = append(kinds, "season")
		}
		for _, kind := range kinds {
			key := kind + ":" + episode.ID
			if state.Emitted[key] {
				continue
			}
			state.Emitted[key] = true
			out = append(out, Event{Key: key, EpisodeID: episode.ID, Kind: kind, Name: episode.Name, Season: episode.Season, Number: episode.Number, ReportedAt: stamp, Precision: precision, URL: episode.URL})
		}
	}
	return state, out
}
