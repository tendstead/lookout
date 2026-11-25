// SPDX-License-Identifier: Elastic-2.0
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/tendstead/lookout/internal/enrichment"
	"github.com/tendstead/lookout/internal/metadata"
	"github.com/tendstead/lookout/internal/store"
	"io"
	"net/http"
	"strings"
	"time"
)

type MetadataService interface {
	Status() enrichment.Report
	Capabilities() enrichment.Capabilities
	Connections(context.Context) ([]store.Connection, error)
	SaveConnection(context.Context, metadata.MediaServer) (store.Connection, error)
	VerifyConnection(context.Context, string) error
	Queue(string) error
	Search(context.Context, string, string) ([]metadata.Candidate, error)
	Match(context.Context, string, string, string) error
	Image(context.Context, string) ([]byte, string, error)
}

func WithMetadata(service MetadataService) Option {
	return func(o *apiOptions) { o.metadata = service }
}
func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 32768)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if e := decoder.Decode(target); e != nil {
		apiError(w, 400, "invalid_json", "Invalid JSON request")
		return false
	}
	var extra any
	if e := decoder.Decode(&extra); e != io.EOF {
		apiError(w, 400, "invalid_json", "One JSON value required")
		return false
	}
	return true
}
func (o apiOptions) serveMetadata(w http.ResponseWriter, r *http.Request) bool {
	if o.metadata == nil {
		return false
	}
	p := r.URL.Path
	parts := strings.Split(strings.Trim(p, "/"), "/")
	if p == "/api/v1/connections" {
		if r.Method == "GET" || r.Method == "HEAD" {
			items, e := o.metadata.Connections(r.Context())
			if e != nil {
				apiError(w, 503, "connections_unavailable", "Connections unavailable")
			} else {
				jsonResponse(w, 200, map[string]any{"items": items, "capabilities": o.metadata.Capabilities(), "progress": o.metadata.Status()})
			}
			return true
		}
		if !writeRequest(w, r) {
			return true
		}
		var c metadata.MediaServer
		if !decodeJSON(w, r, &c) {
			return true
		}
		connection, e := o.metadata.SaveConnection(r.Context(), c)
		if e != nil {
			apiError(w, 400, "invalid_connection", "Check the server URL, credentials and path mappings")
		} else {
			jsonResponse(w, 201, connection)
		}
		return true
	}
	if len(parts) == 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "connections" && (parts[4] == "verify" || parts[4] == "import") {
		if !writeRequest(w, r) {
			return true
		}
		var empty struct{}
		if !decodeJSON(w, r, &empty) {
			return true
		}
		if parts[4] == "verify" {
			ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
			defer cancel()
			if e := o.metadata.VerifyConnection(ctx, parts[3]); e != nil {
				apiError(w, 400, "connection_failed", "Could not verify the configured connection")
			} else {
				jsonResponse(w, 200, map[string]bool{"verified": true})
			}
		} else {
			if e := o.metadata.Queue(parts[3]); e != nil {
				apiError(w, 409, "metadata_busy", "Metadata worker unavailable or busy")
			} else {
				jsonResponse(w, 202, map[string]string{"status": "queued"})
			}
		}
		return true
	}
	if p == "/api/v1/metadata/official" {
		if !writeRequest(w, r) {
			return true
		}
		if !o.metadata.Capabilities().PublicMetadata {
			apiError(w, 403, "public_metadata_disabled", "Public metadata lookups are disabled")
			return true
		}
		var empty struct{}
		if !decodeJSON(w, r, &empty) {
			return true
		}
		if e := o.metadata.Queue("official"); e != nil {
			apiError(w, 409, "metadata_busy", "Metadata worker unavailable or busy")
		} else {
			jsonResponse(w, 202, map[string]string{"status": "queued"})
		}
		return true
	}
	if len(parts) >= 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "titles" {
		if len(parts) == 5 && parts[4] == "artwork" {
			if !readMethod(w, r) {
				return true
			}
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()
			data, mime, e := o.metadata.Image(ctx, parts[3])
			if e != nil {
				apiError(w, 404, "artwork_unavailable", "Artwork unavailable")
				return true
			}
			writeArtwork(w, r, data, mime, "private, no-cache")
			return true
		}
		if len(parts) == 6 && parts[4] == "metadata" && parts[5] == "search" {
			if !readMethod(w, r) {
				return true
			}
			if !o.metadata.Capabilities().PublicMetadata {
				apiError(w, 403, "public_metadata_disabled", "Public metadata lookups are disabled")
				return true
			}
			q := r.URL.Query().Get("q")
			if len(q) > 300 {
				apiError(w, 400, "invalid_query", "Search query too long")
				return true
			}
			ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
			defer cancel()
			items, e := o.metadata.Search(ctx, parts[3], q)
			if errors.Is(e, store.ErrNotFound) {
				apiError(w, 404, "not_found", "Not found")
			} else if e != nil {
				apiError(w, 503, "provider_unavailable", "Metadata provider unavailable")
			} else {
				jsonResponse(w, 200, map[string]any{"items": items})
			}
			return true
		}
		if len(parts) == 5 && parts[4] == "metadata" {
			if !writeRequest(w, r) {
				return true
			}
			if !o.metadata.Capabilities().PublicMetadata {
				apiError(w, 403, "public_metadata_disabled", "Public metadata lookups are disabled")
				return true
			}
			var input struct {
				Provider string `json:"provider"`
				ID       string `json:"id"`
			}
			if !decodeJSON(w, r, &input) {
				return true
			}
			if input.Provider != "tvmaze" || input.ID == "" {
				apiError(w, 400, "invalid_match", "Choose a supported metadata identity")
				return true
			}
			ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
			defer cancel()
			if e := o.metadata.Match(ctx, parts[3], input.Provider, input.ID); e != nil {
				apiError(w, 400, "match_failed", "Metadata match could not be saved")
			} else {
				jsonResponse(w, 200, map[string]bool{"saved": true})
			}
			return true
		}
	}
	return false
}
