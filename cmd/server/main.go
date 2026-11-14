// SPDX-License-Identifier: Elastic-2.0
package main

import (
	"context"
	"github.com/tendstead/lookout/internal/config"
	"github.com/tendstead/lookout/internal/httpapi"
	"github.com/tendstead/lookout/internal/store"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
		c := http.Client{Timeout: 3 * time.Second}
		r, e := c.Get("http://127.0.0.1:8080/readyz")
		if e != nil {
			os.Exit(1)
		}
		r.Body.Close()
		if r.StatusCode != 200 {
			os.Exit(1)
		}
		return
	}
	if run() != nil {
		log.Print("Lookout could not start; check database availability and configuration")
		os.Exit(1)
	}
}
func run() error {
	cfg, err := config.Load(os.LookupEnv, os.ReadFile)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 15*time.Second)
	s, err := store.Open(startup, cfg)
	if err != nil {
		cancel()
		return err
	}
	defer s.Close()
	err = s.Migrate(startup)
	cancel()
	if err != nil {
		return err
	}
	srv := &http.Server{Addr: cfg.ListenAddr, Handler: httpapi.NewHandler(s.Ping, frontendAssets()), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe() }()
	log.Print("Lookout listening on ", cfg.ListenAddr)
	select {
	case err = <-done:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	case <-ctx.Done():
		end, c := context.WithTimeout(context.Background(), 10*time.Second)
		defer c()
		return srv.Shutdown(end)
	}
}
