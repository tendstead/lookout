// SPDX-License-Identifier: Elastic-2.0
package httpapi

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

func jsonResponse(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}
func apiError(w http.ResponseWriter, code int, id, message string) {
	jsonResponse(w, code, map[string]any{"error": map[string]string{"code": id, "message": message}})
}
func WithReadOnly() Option {
	return func(o *apiOptions) { o.readOnly = true }
}

func WithConfiguredSources(types map[string]string, sourceIDs []string) Option {
	return func(o *apiOptions) {
		o.serverTypes = types
		o.sourceIDs = sourceIDs
	}
}

func NewHandler(checkReady func(context.Context) error, assets fs.FS, options ...Option) http.Handler {
	var opts apiOptions
	for _, option := range options {
		option(&opts)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; object-src 'none'; frame-ancestors 'none'; base-uri 'self'")
		for _, part := range strings.Split(r.URL.Path, "/") {
			if part == ".." || part == "." {
				apiError(w, 400, "invalid_path", "Invalid path")
				return
			}
		}
		isHealth := r.URL.Path == "/healthz" || r.URL.Path == "/readyz"
		isAPI := r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/")
		if !isHealth && !isAPI {
			serveStatic(w, r, assets)
			return
		}
		if isAPI && opts.readOnly && r.Method != http.MethodGet && r.Method != http.MethodHead && !(opts.trackingEdits && trackingWritePath(r.URL.Path)) {
			w.Header().Set("Allow", "GET, HEAD")
			apiError(w, http.StatusMethodNotAllowed, "read_only", "Lookout is read-only. Manage server settings in lookout.toml.")
			return
		}
		if isAPI && opts.serveReleases(w, r) {
			return
		}
		if isAPI && opts.serveMetadata(w, r) {
			return
		}
		if isAPI && (opts.serveWantedTracking(w, r) || opts.serveMultiServer(w, r) || opts.serveCatalogue(w, r)) {
			return
		}
		if isAPI && r.URL.Path != "/api/v1/status" {
			apiError(w, 404, "not_found", "Not found")
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Allow", "GET, HEAD")
			apiError(w, 405, "method_not_allowed", "Method not allowed")
			return
		}
		if isAPI {
			stage := "foundation"
			if opts.catalogue != nil {
				stage = "catalogue"
			}
			jsonResponse(w, 200, map[string]any{"service": "lookout", "stage": stage, "read_only": opts.readOnly, "tracking_read_only": opts.readOnly && !opts.trackingEdits})
			return
		}
		status := "ok"
		code := 200
		if r.URL.Path == "/readyz" {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			status = "ready"
			if checkReady(ctx) != nil {
				code = 503
				status = "unavailable"
			}
		}
		jsonResponse(w, code, map[string]string{"status": status})
	})
}
