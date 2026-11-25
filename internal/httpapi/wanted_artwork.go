// SPDX-License-Identifier: Elastic-2.0
package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tendstead/lookout/internal/metadata"
	"github.com/tendstead/lookout/internal/store"
)

const wantedArtworkPath = "/api/v1/wanted/artwork/"

func (o apiOptions) populateWantedArtwork(w *store.Wanted) {
	w.ArtworkURL = ""
	if w.ArtworkTitleID != "" && o.metadata != nil {
		w.ArtworkURL = "/api/v1/titles/" + url.PathEscape(w.ArtworkTitleID) + "/artwork"
		return
	}
	if w.ServerID == "" && o.wantedPublic && o.wantedTV != nil {
		if provider, id := metadata.TrackingArtworkIdentity(w.ProviderIDs["tvmaze"], w.SourceURL); id != "" {
			w.ArtworkURL = wantedArtworkPath + provider + "/" + id
		}
	}
}

func (o apiOptions) wantedArtwork(w http.ResponseWriter, r *http.Request, reference string) {
	if !readMethod(w, r) {
		return
	}
	provider, id, ok := strings.Cut(reference, "/")
	if !ok || !o.wantedPublic || o.wantedTV == nil {
		apiError(w, 404, "artwork_unavailable", "Artwork unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	data, mime, err := o.wantedTV.TrackingArtwork(ctx, provider, id)
	if err != nil {
		apiError(w, 404, "artwork_unavailable", "Artwork unavailable")
		return
	}
	writeArtwork(w, r, data, mime, "private, max-age=3600")
}

func writeArtwork(w http.ResponseWriter, r *http.Request, data []byte, mime, cache string) {
	sum := sha256.Sum256(data)
	etag := `"` + hex.EncodeToString(sum[:]) + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", cache)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(data)
	}
}
