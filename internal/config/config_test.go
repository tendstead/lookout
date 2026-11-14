package config

import (
	"errors"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	for _, tc := range []struct {
		name    string
		env     map[string]string
		file    string
		fileErr error
		bad     bool
	}{
		{name: "default", env: map[string]string{"DATABASE_URL": "postgres://owner@db/lookout"}},
		{name: "missing", bad: true},
		{name: "malformed", env: map[string]string{"DATABASE_URL": "https://secret@example.com"}, bad: true},
		{name: "password file", env: map[string]string{"DATABASE_URL": "postgres://owner@db/lookout", "DB_PASSWORD_FILE": "/secret", "LISTEN_ADDR": ":9000"}, file: "secret\n"},
		{name: "unreadable", env: map[string]string{"DATABASE_URL": "postgres://owner@db/lookout", "DB_PASSWORD_FILE": "/secret"}, fileErr: errors.New("secret"), bad: true},
		{name: "conflict", env: map[string]string{"DATABASE_URL": "postgres://owner:secret@db/lookout", "DB_PASSWORD_FILE": "/secret"}, bad: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Load(func(k string) (string, bool) { v, ok := tc.env[k]; return v, ok }, func(string) ([]byte, error) { return []byte(tc.file), tc.fileErr })
			if (err != nil) != tc.bad {
				t.Fatalf("unexpected outcome: %v", err)
			}
			if err != nil && strings.Contains(err.Error(), "secret") {
				t.Fatal("credential leaked")
			}
			if err == nil {
				want := "127.0.0.1:8080"
				if tc.env["LISTEN_ADDR"] != "" {
					want = tc.env["LISTEN_ADDR"]
				}
				if cfg.ListenAddr != want {
					t.Fatal(cfg.ListenAddr)
				}
			}
		})
	}
}
