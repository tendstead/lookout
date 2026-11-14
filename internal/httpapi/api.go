// SPDX-License-Identifier: Elastic-2.0
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

func jsonResponse(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}
func NewHandler(checkReady func(context.Context) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" && r.URL.Path != "/readyz" {
			jsonResponse(w, 404, map[string]any{"error": map[string]string{"code": "not_found", "message": "Not found"}})
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Allow", "GET, HEAD")
			jsonResponse(w, 405, map[string]string{"error": "Method not allowed"})
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
