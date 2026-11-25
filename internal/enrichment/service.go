// SPDX-License-Identifier: Elastic-2.0
package enrichment

import (
	"context"
	"errors"
	"github.com/tendstead/lookout/internal/metadata"
	"github.com/tendstead/lookout/internal/store"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"
)

type Report struct {
	Phase     string `json:"phase"`
	Matched   int    `json:"matched"`
	Review    int    `json:"review"`
	Unmatched int    `json:"unmatched"`
	Running   bool   `json:"running"`
	Failed    bool   `json:"failed"`
}
type Service struct {
	AllowPublic bool
	artwork     chan struct{}
	Store       *store.Store
	Key         []byte
	TV          *metadata.TVmaze
	mu          sync.Mutex
	report      Report
	queue       chan string
	inventoryMu sync.Mutex
	active      map[string]bool
	configured  map[string]metadata.MediaServer
}

func New(s *store.Store, key []byte) *Service {
	return &Service{Store: s, Key: key, TV: metadata.NewTVmaze(), queue: make(chan string, 2), artwork: make(chan struct{}, 2)}
}

// ConfigureConnections is called once before the workers and HTTP handlers start.
func (s *Service) ConfigureConnections(ctx context.Context, entries []metadata.MediaServer) ([]string, error) {
	sources := []string{}
	configured := make(map[string]metadata.MediaServer, len(entries))
	for _, c := range entries {
		if err := s.validateMappings(ctx, c); err != nil {
			return nil, err
		}
		id, err := s.Store.ConfigureConnection(ctx, c)
		if err != nil {
			return nil, err
		}
		if len(c.Mappings) == 0 {
			for _, kind := range []string{"movie", "series"} {
				source, err := s.ensureInventorySource(ctx, id, kind, c)
				if err != nil {
					return nil, err
				}
				sources = append(sources, source)
			}
		}
		configured[id] = c
	}
	s.configured = configured
	return sources, nil
}

func (s *Service) connectionConfig(ctx context.Context, id string) (metadata.MediaServer, error) {
	if s.configured != nil {
		if c, ok := s.configured[id]; ok {
			return c, nil
		}
		return metadata.MediaServer{}, store.ErrNotFound
	}
	return s.Store.ConnectionConfig(ctx, id, s.Key)
}

func (s *Service) Status() Report {
	s.inventoryMu.Lock()
	active := len(s.active) > 0
	s.inventoryMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.report
	r.Running = r.Running || active
	return r
}
func (s *Service) setReport(r Report) { s.mu.Lock(); s.report = r; s.mu.Unlock() }
func (s *Service) Queue(id string) error {
	if s.Status().Running {
		return errors.New("metadata worker busy")
	}
	if id == "official" && !s.AllowPublic {
		return errors.New("public metadata lookups are disabled")
	}
	if id != "official" && len(s.Key) != 32 {
		return errors.New("connection encryption is not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.report.Running || len(s.queue) > 0 {
		return errors.New("metadata worker busy")
	}
	select {
	case s.queue <- id:
		s.report = Report{Phase: "queued", Running: true}
		return nil
	default:
		return errors.New("metadata worker busy")
	}
}
func (s *Service) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case id := <-s.queue:
			var r Report
			var e error
			if id == "official" {
				r, e = s.RefreshOfficial(ctx)
			} else {
				r, e = s.ImportConnection(ctx, id)
			}
			r.Running = false
			r.Failed = e != nil
			s.setReport(r)
		}
	}
}
func (s *Service) validateMappings(ctx context.Context, c metadata.MediaServer) error {
	sources, e := s.Store.ListSources(ctx)
	if e != nil {
		return e
	}
	roots := map[string]string{}
	for _, source := range sources {
		roots[source.ID] = source.Path
	}
	seen := map[string]bool{}
	for _, m := range c.Mappings {
		if !path.IsAbs(m.ServerPrefix) || m.ServerPrefix == "/" || path.Clean(m.ServerPrefix) != m.ServerPrefix || seen[m.ServerPrefix] || roots[m.SourceID] == "" || roots[m.SourceID] != m.SourcePrefix {
			return errors.New("path mappings must target configured source roots")
		}
		seen[m.ServerPrefix] = true
	}
	return nil
}
func (s *Service) ImportConnection(ctx context.Context, id string) (result Report, err error) {
	s.inventoryMu.Lock()
	if s.active == nil {
		s.active = map[string]bool{}
	}
	if s.active[id] {
		s.inventoryMu.Unlock()
		return Report{}, errors.New("inventory already running")
	}
	s.active[id] = true
	s.inventoryMu.Unlock()
	defer func() { s.inventoryMu.Lock(); delete(s.active, id); s.inventoryMu.Unlock() }()
	defer func() {
		result.Running = false
		result.Failed = err != nil
		if err != nil {
			cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = s.Store.FailConnectionInventory(cleanup, id)
			cancel()
		}
		s.setReport(result)
	}()

	if e := s.Store.ConnectionImportStarted(ctx, id); e != nil {
		return Report{}, e
	}
	r := Report{Phase: "media-server", Running: true}
	s.setReport(r)
	c, e := s.connectionConfig(ctx, id)
	if e != nil {
		return r, errors.New("connection credentials unavailable")
	}
	if e = s.validateMappings(ctx, c); e != nil {
		return r, e
	}
	if e = c.Verify(ctx); e != nil {
		_ = s.Store.ConnectionVerified(ctx, id, false)
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if len(c.Mappings) == 0 {
			_ = s.Store.FailConnectionInventory(cleanup, id)
		}
		return r, e
	}
	_ = s.Store.ConnectionVerified(ctx, id, true)
	if len(c.Mappings) == 0 {
		return s.ImportInventory(ctx, id, c)
	}
	matched := map[string]bool{}
	e = c.InventoryWithTitles(ctx, func(item metadata.ServerItem) error {
		found := false
		for _, remote := range item.Paths {
			source, relative, ok := metadata.MapPath(remote, c.Mappings)
			if !ok {
				continue
			}
			targets, e := s.Store.MetadataTargets(ctx, source, relative, item.Kind)
			if e != nil {
				return e
			}
			for _, title := range targets {
				found = true
				if matched[title] {
					continue
				}
				u := store.MetadataUpdate{Candidate: metadata.Candidate{Provider: c.Kind, ID: item.ID, Name: item.Name, Year: item.Year, Overview: item.Overview, ProviderIDs: item.ProviderIDs}, ConnectionID: id, ArtworkPath: item.ImagePath}
				if tmdb := item.ProviderIDs["tmdb"]; tmdb != "" {
					kind := "movie"
					if item.Kind == "series" {
						kind = "tv"
					}
					u.URL = "https://www.themoviedb.org/" + kind + "/" + url.PathEscape(tmdb)
				}
				if e = s.Store.ApplyMetadata(ctx, title, u); errors.Is(e, store.ErrMetadataConflict) {
					r.Review++
				} else if e != nil {
					return e
				} else {
					r.Matched++
				}
				matched[title] = true
			}
		}
		if !found {
			r.Unmatched++
		}
		s.setReport(r)
		return nil
	}, func(item metadata.InventoryItem) error {
		if item.Technical == nil {
			return nil
		}
		source, relative, ok := metadata.MapPath(item.Path, c.Mappings)
		if !ok {
			return nil
		}
		return s.Store.AttachTechnicalProfile(ctx, source, c.ServerID, relative, item.Technical)
	})
	if e == nil {
		e = s.Store.ReconcileIdentities(ctx)
		if e == nil {
			e = s.Store.ConnectionImportFinished(ctx, id)
		}
	}
	r.Running = false
	s.setReport(r)
	return r, e
}
func (s *Service) RefreshOfficial(ctx context.Context) (Report, error) {
	if !s.AllowPublic {
		return Report{}, errors.New("public metadata lookups are disabled")
	}
	r := Report{Phase: "official", Running: true}
	s.setReport(r)
	// Capture identities before applying metadata: display names can change page order.
	var titles []store.Title
	for offset := 0; ; offset += 100 {
		page, e := s.Store.ListTitles(ctx, store.TitleQuery{Limit: 100, Offset: offset})
		if e != nil {
			return r, e
		}
		titles = append(titles, page.Items...)
		if offset+len(page.Items) >= int(page.Total) {
			break
		}
	}
	for _, title := range titles {
		if title.Category != "series" && title.Category != "anime_series" {
			continue
		}
		if e := ctx.Err(); e != nil {
			return r, e
		}
		var candidate metadata.Candidate
		var err error
		if title.ProviderIDs["tvmaze"] != "" || title.ProviderIDs["tvdb"] != "" || title.ProviderIDs["imdb"] != "" {
			candidate, err = s.TV.Lookup(ctx, title.ProviderIDs)
		} else {
			var results []metadata.Candidate
			results, err = s.TV.Search(ctx, title.Name)
			if err == nil {
				year := 0
				if title.Year != nil {
					year = *title.Year
				}
				var exact bool
				candidate, exact = metadata.ExactCandidate(results, title.Name, year)
				if !exact {
					err = errors.New("ambiguous or unmatched title")
				}
			}
		}
		if err != nil {
			r.Unmatched++
			s.setReport(r)
			continue
		}
		if err = s.Store.ApplyMetadata(ctx, title.ID, store.MetadataUpdate{Candidate: candidate}); errors.Is(err, store.ErrMetadataConflict) {
			r.Review++
		} else if err != nil {
			return r, err
		} else {
			r.Matched++
		}
		s.setReport(r)
	}
	r.Running = false
	s.setReport(r)
	return r, nil
}
func (s *Service) Search(ctx context.Context, titleID, q string) ([]metadata.Candidate, error) {
	if !s.AllowPublic {
		return nil, errors.New("public metadata lookups are disabled")
	}
	title, e := s.Store.GetTitle(ctx, titleID)
	if e != nil {
		return nil, e
	}
	if title.Category != "series" && title.Category != "anime_series" {
		return []metadata.Candidate{}, nil
	}
	if strings.TrimSpace(q) == "" {
		q = title.Name
	}
	return s.TV.Search(ctx, q)
}
func (s *Service) Match(ctx context.Context, titleID, provider, id string) error {
	if !s.AllowPublic {
		return errors.New("public metadata lookups are disabled")
	}
	if provider != "tvmaze" {
		return errors.New("unsupported provider")
	}
	title, e := s.Store.GetTitle(ctx, titleID)
	if e != nil {
		return e
	}
	if title.Category != "series" && title.Category != "anime_series" {
		return errors.New("TVmaze covers episodic titles only")
	}
	candidate, e := s.TV.Show(ctx, id)
	if e != nil {
		return e
	}
	if e = s.Store.ApplyMetadata(ctx, titleID, store.MetadataUpdate{Candidate: candidate, Manual: true}); e != nil {
		return e
	}
	return s.Store.ReconcileIdentities(ctx)
}
func (s *Service) SaveConnection(ctx context.Context, c metadata.MediaServer) (store.Connection, error) {
	if len(s.Key) != 32 {
		return store.Connection{}, errors.New("connection encryption not configured")
	}
	if e := s.validateMappings(ctx, c); e != nil {
		return store.Connection{}, e
	}
	return s.Store.SaveConnection(ctx, c, s.Key)
}
func (s *Service) VerifyConnection(ctx context.Context, id string) error {
	c, e := s.connectionConfig(ctx, id)
	if e != nil {
		return errors.New("connection credentials unavailable")
	}
	e = c.Verify(ctx)
	_ = s.Store.ConnectionVerified(ctx, id, e == nil)
	return e
}
func (s *Service) Image(ctx context.Context, id string) ([]byte, string, error) {
	if b, m, e := s.Store.CachedArtwork(ctx, id); e == nil {
		return b, m, nil
	}
	select {
	case s.artwork <- struct{}{}:
		defer func() { <-s.artwork }()
	case <-ctx.Done():
		return nil, "", ctx.Err()
	}
	if b, m, e := s.Store.CachedArtwork(ctx, id); e == nil {
		return b, m, nil
	}
	ref, e := s.Store.Artwork(ctx, id)
	if e != nil {
		return nil, "", e
	}
	var b []byte
	var mime string
	if ref.ConnectionID != "" && ref.Path != "" {
		c, err := s.connectionConfig(ctx, ref.ConnectionID)
		if err != nil {
			return nil, "", errors.New("artwork connection unavailable")
		}
		b, mime, e = c.Image(ctx, ref.Path)
	} else if ref.URL != "" {
		if !s.AllowPublic {
			return nil, "", errors.New("public metadata lookups are disabled")
		}
		b, mime, e = metadata.PublicImage(ctx, ref.URL)
	} else {
		return nil, "", store.ErrNotFound
	}
	if e != nil {
		return nil, "", e
	}
	if e = s.Store.CacheArtwork(ctx, id, mime, b); e != nil {
		return nil, "", e
	}
	return b, mime, nil
}

type Capabilities struct {
	ConnectionsReady bool `json:"connections_ready"`
	PublicMetadata   bool `json:"public_metadata_enabled"`
}

func (s *Service) Capabilities() Capabilities {
	return Capabilities{ConnectionsReady: s.configured != nil || len(s.Key) == 32, PublicMetadata: s.AllowPublic}
}
func (s *Service) Connections(ctx context.Context) ([]store.Connection, error) {
	items, err := s.Store.Connections(ctx)
	if err != nil || s.configured == nil {
		return items, err
	}
	configured := items[:0]
	for _, item := range items {
		if _, ok := s.configured[item.ID]; ok {
			item.TokenConfigured = true
			configured = append(configured, item)
		}
	}
	return configured, nil
}
