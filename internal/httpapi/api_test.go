package httpapi

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealth(t *testing.T) {
	for _, tc := range []struct {
		path, method string
		down         bool
		code         int
		body         string
	}{
		{"/healthz", "GET", true, 200, "{\"status\":\"ok\"}\n"}, {"/readyz", "GET", false, 200, "{\"status\":\"ready\"}\n"}, {"/readyz", "GET", true, 503, "{\"status\":\"unavailable\"}\n"}, {"/healthz", "POST", false, 405, ""},
	} {
		t.Run(tc.path+tc.method+tc.body, func(t *testing.T) {
			h := NewHandler(func(ctx context.Context) error {
				d, ok := ctx.Deadline()
				if !ok || time.Until(d) > 2*time.Second {
					t.Error("missing bounded timeout")
				}
				if tc.down {
					return errors.New("private database info")
				}
				return nil
			}, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
			if w.Code != tc.code {
				t.Fatal(w.Code)
			}
			if tc.body != "" && w.Body.String() != tc.body {
				t.Fatal(w.Body.String())
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("cached health")
			}
		})
	}
}
func TestCancelledReadiness(t *testing.T) {
	h := NewHandler(func(ctx context.Context) error { return ctx.Err() }, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/readyz", nil).WithContext(ctx))
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
}
