// SPDX-License-Identifier: Elastic-2.0
package enrichment

import (
	"context"
	"errors"
	"github.com/tendstead/lookout/internal/library"
	"github.com/tendstead/lookout/internal/metadata"
	"github.com/tendstead/lookout/internal/store"
	"strings"
	"time"
)

func inventorySource(id, kind string) string { return "api-" + id + "-" + kind }
func (s *Service) ensureInventorySource(ctx context.Context, id, kind string, c metadata.MediaServer) (string, error) {
	source := inventorySource(id, kind)
	if err := s.Store.EnsureSource(ctx, library.Root{ID: source, ServerID: c.ServerID, Label: c.Kind + " " + kind, Path: "/api/" + id + "/" + kind, Category: kind}); err != nil {
		return "", err
	}
	return source, s.Store.SetSourceAdapter(ctx, source, c.Kind)
}

func (s *Service) ImportInventory(ctx context.Context, id string, c metadata.MediaServer) (r Report, err error) {
	r = Report{Phase: "inventory", Running: true}
	if c.ServerID == "" {
		return r, errors.New("assign this connection to a server first")
	}
	if strings.HasPrefix(c.ServerID, "media-") {
		_ = s.Store.EnsureServer(ctx, c.ServerID, c.SuggestedName(ctx))
	}
	scans := map[string]string{}
	counts := map[string]int{}
	defer func() {
		for kind, scan := range scans {
			finish, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			save := s.Store.FinishScan(finish, scan, inventorySource(id, kind), library.ScanStats{Media: counts[kind]}, err)
			cancel()
			if save != nil && err == nil {
				err = save
			}
		}
		if err == nil {
			err = s.Store.ReconcileIdentities(ctx)
			if err == nil {
				err = s.Store.ConnectionImportFinished(ctx, id)
			}
		}
		r.Running = false
		r.Failed = err != nil
	}()

	for _, kind := range []string{"movie", "series"} {
		source, e := s.ensureInventorySource(ctx, id, kind, c)
		if e != nil {
			return r, e
		}
		scan, e := s.Store.StartScan(ctx, source)
		if e != nil {
			return r, e
		}
		scans[kind] = scan
	}
	err = c.Inventory(ctx, func(item metadata.InventoryItem) error {
		kind := item.Title.Kind
		scan, ok := scans[kind]
		if !ok {
			return errors.New("unsupported media inventory type")
		}
		numbering := "unknown"
		if item.SeasonKnown && len(item.Episodes) > 0 {
			numbering = "season_episode"
		}
		// Item IDs are stable even when a provider hides physical paths. Each API
		// entry is evidence of availability, not a downloaded or directly playable file.
		f := library.FileObservation{SourceID: inventorySource(id, kind), Group: item.Title.ID, RelativePath: item.Key, Category: kind, Storage: "file", Modified: time.Unix(0, 0), Title: library.Metadata{Title: item.Title.Name, Year: item.Title.Year, Overview: item.Title.Overview, ProviderIDs: item.Title.ProviderIDs, Provenance: c.Kind}, Season: item.Season, Episodes: item.Episodes, Numbering: numbering}
		if e := s.Store.Observe(ctx, scan, f); e != nil {
			return e
		}
		if e := s.Store.ReportedPath(ctx, f.SourceID, f.RelativePath, item.Path); e != nil {
			return e
		}
		counts[kind]++
		targets, e := s.Store.MetadataTargets(ctx, f.SourceID, item.Title.ID, "series")
		if e != nil {
			return e
		}
		for _, title := range targets {
			e = s.Store.ApplyMetadata(ctx, title, store.MetadataUpdate{Candidate: metadata.Candidate{Provider: c.Kind, ID: item.Title.ID, Name: item.Title.Name, Year: item.Title.Year, Overview: item.Title.Overview, ProviderIDs: item.Title.ProviderIDs}, ConnectionID: id, ArtworkPath: item.Title.ImagePath})
			if errors.Is(e, store.ErrMetadataConflict) {
				r.Review++
			} else if e != nil {
				return e
			}
		}
		r.Matched++
		return nil
	})
	return r, err
}

// Independent bounded inventory jobs; an unavailable remote never holds up all
// other hosts. SourceDue includes a one-hour failure backoff.
func (s *Service) RunInventories(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	slots := make(chan struct{}, 2)
	first := make(map[string]bool, len(s.configured))
	for id := range s.configured {
		first[id] = true
	}
	schedule := func() {
		connections, e := s.Connections(ctx)
		if e != nil {
			return
		}
		for _, c := range connections {
			if !c.TokenConfigured || c.ServerID == "" || (!first[c.ID] && !s.Store.ConnectionRetryAllowed(ctx, c.ID)) {
				continue
			}
			due := false
			if len(c.Mappings) == 0 {
				for _, kind := range []string{"movie", "series"} {
					v, err := s.Store.SourceDue(ctx, inventorySource(c.ID, kind))
					if err != nil || v {
						due = true
					}
				}
			} else {
				due = s.Store.ConnectionDue(ctx, c.ID)
			}
			if !due && !first[c.ID] {
				continue
			}
			select {
			case slots <- struct{}{}:
			default:
				return
			}
			delete(first, c.ID)
			go func(id string) {
				defer func() { <-slots }()
				request, cancel := context.WithTimeout(ctx, 30*time.Minute)
				defer cancel()
				_, _ = s.ImportConnection(request, id)
			}(c.ID)
		}
	}
	schedule()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			schedule()
		}
	}
}
