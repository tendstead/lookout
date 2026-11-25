// SPDX-License-Identifier: Elastic-2.0
package tracking

import (
	"context"
	"errors"
	"github.com/tendstead/lookout/internal/metadata"
	"github.com/tendstead/lookout/internal/store"
	"sync"
	"time"
)

type Status struct {
	Running    bool       `json:"running"`
	Checked    int        `json:"checked"`
	Errors     int        `json:"errors"`
	Events     int        `json:"events"`
	FinishedAt *time.Time `json:"finished_at"`
	Allowed    bool       `json:"allowed"`
}
type Service struct {
	Store       *store.Store
	TV          *metadata.TVmaze
	Allowed     bool
	AutoFollow  bool
	NotifyReady func() bool
	mu          sync.Mutex
	status      Status
	queue       chan struct{}
}

func New(s *store.Store, tv *metadata.TVmaze, allowed, auto bool, ready func() bool) *Service {
	return &Service{Store: s, TV: tv, Allowed: allowed, AutoFollow: auto, NotifyReady: ready, queue: make(chan struct{}, 1)}
}
func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.status
	v.Allowed = s.Allowed
	return v
}
func (s *Service) setStatus(v Status) { s.mu.Lock(); s.status = v; s.mu.Unlock() }
func (s *Service) Queue() error {
	if !s.Allowed {
		return errors.New("public metadata lookups are disabled")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status.Running || len(s.queue) > 0 {
		return errors.New("release refresh already running")
	}
	s.queue <- struct{}{}
	return nil
}
func (s *Service) Run(ctx context.Context) {
	if !s.Allowed {
		return
	}
	_ = s.Refresh(ctx, false)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.queue:
			_ = s.Refresh(ctx, true)
		case <-ticker.C:
			_ = s.Refresh(ctx, false)
		}
	}
}
func (s *Service) Refresh(ctx context.Context, force bool) error {
	if !s.Allowed {
		return errors.New("public metadata lookups are disabled")
	}
	r := Status{Running: true, Allowed: true}
	s.setStatus(r)
	defer func() { now := time.Now().UTC(); r.Running = false; r.FinishedAt = &now; s.setStatus(r) }()
	if s.AutoFollow {
		if e := s.Store.SeedFollows(ctx); e != nil {
			return e
		}
	}
	now := time.Now().UTC()
	if e := s.Store.SyncFollowProviders(ctx, now); e != nil {
		return e
	}
	follows, e := s.Store.ListFollows(ctx)
	if e != nil {
		return e
	}
	for _, follow := range follows {
		if e = ctx.Err(); e != nil {
			return e
		}
		if !follow.Enabled || follow.ProviderID == "" {
			continue
		}
		due := refreshDue(follow, now, force)
		if !due && follow.LastError == "" {
			due, e = s.Store.CachedDue(ctx, follow, now)
			if e != nil {
				return e
			}
		}
		if !due {
			continue
		}
		request, cancel := context.WithTimeout(ctx, 30*time.Second)
		episodes, e := s.TV.Episodes(request, follow.ProviderID)
		cancel()
		if e != nil {
			r.Errors++
			_ = s.Store.ScheduleFailure(ctx, follow.TitleID, time.Now().UTC())
			s.setStatus(r)
			continue
		}
		notify := s.NotifyReady != nil && s.NotifyReady()
		count, e := s.Store.SaveEpisodeSnapshot(ctx, follow, episodes, time.Now().UTC(), notify)
		if e != nil {
			return e
		}
		r.Checked++
		r.Events += count
		s.setStatus(r)
	}
	return s.refreshWantedSchedules(ctx, &r, now, force)
}

func (s *Service) refreshWantedSchedules(ctx context.Context, status *Status, now time.Time, force bool) error {
	providers, err := s.Store.DueWantedSchedules(ctx, now, force)
	if err != nil {
		return err
	}
	for _, provider := range providers {
		if err = ctx.Err(); err != nil {
			return err
		}
		request, cancel := context.WithTimeout(ctx, 30*time.Second)
		episodes, lookupErr := s.TV.Episodes(request, provider)
		if lookupErr != nil {
			cancel()
			status.Errors++
			if err = s.Store.WantedScheduleFailure(ctx, provider, now); err != nil {
				return err
			}
			s.setStatus(*status)
			continue
		}
		seasons, seasonErr := s.TV.Seasons(request, provider)
		cancel()
		if seasonErr != nil {
			status.Errors++
			seasons = nil // Preserve the last known season data if that endpoint is unavailable.
		}
		if err = s.Store.SaveWantedSchedule(ctx, provider, episodes, seasons, time.Now().UTC()); err != nil {
			return err
		}
		status.Checked++
		s.setStatus(*status)
	}
	notify := s.NotifyReady != nil && s.NotifyReady()
	count, err := s.Store.QueueWantedAirAlerts(ctx, time.Now().UTC(), notify)
	status.Events += count
	return err
}

func (s *Service) Overview(ctx context.Context, limit, offset int) (store.ReleaseOverview, error) {
	return s.Store.ReleaseOverview(ctx, time.Now().UTC(), limit, offset)
}
func (s *Service) Follow(ctx context.Context, id string) (store.Follow, error) {
	return s.Store.GetFollow(ctx, id)
}
func (s *Service) SetFollow(ctx context.Context, id string, enabled bool) error {
	if e := s.Store.SetFollow(ctx, id, enabled, time.Now().UTC()); e != nil {
		return e
	}
	if s.Allowed {
		_ = s.Queue()
	}
	return nil
}

func refreshDue(f store.Follow, now time.Time, force bool) bool {
	return force || !now.Before(f.NextRefresh) || !f.Initialized && f.LastError == ""
}

func (s *Service) Gaps(ctx context.Context, q store.TitleQuery, kind string) (store.SeasonGapPage, error) {
	return s.Store.SeasonGaps(ctx, q, kind, time.Now().UTC())
}
