package tracking

import (
	"github.com/tendstead/lookout/internal/store"
	"testing"
	"time"
)

func TestInitialProviderFailureHonoursBackoff(t *testing.T) {
	now := time.Now()
	f := store.Follow{Initialized: false, LastError: "provider unavailable", NextRefresh: now.Add(30 * time.Minute)}
	if refreshDue(f, now, false) {
		t.Fatal("failed initial schedule retried before backoff")
	}
	if !refreshDue(f, now, true) {
		t.Fatal("explicit refresh did not override backoff")
	}
	f.LastError = ""
	if !refreshDue(f, now, false) {
		t.Fatal("new uninitialized follow not checked")
	}
}
