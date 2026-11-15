package library

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"io/fs"
	"testing"
)

type memoryArchive struct{ *bytes.Reader }

func (m memoryArchive) Close() error { return nil }

type zipReaderFixture struct {
	*fakeReader
	data []byte
}

func (f zipReaderFixture) Open(_ context.Context, _ string) (io.ReadCloser, error) {
	return memoryArchive{bytes.NewReader(f.data)}, nil
}
func TestZIPMembersAreCataloguedWithoutExtracting(t *testing.T) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for _, name := range []string{"Season 1/Show.S01E02.mkv", "../escape.S01E03.mkv", "notes.txt"} {
		w, e := z.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		_, _ = w.Write([]byte("not actual video; only directory must be read"))
	}
	if e := z.Close(); e != nil {
		t.Fatal(e)
	}
	reader := zipReaderFixture{fakeReader: &fakeReader{dirs: map[string][]fs.FileInfo{}}, data: b.Bytes()}
	items, e := InspectZIP(context.Background(), reader, "/media/Show/season.zip", int64(b.Len()))
	if e != nil || len(items) != 1 || items[0].Name != "Season 1/Show.S01E02.mkv" {
		t.Fatalf("%#v %v", items, e)
	}
}
