// SPDX-License-Identifier: Elastic-2.0
package config

import (
	"errors"
	"net/url"
	"strings"
)

type Config struct {
	ListenAddr           string
	DatabaseURL          string
	DatabasePasswordFile string
}

func Load(lookup func(string) (string, bool), readFile func(string) ([]byte, error)) (Config, error) {
	var c Config
	c.ListenAddr, _ = lookup("LISTEN_ADDR")
	if c.ListenAddr == "" {
		c.ListenAddr = "127.0.0.1:8080"
	}
	raw, _ := lookup("DATABASE_URL")
	u, err := url.Parse(raw)
	if err != nil || u == nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Hostname() == "" || u.User == nil || u.User.Username() == "" || u.Path == "" || u.Path == "/" {
		return c, errors.New("DATABASE_URL must identify a PostgreSQL host, user and database")
	}
	query, queryErr := url.ParseQuery(u.RawQuery)
	if queryErr != nil {
		return c, errors.New("invalid database URL parameters")
	}
	c.DatabasePasswordFile, _ = lookup("DB_PASSWORD_FILE")
	if c.DatabasePasswordFile != "" {
		if _, set := u.User.Password(); set || query.Has("password") {
			return c, errors.New("configure a database password through one mechanism only")
		}
		b, e := readFile(c.DatabasePasswordFile)
		if e != nil || len(strings.TrimSpace(string(b))) == 0 {
			return c, errors.New("cannot read database password file")
		}
		u.User = url.UserPassword(u.User.Username(), strings.TrimSpace(string(b)))
	}
	c.DatabaseURL = u.String()
	return c, nil
}
