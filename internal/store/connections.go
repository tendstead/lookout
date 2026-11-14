// SPDX-License-Identifier: Elastic-2.0
package store

import (
	"context"
	"encoding/json"
	"github.com/tendstead/lookout/internal/metadata"
	"github.com/tendstead/lookout/internal/secrets"
	"time"
)

type Connection struct {
	ID              string                 `json:"id"`
	Kind            string                 `json:"kind"`
	BaseURL         string                 `json:"url"`
	Mappings        []metadata.PathMapping `json:"path_mappings"`
	Verified        bool                   `json:"verified"`
	LastError       string                 `json:"last_error,omitempty"`
	CheckedAt       *time.Time             `json:"checked_at"`
	TokenConfigured bool                   `json:"token_configured"`
}

func (s *Store) SaveConnection(ctx context.Context, c metadata.MediaServer, key []byte) (Connection, error) {
	var out Connection
	if e := c.Validate(); e != nil {
		return out, e
	}
	sealed, e := secrets.Seal(key, []byte(c.Token))
	if e != nil {
		return out, e
	}
	if c.Mappings == nil {
		c.Mappings = []metadata.PathMapping{}
	}
	raw, e := json.Marshal(c.Mappings)
	if e != nil {
		return out, e
	}
	e = s.pool.QueryRow(ctx, `INSERT INTO metadata_connections(kind,base_url,token_cipher,path_mappings) VALUES($1,$2,$3,$4) ON CONFLICT(kind,base_url) DO UPDATE SET token_cipher=excluded.token_cipher,path_mappings=excluded.path_mappings,verified=false,last_error='' RETURNING id::text`, c.Kind, c.BaseURL, sealed, raw).Scan(&out.ID)
	if e != nil {
		return out, e
	}
	out.Kind = c.Kind
	out.BaseURL = c.BaseURL
	out.Mappings = c.Mappings
	out.TokenConfigured = true
	return out, nil
}
func (s *Store) Connections(ctx context.Context) ([]Connection, error) {
	rows, e := s.pool.Query(ctx, "SELECT id::text,kind,base_url,path_mappings,verified,last_error,checked_at FROM metadata_connections ORDER BY kind,base_url")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Connection{}
	for rows.Next() {
		var c Connection
		var raw []byte
		if e = rows.Scan(&c.ID, &c.Kind, &c.BaseURL, &raw, &c.Verified, &c.LastError, &c.CheckedAt); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(raw, &c.Mappings); e != nil {
			return nil, e
		}
		c.TokenConfigured = true
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *Store) ConnectionConfig(ctx context.Context, id string, key []byte) (metadata.MediaServer, error) {
	var c metadata.MediaServer
	var sealed, raw []byte
	e := s.pool.QueryRow(ctx, "SELECT kind,base_url,token_cipher,path_mappings FROM metadata_connections WHERE id::text=$1", id).Scan(&c.Kind, &c.BaseURL, &sealed, &raw)
	if e != nil {
		return c, e
	}
	token, e := secrets.Open(key, sealed)
	if e != nil {
		return c, e
	}
	c.Token = string(token)
	e = json.Unmarshal(raw, &c.Mappings)
	return c, e
}
func (s *Store) ConnectionVerified(ctx context.Context, id string, verified bool) error {
	message := ""
	if !verified {
		message = "Connection could not be verified"
	}
	_, e := s.pool.Exec(ctx, "UPDATE metadata_connections SET verified=$2,last_error=$3,checked_at=now() WHERE id::text=$1", id, verified, message)
	return e
}
