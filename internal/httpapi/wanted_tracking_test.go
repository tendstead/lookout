// SPDX-License-Identifier: Elastic-2.0
package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tendstead/lookout/internal/metadata"
	"github.com/tendstead/lookout/internal/store"
)

type trackingPreviewStore struct{}

func (trackingPreviewStore) PreviewWanted(_ context.Context, w *store.Wanted, _ []metadata.Episode) error {
	w.Status = "missing"
	return nil
}

func (trackingPreviewStore) FindTrackedTitle(context.Context, store.Wanted) (string, error) {
	return "", nil
}

func (trackingPreviewStore) UpdateWanted(_ context.Context, w store.Wanted) (store.Wanted, error) {
	return w, nil
}

func TestTrackingEditsAreOptInInTOML(t *testing.T) {
	handler := NewHandler(nil, nil, WithReadOnly(), WithWantedTracking(trackingPreviewStore{}, nil, false, true))
	for _, example := range []struct {
		path, body, want string
		code             int
	}{
		{"/api/v1/wanted/preview", `{"input":"Unknown film"}`, `"name":"Unknown film"`, 200},
		{"/api/v1/wanted/preview", `{"input":"https://example.org/watch","name":"Unknown film"}`, `"source_url":"https://example.org/watch"`, 200},
		{"/api/v1/wanted/preview", `{"input":"https://127.0.0.1/"}`, "invalid_tracking_preview", 400},
		{"/api/v1/servers/set", `{}`, "read_only", 405},
	} {
		req := httptest.NewRequest(http.MethodPost, example.path, strings.NewReader(example.body))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		if recorder.Code != example.code || !strings.Contains(recorder.Body.String(), example.want) {
			t.Errorf("%s %s: got %d %s", example.path, example.body, recorder.Code, recorder.Body.String())
		}
	}
}

func TestScopedOrPausedTrackingPreviewSkipsPublicMetadata(t *testing.T) {
	var lookups atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lookups.Add(1)
		http.Error(w, "unexpected lookup", http.StatusNotFound)
	}))
	defer provider.Close()
	tv := &metadata.TVmaze{BaseURL: provider.URL, HTTP: provider.Client()}
	handler := NewHandler(nil, nil, WithWantedTracking(trackingPreviewStore{}, tv, true, true))
	for _, example := range []struct{ body, want string }{
		{`{"input":"Scope Show","category":"series","server_id":"primary","air_alerts":true}`, `"air_alerts":false`},
		{`{"input":"https://www.tvmaze.com/shows/44933/severance","name":"Scope Show","category":"series","server_id":"primary"}`, `"server_id":"primary"`},
		{`{"input":"Scope Show","category":"series","provider_ids":{"tvmaze":"44933"},"paused":true}`, `"paused":true`},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/wanted/preview", strings.NewReader(example.body))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), example.want) {
			t.Errorf("%s: got %d %s", example.body, recorder.Code, recorder.Body.String())
		}
	}
	if count := lookups.Load(); count != 0 {
		t.Fatalf("received %d public requests while previewing a scoped or paused entry", count)
	}
}
