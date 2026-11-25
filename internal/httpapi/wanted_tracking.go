// SPDX-License-Identifier: Elastic-2.0
package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tendstead/lookout/internal/metadata"
	"github.com/tendstead/lookout/internal/store"
)

type wantedTrackingStore interface {
	PreviewWanted(context.Context, *store.Wanted, []metadata.Episode) error
	FindTrackedTitle(context.Context, store.Wanted) (string, error)
	UpdateWanted(context.Context, store.Wanted) (store.Wanted, error)
}

func WithWantedTracking(s wantedTrackingStore, tv *metadata.TVmaze, allowPublic, allowEdits bool) Option {
	return func(o *apiOptions) {
		o.wantedTracking = s
		o.wantedTV = tv
		o.wantedPublic = allowPublic
		o.trackingEdits = allowEdits
	}
}

func trackingWritePath(path string) bool {
	if path == "/api/v1/wanted" || path == "/api/v1/wanted/preview" || path == "/api/v1/releases/refresh" {
		return true
	}
	parts := strings.Split(strings.TrimPrefix(path, "/api/v1/"), "/")
	return len(parts) == 3 && parts[1] != "" &&
		(parts[0] == "wanted" && (parts[2] == "settings" || parts[2] == "remove") ||
			parts[0] == "titles" && parts[2] == "follow")
}

func (o apiOptions) serveWantedTracking(w http.ResponseWriter, r *http.Request) bool {
	if o.wantedTracking == nil {
		return false
	}
	p := r.URL.Path
	if reference, ok := strings.CutPrefix(p, wantedArtworkPath); ok {
		o.wantedArtwork(w, r, reference)
		return true
	}
	if p == "/api/v1/wanted/preview" {
		if !writeRequest(w, r) {
			return true
		}
		var input struct {
			store.Wanted
			Input string `json:"input"`
		}
		if !decodeJSON(w, r, &input) {
			return true
		}
		ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
		defer cancel()
		draft, candidates, existing, err := o.previewTrackedTitle(ctx, input.Input, input.Wanted)
		if err != nil {
			apiError(w, 400, "invalid_tracking_preview", "Check the title or public HTTPS link")
		} else {
			o.populateWantedArtwork(&draft)
			jsonResponse(w, 200, map[string]any{"draft": draft, "candidates": candidates, "existing_id": existing})
		}
		return true
	}
	if !strings.HasPrefix(p, "/api/v1/wanted/") || !strings.HasSuffix(p, "/settings") {
		return false
	}
	if !writeRequest(w, r) {
		return true
	}
	var draft store.Wanted
	if !decodeJSON(w, r, &draft) {
		return true
	}
	draft.ID = strings.TrimSuffix(strings.TrimPrefix(p, "/api/v1/wanted/"), "/settings")
	saved, err := o.wantedTracking.UpdateWanted(r.Context(), draft)
	if errors.Is(err, store.ErrNotFound) {
		apiError(w, 404, "not_found", "Tracked title not found")
	} else if err != nil {
		apiError(w, 400, "invalid_wanted", "Check the target and alert settings")
	} else {
		if o.releases != nil && saved.Category == "series" && saved.ServerID == "" && !saved.Paused {
			_ = o.releases.Queue()
		}
		o.populateWantedArtwork(&saved)
		jsonResponse(w, 200, saved)
	}
	return true
}

func (o apiOptions) previewTrackedTitle(ctx context.Context, input string, draft store.Wanted) (store.Wanted, []metadata.Candidate, string, error) {
	input = strings.TrimSpace(input)
	candidates := []metadata.Candidate{}
	if len(input) > 2048 || draft.LocalTitleID == "" && input == "" && strings.TrimSpace(draft.Name) == "" {
		return draft, nil, "", errors.New("invalid preview input")
	}
	if len(draft.ServerID) > 100 {
		return draft, nil, "", errors.New("invalid tracking server")
	}
	if draft.ServerID != "" {
		draft.AirAlerts = false
	}
	publicMetadata := o.wantedPublic && o.wantedTV != nil && draft.ServerID == "" && !draft.Paused
	if draft.LocalTitleID != "" && o.catalogue != nil {
		local, err := o.catalogue.GetTitle(ctx, draft.LocalTitleID)
		if err != nil {
			return draft, nil, "", err
		}
		draft.LocalTitleID = local.ID
		draft.Name = local.Name
		draft.Year = local.Year
		draft.Category = "movie"
		if local.Category == "series" || local.Category == "anime_series" {
			draft.Category = "series"
		}
		draft.ProviderIDs = local.ProviderIDs
	}
	link, err := url.Parse(input)
	if err == nil && link.IsAbs() && link.Host != "" {
		link, err = metadata.TrackingSourceURL(input)
		if err != nil || link.Scheme != "https" {
			return draft, nil, "", errors.New("expected public HTTPS URL")
		}
		draft.SourceURL = link.String()
		if publicMetadata && len(draft.ProviderIDs) == 0 {
			resolution, err := o.wantedTV.ResolveTracking(ctx, input)
			if err == nil {
				applyTrackingCandidate(&draft, resolution.Title)
				draft.Category = resolution.Category
				if draft.Season == nil {
					draft.Season = resolution.Season
				}
				if draft.Episode == nil {
					draft.Episode = resolution.Episode
				}
				if resolution.SourceURL != "" {
					draft.SourceURL = resolution.SourceURL
				}
			}
		}
	} else if draft.Name == "" {
		draft.Name = input
	}
	if publicMetadata && draft.Name != "" && len(draft.ProviderIDs) == 0 &&
		draft.Category != "movie" && draft.SourceURL == "" {
		if items, err := o.wantedTV.Search(ctx, draft.Name); err == nil {
			candidates = append(candidates, items[:min(len(items), 8)]...)
			if exact, ok := metadata.ExactCandidate(items, draft.Name, 0); ok {
				applyTrackingCandidate(&draft, exact)
				draft.Category = "series"
			}
		}
	}
	if draft.Category == "" {
		draft.Category = "movie"
	}
	if draft.AlertMode == "" {
		draft.AlertMode = "episode"
	}
	if draft.Name == "" {
		draft.Servers = []store.WantedServer{}
		draft.Episodes = []store.WantedEpisode{}
		return draft, candidates, "", nil
	}
	var episodes []metadata.Episode
	if publicMetadata && draft.Category == "series" {
		if draft.ProviderIDs["tvmaze"] == "" && (draft.ProviderIDs["tvdb"] != "" || draft.ProviderIDs["imdb"] != "") {
			if match, err := o.wantedTV.Lookup(ctx, draft.ProviderIDs); err == nil {
				for provider, id := range match.ProviderIDs {
					if draft.ProviderIDs[provider] == "" {
						draft.ProviderIDs[provider] = id
					}
				}
			}
		}
		if id := draft.ProviderIDs["tvmaze"]; id != "" {
			episodes, _ = o.wantedTV.Episodes(ctx, id)
		}
	}
	// Gather the whole series once so scope suggestions can switch between
	// seasons and episodes even when the input already targets one of them.
	season, episode := draft.Season, draft.Episode
	draft.Season, draft.Episode = nil, nil
	if err := o.wantedTracking.PreviewWanted(ctx, &draft, episodes); err != nil {
		return draft, nil, "", err
	}
	draft.Season, draft.Episode = season, episode
	if draft.Category == "series" && draft.ServerID == "" && !draft.Paused && draft.LocalTitleID != "" && o.releases != nil {
		if follow, err := o.releases.Follow(ctx, draft.LocalTitleID); err == nil && follow.Enabled {
			draft.AirAlerts = true
		}
	}
	existing, err := o.wantedTracking.FindTrackedTitle(ctx, draft)
	if id := draft.ProviderIDs["tvmaze"]; err == nil && publicMetadata && draft.Category == "series" && id != "" {
		request, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		seasons, _ := o.wantedTV.Seasons(request, id)
		for _, season := range seasons {
			name := strings.TrimSpace(season.Name)
			if name != "" {
				if draft.SeasonNames == nil {
					draft.SeasonNames = make(map[int]string)
				}
				draft.SeasonNames[season.Number] = name
			}
		}
	}
	return draft, candidates, existing, err
}

func applyTrackingCandidate(draft *store.Wanted, candidate metadata.Candidate) {
	draft.Name = candidate.Name
	draft.ProviderIDs = candidate.ProviderIDs
	if candidate.Year != 0 {
		year := candidate.Year
		draft.Year = &year
	}
}
