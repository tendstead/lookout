// SPDX-License-Identifier: Elastic-2.0
package store

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/tendstead/lookout/internal/metadata"
	"github.com/tendstead/lookout/internal/secrets"
	"net/url"
	"time"
)

type Connection struct {
	ServerID        string                 `json:"server_id,omitempty"`
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
	if err := c.Validate(); err != nil {
		return Connection{}, err
	}
	sealed, err := secrets.Seal(key, []byte(c.Token))
	if err != nil {
		return Connection{}, err
	}
	return s.saveConnection(ctx, c, sealed)
}

// ConfigureConnection reserves the existing connection identity for a TOML entry.
// Its token is supplied by the caller at runtime. Any legacy ciphertext is retained.
func (s *Store) ConfigureConnection(ctx context.Context, c metadata.MediaServer) (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	if c.ServerID == "" {
		return "", errors.New("configured connections require a stable server ID")
	}
	v, err := s.saveConnection(ctx, c, nil)
	return v.ID, err
}

func (s *Store) saveConnection(ctx context.Context, c metadata.MediaServer, sealed []byte) (Connection, error) {
	var out Connection
	if c.Mappings == nil {
		c.Mappings = []metadata.PathMapping{}
	}
	raw, e := json.Marshal(c.Mappings)
	if e != nil {
		return out, e
	}
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return out, e
	}
	defer tx.Rollback(ctx)
	// Validate explicit linkage before changing credentials or mappings.
	for _, m := range c.Mappings {
		var host string
		if e = tx.QueryRow(ctx, `SELECT server_id FROM library_sources WHERE id=$1 AND root_path=$2 AND adapter='sftp'`, m.SourceID, m.SourcePrefix).Scan(&host); e != nil {
			return out, e
		}
		if c.ServerID != "" && c.ServerID != host {
			return out, ErrMetadataConflict
		}
		c.ServerID = host
	}
	var existingID, existingServer string
	var count int
	e = tx.QueryRow(ctx, `SELECT id::text,COALESCE(server_id,'') FROM metadata_connections WHERE kind=$1 AND base_url=$2`, c.Kind, c.BaseURL).Scan(&existingID, &existingServer)
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return out, e
	}
	if sealed == nil && existingID == "" {
		// Retain the inventory and artwork identity when a configured URL changes.
		e = tx.QueryRow(ctx, `SELECT COALESCE(min(id::text),''),count(*) FROM metadata_connections WHERE kind=$1 AND server_id=$2`, c.Kind, c.ServerID).Scan(&existingID, &count)
		if e != nil {
			return out, e
		}
		if count > 1 {
			return out, errors.New("multiple existing connections for server; use its existing URL")
		}
		if existingID != "" {
			existingServer = c.ServerID
		}
	}
	if c.ServerID == "" && existingServer != "" {
		c.ServerID = existingServer
	}
	if sealed != nil && c.ServerID != "" && len(c.Mappings) == 0 {
		var hasSFTP bool
		if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM library_sources WHERE server_id=$1 AND adapter='sftp')`, c.ServerID).Scan(&hasSFTP); e != nil {
			return out, e
		}
		if hasSFTP {
			return out, errors.New("link SFTP storage with explicit path mappings")
		}
	}
	if existingServer != "" && c.ServerID != "" && existingServer != c.ServerID {
		return out, errors.New("existing inventory server cannot be reassigned")
	}
	switch {
	case sealed != nil:
		e = tx.QueryRow(ctx, `INSERT INTO metadata_connections(kind,base_url,token_cipher,path_mappings) VALUES($1,$2,$3,$4) ON CONFLICT(kind,base_url) DO UPDATE SET token_cipher=excluded.token_cipher,path_mappings=excluded.path_mappings,verified=false,last_error='' RETURNING id::text`, c.Kind, c.BaseURL, sealed, raw).Scan(&out.ID)
	case existingID == "":
		e = tx.QueryRow(ctx, `INSERT INTO metadata_connections(kind,base_url,token_cipher,path_mappings) VALUES($1,$2,''::bytea,$3) RETURNING id::text`, c.Kind, c.BaseURL, raw).Scan(&out.ID)
	default:
		out.ID = existingID
		_, e = tx.Exec(ctx, `UPDATE metadata_connections SET token_cipher=CASE WHEN base_url=$2 THEN token_cipher ELSE ''::bytea END,base_url=$2,path_mappings=$3,verified=false,last_error='',checked_at=NULL WHERE id::text=$1`, out.ID, c.BaseURL, raw)
	}
	if e != nil {
		return out, e
	}
	if c.ServerID == "" {
		c.ServerID = "media-" + out.ID
		u, _ := url.Parse(c.BaseURL)
		if _, e = tx.Exec(ctx, `INSERT INTO library_servers(id,name) VALUES($1,$2)`, c.ServerID, u.Hostname()); e != nil {
			return out, e
		}
	}
	if _, e = tx.Exec(ctx, `UPDATE metadata_connections SET server_id=$2 WHERE id::text=$1`, out.ID, c.ServerID); e != nil {
		return out, e
	}
	if e = tx.Commit(ctx); e != nil {
		return out, e
	}
	out.ServerID = c.ServerID
	out.Kind = c.Kind
	out.BaseURL = c.BaseURL
	out.Mappings = c.Mappings
	out.TokenConfigured = true
	return out, nil
}
func (s *Store) Connections(ctx context.Context) ([]Connection, error) {
	rows, e := s.pool.Query(ctx, "SELECT id::text,kind,base_url,path_mappings,verified,last_error,checked_at,COALESCE(server_id,''),octet_length(token_cipher)>0 FROM metadata_connections ORDER BY kind,base_url")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Connection{}
	for rows.Next() {
		var c Connection
		var raw []byte
		if e = rows.Scan(&c.ID, &c.Kind, &c.BaseURL, &raw, &c.Verified, &c.LastError, &c.CheckedAt, &c.ServerID, &c.TokenConfigured); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(raw, &c.Mappings); e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *Store) ConnectionConfig(ctx context.Context, id string, key []byte) (metadata.MediaServer, error) {
	var c metadata.MediaServer
	var sealed, raw []byte
	e := s.pool.QueryRow(ctx, "SELECT kind,base_url,token_cipher,path_mappings,COALESCE(server_id,'') FROM metadata_connections WHERE id::text=$1", id).Scan(&c.Kind, &c.BaseURL, &sealed, &raw, &c.ServerID)
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
