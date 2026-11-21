package main

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestFileConfigLoadsInlineSecretsAndServerTypes(t *testing.T) {
	file := filepath.Join(t.TempDir(), "lookout.toml")
	if err := os.WriteFile(file, []byte(`
[database]
url = "postgres://lookout@db/lookout"
password = 'p#@ss' # Inline comments work.

[[servers]]
id = "family"
type = "jellyfin"
url = "https://jellyfin.example"
token = "api-token"

[[servers]]
id = "home"
type = "ssh"
host = "nas.example"
username = "reader"
password = 'another#password'
known_hosts_file = "known_hosts"

[[servers.roots]]
id = "films"
path = "/media/Films"
category = "movie"
`), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadFileConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	db, err := url.Parse(cfg.database.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if password, _ := db.User.Password(); password != "p#@ss" {
		t.Fatal("incorrect database password")
	}
	if len(cfg.connections) != 1 || cfg.connections[0].Token != "api-token" || cfg.connections[0].ServerID != "family" {
		t.Fatal("expected a configured API inventory")
	}
	if len(cfg.catalogue.Servers) != 1 || cfg.catalogue.Servers[0].Type != "ssh" || cfg.catalogue.Servers[0].SSH.Password != "another#password" {
		t.Fatal("expected SSH shell with password")
	}
	if cfg.catalogue.Servers[0].SSH.KnownHostsFile != filepath.Join(filepath.Dir(file), "known_hosts") {
		t.Fatal("key paths must be relative to the configuration file")
	}
}

func TestFileConfigErrorsDoNotExposeSecrets(t *testing.T) {
	file := filepath.Join(t.TempDir(), "lookout.toml")
	if err := os.WriteFile(file, []byte("[database]\npassword = \"private-value\" trailing"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadFileConfig(file); err == nil || err.Error() != "invalid or unreadable Lookout TOML" {
		t.Fatal("expected a sanitized TOML parse error")
	}
}
