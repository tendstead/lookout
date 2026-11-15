// SPDX-License-Identifier: Elastic-2.0
package library

import (
	"archive/zip"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"io/fs"
	"math"
	"path"
	"strings"
	"unicode"
)

type ArchiveEntry struct {
	Name string
	Size int64
}
type archiveBudget struct {
	ctx    context.Context
	reader io.ReaderAt
	used   int
}

func (b *archiveBudget) ReadAt(p []byte, offset int64) (int, error) {
	if e := b.ctx.Err(); e != nil {
		return 0, e
	}
	if offset < 0 || len(p) > 12*1024*1024-b.used {
		return 0, errors.New("archive directory read limit exceeded")
	}
	n, e := b.reader.ReadAt(p, offset)
	b.used += n
	return n, e
}
func InspectZIP(ctx context.Context, reader Reader, address string, size int64) ([]ArchiveEntry, error) {
	if size < 22 {
		return nil, errors.New("invalid ZIP size")
	}
	f, e := reader.Open(ctx, address)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	at, ok := f.(io.ReaderAt)
	if !ok {
		return nil, errors.New("archive random reads unavailable")
	}
	bounded := &archiveBudget{ctx: ctx, reader: at}
	tail := make([]byte, int(min(size, int64(65577))))
	if _, e = bounded.ReadAt(tail, size-int64(len(tail))); e != nil {
		return nil, e
	}
	end := -1
	for i := len(tail) - 22; i >= 0; i-- {
		if binary.LittleEndian.Uint32(tail[i:i+4]) == 0x06054b50 && i+22+int(binary.LittleEndian.Uint16(tail[i+20:i+22])) == len(tail) {
			end = i
			break
		}
	}
	if end < 0 {
		return nil, errors.New("ZIP directory missing")
	}
	header := tail[end:]
	count := uint64(binary.LittleEndian.Uint16(header[10:12]))
	length := uint64(binary.LittleEndian.Uint32(header[12:16]))
	offset := uint64(binary.LittleEndian.Uint32(header[16:20]))
	if count == 0xffff || length == 0xffffffff || offset == 0xffffffff {
		if end < 20 || binary.LittleEndian.Uint32(tail[end-20:end-16]) != 0x07064b50 {
			return nil, errors.New("ZIP64 locator unavailable")
		}
		position := binary.LittleEndian.Uint64(tail[end-12 : end-4])
		if position > uint64(size-56) {
			return nil, errors.New("ZIP64 directory outside file")
		}
		record := make([]byte, 56)
		if _, e = bounded.ReadAt(record, int64(position)); e != nil {
			return nil, e
		}
		if binary.LittleEndian.Uint32(record[:4]) != 0x06064b50 {
			return nil, errors.New("invalid ZIP64 directory")
		}
		count = binary.LittleEndian.Uint64(record[32:40])
		length = binary.LittleEndian.Uint64(record[40:48])
		offset = binary.LittleEndian.Uint64(record[48:56])
	}
	if count > 10000 || length > 4*1024*1024 || offset > uint64(size) || length > uint64(size)-offset {
		return nil, errors.New("ZIP directory exceeds limits")
	}
	directory := make([]byte, int(length))
	if _, e = bounded.ReadAt(directory, int64(offset)); e != nil {
		return nil, e
	}
	records := 0
	for cursor := 0; cursor < len(directory); {
		if len(directory)-cursor < 46 || binary.LittleEndian.Uint32(directory[cursor:cursor+4]) != 0x02014b50 {
			return nil, errors.New("invalid ZIP directory record")
		}
		nameLength := int(binary.LittleEndian.Uint16(directory[cursor+28 : cursor+30]))
		extraLength := int(binary.LittleEndian.Uint16(directory[cursor+30 : cursor+32]))
		commentLength := int(binary.LittleEndian.Uint16(directory[cursor+32 : cursor+34]))
		next := cursor + 46 + nameLength + extraLength + commentLength
		if nameLength > 4096 || next > len(directory) {
			return nil, errors.New("ZIP directory record exceeds limits")
		}
		records++
		if records > 10000 {
			return nil, errors.New("ZIP entry count exceeds limit")
		}
		cursor = next
	}
	if uint64(records) != count {
		return nil, errors.New("ZIP directory count mismatch")
	}
	z, e := zip.NewReader(bounded, size)
	if e != nil {
		return nil, errors.New("ZIP directory unreadable")
	}
	out := []ArchiveEntry{}
	for _, member := range z.File {
		if member.FileInfo().IsDir() || member.Mode()&fs.ModeSymlink != 0 || !IsMedia(member.Name) || path.IsAbs(member.Name) || path.Clean(member.Name) != member.Name || strings.HasPrefix(member.Name, "../") || strings.Contains(member.Name, "\\") || len(member.Name) > 4096 || member.UncompressedSize64 > math.MaxInt64 {
			continue
		}
		safe := true
		for _, r := range member.Name {
			if unicode.IsControl(r) {
				safe = false
				break
			}
		}
		if !safe {
			continue
		}
		out = append(out, ArchiveEntry{Name: member.Name, Size: int64(member.UncompressedSize64)})
	}
	return out, nil
}
