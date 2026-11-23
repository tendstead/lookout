// SPDX-License-Identifier: Elastic-2.0
package metadata

import (
	"context"
	"encoding/json"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/tendstead/lookout/internal/library"
)

type plexItemPage struct {
	Container struct {
		Total int        `json:"totalSize"`
		Items []plexItem `json:"Metadata"`
	} `json:"MediaContainer"`
}

type plexItem struct {
	ID      string `json:"ratingKey"`
	Name    string `json:"title"`
	Type    string `json:"type"`
	Year    int    `json:"year"`
	Summary string `json:"summary"`
	Thumb   string `json:"thumb"`
	Edition string `json:"editionTitle"`
	Season  *int   `json:"parentIndex"`
	Number  *int   `json:"index"`
	GUIDs   []struct {
		ID string `json:"id"`
	} `json:"Guid"`
	Locations []struct {
		Path string `json:"path"`
	} `json:"Location"`
	Media []plexMedia `json:"Media"`
}

type plexMedia struct {
	ID           json.Number `json:"id"`
	Edition      string      `json:"editionTitle"`
	Container    string      `json:"container"`
	Duration     *int64      `json:"duration"`
	Bitrate      *int64      `json:"bitrate"`
	Width        *int        `json:"width"`
	Height       *int        `json:"height"`
	VideoCodec   string      `json:"videoCodec"`
	VideoProfile string      `json:"videoProfile"`
	Parts        []plexPart  `json:"Part"`
}

type plexPart struct {
	Key       string       `json:"key"`
	ID        json.Number  `json:"id"`
	File      string       `json:"file"`
	Container string       `json:"container"`
	Size      *int64       `json:"size"`
	Duration  *int64       `json:"duration"`
	Bitrate   *int64       `json:"bitrate"`
	Streams   []plexStream `json:"Stream"`
}

type plexStream struct {
	Type            int         `json:"streamType"`
	Codec           string      `json:"codec"`
	LanguageTag     string      `json:"languageTag"`
	LanguageCode    string      `json:"languageCode"`
	Language        string      `json:"language"`
	Profile         string      `json:"profile"`
	Bitrate         *int64      `json:"bitrate"`
	Width           *int        `json:"width"`
	Height          *int        `json:"height"`
	BitDepth        *int        `json:"bitDepth"`
	Channels        *int        `json:"channels"`
	FrameRate       json.Number `json:"frameRate"`
	ColorTransfer   string      `json:"colorTrc"`
	DolbyVision     plexFlag    `json:"DOVIPresent"`
	Forced          plexFlag    `json:"forced"`
	Default         plexFlag    `json:"default"`
	HearingImpaired plexFlag    `json:"hearingImpaired"`
}

// Plex uses booleans and numeric flags depending on the field and server.
type plexFlag struct{ value *bool }

func (flag *plexFlag) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case "true", "1", `"1"`, `"true"`:
		value := true
		flag.value = &value
	case "false", "0", `"0"`, `"false"`:
		value := false
		flag.value = &value
	default:
		flag.value = nil
	}
	return nil
}

func (item plexItem) copies() []inventoryCopy {
	var copies []inventoryCopy
	for _, media := range item.Media {
		for index, part := range media.Parts {
			kind, identity := "plex-part", part.ID.String()
			if identity == "" {
				kind, identity = "path", part.File
			}
			if identity == "" && len(media.Parts) == 1 {
				kind, identity = "plex-version", media.ID.String()
			}
			if identity != "" {
				copies = append(copies, inventoryCopy{path: part.File, identityKind: kind, identity: identity, revision: part.Key, profile: item.partProfile(media, part, index)})
			}
		}
	}
	return copies
}

func (item plexItem) partProfile(media plexMedia, part plexPart, index int) *library.TechnicalProfile {
	profile := &library.TechnicalProfile{
		Source: "plex", VersionID: media.ID.String(), PartID: part.ID.String(),
		PartNumber: index + 1, PartCount: len(media.Parts), Edition: strings.TrimSpace(firstKnown(media.Edition, item.Edition)),
		SizeBytes: nonnegative(part.Size), DurationMS: nonnegative(part.Duration), BitrateBPS: plexBitrate(part.Bitrate),
		Container: plexCodec(part.Container),
	}
	// Media-level summaries describe the complete version. They describe
	// an individual file only when that version consists of a single part.
	if len(media.Parts) == 1 {
		profile.Container = firstKnown(profile.Container, plexCodec(media.Container))
		profile.DurationMS = firstKnown(profile.DurationMS, nonnegative(media.Duration))
		profile.BitrateBPS = firstKnown(profile.BitrateBPS, plexBitrate(media.Bitrate))
		profile.Width, profile.Height = positive(media.Width), positive(media.Height)
		profile.VideoCodec, profile.VideoProfile = plexCodec(media.VideoCodec), strings.TrimSpace(media.VideoProfile)
	}
	if part.Streams != nil {
		profile.Audio = []library.AudioTrack{}
		profile.Subtitles = []library.SubtitleTrack{}
	}
	video := false
	for _, stream := range part.Streams {
		codec := plexCodec(stream.Codec)
		language := library.NormalizeLanguage(firstKnown(stream.LanguageTag, firstKnown(stream.LanguageCode, stream.Language)))
		switch stream.Type {
		case 1:
			if video {
				continue
			}
			video = true
			profile.Width = firstKnown(positive(stream.Width), profile.Width)
			profile.Height = firstKnown(positive(stream.Height), profile.Height)
			profile.VideoCodec = firstKnown(codec, profile.VideoCodec)
			profile.VideoProfile = firstKnown(strings.TrimSpace(stream.Profile), profile.VideoProfile)
			profile.BitDepth = positive(stream.BitDepth)
			profile.VideoBitrateBPS = plexBitrate(stream.Bitrate)
			if rate, err := stream.FrameRate.Float64(); err == nil && rate > 0 && !math.IsInf(rate, 0) {
				profile.FrameRate = &rate
			}
			profile.HDR = stream.hdr()
		case 2:
			profile.Audio = append(profile.Audio, library.AudioTrack{
				Codec: codec, Language: language, Channels: positive(stream.Channels), BitrateBPS: plexBitrate(stream.Bitrate),
			})
		case 3:
			profile.Subtitles = append(profile.Subtitles, library.SubtitleTrack{
				Codec: codec, Language: language, Forced: stream.Forced.value, Default: stream.Default.value, HearingImpaired: stream.HearingImpaired.value,
			})
		}
	}
	return profile
}

func (stream plexStream) hdr() string {
	if stream.DolbyVision.value != nil && *stream.DolbyVision.value {
		return "Dolby Vision"
	}
	switch strings.ToLower(stream.ColorTransfer) {
	case "smpte2084":
		return "HDR"
	case "arib-std-b67":
		return "HLG"
	case "bt709", "smpte170m", "bt2020-10", "bt2020-12":
		return "SDR"
	default:
		return ""
	}
}

func plexBitrate(kbps *int64) *int64 {
	if kbps == nil || *kbps <= 0 || *kbps > math.MaxInt64/1000 {
		return nil
	}
	bps := *kbps * 1000
	return &bps
}

func plexCodec(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

// Plex list endpoints can omit Stream elements. Item metadata can include
// them without probing media. Keep the listing on unsupported or failed lookups.
func (p *MediaServer) fillPlexPartDetails(ctx context.Context, items []plexItem) {
	const batchSize = 20
	var pending []int
	for i, item := range items {
		if item.ID == "" || len(item.ID) > 64 || strings.Contains(item.ID, ",") {
			continue
		}
		if item.needsPartDetails() {
			pending = append(pending, i)
		}
	}
	for start := 0; start < len(pending) && ctx.Err() == nil; start += batchSize {
		batch := pending[start:min(start+batchSize, len(pending))]
		ids := make([]string, 0, len(batch))
		for _, index := range batch {
			ids = append(ids, url.PathEscape(items[index].ID))
		}
		var details plexItemPage
		requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := readJSON(requestCtx, p.client(), p.BaseURL+"/library/metadata/"+strings.Join(ids, ","), p.headers(), &details)
		cancel()
		if err != nil {
			continue
		}
		byID := make(map[string]plexItem, len(details.Container.Items))
		for _, item := range details.Container.Items {
			byID[item.ID] = item
		}
		for _, index := range batch {
			if detail, ok := byID[items[index].ID]; ok {
				items[index].mergeDetails(detail)
			}
		}
	}
}

func (item plexItem) needsPartDetails() bool {
	if len(item.Media) == 0 {
		return true
	}
	for _, media := range item.Media {
		if len(media.Parts) == 0 {
			return true
		}
		for _, part := range media.Parts {
			if part.Streams == nil {
				return true
			}
		}
	}
	return false
}

func (item *plexItem) mergeDetails(detail plexItem) {
	item.Edition = firstKnown(item.Edition, detail.Edition)
	for _, detailMedia := range detail.Media {
		matched := false
		for index := range item.Media {
			if item.Media[index].matches(detailMedia) {
				item.Media[index].mergeDetails(detailMedia)
				matched = true
				break
			}
		}
		if !matched {
			item.Media = append(item.Media, detailMedia)
		}
	}
}

func (media plexMedia) matches(other plexMedia) bool {
	if media.ID != "" && other.ID != "" {
		return media.ID == other.ID
	}
	for _, part := range media.Parts {
		for _, otherPart := range other.Parts {
			if part.matches(otherPart) {
				return true
			}
		}
	}
	return false
}

func (media *plexMedia) mergeDetails(detail plexMedia) {
	media.ID = firstKnown(media.ID, detail.ID)
	media.Edition = firstKnown(media.Edition, detail.Edition)
	media.Container = firstKnown(media.Container, detail.Container)
	media.Duration = firstKnown(media.Duration, detail.Duration)
	media.Bitrate = firstKnown(media.Bitrate, detail.Bitrate)
	media.Width = firstKnown(media.Width, detail.Width)
	media.Height = firstKnown(media.Height, detail.Height)
	media.VideoCodec = firstKnown(media.VideoCodec, detail.VideoCodec)
	media.VideoProfile = firstKnown(media.VideoProfile, detail.VideoProfile)
	for _, detailPart := range detail.Parts {
		matched := false
		for index := range media.Parts {
			if media.Parts[index].matches(detailPart) {
				media.Parts[index].mergeDetails(detailPart)
				matched = true
				break
			}
		}
		if !matched {
			media.Parts = append(media.Parts, detailPart)
		}
	}
}

func (part plexPart) matches(other plexPart) bool {
	if part.ID != "" && other.ID != "" {
		return part.ID == other.ID
	}
	return part.File != "" && part.File == other.File
}

func (part *plexPart) mergeDetails(detail plexPart) {
	part.Key = firstKnown(part.Key, detail.Key)
	part.ID = firstKnown(part.ID, detail.ID)
	part.File = firstKnown(part.File, detail.File)
	part.Container = firstKnown(part.Container, detail.Container)
	part.Size = firstKnown(part.Size, detail.Size)
	part.Duration = firstKnown(part.Duration, detail.Duration)
	part.Bitrate = firstKnown(part.Bitrate, detail.Bitrate)
	if part.Streams == nil {
		part.Streams = detail.Streams
	}
}
