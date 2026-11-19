// SPDX-License-Identifier: Elastic-2.0
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tendstead/lookout/internal/catalogue"
	"github.com/tendstead/lookout/internal/config"
	"github.com/tendstead/lookout/internal/enrichment"
	"github.com/tendstead/lookout/internal/httpapi"
	"github.com/tendstead/lookout/internal/notify"
	"github.com/tendstead/lookout/internal/store"
	"github.com/tendstead/lookout/internal/tracking"
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
	file := defaultConfigFile
	if len(os.Args) > 1 && os.Args[1] == "-check-config" {
		if len(os.Args) > 2 {
			file = os.Args[2]
		}
		if _, err := loadFileConfig(file); err != nil {
			log.Print(err)
			os.Exit(1)
		}
		log.Print("Lookout configuration is valid")
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "-config" {
		if len(os.Args) != 3 {
			log.Print("usage: lookout -config /path/to/lookout.toml")
			os.Exit(1)
		}
		file = os.Args[2]
	} else if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
		file = "" // An existing deployment may still use the environment and catalogue JSON.
	}
	if err := run(file); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run(file string) error {
	var settings fileSettings
	var key []byte
	var err error
	if file != "" {
		settings, err = loadFileConfig(file)
		if err != nil {
			return err
		}
	} else {
		settings, key, err = loadLegacySettings()
		if err != nil {
			return err
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	s, err := store.Open(startup, settings.database)
	if err != nil {
		return errors.New("could not connect to the Lookout database")
	}
	defer s.Close()
	if err := s.Migrate(startup); err != nil {
		return errors.New("could not migrate the Lookout database")
	}

	types := make(map[string]string, len(settings.servers))
	for id, server := range settings.servers {
		if err := s.EnsureServer(startup, id, server.Name); err != nil {
			return errors.New("could not register a configured server")
		}
		if err := s.SaveServer(startup, server); err != nil {
			return errors.New("could not apply configured server settings")
		}
		types[id] = server.Type
	}
	libraryService, err := catalogue.New(startup, s, settings.catalogue)
	if err != nil {
		return errors.New("could not initialize the catalogue; check source IDs and paths")
	}
	metadataService := enrichment.New(s, key)
	metadataService.AllowPublic = settings.public
	var sourceIDs []string
	if file != "" {
		sourceIDs, err = metadataService.ConfigureConnections(startup, settings.connections)
		if err != nil {
			return errors.New("could not register configured media connections; check server IDs, URLs and path mappings")
		}
		for _, root := range settings.catalogue.Roots {
			sourceIDs = append(sourceIDs, root.ID)
		}
	}
	cancel()

	go libraryService.Run(ctx)
	go metadataService.Run(ctx)
	go metadataService.RunInventories(ctx)
	telegramService := notify.New(s, settings.telegram, settings.deliver)
	go telegramService.Run(ctx)
	releaseService := tracking.New(s, metadataService.TV, metadataService.AllowPublic, settings.follow, telegramService.DeliveryAuthorized)
	go releaseService.Run(ctx)
	options := []httpapi.Option{
		httpapi.WithCatalogue(s, libraryService),
		httpapi.WithMultiServer(s),
		httpapi.WithMetadata(metadataService),
		httpapi.WithReleaseTracking(releaseService, telegramService),
	}
	if file != "" {
		options = append(options, httpapi.WithReadOnly(), httpapi.WithConfiguredSources(types, sourceIDs))
	}
	srv := &http.Server{
		Addr: settings.database.ListenAddr, Handler: httpapi.NewHandler(s.Ping, frontendAssets(), options...),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second,
		IdleTimeout: 60 * time.Second,
	}
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe() }()
	log.Print("Lookout listening on ", settings.database.ListenAddr)
	select {
	case err = <-done:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	case <-ctx.Done():
		end, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		return srv.Shutdown(end)
	}
}

func loadLegacySettings() (fileSettings, []byte, error) {
	var settings fileSettings
	var err error
	settings.database, err = config.Load(os.LookupEnv, os.ReadFile)
	if err != nil {
		return settings, nil, err
	}
	settings.catalogue, err = catalogue.LoadConfig(os.Getenv("CATALOGUE_CONFIG_FILE"))
	if err != nil {
		return settings, nil, err
	}
	var key []byte
	if file := os.Getenv("CONNECTION_KEY_FILE"); file != "" {
		key, _ = os.ReadFile(file)
	}
	settings.public = os.Getenv("ALLOW_PUBLIC_METADATA") == "true"
	settings.follow = os.Getenv("FOLLOW_IMPORTED_SERIES") == "true"
	settings.telegram, err = notify.Load(os.Getenv("TELEGRAM_CONFIG_FILE"))
	if err != nil {
		log.Print("Telegram configuration could not be loaded; notifications disabled")
	}
	settings.deliver = os.Getenv("TELEGRAM_DELIVERY_ENABLED") == "true"
	return settings, key, nil
}
