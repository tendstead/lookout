// SPDX-License-Identifier: Elastic-2.0
package store

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tendstead/lookout/internal/config"
	"time"
)

type Store struct{ pool *pgxpool.Pool }

func Open(ctx context.Context, c config.Config) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(c.DatabaseURL)
	if err != nil {
		return nil, errors.New("invalid database configuration")
	}
	cfg.MaxConns = 4
	cfg.MinConns = 0
	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, errors.New("database pool unavailable")
	}
	return &Store{p}, nil
}
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }
func (s *Store) Close()                         { s.pool.Close() }

type migration struct {
	version int
	sql     string
}

func (s *Store) Migrate(ctx context.Context) error {
	return s.migrate(ctx, []migration{{1, initial}, {2, catalogue}, {3, metadataSchema}})
}
func (s *Store) apply(ctx context.Context, version int, sql string) error {
	return s.migrate(ctx, []migration{{version, sql}})
}
func (s *Store) migrate(ctx context.Context, migrations []migration) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		end, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = tx.Rollback(end)
	}()
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(726431209)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "CREATE TABLE IF NOT EXISTS lookout_schema_versions(version integer PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())"); err != nil {
		return err
	}
	var current int
	if err = tx.QueryRow(ctx, "SELECT COALESCE(max(version),0) FROM lookout_schema_versions").Scan(&current); err != nil {
		return err
	}
	if current > migrations[len(migrations)-1].version {
		return errors.New("database schema is newer than this application")
	}
	for _, m := range migrations {
		if current < m.version {
			if _, err = tx.Exec(ctx, m.sql); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, "INSERT INTO lookout_schema_versions(version) VALUES ($1)", m.version); err != nil {
				return fmt.Errorf("record migration: %w", err)
			}
		}
	}
	return tx.Commit(ctx)
}
