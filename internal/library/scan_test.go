package library

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"strings"
	"testing"
	"time"
)

type entry struct {
	name string
	mode fs.FileMode
}

func (e entry) Name() string       { return e.name }
func (e entry) Size() int64        { return 99 }
func (e entry) Mode() fs.FileMode  { return e.mode }
func (e entry) ModTime() time.Time { return time.Unix(10, 0) }
func (e entry) IsDir() bool        { return e.mode.IsDir() }
func (e entry) Sys() any           { return nil }

type fakeReader struct {
	dirs    map[string][]fs.FileInfo
	opened  []string
	failure string
}

func (f *fakeReader) ReadDir(_ context.Context, p string) ([]fs.FileInfo, error) {
	if p == f.failure {
		return nil, errors.New("offline")
	}
	v, ok := f.dirs[p]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return v, nil
}
func (f *fakeReader) Lstat(_ context.Context, p string) (fs.FileInfo, error) {
	if _, ok := f.dirs[p]; ok {
		return entry{"root", fs.ModeDir}, nil
	}
	return nil, fs.ErrNotExist
}
func (f *fakeReader) RealPath(_ context.Context, p string) (string, error) { return p, nil }
func (f *fakeReader) Open(_ context.Context, p string) (io.ReadCloser, error) {
	f.opened = append(f.opened, p)
	return io.NopCloser(strings.NewReader("<tvshow><title>Trusted title</title></tvshow>")), nil
}
func TestScanReadOnlyConfined(t *testing.T) {
	f := &fakeReader{dirs: map[string][]fs.FileInfo{"/media": {entry{"Show", fs.ModeDir}, entry{"escape", fs.ModeSymlink}, entry{"../outside.mkv", 0}}, "/media/Show": {entry{"tvshow.nfo", 0}, entry{"Season 1", fs.ModeDir}}, "/media/Show/Season 1": {entry{"Show.S01E02.mkv", 0}, entry{"subtitle.srt", 0}, entry{"escape.mkv", fs.ModeSymlink}}}}
	var files []FileObservation
	_, err := Scan(context.Background(), f, Root{ID: "a", Path: "/media", Category: "series"}, func(v FileObservation) error { files = append(files, v); return nil })
	if err == nil {
		t.Fatal("malicious entry must fail closed")
	}
	f.dirs["/media"] = f.dirs["/media"][:2]
	stats, err := Scan(context.Background(), f, Root{ID: "a", Path: "/media", Category: "series"}, func(v FileObservation) error { files = append(files, v); return nil })
	if err != nil || stats.Media != 1 || len(files) != 1 || files[0].Title.Title != "Trusted title" || files[0].RelativePath != "Show/Season 1/Show.S01E02.mkv" {
		t.Fatalf("%#v %#v %v", stats, files, err)
	}
	for _, p := range f.opened {
		if !strings.HasPrefix(p, "/media/") {
			t.Fatal("escaped")
		}
	}
}
func TestPartialScanReturnsFailure(t *testing.T) {
	f := &fakeReader{dirs: map[string][]fs.FileInfo{"/media": {entry{"Show", fs.ModeDir}}}, failure: "/media/Show"}
	_, err := Scan(context.Background(), f, Root{ID: "a", Path: "/media", Category: "series"}, func(FileObservation) error { return nil })
	if err == nil {
		t.Fatal("partial scan reported complete")
	}
}
func TestFilmCollectionsProduceIndividualTitles(t *testing.T) {
	f := &fakeReader{dirs: map[string][]fs.FileInfo{"/media": {entry{"Collection", fs.ModeDir}}, "/media/Collection": {entry{"Film A (2000)", fs.ModeDir}, entry{"Film B (2001)", fs.ModeDir}, entry{"Flat.Film.2020.1080p.mkv", 0}, entry{"Other.Film.2021.1080p.mkv", 0}}, "/media/Collection/Film A (2000)": {entry{"release.mkv", 0}}, "/media/Collection/Film B (2001)": {entry{"release.mkv", 0}}}}
	var files []FileObservation
	_, err := Scan(context.Background(), f, Root{ID: "a", Path: "/media", Category: "movie"}, func(v FileObservation) error { files = append(files, v); return nil })
	if err != nil || len(files) != 4 {
		t.Fatal(err, len(files))
	}
	names := map[string]bool{}
	groups := map[string]bool{}
	for _, v := range files {
		names[v.Title.Title] = true
		groups[v.Group] = true
	}
	for _, name := range []string{"Film A", "Film B", "Flat Film", "Other Film"} {
		if !names[name] {
			t.Fatalf("missing film title %s: %#v", name, names)
		}
	}
	if len(groups) != 4 {
		t.Fatal("collection collapsed individual films")
	}
}
