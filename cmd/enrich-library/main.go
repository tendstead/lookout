// SPDX-License-Identifier: Elastic-2.0
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/tendstead/lookout/internal/config"
	"github.com/tendstead/lookout/internal/enrichment"
	"github.com/tendstead/lookout/internal/metadata"
	"github.com/tendstead/lookout/internal/store"
	"os"
	"strings"
	"time"
)

func main() {
	file := flag.String("config", "", "media-server credential JSON")
	mapping := flag.String("mappings", "", "explicit path mappings JSON")
	keyFile := flag.String("key", "", "32-byte local connection encryption key")
	fallback := flag.String("fallback-url", "", "authorized local media-server fallback URL")
	official := flag.Bool("official", false, "also query official keyless television metadata")
	flag.Parse()
	if e := run(*file, *mapping, *keyFile, *fallback, *official); e != nil {
		message := "Metadata import failed; check local configuration"
		if strings.HasPrefix(e.Error(), "provider ") {
			message = e.Error()
		}
		fmt.Fprintln(os.Stderr, message)
		os.Exit(1)
	}
}
func run(file, mapping, keyFile, fallback string, official bool) error {
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
	key, e := os.ReadFile(keyFile)
	if e != nil || len(key) != 32 {
		return errors.New("connection key missing")
	}
	service := enrichment.New(s, key)
	service.AllowPublic = official
	if file != "" {
		var c metadata.MediaServer
		raw, e := os.ReadFile(file)
		if e != nil {
			return e
		}
		if e = json.Unmarshal(raw, &c); e != nil {
			return e
		}
		c.Token = strings.TrimSpace(c.Token)
		if e = c.Verify(ctx); e != nil {
			if fallback == "" {
				return e
			}
			fmt.Println("Proxy unavailable to container:", e.Error())
			c.BaseURL = fallback
			if e = c.Verify(ctx); e != nil {
				return e
			}
			fmt.Println("Using authorized direct local media-server fallback")
		}
		raw, e = os.ReadFile(mapping)
		if e != nil {
			return e
		}
		if e = json.Unmarshal(raw, &c.Mappings); e != nil {
			return e
		}
		connection, e := s.SaveConnection(ctx, c, key)
		if e != nil {
			return e
		}
		report, e := service.ImportConnection(ctx, connection.ID)
		if e != nil {
			return e
		}
		printReport(report)
	}
	if official {
		report, e := service.RefreshOfficial(ctx)
		if e != nil {
			return e
		}
		printReport(report)
	}
	return nil
}
func printReport(r enrichment.Report) {
	fmt.Printf("%s: %d matched, %d need review, %d unmatched/outside scope\n", r.Phase, r.Matched, r.Review, r.Unmatched)
}
