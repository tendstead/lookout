package enrichment

import (
	"context"
	"github.com/tendstead/lookout/internal/metadata"
	"github.com/tendstead/lookout/internal/store"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIOnlyInventoryDoesNotRequireSFTP(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/System/Info" {
			w.Write([]byte(`{"ServerName":"Family Jellyfin"}`))
			return
		}
		if r.URL.Query().Get("IncludeItemTypes") == "Episode" {
			w.Write([]byte(`{"TotalRecordCount":1,"Items":[{"Id":"ep","ParentIndexNumber":1,"IndexNumber":2}]}`))
			return
		}
		w.Write([]byte(`{"TotalRecordCount":2,"Items":[{"Id":"movie","Name":"Film","Type":"Movie","ProviderIds":{"Imdb":"tt1"}},{"Id":"show","Name":"Show","Type":"Series","ProviderIds":{"Tvdb":"7"}}]}`))
	}))
	defer api.Close()
	key := make([]byte, 32)
	service := New(s, key)
	c, e := service.SaveConnection(ctx, metadata.MediaServer{Kind: "jellyfin", BaseURL: api.URL, Token: "fixture"})
	if e != nil {
		t.Fatal(e)
	}
	for range 2 {
		if _, e = service.ImportConnection(ctx, c.ID); e != nil {
			t.Fatal(e)
		}
	}
	page, e := s.ListTitles(ctx, store.TitleQuery{})
	if e != nil || page.Total != 2 {
		t.Fatal(page, e)
	}
	for _, title := range page.Items {
		if len(title.Servers) != 1 || title.Servers[0].Name != "Family Jellyfin" {
			t.Fatal(title)
		}
		d, e := s.GetTitle(ctx, title.ID)
		if e != nil || len(d.Files) != 1 {
			t.Fatal(d, e)
		}
		if title.Category == "series" && (d.Files[0].Season == nil || *d.Files[0].Season != 1 || d.Files[0].Episodes[0] != 2) {
			t.Fatal(d)
		}
	}
}
