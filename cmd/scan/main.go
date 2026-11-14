// SPDX-License-Identifier: Elastic-2.0
// scan inventories remote media using only SFTP read operations. Output is local.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/tendstead/lookout/internal/library"
	"os"
	"time"
)

func main() {
	configFile := flag.String("config", "", "local JSON source configuration")
	output := flag.String("output", "", "local JSONL evidence output")
	flag.Parse()
	if *configFile == "" || *output == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*configFile, *output); err != nil {
		fmt.Fprintln(os.Stderr, "Scan failed:", err)
		os.Exit(1)
	}
}
func run(configFile, output string) error {
	var config struct {
		SSH   library.SSHConfig `json:"ssh"`
		Roots []library.Root    `json:"roots"`
	}
	data, err := os.ReadFile(configFile)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(data, &config); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	reader, err := library.Connect(ctx, config.SSH)
	if err != nil {
		return err
	}
	defer reader.Close()
	file, err := os.OpenFile(output, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	summary := map[string]library.ScanStats{}
	for _, root := range config.Roots {
		stats, err := library.Scan(ctx, reader, root, func(f library.FileObservation) error { return enc.Encode(f) })
		if err != nil {
			return fmt.Errorf("root %s incomplete: %w", root.ID, err)
		}
		summary[root.ID] = stats
		fmt.Printf("%s: %d media, %d directories, %d sidecars, %d warnings\n", root.ID, stats.Media, stats.Directories, stats.Sidecars, stats.Warnings)
	}
	result, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(output+".summary.json", result, 0600)
}
