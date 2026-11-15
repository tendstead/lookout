package httpapi

import (
	"context"
	"github.com/tendstead/lookout/internal/store"
	"net/http/httptest"
	"strings"
	"testing"
)

type catalogueFixture struct {
	queued int
	query  store.TitleQuery
}

func (f *catalogueFixture) ListTitles(_ context.Context, q store.TitleQuery) (store.TitlePage, error) {
	f.query = q
	return store.TitlePage{Items: []store.Title{{ID: "title-1", Name: "Example", FileCount: 4}}, Total: 1, Limit: q.Limit}, nil
}
func (f *catalogueFixture) GetTitle(_ context.Context, id string) (store.TitleDetail, error) {
	if id != "title-1" {
		return store.TitleDetail{}, store.ErrNotFound
	}
	return store.TitleDetail{Title: store.Title{ID: id, Name: "Example"}, Files: []store.MediaFile{}}, nil
}
func (f *catalogueFixture) ListSources(context.Context) ([]store.Source, error) {
	return []store.Source{}, nil
}
func (f *catalogueFixture) RequestScan(id string) error {
	if id != "series" {
		return store.ErrNotFound
	}
	f.queued++
	return nil
}
func TestCatalogueAPI(t *testing.T) {
	f := &catalogueFixture{}
	handler := NewHandler(func(context.Context) error { return nil }, nil, WithCatalogue(f, f))
	for _, tc := range []struct {
		method, url, origin, contentType string
		code                             int
		contains                         string
	}{
		{"GET", "/api/v1/titles?limit=24", "", "", 200, "Example"},
		{"GET", "/api/v1/titles?limit=101", "", "", 400, "invalid_query"},
		{"GET", "/api/v1/titles?offset=-1", "", "", 400, "invalid_query"},
		{"GET", "/api/v1/titles/title-1", "", "", 200, "files"},
		{"GET", "/api/v1/titles/missing", "", "", 404, "not_found"},
		{"POST", "/api/v1/sources/series/scan", "https://attacker.invalid", "application/json", 403, "origin_forbidden"},
		{"POST", "/api/v1/sources/series/scan", "", "text/plain", 415, "json_required"},
		{"POST", "/api/v1/sources/series/scan", "", "application/json", 202, "queued"},
		{"POST", "/api/v1/sources/missing/scan", "", "application/json", 404, "not_found"},
	} {
		t.Run(tc.method+tc.url+tc.origin+tc.contentType, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.url, strings.NewReader("{}"))
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != tc.code || !strings.Contains(w.Body.String(), tc.contains) {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
		})
	}
	if f.queued != 1 {
		t.Fatal("unexpected scan execution")
	}
}

func TestSuggestionsAreBoundedLocalCatalogueLookups(t *testing.T) {
	f := &catalogueFixture{}
	h := NewHandler(func(context.Context) error { return nil }, nil, WithCatalogue(f, f))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/titles/suggestions?q=Example&category=movie&limit=100", nil))
	if w.Code != 200 || !f.query.LocalOnly || f.query.Limit != 8 || f.query.Search != "Example" || f.query.Category != "movie" || !strings.Contains(w.Body.String(), "Example") {
		t.Fatalf("not a bounded local lookup: %d %+v %s", w.Code, f.query, w.Body.String())
	}
	for _, path := range []string{"/api/v1/titles/suggestions?q=x", "/api/v1/titles/suggestions?q="} {
		w = httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"items":[]`) {
			t.Fatal("short query should be empty", w.Code, w.Body.String())
		}
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/titles/suggestions?q=Example&category=invalid", nil))
	if w.Code != 400 {
		t.Fatal("invalid query accepted", w.Code)
	}
}
