package httpapi

import (
	"context"
	"github.com/tendstead/lookout/internal/enrichment"
	"github.com/tendstead/lookout/internal/metadata"
	"github.com/tendstead/lookout/internal/store"
	"net/http/httptest"
	"strings"
	"testing"
)

type metadataFixture struct {
	received string
	public   bool
}

func (f *metadataFixture) Status() enrichment.Report { return enrichment.Report{} }
func (f *metadataFixture) Capabilities() enrichment.Capabilities {
	return enrichment.Capabilities{ConnectionsReady: true, PublicMetadata: f.public}
}
func (f *metadataFixture) Connections(context.Context) ([]store.Connection, error) {
	return []store.Connection{{ID: "id", Kind: "jellyfin", BaseURL: "http://local", TokenConfigured: true}}, nil
}
func (f *metadataFixture) SaveConnection(_ context.Context, c metadata.MediaServer) (store.Connection, error) {
	f.received = c.Token
	return store.Connection{ID: "id", Kind: c.Kind, TokenConfigured: true}, nil
}
func (f *metadataFixture) VerifyConnection(context.Context, string) error { return nil }
func (f *metadataFixture) Queue(string) error                             { return nil }
func (f *metadataFixture) Search(context.Context, string, string) ([]metadata.Candidate, error) {
	return []metadata.Candidate{{ID: "9", Provider: "tvmaze", Name: "Example"}}, nil
}
func (f *metadataFixture) Match(context.Context, string, string, string) error { return nil }
func (f *metadataFixture) Image(context.Context, string) ([]byte, string, error) {
	return []byte{137, 80, 78, 71}, "image/png", nil
}
func TestMetadataAPIProtectsSecretsAndPublicPermission(t *testing.T) {
	f := &metadataFixture{}
	h := NewHandler(func(context.Context) error { return nil }, nil, WithMetadata(f))
	for _, tc := range []struct {
		method, path, body string
		code               int
	}{
		{"GET", "/api/v1/connections", "", 200}, {"POST", "/api/v1/connections", `{"kind":"jellyfin","url":"http://local","token":"private-token","path_mappings":[]}`, 201}, {"GET", "/api/v1/titles/id/metadata/search", "", 403}, {"POST", "/api/v1/connections/id/verify", "{}", 200}, {"POST", "/api/v1/metadata/official", "{}", 403}, {"POST", "/api/v1/connections", `{"kind":"jellyfin","token":"private-token","unknown":true}`, 400},
	} {
		r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.code || strings.Contains(w.Body.String(), "private-token") {
			t.Fatalf("%s: %d %s", tc.path, w.Code, w.Body.String())
		}
	}
	if f.received != "private-token" {
		t.Fatal("save not called")
	}
}
