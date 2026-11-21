// SPDX-License-Identifier: Elastic-2.0
package library

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"path"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// The script uses paths only as arguments. JSON lines preserve spaces and newlines
// in names. File bytes are requested separately in bounded ranges.
const inventoryScript = `
import json, os, stat, sys
op, root, p, *args = sys.argv[1:]
if os.path.commonpath((root, p)) != root:
    raise ValueError("path outside root")
if op == "realpath":
    print(json.dumps(os.path.realpath(p)))
    sys.exit()
if op == "stat":
    if os.path.realpath(os.path.dirname(p)) != os.path.dirname(p):
        raise ValueError("symlink in parent path")
else:
    if os.path.realpath(p) != p:
        raise ValueError("symlink in path")

def entry(name, st):
    name.encode("utf-8")
    print(json.dumps([name, st.st_mode, st.st_size, st.st_mtime_ns]))

if op == "stat":
    entry(os.path.basename(p), os.lstat(p))
elif op == "list":
    with os.scandir(p) as entries:
        for item in entries:
            entry(item.name, item.stat(follow_symlinks=False))
elif op == "read":
    offset, length = map(int, args)
    if offset < 0 or not 0 <= length <= 4194304:
        raise ValueError("invalid read range")
    fd = os.open(p, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(fd, "rb") as f:
        if not stat.S_ISREG(os.fstat(f.fileno()).st_mode):
            raise ValueError("expected a regular file")
        f.seek(offset)
        sys.stdout.buffer.write(f.read(length))
else:
    raise ValueError("unknown operation")
`

type ShellReader struct {
	root   string
	run    func(context.Context, string, int64) ([]byte, error)
	client *ssh.Client
	stop   func() bool
}

func ConnectShell(ctx context.Context, cfg SSHConfig, root string) (*ShellReader, error) {
	client, stop, err := dialSSH(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &ShellReader{root: root, client: client, stop: stop, run: func(ctx context.Context, command string, limit int64) ([]byte, error) {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		session, err := client.NewSession()
		if err != nil {
			return nil, errors.New("SSH shell session unavailable")
		}
		defer session.Close()
		cancelSession := context.AfterFunc(ctx, func() { _ = session.Close() })
		defer cancelSession()
		stdout, err := session.StdoutPipe()
		if err != nil {
			return nil, errors.New("SSH inventory output unavailable")
		}
		if err := session.Start(command); err != nil {
			return nil, errors.New("SSH inventory command could not start")
		}
		output, err := io.ReadAll(io.LimitReader(stdout, limit+1))
		if err != nil || int64(len(output)) > limit {
			return nil, errors.New("SSH inventory output unavailable or exceeds limit")
		}
		if ctx.Err() != nil || session.Wait() != nil || ctx.Err() != nil {
			return nil, errors.New("SSH inventory command failed; check Python 3 and library path permissions")
		}
		return output, nil
	}}, nil
}

func (r *ShellReader) Close() error {
	r.stop()
	return r.client.Close()
}

func (r *ShellReader) checkPath(p string) error {
	if !path.IsAbs(p) || path.Clean(p) != p || strings.ContainsRune(p, 0) || (p != r.root && !strings.HasPrefix(p, r.root+"/")) {
		return errors.New("invalid path outside configured root")
	}
	return nil
}

func (r *ShellReader) command(ctx context.Context, limit int64, op, p string, args ...string) ([]byte, error) {
	if err := r.checkPath(p); err != nil {
		return nil, err
	}
	argv := append([]string{inventoryScript, op, r.root, p}, args...)
	for i, arg := range argv {
		argv[i] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
	}
	return r.run(ctx, "python3 -I -c "+strings.Join(argv, " "), limit)
}

type shellInfo struct {
	name string
	mode fs.FileMode
	size int64
	time time.Time
}

func (f shellInfo) Name() string       { return f.name }
func (f shellInfo) Mode() fs.FileMode  { return f.mode }
func (f shellInfo) Size() int64        { return f.size }
func (f shellInfo) ModTime() time.Time { return f.time }
func (f shellInfo) IsDir() bool        { return f.mode.IsDir() }
func (f shellInfo) Sys() any           { return nil }

func decodeShellInfo(input []byte) (fs.FileInfo, error) {
	var name string
	var mode uint32
	var size, mtime int64
	fields := [...]any{&name, &mode, &size, &mtime}
	if err := json.Unmarshal(input, &fields); err != nil {
		return nil, errors.New("invalid SSH file attributes")
	}
	bits := fs.FileMode(mode & 0777)
	switch mode & 0170000 {
	case 0040000:
		bits |= fs.ModeDir
	case 0120000:
		bits |= fs.ModeSymlink
	case 0100000:
	default:
		bits |= fs.ModeIrregular
	}
	return shellInfo{name: name, mode: bits, size: size, time: time.Unix(0, mtime)}, nil
}

func (r *ShellReader) ReadDir(ctx context.Context, p string) ([]fs.FileInfo, error) {
	out, err := r.command(ctx, 8*1024*1024, "list", p)
	if err != nil {
		return nil, err
	}
	var entries []fs.FileInfo
	for line := range bytes.SplitSeq(bytes.TrimSuffix(out, []byte("\n")), []byte("\n")) {
		if len(out) == 0 {
			break
		}
		info, err := decodeShellInfo(line)
		if err != nil {
			return nil, err
		}
		entries = append(entries, info)
	}
	return entries, nil
}

func (r *ShellReader) Lstat(ctx context.Context, p string) (fs.FileInfo, error) {
	out, err := r.command(ctx, 8192, "stat", p)
	if err != nil {
		return nil, err
	}
	return decodeShellInfo(out)
}

func (r *ShellReader) RealPath(ctx context.Context, p string) (string, error) {
	out, err := r.command(ctx, 16384, "realpath", p)
	if err != nil {
		return "", err
	}
	var real string
	if err := json.Unmarshal(out, &real); err != nil {
		return "", errors.New("invalid SSH path response")
	}
	return real, nil
}

func (r *ShellReader) Open(ctx context.Context, p string) (io.ReadCloser, error) {
	if err := r.checkPath(p); err != nil {
		return nil, err
	}
	return &shellFile{reader: r, ctx: ctx, path: p}, nil
}

type shellFile struct {
	reader *ShellReader
	ctx    context.Context
	path   string
	offset int64
}

func (f *shellFile) Read(p []byte) (int, error) {
	n, err := f.ReadAt(p, f.offset)
	f.offset += int64(n)
	return n, err
}

func (f *shellFile) ReadAt(p []byte, offset int64) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if offset < 0 || len(p) > 4*1024*1024 {
		return 0, errors.New("invalid SSH inventory read range")
	}
	out, err := f.reader.command(f.ctx, int64(len(p)), "read", f.path, strconv.FormatInt(offset, 10), strconv.Itoa(len(p)))
	if err != nil {
		return 0, err
	}
	if n := copy(p, out); n < len(p) {
		return n, io.EOF
	}
	return len(p), nil
}

func (f *shellFile) Close() error { return nil }
