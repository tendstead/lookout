// SPDX-License-Identifier: Elastic-2.0
package catalogue

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/tendstead/lookout/internal/library"
	"github.com/tendstead/lookout/internal/store"
	"log"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

type Config struct {
	SSH   library.SSHConfig `json:"ssh"`
	Roots []library.Root    `json:"roots"`
}

func LoadConfig(file string) (Config, error) {
	var c Config
	if file == "" {
		return c, nil
	}
	raw, e := os.ReadFile(file)
	if e != nil {
		return c, errors.New("catalogue configuration unavailable")
	}
	if e = json.Unmarshal(raw, &c); e != nil {
		return c, errors.New("invalid catalogue configuration")
	}
	ids := map[string]bool{}
	paths := map[string]bool{}
	for _, r := range c.Roots {
		if r.ID == "" || strings.ContainsAny(r.ID, "/\\") || ids[r.ID] || paths[r.Path] || !path.IsAbs(r.Path) || path.Clean(r.Path) != r.Path || r.Path == "/" {
			return c, errors.New("invalid or repeated source root")
		}
		switch r.Category {
		case "series", "movie", "anime_series", "anime_movie":
		default:
			return c, errors.New("unknown source category")
		}
		ids[r.ID] = true
		paths[r.Path] = true
	}
	return c, nil
}

type Service struct {
	store   *store.Store
	config  Config
	queue   chan string
	mu      sync.Mutex
	pending map[string]bool
}

func New(ctx context.Context, s *store.Store, c Config) (*Service, error) {
	for _, r := range c.Roots {
		if e := s.EnsureSource(ctx, r); e != nil {
			return nil, e
		}
	}
	if e := s.RecoverScans(ctx); e != nil {
		return nil, e
	}
	return &Service{store: s, config: c, queue: make(chan string, 16), pending: map[string]bool{}}, nil
}
func (s *Service) RequestScan(id string) error {
	found := false
	for _, r := range s.config.Roots {
		if r.ID == id {
			found = true
		}
	}
	if !found {
		return store.ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending[id] {
		return nil
	}
	select {
	case s.queue <- id:
		s.pending[id] = true
		return nil
	default:
		return errors.New("scan queue full")
	}
}
func (s *Service) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case id := <-s.queue:
			s.scan(ctx, id)
			s.mu.Lock()
			delete(s.pending, id)
			s.mu.Unlock()
		case <-ticker.C:
			for _, r := range s.config.Roots {
				_ = s.RequestScan(r.ID)
			}
		}
	}
}
func (s *Service) scan(ctx context.Context, id string) {
	var root library.Root
	for _, r := range s.config.Roots {
		if r.ID == id {
			root = r
		}
	}
	job, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	scan, e := s.store.StartScan(job, id)
	if e != nil {
		log.Printf("Source %s scan could not start", id)
		return
	}
	reader, e := library.Connect(job, s.config.SSH)
	var stats library.ScanStats
	if e == nil {
		stats, e = library.Scan(job, reader, root, func(f library.FileObservation) error { return s.store.Observe(job, scan, f) })
		_ = reader.Close()
	}
	finish, end := context.WithTimeout(context.Background(), 5*time.Second)
	defer end()
	if err := s.store.FinishScan(finish, scan, id, stats, e); err != nil {
		log.Printf("Source %s scan status could not be saved", id)
	}
	if e != nil {
		log.Printf("Source %s scan incomplete; inventory retained", id)
	} else {
		log.Printf("Source %s scan complete: %d media", id, stats.Media)
	}
}
