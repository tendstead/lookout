// SPDX-License-Identifier: Elastic-2.0
package httpapi

import (
	"context"
	"errors"
	"github.com/tendstead/lookout/internal/notify"
	"github.com/tendstead/lookout/internal/store"
	"github.com/tendstead/lookout/internal/tracking"
	"net/http"
	"strings"
	"time"
)

type ReleaseService interface {
	Status() tracking.Status
	Overview(context.Context, int, int) (store.ReleaseOverview, error)
	Follow(context.Context, string) (store.Follow, error)
	SetFollow(context.Context, string, bool) error
	Queue() error
}
type TelegramService interface {
	Status() notify.Status
	Verify(context.Context) error
}

func WithReleaseTracking(releases ReleaseService, telegram TelegramService) Option {
	return func(o *apiOptions) { o.releases = releases; o.telegram = telegram }
}
func (o apiOptions) serveReleases(w http.ResponseWriter, r *http.Request) bool {
	p := r.URL.Path
	parts := strings.Split(strings.Trim(p, "/"), "/")
	if o.telegram != nil && p == "/api/v1/telegram/status" {
		if readMethod(w, r) {
			jsonResponse(w, 200, o.telegram.Status())
		}
		return true
	}
	if o.telegram != nil && p == "/api/v1/telegram/verify" {
		if !writeRequest(w, r) {
			return true
		}
		var input struct{}
		if !decodeJSON(w, r, &input) {
			return true
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if e := o.telegram.Verify(ctx); e != nil {
			apiError(w, 400, "telegram_unavailable", "Telegram bot/chat could not be verified")
		} else {
			jsonResponse(w, 200, o.telegram.Status())
		}
		return true
	}
	if o.releases == nil {
		return false
	}
	if p == "/api/v1/releases" {
		if !readMethod(w, r) {
			return true
		}
		q, e := titleQuery(r)
		if e != nil {
			apiError(w, 400, "invalid_query", "Invalid release query")
			return true
		}
		v, e := o.releases.Overview(r.Context(), q.Limit, q.Offset)
		if e != nil {
			apiError(w, 503, "releases_unavailable", "Release data unavailable")
		} else {
			jsonResponse(w, 200, map[string]any{"data": v, "progress": o.releases.Status()})
		}
		return true
	}
	if p == "/api/v1/releases/refresh" {
		if !writeRequest(w, r) {
			return true
		}
		var input struct{}
		if !decodeJSON(w, r, &input) {
			return true
		}
		if e := o.releases.Queue(); e != nil {
			apiError(w, 409, "refresh_unavailable", "Release refresh is disabled or already running")
		} else {
			jsonResponse(w, 202, map[string]string{"status": "queued"})
		}
		return true
	}
	if len(parts) == 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "titles" && parts[4] == "follow" {
		if r.Method == "GET" || r.Method == "HEAD" {
			v, e := o.releases.Follow(r.Context(), parts[3])
			if errors.Is(e, store.ErrNotFound) {
				apiError(w, 404, "not_found", "Not found")
			} else if e != nil {
				apiError(w, 503, "follow_unavailable", "Follow settings unavailable")
			} else {
				jsonResponse(w, 200, v)
			}
			return true
		}
		if !writeRequest(w, r) {
			return true
		}
		var input struct {
			Enabled *bool `json:"enabled"`
		}
		if !decodeJSON(w, r, &input) {
			return true
		}
		if input.Enabled == nil {
			apiError(w, 400, "invalid_follow", "Choose whether releases should be followed")
			return true
		}
		if e := o.releases.SetFollow(r.Context(), parts[3], *input.Enabled); e != nil {
			apiError(w, 400, "follow_failed", "Follow settings could not be saved")
		} else {
			jsonResponse(w, 200, map[string]bool{"enabled": *input.Enabled})
		}
		return true
	}
	return false
}
