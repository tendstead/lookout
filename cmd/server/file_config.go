// SPDX-License-Identifier: Elastic-2.0
package main

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/tendstead/lookout/internal/catalogue"
	"github.com/tendstead/lookout/internal/config"
	"github.com/tendstead/lookout/internal/library"
	"github.com/tendstead/lookout/internal/metadata"
	"github.com/tendstead/lookout/internal/notify"
	"github.com/tendstead/lookout/internal/store"
)

const defaultConfigFile = "/run/lookout/lookout.toml"

type fileSettings struct {
	database      config.Config
	catalogue     catalogue.Config
	servers       map[string]store.Server
	connections   []metadata.MediaServer
	telegram      *notify.Config
	deliver       bool
	public        bool
	follow        bool
	trackingEdits bool
}

type mediaSettings struct {
	URL   string `toml:"url"`
	Token string `toml:"token"`
}

type serverSettings struct {
	mediaSettings
	ID       string         `toml:"id"`
	Name     string         `toml:"name"`
	Type     string         `toml:"type"`
	Scan     int            `toml:"scan_hours"`
	Stale    int            `toml:"stale_hours"`
	Host     string         `toml:"host"`
	Port     int            `toml:"port"`
	User     string         `toml:"username"`
	Password string         `toml:"password"`
	Key      string         `toml:"private_key_file"`
	Known    string         `toml:"known_hosts_file"`
	Roots    []library.Root `toml:"roots"`
	Enrich   *struct {
		mediaSettings
		Type     string `toml:"type"`
		Mappings []struct {
			Prefix string `toml:"server_prefix"`
			Source string `toml:"source_id"`
		} `toml:"path_mappings"`
	} `toml:"metadata"`
}

func loadFileConfig(file string) (fileSettings, error) {
	var settings fileSettings
	var raw struct {
		Listen   string `toml:"listen_addr"`
		Database struct {
			URL      string `toml:"url"`
			Password string `toml:"password"`
		} `toml:"database"`
		Servers  []serverSettings `toml:"servers"`
		Telegram *struct {
			Token   string `toml:"bot_token"`
			Chat    string `toml:"chat_id"`
			Enabled bool   `toml:"delivery_enabled"`
		} `toml:"telegram"`
		Public        bool `toml:"allow_public_metadata"`
		Follow        bool `toml:"follow_imported_series"`
		TrackingEdits bool `toml:"allow_tracking_edits"`
	}
	decoded, err := toml.DecodeFile(file, &raw)
	if err != nil {
		return settings, errors.New("invalid or unreadable Lookout TOML")
	}
	if len(decoded.Undecoded()) != 0 {
		return settings, errors.New("unknown setting in Lookout TOML; check field names against the example")
	}
	if raw.Listen == "" {
		raw.Listen = ":8080"
	}
	_, port, err := net.SplitHostPort(raw.Listen)
	listenPort, portErr := strconv.Atoi(port)
	if err != nil || portErr != nil || listenPort < 1 || listenPort > 65535 {
		return settings, errors.New("listen_addr must specify an address and valid port")
	}
	settings.database, err = config.Load(func(key string) (string, bool) {
		switch key {
		case "DATABASE_URL":
			return raw.Database.URL, true
		case "LISTEN_ADDR":
			return raw.Listen, true
		default:
			return "", false
		}
	}, nil)
	if err != nil {
		return settings, errors.New("invalid [database] URL; specify PostgreSQL host, username and database")
	}
	if raw.Database.Password != "" {
		database, _ := url.Parse(settings.database.DatabaseURL)
		if _, set := database.User.Password(); set || database.Query().Has("password") {
			return settings, errors.New("configure the database password through one mechanism only")
		}
		database.User = url.UserPassword(database.User.Username(), raw.Database.Password)
		settings.database.DatabaseURL = database.String()
	}
	settings.public, settings.follow, settings.trackingEdits = raw.Public, raw.Follow, raw.TrackingEdits
	if raw.Telegram != nil {
		settings.telegram, err = notify.NewConfig(raw.Telegram.Token, raw.Telegram.Chat)
		if err != nil {
			return settings, err
		}
		settings.deliver = raw.Telegram.Enabled
	}
	settings.servers = make(map[string]store.Server, len(raw.Servers))
	endpoints := make(map[[2]string]bool)
	for index, v := range raw.Servers {
		if err = settings.addServer(v, filepath.Dir(file), endpoints); err != nil {
			return settings, fmt.Errorf("servers entry %d: %w", index+1, err)
		}
	}
	settings.catalogue, err = catalogue.ValidateConfig(settings.catalogue)
	return settings, err
}

func (s *fileSettings) addServer(v serverSettings, dir string, endpoints map[[2]string]bool) error {
	if v.ID == "" || len(v.ID) > 100 || strings.ContainsAny(v.ID, "/\\\x00") || s.servers[v.ID].ID != "" {
		return errors.New("set a unique stable id no longer than 100 characters, without slashes")
	}
	if v.Name == "" {
		v.Name = v.ID
	}
	if v.Scan == 0 {
		v.Scan = 6
	}
	if v.Stale == 0 {
		v.Stale = max(24, v.Scan)
	}
	if len(v.Name) > 100 || strings.TrimSpace(v.Name) == "" || v.Scan < 1 || v.Scan > 168 || v.Stale < v.Scan || v.Stale > 720 {
		return errors.New("invalid name, scan_hours or stale_hours")
	}
	switch v.Type {
	case "jellyfin", "plex":
		if v.Host != "" || v.Port != 0 || v.User != "" || v.Password != "" || v.Key != "" || v.Known != "" || len(v.Roots) != 0 || v.Enrich != nil {
			return errors.New("Jellyfin and Plex inventory use url and token")
		}
		if err := s.addConnection(v.ID, v.Type, v.mediaSettings, nil, endpoints); err != nil {
			return err
		}
	case "sftp", "ssh":
		if v.URL != "" || v.Token != "" {
			return errors.New("put optional Jellyfin or Plex settings under servers.metadata")
		}
		if v.Host == "" || v.User == "" || v.Known == "" || len(v.Roots) == 0 || v.Port < 0 || v.Port > 65535 || (v.Key == "") == (v.Password == "") {
			return errors.New("SSH and SFTP need host, username, known_hosts_file, roots and exactly one of private_key_file or password")
		}
		roots := make(map[string]string, len(v.Roots))
		for i := range v.Roots {
			root := &v.Roots[i]
			roots[root.ID] = root.Path
			if root.Label == "" {
				root.Label = path.Base(root.Path)
			}
		}
		if v.Enrich != nil {
			var mappings []metadata.PathMapping
			seen := make(map[string]bool, len(v.Enrich.Mappings))
			for _, m := range v.Enrich.Mappings {
				if seen[m.Prefix] || roots[m.Source] == "" || !path.IsAbs(m.Prefix) || path.Clean(m.Prefix) != m.Prefix || m.Prefix == "/" {
					return errors.New("metadata mappings must link an absolute media-server path to a root id on this server")
				}
				seen[m.Prefix] = true
				mappings = append(mappings, metadata.PathMapping{ServerPrefix: m.Prefix, SourceID: m.Source, SourcePrefix: roots[m.Source]})
			}
			if len(mappings) == 0 {
				return errors.New("optional metadata requires path mappings")
			}
			if err := s.addConnection(v.ID, v.Enrich.Type, v.Enrich.mediaSettings, mappings, endpoints); err != nil {
				return err
			}
		}
		s.catalogue.Servers = append(s.catalogue.Servers, catalogue.ServerConfig{
			ID: v.ID, Name: v.Name, Type: v.Type, Roots: v.Roots,
			SSH: library.SSHConfig{Host: v.Host, Port: v.Port, User: v.User, Password: v.Password,
				KeyFile: configRelative(dir, v.Key), KnownHostsFile: configRelative(dir, v.Known)},
		})
	default:
		return errors.New("type must be jellyfin, plex, sftp or ssh")
	}
	s.servers[v.ID] = store.Server{ID: v.ID, Name: v.Name, Type: v.Type, ScanHours: v.Scan, StaleHours: v.Stale}
	return nil
}

func (s *fileSettings) addConnection(id, kind string, v mediaSettings, mappings []metadata.PathMapping, endpoints map[[2]string]bool) error {
	connection := metadata.MediaServer{ServerID: id, Kind: kind, BaseURL: v.URL, Token: v.Token, Mappings: mappings}
	if err := connection.Validate(); err != nil {
		return err
	}
	endpoint := [2]string{kind, connection.BaseURL}
	if endpoints[endpoint] {
		return errors.New("a media server URL can only be configured once per type")
	}
	endpoints[endpoint] = true
	s.connections = append(s.connections, connection)
	return nil
}

func configRelative(dir, file string) string {
	if file != "" && !filepath.IsAbs(file) {
		return filepath.Join(dir, file)
	}
	return file
}
