package library

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestShellReaderListsAndReadsFiles(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("SSH shell adapter requires Python 3")
	}
	root := filepath.Join(t.TempDir(), "media's files")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "movie's\nname.zip")
	if err := os.WriteFile(file, []byte("abcdef"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(file, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	r := &ShellReader{
		root: root,
		run: func(ctx context.Context, command string, limit int64) ([]byte, error) {
			return exec.CommandContext(ctx, "sh", "-c", command).Output()
		},
	}
	ctx := context.Background()
	items, err := r.ReadDir(ctx, root)
	if err != nil || len(items) != 2 {
		t.Fatalf("listing failed: %v", err)
	}
	f, err := r.Open(ctx, file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b := make([]byte, 3)
	n, err := f.(io.ReaderAt).ReadAt(b, 2)
	if err != nil || n != 3 || string(b) != "cde" {
		t.Fatal("expected bounded offset read")
	}
	if _, err := r.Open(ctx, filepath.Join(root, "..", "secret")); err == nil {
		t.Fatal("expected out-of-root path rejection")
	}
	f, err = r.Open(ctx, filepath.Join(root, "link"))
	if err == nil {
		defer f.Close()
		_, err = f.Read(b)
	}
	if err == nil {
		t.Fatal("expected symlink read rejection")
	}
}
