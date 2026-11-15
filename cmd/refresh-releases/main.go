// SPDX-License-Identifier: Elastic-2.0
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/tendstead/lookout/internal/config"
	"github.com/tendstead/lookout/internal/metadata"
	"github.com/tendstead/lookout/internal/store"
	"github.com/tendstead/lookout/internal/tracking"
	"os"
	"time"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "Schedule refresh failed; cached catalogue remains intact")
		os.Exit(1)
	}
}
func run() error {
	cfg, e := config.Load(os.LookupEnv, os.ReadFile)
	if e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	s, e := store.Open(ctx, cfg)
	if e != nil {
		return e
	}
	defer s.Close()
	if e = s.Migrate(ctx); e != nil {
		return e
	}
	service := tracking.New(s, metadata.NewTVmaze(), os.Getenv("ALLOW_PUBLIC_METADATA") == "true", true, func() bool { return false })
	if e = service.Refresh(ctx, true); e != nil {
		return e
	}
	overview, e := service.Overview(ctx, 100, 0)
	if e != nil {
		return e
	}
	report := map[string]any{"following": overview.Following, "ready": overview.Ready, "unmatched": overview.Unmatched, "upcoming_total": overview.UpcomingTotal, "new_events": overview.RecentTotal, "refresh": service.Status(), "telegram_sends": 0}
	return json.NewEncoder(os.Stdout).Encode(report)
}
