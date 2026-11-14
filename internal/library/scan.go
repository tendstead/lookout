// SPDX-License-Identifier: Elastic-2.0
package library

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
)

func mergeMetadata(seed, extra Metadata) Metadata {
	if extra.Title != "" {
		seed.Title = extra.Title
	}
	if extra.Year != 0 {
		seed.Year = extra.Year
	}
	if extra.Overview != "" {
		seed.Overview = extra.Overview
	}
	ids := map[string]string{}
	for k, v := range seed.ProviderIDs {
		ids[k] = v
	}
	for k, v := range extra.ProviderIDs {
		ids[k] = v
	}
	seed.ProviderIDs = ids
	if extra.Provenance != "" {
		seed.Provenance = extra.Provenance
	}
	return seed
}
func Scan(ctx context.Context, r Reader, root Root, emit func(FileObservation) error) (ScanStats, error) {
	var stats ScanStats
	if !path.IsAbs(root.Path) || path.Clean(root.Path) != root.Path || root.Path == "/" {
		return stats, errors.New("source root must be a clean absolute directory")
	}
	info, err := r.Lstat(ctx, root.Path)
	if err != nil {
		return stats, fmt.Errorf("source root unavailable: %w", err)
	}
	if !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
		return stats, errors.New("source root is not a real directory")
	}
	real, err := r.RealPath(ctx, root.Path)
	if err != nil || real != root.Path {
		return stats, errors.New("source root resolves outside configured path")
	}
	readNFO := func(p string, size int64) (Metadata, error) {
		if size > MaxSidecarBytes {
			return Metadata{}, errors.New("sidecar too large")
		}
		f, e := r.Open(ctx, p)
		if e != nil {
			return Metadata{}, e
		}
		defer f.Close()
		b, e := io.ReadAll(io.LimitReader(f, MaxSidecarBytes+1))
		if e != nil {
			return Metadata{}, e
		}
		return ParseNFO(b)
	}
	var walk func(string, string, Metadata, int) error
	walk = func(dir, group string, seed Metadata, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if depth > 40 {
			return errors.New("source nesting exceeds limit")
		}
		entries, e := r.ReadDir(ctx, dir)
		if e != nil {
			return fmt.Errorf("directory listing failed: %w", e)
		}
		stats.Directories++
		byName := make(map[string]fs.FileInfo, len(entries))
		for _, entry := range entries {
			n := entry.Name()
			if n == "." || n == ".." {
				continue
			}
			if n == "" || strings.ContainsAny(n, "/\\") || path.Clean(n) != n {
				return errors.New("invalid directory entry")
			}
			byName[n] = entry
		}
		if depth == 1 || root.Category == "movie" || root.Category == "anime_movie" {
			for _, name := range []string{"tvshow.nfo", "movie.nfo"} {
				if v, ok := byName[name]; ok && v.Mode().IsRegular() {
					m, e := readNFO(path.Join(dir, name), v.Size())
					if e != nil {
						stats.Warnings++
					} else {
						seed = mergeMetadata(seed, m)
						stats.Sidecars++
					}
				}
			}
		}
		directMedia := 0
		for _, v := range entries {
			if v.Mode().IsRegular() && IsMedia(v.Name()) {
				directMedia++
			}
		}
		for _, entry := range entries {
			n := entry.Name()
			if n == "." || n == ".." {
				continue
			}
			p := path.Join(dir, n)
			if !strings.HasPrefix(p, root.Path+"/") {
				return errors.New("path escaped source root")
			}
			if entry.Mode()&fs.ModeSymlink != 0 {
				stats.SymlinksSkipped++
				continue
			}
			childGroup := group
			childSeed := seed
			if depth == 0 {
				childGroup = n
				childSeed = ParseTitle(n)
			}
			if entry.IsDir() {
				if e = walk(p, childGroup, childSeed, depth+1); e != nil {
					return e
				}
				continue
			}
			if !entry.Mode().IsRegular() || !IsMedia(n) {
				continue
			}
			if stats.Media >= 1000000 {
				return errors.New("source file count exceeds limit")
			}
			if depth == 0 {
				childGroup = strings.TrimSuffix(n, path.Ext(n))
				childSeed = ParseTitle(childGroup)
			}
			if root.Category == "movie" || root.Category == "anime_movie" {
				if directMedia == 1 && depth > 0 {
					childGroup = strings.TrimPrefix(dir, root.Path+"/")
					childSeed = ParseTitle(path.Base(dir))
					if seed.Provenance == "nfo" {
						childSeed = mergeMetadata(childSeed, seed)
					}
				} else {
					childGroup = strings.TrimPrefix(p, root.Path+"/")
					childSeed = ParseMovieFilename(n)
				}
			}
			season, eps := ParseNumbering(n, path.Base(dir))
			numbering := "unknown"
			if len(eps) > 0 {
				numbering = "season_episode"
			}
			episodeTitle := ""
			sidecar := strings.TrimSuffix(n, path.Ext(n)) + ".nfo"
			if v, ok := byName[sidecar]; ok && v.Mode().IsRegular() {
				m, e := readNFO(path.Join(dir, sidecar), v.Size())
				if e != nil {
					stats.Warnings++
				} else {
					stats.Sidecars++
					if m.Kind == "episodedetails" {
						episodeTitle = m.Title
						if m.Season != nil && m.Episode != nil {
							season = *m.Season
							eps = []int{*m.Episode}
							numbering = "nfo"
						}
					} else {
						childSeed = mergeMetadata(childSeed, m)
					}
				}
			}
			relative := strings.TrimPrefix(p, root.Path+"/")
			file := FileObservation{SourceID: root.ID, RelativePath: relative, Group: childGroup, Category: root.Category, Size: entry.Size(), Modified: entry.ModTime().UTC(), Title: childSeed, Season: season, Episodes: eps, Numbering: numbering, EpisodeTitle: episodeTitle}
			if e = emit(file); e != nil {
				return e
			}
			stats.Media++
		}
		return nil
	}
	err = walk(root.Path, "", Metadata{}, 0)
	return stats, err
}
