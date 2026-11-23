// SPDX-License-Identifier: Elastic-2.0
package library

import (
	"strings"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

type TechnicalProfile struct {
	Source          string          `json:"source"`
	ObservedAt      time.Time       `json:"observed_at"`
	VersionID       string          `json:"version_id,omitempty"`
	PartID          string          `json:"part_id,omitempty"`
	PartNumber      int             `json:"part_number,omitempty"`
	PartCount       int             `json:"part_count,omitempty"`
	Edition         string          `json:"edition,omitempty"`
	SizeBytes       *int64          `json:"size_bytes,omitempty"`
	DurationMS      *int64          `json:"duration_ms,omitempty"`
	Container       string          `json:"container,omitempty"`
	Width           *int            `json:"width,omitempty"`
	Height          *int            `json:"height,omitempty"`
	BitrateBPS      *int64          `json:"bitrate_bps,omitempty"`
	VideoCodec      string          `json:"video_codec,omitempty"`
	VideoProfile    string          `json:"video_profile,omitempty"`
	VideoBitrateBPS *int64          `json:"video_bitrate_bps,omitempty"`
	FrameRate       *float64        `json:"frame_rate,omitempty"`
	BitDepth        *int            `json:"bit_depth,omitempty"`
	HDR             string          `json:"hdr,omitempty"`
	Audio           []AudioTrack    `json:"audio"`
	Subtitles       []SubtitleTrack `json:"subtitles"`
}

type AudioTrack struct {
	Codec      string `json:"codec,omitempty"`
	Language   string `json:"language,omitempty"`
	Channels   *int   `json:"channels,omitempty"`
	BitrateBPS *int64 `json:"bitrate_bps,omitempty"`
}

type SubtitleTrack struct {
	Codec           string `json:"codec,omitempty"`
	Language        string `json:"language,omitempty"`
	Forced          *bool  `json:"forced,omitempty"`
	Default         *bool  `json:"default,omitempty"`
	HearingImpaired *bool  `json:"hearing_impaired,omitempty"`
}

// NormalizeLanguage canonicalizes reported tags without inferring a region.
// Preserve unsupported labels so users can still see what the server reported.
func NormalizeLanguage(value string) string {
	value = strings.TrimSpace(value)
	switch strings.ToLower(value) {
	case "", "und", "unknown", "undefined", "undetermined", "zxx", "mis", "n/a":
		return ""
	case "german", "deutsch":
		return "de"
	}
	if tag := englishLanguageNames[strings.ToLower(value)]; tag != "" {
		return tag
	}
	tag, err := language.Parse(strings.ReplaceAll(value, "_", "-"))
	if err == nil && tag != language.Und {
		return tag.String()
	}
	return value
}

var englishLanguageNames = func() map[string]string {
	names := make(map[string]string)
	displayName := display.English.Languages()
	for first := byte('a'); first <= 'z'; first++ {
		for second := byte('a'); second <= 'z'; second++ {
			code := string([]byte{first, second})
			tag, err := language.Parse(code)
			if err != nil || tag == language.Und || tag.String() != code {
				continue
			}
			if name := displayName.Name(tag); name != "" {
				names[strings.ToLower(name)] = code
			}
		}
	}
	return names
}()
