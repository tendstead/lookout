// SPDX-License-Identifier: Elastic-2.0
package metadata

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/tendstead/lookout/internal/library"
)

type jellyfinItemPage struct {
	Total int            `json:"TotalRecordCount"`
	Items []jellyfinItem `json:"Items"`
}

type jellyfinItem struct {
	ID           string                `json:"Id"`
	Name         string                `json:"Name"`
	Type         string                `json:"Type"`
	Path         string                `json:"Path"`
	Year         int                   `json:"ProductionYear"`
	Overview     string                `json:"Overview"`
	IDs          map[string]string     `json:"ProviderIds"`
	Images       map[string]string     `json:"ImageTags"`
	MediaSources []jellyfinMediaSource `json:"MediaSources"`
	MediaStreams []jellyfinStream      `json:"MediaStreams"`
	Duration     *int64                `json:"RunTimeTicks"`
	Container    string                `json:"Container"`
	Season       *int                  `json:"ParentIndexNumber"`
	Number       *int                  `json:"IndexNumber"`
	End          *int                  `json:"IndexNumberEnd"`
	PartCount    int                   `json:"PartCount"`
}

type jellyfinMediaSource struct {
	ETag      string           `json:"ETag"`
	ID        string           `json:"Id"`
	Path      string           `json:"Path"`
	Container string           `json:"Container"`
	Size      *int64           `json:"Size"`
	Duration  *int64           `json:"RunTimeTicks"`
	Bitrate   *int64           `json:"Bitrate"`
	Streams   []jellyfinStream `json:"MediaStreams"`
}

type jellyfinStream struct {
	Type            string   `json:"Type"`
	Codec           string   `json:"Codec"`
	Language        string   `json:"Language"`
	Profile         string   `json:"Profile"`
	Bitrate         *int64   `json:"BitRate"`
	Width           *int     `json:"Width"`
	Height          *int     `json:"Height"`
	BitDepth        *int     `json:"BitDepth"`
	Channels        *int     `json:"Channels"`
	AverageRate     *float64 `json:"AverageFrameRate"`
	RealRate        *float64 `json:"RealFrameRate"`
	VideoRange      string   `json:"VideoRange"`
	VideoRangeType  string   `json:"VideoRangeType"`
	Default         *bool    `json:"IsDefault"`
	Forced          *bool    `json:"IsForced"`
	HearingImpaired *bool    `json:"IsHearingImpaired"`
}

func (item jellyfinItem) copies() []inventoryCopy {
	copies := make([]inventoryCopy, 0, len(item.MediaSources))
	for _, source := range item.MediaSources {
		if sameJellyfinItemID(source.ID, item.ID) || source.Path != "" && source.Path == item.Path {
			source = item.withPartFields(source)
		}
		if source.ID == "" && source.Path == "" {
			continue
		}
		kind, identity := "jellyfin-source", source.ID
		if identity == "" {
			kind, identity = "path", source.Path
		}
		copies = append(copies, inventoryCopy{path: source.Path, identityKind: kind, identity: identity, revision: source.ETag, profile: source.profile()})
	}
	if len(copies) == 0 && (item.MediaStreams != nil || item.Duration != nil || item.Container != "") {
		source := item.withPartFields(jellyfinMediaSource{})
		copies = append(copies, inventoryCopy{path: item.Path, profile: source.profile()})
	}
	return copies
}

func sameJellyfinItemID(left, right string) bool {
	return left != "" && right != "" &&
		normalizedJellyfinID(left) == normalizedJellyfinID(right)
}

func (item jellyfinItem) withPartFields(source jellyfinMediaSource) jellyfinMediaSource {
	source.Path = firstKnown(source.Path, item.Path)
	source.Container = firstKnown(source.Container, item.Container)
	source.Duration = firstKnown(source.Duration, item.Duration)
	if source.Streams == nil {
		source.Streams = item.MediaStreams
	}
	return source
}

func (p *MediaServer) jellyfinCopies(ctx context.Context, item jellyfinItem) ([]inventoryCopy, error) {
	copies := item.copies()
	if item.PartCount <= 1 {
		return copies, nil
	}
	if item.ID == "" {
		return nil, errors.New("Jellyfin multipart video is missing an identifier")
	}

	var parts jellyfinItemPage
	if err := readJSON(ctx, p.client(), p.BaseURL+"/Videos/"+url.PathEscape(item.ID)+"/AdditionalParts", p.headers(), &parts); err != nil {
		return nil, fmt.Errorf("Jellyfin additional parts: %w", err)
	}
	if len(parts.Items) != item.PartCount-1 {
		return nil, errors.New("incomplete Jellyfin additional parts inventory")
	}

	mainIndex := primaryJellyfinCopy(item, copies)
	if mainIndex < 0 {
		mainIndex = len(copies)
		source := item.withPartFields(jellyfinMediaSource{})
		copies = append(copies, inventoryCopy{path: item.Path, profile: source.profile()})
	}
	main := copies[mainIndex].profile
	versionID := firstKnown(main.VersionID, item.ID)
	main.VersionID, main.PartID = versionID, item.ID
	main.PartNumber, main.PartCount = 1, item.PartCount

	partIDs := map[string]bool{normalizedJellyfinID(item.ID): true}
	paths := map[string]bool{}
	for _, copy := range copies {
		if copy.path != "" {
			paths[copy.path] = true
		}
	}
	for i, part := range parts.Items {
		partCopies := part.copies()
		var copy inventoryCopy
		if index := primaryJellyfinCopy(part, partCopies); index >= 0 {
			copy = partCopies[index]
		} else {
			source := part.withPartFields(jellyfinMediaSource{})
			copy = inventoryCopy{path: part.Path, profile: source.profile()}
		}
		profile := copy.profile
		partID := firstKnown(part.ID, profile.VersionID)
		normalizedID := normalizedJellyfinID(partID)
		noIdentity := partID == "" && copy.path == ""
		if noIdentity || partIDs[normalizedID] || paths[copy.path] {
			return nil, errors.New("invalid or repeated Jellyfin part")
		}
		if partID != "" {
			partIDs[normalizedID] = true
		}
		if copy.path != "" {
			paths[copy.path] = true
		}
		profile.VersionID, profile.PartID = versionID, partID
		profile.PartNumber, profile.PartCount = i+2, item.PartCount
		copy.identityKind, copy.identity = "jellyfin-part", partID
		if partID == "" {
			copy.identityKind, copy.identity = "path", copy.path
		}
		copies = append(copies, copy)
	}
	return copies, nil
}

// Prefer the provider's item ID when it includes both the primary video and
// alternate versions. A matching path also identifies the primary file.
func primaryJellyfinCopy(item jellyfinItem, copies []inventoryCopy) int {
	for i, copy := range copies {
		if copy.identityKind == "jellyfin-source" && sameJellyfinItemID(copy.identity, item.ID) {
			return i
		}
	}
	for i, copy := range copies {
		if copy.identity == "" || item.Path != "" && copy.path == item.Path {
			return i
		}
	}
	return -1
}

func normalizedJellyfinID(value string) string {
	return strings.ToLower(strings.ReplaceAll(value, "-", ""))
}

func (source jellyfinMediaSource) profile() *library.TechnicalProfile {
	profile := &library.TechnicalProfile{
		Source: "jellyfin", VersionID: source.ID, SizeBytes: nonnegative(source.Size),
		BitrateBPS: positive(source.Bitrate), Container: strings.ToLower(strings.TrimSpace(source.Container)),
	}
	if source.Duration != nil && *source.Duration >= 0 {
		millis := *source.Duration / 10_000 // .NET ticks are 100 nanoseconds.
		profile.DurationMS = &millis
	}
	if source.Streams != nil {
		profile.Audio = []library.AudioTrack{}
		profile.Subtitles = []library.SubtitleTrack{}
	}
	video := false
	for _, stream := range source.Streams {
		codec := strings.ToLower(strings.TrimSpace(stream.Codec))
		language := library.NormalizeLanguage(stream.Language)
		switch stream.Type {
		case "Video":
			if video {
				continue
			}
			video = true
			profile.Width, profile.Height, profile.BitDepth = positive(stream.Width), positive(stream.Height), positive(stream.BitDepth)
			profile.VideoCodec, profile.VideoProfile = codec, strings.TrimSpace(stream.Profile)
			profile.VideoBitrateBPS = positive(stream.Bitrate)
			profile.FrameRate = firstKnown(positive(stream.AverageRate), positive(stream.RealRate))
			profile.HDR = firstKnown(jellyfinHDR(stream.VideoRangeType), jellyfinHDR(stream.VideoRange))
		case "Audio":
			profile.Audio = append(profile.Audio, library.AudioTrack{
				Codec: codec, Language: language, Channels: positive(stream.Channels), BitrateBPS: positive(stream.Bitrate),
			})
		case "Subtitle":
			profile.Subtitles = append(profile.Subtitles, library.SubtitleTrack{
				Codec: codec, Language: language, Default: stream.Default, Forced: stream.Forced, HearingImpaired: stream.HearingImpaired,
			})
		}
	}
	return profile
}

func jellyfinHDR(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "unknown":
		return ""
	case "dolbyvision", "dovi":
		return "Dolby Vision"
	case "hdr10plus":
		return "HDR10+"
	case "doviwithhdr10":
		return "Dolby Vision / HDR10"
	case "doviwithhlg":
		return "Dolby Vision / HLG"
	case "doviwithsdr":
		return "Dolby Vision / SDR"
	case "doviwithhdr10plus":
		return "Dolby Vision / HDR10+"
	case "doviwithel":
		return "Dolby Vision Profile 7"
	case "doviwithelhdr10plus":
		return "Dolby Vision Profile 7 / HDR10+"
	case "doviinvalid":
		return "Dolby Vision (invalid configuration)"
	default:
		return value
	}
}
