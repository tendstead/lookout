package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/jackc/pgx/v5"
	"github.com/tendstead/lookout/internal/config"
	urlpkg "net/url"
	"os"
	"strings"
	"sync"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("run make service-test")
	}
	admin, e := Open(context.Background(), config.Config{DatabaseURL: url})
	if e != nil {
		t.Fatal(e)
	}
	var random [8]byte
	if _, e = rand.Read(random[:]); e != nil {
		t.Fatal(e)
	}
	schema := "test_" + hex.EncodeToString(random[:])
	if _, e = admin.pool.Exec(context.Background(), "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); e != nil {
		t.Fatal(e)
	}
	parsed, e := urlpkg.Parse(url)
	if e != nil {
		t.Fatal(e)
	}
	q := parsed.Query()
	q.Set("search_path", schema)
	parsed.RawQuery = q.Encode()
	s, e := Open(context.Background(), config.Config{DatabaseURL: parsed.String()})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		s.Close()
		_, _ = admin.pool.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		admin.Close()
	})
	return s
}
func TestMigrateTwice(t *testing.T) {
	s := testStore(t)
	for range 2 {
		if e := s.Migrate(context.Background()); e != nil {
			t.Fatal(e)
		}
	}
}
func TestConcurrentMigrate(t *testing.T) {
	s := testStore(t)
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			if e := s.Migrate(context.Background()); e != nil {
				t.Error(e)
			}
		})
	}
	wg.Wait()
}
func TestPoolBound(t *testing.T) {
	s := testStore(t)
	if s.pool.Config().MaxConns != 4 {
		t.Fatal("unbounded pool")
	}
}
func TestFailedMigrationRollsBack(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	e := s.apply(ctx, 999, "CREATE TABLE rollback_probe(id int); SELECT no_such_function();")
	if e == nil {
		t.Fatal("expected SQL failure")
	}
	var present bool
	if e = s.pool.QueryRow(ctx, "SELECT to_regclass('rollback_probe') IS NOT NULL").Scan(&present); e != nil || present {
		t.Fatalf("not rolled back: %v", e)
	}
}
func TestNewerSchemaRejected(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if e := s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	_, e := s.pool.Exec(ctx, "INSERT INTO lookout_schema_versions(version) VALUES (999)")
	if e != nil {
		t.Fatal(e)
	}
	defer s.pool.Exec(ctx, "DELETE FROM lookout_schema_versions WHERE version=999")
	if e = s.Migrate(ctx); e == nil || !strings.Contains(e.Error(), "newer") {
		t.Fatal("accepted future schema")
	}
}
