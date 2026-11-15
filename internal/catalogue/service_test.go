package catalogue

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMultipleServersAllowIdenticalRoots(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.json")
	if e := os.WriteFile(file, []byte(`{"servers":[{"id":"a","ssh":{"host":"a"},"roots":[{"id":"a-films","path":"/films","category":"movie"}]},{"id":"b","ssh":{"host":"b"},"roots":[{"id":"b-films","path":"/films","category":"movie"}]}]}`), 0600); e != nil {
		t.Fatal(e)
	}
	c, e := LoadConfig(file)
	if e != nil {
		t.Fatal(e)
	}
	if len(c.Servers) != 2 || len(c.Roots) != 2 || c.Roots[1].ServerID != "b" {
		t.Fatalf("multi host config lost: %#v", c)
	}
}
func TestLegacyConfigHasStableServer(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(file, []byte(`{"ssh":{"host":"nas"},"roots":[{"id":"films","path":"/films","category":"movie"}]}`), 0600)
	c, e := LoadConfig(file)
	if e != nil {
		t.Fatal(e)
	}
	if len(c.Servers) != 1 || c.Servers[0].ID != "legacy" || c.Roots[0].ServerID != "legacy" {
		t.Fatal(c)
	}
}
func TestDuplicateSourceAcrossServersRejected(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(file, []byte(`{"servers":[{"id":"a","roots":[{"id":"films","path":"/films","category":"movie"}]},{"id":"b","roots":[{"id":"films","path":"/films","category":"movie"}]}]}`), 0600)
	if _, e := LoadConfig(file); e == nil {
		t.Fatal("duplicate source ID accepted")
	}
}
