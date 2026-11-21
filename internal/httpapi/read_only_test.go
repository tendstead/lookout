package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadOnlyRejectsWritesBeforeRouting(t *testing.T) {
	h := NewHandler(func(context.Context) error { return nil }, nil, WithReadOnly())
	for _, route := range []string{"/api/v1/servers/home", "/api/v1/wanted", "/api/v1/connections", "/api/v1/sources/films/scan"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, route, nil))
		if w.Code != http.StatusMethodNotAllowed || !strings.Contains(w.Body.String(), "read_only") {
			t.Fatalf("POST %s returned %d", route, w.Code)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/status", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"read_only":true`) {
		t.Fatal("GET status must advertise read-only mode")
	}
}
