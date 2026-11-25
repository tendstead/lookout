// SPDX-License-Identifier: Elastic-2.0
package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const trackingMetadataLimit = 512 * 1024

type trackingMetadataBuffer struct {
	buffer bytes.Buffer
}

func (b *trackingMetadataBuffer) Write(p []byte) (int, error) {
	if len(p) > trackingMetadataLimit-b.buffer.Len() {
		return 0, errors.New("public video metadata exceeds limit")
	}
	return b.buffer.Write(p)
}

// NewYTDLPInspector runs an administrator-configured executable. The runner is
// responsible for isolating the extractor from private networks and local data.
func NewYTDLPInspector(executable string) func(context.Context, string) (Candidate, error) {
	running := make(chan struct{}, 2)
	return func(ctx context.Context, input string) (Candidate, error) {
		u, err := TrackingSourceURL(input)
		if err != nil || !filepath.IsAbs(executable) {
			return Candidate{}, errors.New("public video worker is unavailable")
		}
		id := youtubeTrackingID(u)
		if id == "" {
			return Candidate{}, errors.New("public video worker only accepts YouTube videos")
		}
		select {
		case running <- struct{}{}:
			defer func() { <-running }()
		default:
			return Candidate{}, errors.New("public video worker is busy")
		}
		videoURL := canonicalYouTubeTrackingURL(id)
		request, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(request, executable,
			"--ignore-config", "--no-plugin-dirs", "--no-remote-components",
			"--no-cookies", "--no-cookies-from-browser", "--no-cache-dir",
			"--simulate", "--print", "%(.{id,title,extractor_key,availability})j", "--no-playlist",
			"--use-extractors", "^youtube$", "--socket-timeout", "4",
			"--retries", "0", "--extractor-retries", "0",
			"--", videoURL)
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "LANG=C.UTF-8"}
		cmd.WaitDelay = time.Second
		var output trackingMetadataBuffer
		cmd.Stdout = &output
		if err := cmd.Run(); err != nil {
			return Candidate{}, errors.New("public video worker could not inspect the link")
		}
		var metadata struct {
			ID           string `json:"id"`
			Title        string `json:"title"`
			ExtractorKey string `json:"extractor_key"`
			Availability string `json:"availability"`
		}
		if err := json.Unmarshal(output.buffer.Bytes(), &metadata); err != nil || metadata.ID != id ||
			metadata.ExtractorKey != "Youtube" || metadata.Availability != "" &&
			metadata.Availability != "public" && metadata.Availability != "unlisted" {
			return Candidate{}, errors.New("public video worker could not verify the video")
		}
		title := strings.TrimSpace(metadata.Title)
		if title == "" {
			return Candidate{}, errors.New("public video title is unavailable")
		}
		return Candidate{Provider: "youtube", ID: id, Name: title, URL: videoURL}, nil
	}
}
