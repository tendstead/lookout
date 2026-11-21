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

type ServerConfig struct {
	ID    string            `json:"id"`
	Name  string            `json:"name"`
	Type  string            `json:"type,omitempty"`
	SSH   library.SSHConfig `json:"ssh"`
	Roots []library.Root    `json:"roots"`
}
type Config struct {
	SSH     library.SSHConfig `json:"ssh"`
	Roots   []library.Root    `json:"roots"`
	Servers []ServerConfig    `json:"servers"`
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
	return ValidateConfig(c)
}

func ValidateConfig(c Config) (Config, error) {
	if len(c.Servers) > 0 && len(c.Roots) > 0 {
		return c, errors.New("use legacy roots or servers, not both")
	}
	if len(c.Servers) == 0 && len(c.Roots) > 0 {
		c.Servers = []ServerConfig{{ID: "legacy", Name: c.SSH.Host, SSH: c.SSH, Roots: c.Roots}}
	}
	c.Roots = nil
	ids := map[string]bool{}
	servers := map[string]bool{}
	for i := range c.Servers {
		v := &c.Servers[i]
		if v.ID == "" || len(v.ID) > 100 || strings.ContainsAny(v.ID, "/\\") || servers[v.ID] {
			return c, errors.New("invalid or repeated server ID")
		}
		servers[v.ID] = true
		paths := map[string]bool{}
		for j := range v.Roots {
			r := &v.Roots[j]
			r.ServerID = v.ID
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
			c.Roots = append(c.Roots, *r)
		}
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
	for _, v := range c.Servers {
		if e := s.EnsureServer(ctx, v.ID, v.Name); e != nil {
			return nil, e
		}
	}
	for _, r := range c.Roots {
		if e := s.EnsureSource(ctx, r); e != nil {
			return nil, e
		}
	}
	if e := s.RecoverScans(ctx); e != nil {
		return nil, e
	}
	if e := s.ReconcileIdentities(ctx); e != nil {
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
	slots := make(chan struct{}, 4)
	queues := map[string]chan string{}
	hosts := map[string]string{}
	var workers sync.WaitGroup
	for _, root := range s.config.Roots {
		hosts[root.ID] = root.ServerID
		if queues[root.ServerID] != nil {
			continue
		}
		queue := make(chan string, len(s.config.Roots))
		queues[root.ServerID] = queue
		workers.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				case id := <-queue:
					select {
					case slots <- struct{}{}:
					case <-ctx.Done():
						return
					}
					s.scan(ctx, id)
					<-slots
					s.mu.Lock()
					delete(s.pending, id)
					s.mu.Unlock()
				}
			}
		})
	}
	defer workers.Wait()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	schedule := func() {
		for _, r := range s.config.Roots {
			if due, e := s.store.SourceDue(ctx, r.ID); e == nil && due {
				_ = s.RequestScan(r.ID)
			}
		}
	}
	schedule()
	for {
		select {
		case <-ctx.Done():
			return
		case id := <-s.queue:
			if queue := queues[hosts[id]]; queue != nil {
				select {
				case queue <- id:
				case <-ctx.Done():
					return
				}
			}
		case <-ticker.C:
			schedule()
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
	ssh, kind := s.config.SSH, ""
	for _, v := range s.config.Servers {
		if v.ID == root.ServerID {
			ssh, kind = v.SSH, v.Type
		}
	}
	var reader interface {
		library.Reader
		Close() error
	}
	if kind == "ssh" {
		reader, e = library.ConnectShell(job, ssh, root.Path)
	} else {
		reader, e = library.Connect(job, ssh)
	}
	if e != nil {
		log.Printf("Source %s connection failed: %v", id, e)
	}
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
		_ = s.store.ReconcileIdentities(job)
	}
}
