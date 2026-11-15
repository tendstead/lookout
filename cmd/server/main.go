// SPDX-License-Identifier: Elastic-2.0
package main

import (
	"context"
	"github.com/tendstead/lookout/internal/catalogue"
	"github.com/tendstead/lookout/internal/config"
	"github.com/tendstead/lookout/internal/enrichment"
	"github.com/tendstead/lookout/internal/httpapi"
	"github.com/tendstead/lookout/internal/notify"
	"github.com/tendstead/lookout/internal/store"
	"github.com/tendstead/lookout/internal/tracking"
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
	catalogueConfig, err := catalogue.LoadConfig(os.Getenv("CATALOGUE_CONFIG_FILE"))
	if err != nil {
		return err
	}
	libraryService, err := catalogue.New(ctx, s, catalogueConfig)
	if err != nil {
		return err
	}
	go libraryService.Run(ctx)
	var connectionKey []byte
	if file := os.Getenv("CONNECTION_KEY_FILE"); file != "" {
		connectionKey, _ = os.ReadFile(file)
	}
	metadataService := enrichment.New(s, connectionKey)
	metadataService.AllowPublic = os.Getenv("ALLOW_PUBLIC_METADATA") == "true"
	go metadataService.Run(ctx)
	telegramConfig, telegramErr := notify.Load(os.Getenv("TELEGRAM_CONFIG_FILE"))
	if telegramErr != nil {
		log.Print("Telegram configuration could not be loaded; notifications disabled")
	}
	telegramService := notify.New(s, telegramConfig, os.Getenv("TELEGRAM_DELIVERY_ENABLED") == "true")
	go telegramService.Run(ctx)
	releaseService := tracking.New(s, metadataService.TV, metadataService.AllowPublic, os.Getenv("FOLLOW_IMPORTED_SERIES") == "true", telegramService.Ready)
	go releaseService.Run(ctx)
	srv := &http.Server{Addr: cfg.ListenAddr, Handler: httpapi.NewHandler(s.Ping, frontendAssets(), httpapi.WithCatalogue(s, libraryService), httpapi.WithMetadata(metadataService), httpapi.WithReleaseTracking(releaseService, telegramService)), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
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
