// SPDX-License-Identifier: Elastic-2.0
package notify

import (
	"context"
	"errors"
	"fmt"
	"github.com/tendstead/lookout/internal/store"
	"sync"
	"time"
)

type Status struct {
	Configured      bool       `json:"configured"`
	Verified        bool       `json:"verified"`
	DeliveryEnabled bool       `json:"delivery_enabled"`
	CheckedAt       *time.Time `json:"checked_at"`
	LastError       string     `json:"last_error,omitempty"`
}
type Service struct {
	store  *store.Store
	client *Client
	mu     sync.Mutex
	status Status
}

func New(s *store.Store, c *Config, enabled bool) *Service {
	n := &Service{store: s, status: Status{Configured: c != nil, DeliveryEnabled: enabled && c != nil}}
	if c != nil {
		n.client = &Client{Token: c.Token, ChatID: c.ChatID}
	}
	return n
}
func (s *Service) Status() Status { s.mu.Lock(); defer s.mu.Unlock(); return s.status }
func (s *Service) Ready() bool {
	v := s.Status()
	return v.Configured && v.Verified && v.DeliveryEnabled
}
func (s *Service) Verify(ctx context.Context) error {
	if s.client == nil {
		return errors.New("Telegram is not configured")
	}
	e := s.client.Verify(ctx)
	now := time.Now().UTC()
	s.mu.Lock()
	s.status.Verified = e == nil
	s.status.CheckedAt = &now
	s.status.LastError = ""
	if e != nil {
		s.status.LastError = "Telegram connection could not be verified"
	}
	s.mu.Unlock()
	return e
}
func (s *Service) Run(ctx context.Context) {
	if s.client == nil {
		return
	}
	check, cancel := context.WithTimeout(ctx, 15*time.Second)
	_ = s.Verify(check)
	cancel()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !s.Status().Verified {
				v := s.Status()
				if v.CheckedAt == nil || time.Since(*v.CheckedAt) > 30*time.Minute {
					check, cancel := context.WithTimeout(ctx, 15*time.Second)
					_ = s.Verify(check)
					cancel()
				}
				continue
			}
			if s.Ready() {
				_ = s.DeliverOne(ctx, time.Now().UTC())
			}
		}
	}
}
func (s *Service) DeliverOne(ctx context.Context, now time.Time) error {
	if !s.Ready() {
		return nil
	}
	d, e := s.store.ClaimNotification(ctx, now)
	if e != nil || d == nil {
		return e
	}
	description := "New episode reported aired"
	if d.Kind == "season" {
		description = "New season premiere reported aired"
	}
	number := ""
	if d.Number != nil {
		number = fmt.Sprintf(" · S%02dE%02d", d.Season, *d.Number)
	}
	message := description + "\n" + d.Title + number + "\n" + d.Name
	if d.URL != "" {
		message += "\n" + d.URL
	}
	message += "\nProvider: TVmaze. This does not confirm a local copy."
	send, cancel := context.WithTimeout(ctx, 10*time.Second)
	e = s.client.Send(send, message)
	cancel()
	retry := time.Minute * time.Duration(1<<min(d.Attempts, 8))
	var api *APIError
	if errors.As(e, &api) && api.RetryAfter > retry {
		retry = api.RetryAfter
	}
	save, end := context.WithTimeout(context.Background(), 5*time.Second)
	defer end()
	return s.store.MarkNotification(save, d.ID, e == nil, retry, now)
}
