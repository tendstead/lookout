// SPDX-License-Identifier: Elastic-2.0
package httpapi

import (
	"context"
	"errors"
	"github.com/tendstead/lookout/internal/store"
	"net/http"
	"strings"
)

type MultiServerStore interface {
	ListServersForSources(context.Context, []string) ([]store.Server, error)
	SaveServer(context.Context, store.Server) error
	ListWanted(context.Context) ([]store.Wanted, error)
	AddWanted(context.Context, store.Wanted) (store.Wanted, error)
	DeleteWanted(context.Context, string) error
	SetIdentity(context.Context, string, string) error
}

func WithMultiServer(s MultiServerStore) Option { return func(o *apiOptions) { o.multi = s } }
func (o apiOptions) serveMultiServer(w http.ResponseWriter, r *http.Request) bool {
	if o.multi == nil {
		return false
	}
	p := r.URL.Path
	if p == "/api/v1/servers" {
		if !readMethod(w, r) {
			return true
		}
		v, e := o.multi.ListServersForSources(r.Context(), o.sourceIDs)
		if e != nil {
			apiError(w, 503, "servers_unavailable", "Servers unavailable")
		} else {
			if o.serverTypes != nil {
				configured := v[:0]
				for _, server := range v {
					if kind, ok := o.serverTypes[server.ID]; ok {
						server.Type = kind
						configured = append(configured, server)
					}
				}
				v = configured
			}
			jsonResponse(w, 200, map[string]any{"items": v})
		}
		return true
	}
	if strings.HasPrefix(p, "/api/v1/servers/") {
		if !writeRequest(w, r) {
			return true
		}
		var v store.Server
		if !decodeJSON(w, r, &v) {
			return true
		}
		v.ID = strings.TrimPrefix(p, "/api/v1/servers/")
		e := o.multi.SaveServer(r.Context(), v)
		if errors.Is(e, store.ErrNotFound) {
			apiError(w, 404, "not_found", "Server not found")
		} else if e != nil {
			apiError(w, 400, "invalid_server", "Check name and scan/freshness intervals")
		} else {
			jsonResponse(w, 200, map[string]bool{"saved": true})
		}
		return true
	}
	if p == "/api/v1/wanted" {
		if r.Method == "GET" || r.Method == "HEAD" {
			v, e := o.multi.ListWanted(r.Context())
			if e != nil {
				apiError(w, 503, "wanted_unavailable", "Wanted unavailable")
			} else {
				jsonResponse(w, 200, map[string]any{"items": v})
			}
			return true
		}
		if !writeRequest(w, r) {
			return true
		}
		var v store.Wanted
		if !decodeJSON(w, r, &v) {
			return true
		}
		v, e := o.multi.AddWanted(r.Context(), v)
		if e != nil {
			apiError(w, 400, "invalid_wanted", "Check the title, category and target")
		} else {
			jsonResponse(w, 201, v)
		}
		return true
	}
	if strings.HasPrefix(p, "/api/v1/wanted/") && strings.HasSuffix(p, "/remove") {
		if !writeRequest(w, r) {
			return true
		}
		id := strings.TrimSuffix(strings.TrimPrefix(p, "/api/v1/wanted/"), "/remove")
		if e := o.multi.DeleteWanted(r.Context(), id); e != nil {
			apiError(w, 404, "not_found", "Wanted entry not found")
		} else {
			jsonResponse(w, 200, map[string]bool{"removed": true})
		}
		return true
	}
	if strings.HasPrefix(p, "/api/v1/titles/") && strings.HasSuffix(p, "/identity") {
		if !writeRequest(w, r) {
			return true
		}
		var v struct {
			TargetID string `json:"target_id"`
		}
		if !decodeJSON(w, r, &v) {
			return true
		}
		id := strings.TrimSuffix(strings.TrimPrefix(p, "/api/v1/titles/"), "/identity")
		if e := o.multi.SetIdentity(r.Context(), id, v.TargetID); e != nil {
			apiError(w, 400, "invalid_identity", "Choose a compatible title; conflicting identities cannot be merged")
		} else {
			jsonResponse(w, 200, map[string]bool{"saved": true})
		}
		return true
	}
	return false
}
