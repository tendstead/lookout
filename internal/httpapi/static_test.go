package httpapi

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestStaticBoundary(t *testing.T) {
	assets := fstest.MapFS{"index.html": {Data: []byte("<html>Lookout</html>")}, "assets/app-12345678.js": {Data: []byte("export{}")}}
	h := NewHandler(func(context.Context) error { return nil }, assets)
	for _, tc := range []struct {
		method, path string
		code         int
		content      string
	}{
		{"GET", "/", 200, "<html>Lookout"}, {"GET", "/library", 200, "<html>Lookout"}, {"GET", "/missing.js", 404, "Not found"}, {"GET", "/api/v1/nope", 404, "not_found"}, {"POST", "/api/v1/status", 405, "method_not_allowed"}, {"GET", "/api/v1/status", 200, "foundation"}, {"GET", "/assets/app-12345678.js", 200, "export{}"}, {"GET", "/../go.mod", 400, "invalid_path"},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
			if w.Code != tc.code || !strings.Contains(w.Body.String(), tc.content) {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
			if strings.HasPrefix(tc.path, "/api/") && !strings.Contains(w.Header().Get("Content-Type"), "json") {
				t.Fatal("API served HTML")
			}
			if tc.path == "/" && w.Header().Get("Cache-Control") != "no-cache" {
				t.Fatal("HTML cached")
			}
			if strings.HasPrefix(tc.path, "/assets/") && !strings.Contains(w.Header().Get("Cache-Control"), "immutable") {
				t.Fatal("hash asset not cached")
			}
		})
	}
}
