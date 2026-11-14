// SPDX-License-Identifier: Elastic-2.0
package httpapi

import (
	"context"
	"errors"
	"github.com/tendstead/lookout/internal/store"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type LibraryStore interface {
	ListTitles(context.Context, store.TitleQuery) (store.TitlePage, error)
	GetTitle(context.Context, string) (store.TitleDetail, error)
	ListSources(context.Context) ([]store.Source, error)
}
type ScanRequester interface{ RequestScan(string) error }
type apiOptions struct {
	catalogue LibraryStore
	scans     ScanRequester
}
type Option func(*apiOptions)

func WithCatalogue(s LibraryStore, scans ScanRequester) Option {
	return func(o *apiOptions) { o.catalogue = s; o.scans = scans }
}
func readMethod(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != "GET" && r.Method != "HEAD" {
		w.Header().Set("Allow", "GET, HEAD")
		apiError(w, 405, "method_not_allowed", "Method not allowed")
		return false
	}
	return true
}
func writeRequest(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		apiError(w, 405, "method_not_allowed", "Method not allowed")
		return false
	}
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		apiError(w, 403, "origin_forbidden", "Same-origin request required")
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, e := url.Parse(origin)
		if e != nil || (u.Scheme != "http" && u.Scheme != "https") || !strings.EqualFold(u.Host, r.Host) {
			apiError(w, 403, "origin_forbidden", "Same-origin request required")
			return false
		}
	}
	if strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]) != "application/json" {
		apiError(w, 415, "json_required", "JSON request required")
		return false
	}
	return true
}
func titleQuery(r *http.Request) (store.TitleQuery, error) {
	q := store.TitleQuery{Search: strings.TrimSpace(r.URL.Query().Get("q")), Category: r.URL.Query().Get("category"), Limit: 48}
	var e error
	if v := r.URL.Query().Get("limit"); v != "" {
		q.Limit, e = strconv.Atoi(v)
		if e != nil {
			return q, e
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		q.Offset, e = strconv.Atoi(v)
		if e != nil {
			return q, e
		}
	}
	if len(q.Search) > 300 || q.Limit < 1 || q.Limit > 100 || q.Offset < 0 || q.Offset > 1000000 {
		return q, errors.New("invalid pagination")
	}
	switch q.Category {
	case "", "movie", "series", "anime_movie", "anime_series":
	default:
		return q, errors.New("invalid category")
	}
	return q, nil
}
func (o apiOptions) serveCatalogue(w http.ResponseWriter, r *http.Request) bool {
	if o.catalogue == nil {
		return false
	}
	p := r.URL.Path
	if p == "/api/v1/titles" {
		if !readMethod(w, r) {
			return true
		}
		q, e := titleQuery(r)
		if e != nil {
			apiError(w, 400, "invalid_query", "Invalid catalogue query")
			return true
		}
		v, e := o.catalogue.ListTitles(r.Context(), q)
		if e != nil {
			apiError(w, 503, "catalogue_unavailable", "Catalogue temporarily unavailable")
		} else {
			jsonResponse(w, 200, v)
		}
		return true
	}
	if strings.HasPrefix(p, "/api/v1/titles/") {
		if !readMethod(w, r) {
			return true
		}
		id := strings.TrimPrefix(p, "/api/v1/titles/")
		if strings.Contains(id, "/") {
			return false
		}
		v, e := o.catalogue.GetTitle(r.Context(), id)
		if errors.Is(e, store.ErrNotFound) {
			apiError(w, 404, "not_found", "Not found")
		} else if e != nil {
			apiError(w, 503, "catalogue_unavailable", "Catalogue temporarily unavailable")
		} else {
			jsonResponse(w, 200, v)
		}
		return true
	}
	if p == "/api/v1/sources" {
		if !readMethod(w, r) {
			return true
		}
		v, e := o.catalogue.ListSources(r.Context())
		if e != nil {
			apiError(w, 503, "catalogue_unavailable", "Catalogue temporarily unavailable")
		} else {
			jsonResponse(w, 200, map[string]any{"items": v})
		}
		return true
	}
	parts := strings.Split(strings.Trim(p, "/"), "/")
	if len(parts) == 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "sources" && parts[4] == "scan" {
		if !writeRequest(w, r) {
			return true
		}
		if o.scans == nil {
			apiError(w, 503, "scanner_unavailable", "Scanner not configured")
			return true
		}
		e := o.scans.RequestScan(parts[3])
		if errors.Is(e, store.ErrNotFound) {
			apiError(w, 404, "not_found", "Not found")
		} else if e != nil {
			apiError(w, 503, "scan_busy", "Scan queue unavailable")
		} else {
			jsonResponse(w, 202, map[string]string{"status": "queued", "source_id": parts[3]})
		}
		return true
	}
	return false
}
