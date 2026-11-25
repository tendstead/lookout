// SPDX-License-Identifier: Elastic-2.0
package metadata

import (
	"context"
	"errors"
	"net/http"
	"strconv"
)

// TrackingArtworkIdentity derives the provider key from tracking identities.
func TrackingArtworkIdentity(tvmazeID, sourceURL string) (provider, id string) {
	if validTVmazeArtworkID(tvmazeID) {
		return "tvmaze", tvmazeID
	}
	if source, err := TrackingSourceURL(sourceURL); err == nil {
		if id := youtubeTrackingID(source); id != "" {
			return "youtube", id
		}
	}
	return "", ""
}

func validTVmazeArtworkID(id string) bool {
	n, err := strconv.Atoi(id)
	return err == nil && n > 0 && strconv.Itoa(n) == id
}

// TrackingArtwork retrieves one bounded image from a supported public provider.
func (p *TVmaze) TrackingArtwork(ctx context.Context, provider, id string) ([]byte, string, error) {
	var address string
	switch provider {
	case "tvmaze":
		if !validTVmazeArtworkID(id) {
			return nil, "", errors.New("invalid TVmaze artwork identity")
		}
		show, err := p.Show(ctx, id)
		if err != nil || show.ID != id || show.ImageURL == "" {
			return nil, "", errors.New("TVmaze artwork unavailable")
		}
		address = show.ImageURL
	case "youtube":
		if !youtubeVideoID.MatchString(id) {
			return nil, "", errors.New("invalid YouTube artwork identity")
		}
		address = "https://i.ytimg.com/vi/" + id + "/hqdefault.jpg"
	default:
		return nil, "", errors.New("unsupported artwork provider")
	}
	var transport http.RoundTripper
	if p.HTTP != nil {
		transport = p.HTTP.Transport
	}
	return publicProviderImage(ctx, provider, address, transport)
}
