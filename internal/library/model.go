// SPDX-License-Identifier: Elastic-2.0
package library

import (
	"context"
	"io"
	"io/fs"
	"time"
)

type Root struct {
	ServerID string `json:"server_id,omitempty"`
	ID       string `json:"id"`
	Label    string `json:"label"`
	Path     string `json:"path"`
	Category string `json:"category"`
}
type Metadata struct {
	Title       string            `json:"title"`
	Year        int               `json:"year,omitempty"`
	Overview    string            `json:"overview,omitempty"`
	ProviderIDs map[string]string `json:"provider_ids"`
	Kind        string            `json:"kind,omitempty"`
	Season      *int              `json:"season,omitempty"`
	Episode     *int              `json:"episode,omitempty"`
	Provenance  string            `json:"provenance,omitempty"`
}
type FileObservation struct {
	Storage       string    `json:"storage,omitempty"`
	ArchiveMember string    `json:"archive_member,omitempty"`
	SourceID      string    `json:"source_id"`
	RelativePath  string    `json:"relative_path"`
	Group         string    `json:"group"`
	Category      string    `json:"category"`
	Size          int64     `json:"size"`
	Modified      time.Time `json:"modified"`
	Title         Metadata  `json:"title"`
	Season        int       `json:"season,omitempty"`
	Episodes      []int     `json:"episodes,omitempty"`
	Numbering     string    `json:"numbering"`
	EpisodeTitle  string    `json:"episode_title,omitempty"`
}
type ScanStats struct {
	ArchivedMedia   int `json:"archived_media"`
	Archives        int `json:"archives"`
	Directories     int `json:"directories"`
	Media           int `json:"media"`
	Sidecars        int `json:"sidecars"`
	SymlinksSkipped int `json:"symlinks_skipped"`
	Warnings        int `json:"warnings"`
}

// Reader deliberately excludes every mutating filesystem operation.
type Reader interface {
	ReadDir(context.Context, string) ([]fs.FileInfo, error)
	Lstat(context.Context, string) (fs.FileInfo, error)
	RealPath(context.Context, string) (string, error)
	Open(context.Context, string) (io.ReadCloser, error)
}
