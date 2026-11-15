package httpapi

import (
	"context"
	"github.com/tendstead/lookout/internal/notify"
	"github.com/tendstead/lookout/internal/store"
	"github.com/tendstead/lookout/internal/tracking"
	"net/http/httptest"
	"strings"
	"testing"
)

type releaseFixture struct {
	enabled  bool
	verified int
}

func (f *releaseFixture) Status() tracking.Status { return tracking.Status{Allowed: true} }
func (f *releaseFixture) Overview(context.Context, int, int) (store.ReleaseOverview, error) {
	return store.ReleaseOverview{Following: 2, Ready: 1, Unmatched: 1, Upcoming: []store.UpcomingEpisode{}, Recent: []store.ReleaseEvent{}}, nil
}
func (f *releaseFixture) Follow(context.Context, string) (store.Follow, error) {
	return store.Follow{Enabled: f.enabled}, nil
}
func (f *releaseFixture) SetFollow(_ context.Context, _ string, enabled bool) error {
	f.enabled = enabled
	return nil
}
func (f *releaseFixture) Queue() error { return nil }

type telegramFixture struct{ checks int }

func (f *telegramFixture) Status() notify.Status {
	return notify.Status{Configured: true, Verified: true}
}
func (f *telegramFixture) Verify(context.Context) error { f.checks++; return nil }
func TestReleaseAndReadOnlyTelegramVerificationAPI(t *testing.T) {
	release := &releaseFixture{}
	telegram := &telegramFixture{}
	h := NewHandler(func(context.Context) error { return nil }, nil, WithReleaseTracking(release, telegram))
	for _, tc := range []struct {
		method, path, body string
		code               int
	}{
		{"GET", "/api/v1/releases", "", 200}, {"GET", "/api/v1/releases?limit=101", "", 400}, {"POST", "/api/v1/titles/id/follow", `{"enabled":true}`, 200}, {"POST", "/api/v1/titles/id/follow", `{}`, 400}, {"GET", "/api/v1/telegram/status", "", 200}, {"POST", "/api/v1/telegram/verify", `{}`, 200}, {"POST", "/api/v1/telegram/send", `{"text":"not allowed"}`, 404},
	} {
		r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Fatalf("%s %d %s", tc.path, w.Code, w.Body.String())
		}
	}
	if !release.enabled || telegram.checks != 1 {
		t.Fatal("unexpected follow/verify action")
	}
}
