// SPDX-License-Identifier: Elastic-2.0
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/tendstead/lookout/internal/config"
	"github.com/tendstead/lookout/internal/library"
	"github.com/tendstead/lookout/internal/store"
	"os"
	"time"
)

func main() {
	cfgFile := flag.String("config", "", "local source config")
	manifest := flag.String("manifest", "", "completed local JSONL scan manifest")
	flag.Parse()
	if e := run(*cfgFile, *manifest); e != nil {
		fmt.Fprintln(os.Stderr, "Import failed:", e)
		os.Exit(1)
	}
}
func run(cfgFile, manifest string) error {
	if cfgFile == "" || manifest == "" {
		return errors.New("config and manifest are required")
	}
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
	var sources struct {
		Roots []library.Root `json:"roots"`
	}
	raw, e := os.ReadFile(cfgFile)
	if e != nil {
		return e
	}
	if e = json.Unmarshal(raw, &sources); e != nil {
		return e
	}
	summaries := map[string]library.ScanStats{}
	raw, e = os.ReadFile(manifest + ".summary.json")
	if e != nil {
		return errors.New("completed scan summary required")
	}
	if e = json.Unmarshal(raw, &summaries); e != nil {
		return e
	}
	scans := map[string]string{}
	stats := map[string]library.ScanStats{}
	for _, root := range sources.Roots {
		if _, ok := summaries[root.ID]; !ok {
			return errors.New("manifest does not cover all configured roots")
		}
		if e = s.EnsureSource(ctx, root); e != nil {
			return e
		}
		id, e := s.StartScan(ctx, root.ID)
		if e != nil {
			return e
		}
		scans[root.ID] = id
	}
	f, e := os.Open(manifest)
	if e != nil {
		return e
	}
	defer f.Close()
	reader := bufio.NewScanner(f)
	reader.Buffer(make([]byte, 65536), 4*1024*1024)
	for reader.Scan() {
		var observation library.FileObservation
		if e = json.Unmarshal(reader.Bytes(), &observation); e != nil {
			return e
		}
		scan, ok := scans[observation.SourceID]
		if !ok {
			return errors.New("unexpected source in scan manifest")
		}
		if e = s.Observe(ctx, scan, observation); e != nil {
			return e
		}
		v := stats[observation.SourceID]
		v.Media++
		stats[observation.SourceID] = v
	}
	if e = reader.Err(); e != nil {
		return e
	}
	for _, root := range sources.Roots {
		if stats[root.ID].Media != summaries[root.ID].Media+summaries[root.ID].ArchivedMedia {
			return errors.New("manifest count disagrees with completed summary")
		}
		if e = s.FinishScan(ctx, scans[root.ID], root.ID, summaries[root.ID], nil); e != nil {
			return e
		}
		fmt.Printf("Imported %s: %d files\n", root.ID, stats[root.ID].Media)
	}
	page, e := s.ListTitles(ctx, store.TitleQuery{Limit: 1})
	if e == nil {
		fmt.Println("Catalogue title total:", page.Total)
	}
	return e
}
