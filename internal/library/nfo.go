// SPDX-License-Identifier: Elastic-2.0
package library

import (
	"bytes"
	"encoding/xml"
	"errors"
	"strings"
)

const MaxSidecarBytes = 2 * 1024 * 1024

func ParseNFO(data []byte) (Metadata, error) {
	m := Metadata{ProviderIDs: map[string]string{}, Provenance: "nfo"}
	if len(data) > MaxSidecarBytes {
		return m, errors.New("sidecar too large")
	}
	var n struct {
		XMLName xml.Name
		Title   string `xml:"title"`
		Year    int    `xml:"year"`
		Plot    string `xml:"plot"`
		TVDB    string `xml:"tvdbid"`
		TMDB    string `xml:"tmdbid"`
		IMDB    string `xml:"imdbid"`
		Season  *int   `xml:"season"`
		Episode *int   `xml:"episode"`
		IDs     []struct {
			Type  string `xml:"type,attr"`
			Value string `xml:",chardata"`
		} `xml:"uniqueid"`
	}
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = true
	if err := d.Decode(&n); err != nil {
		return m, errors.New("invalid NFO")
	}
	m.Kind = n.XMLName.Local
	if m.Kind != "tvshow" && m.Kind != "movie" && m.Kind != "episodedetails" {
		return m, errors.New("unsupported NFO root")
	}
	m.Title = strings.TrimSpace(n.Title)
	m.Year = n.Year
	m.Overview = n.Plot
	m.Season = n.Season
	m.Episode = n.Episode
	if m.Kind != "episodedetails" {
		for k, v := range map[string]string{"tvdb": n.TVDB, "tmdb": n.TMDB, "imdb": n.IMDB} {
			if v != "" {
				m.ProviderIDs[k] = strings.TrimSpace(v)
			}
		}
		for _, id := range n.IDs {
			switch strings.ToLower(id.Type) {
			case "tvdb", "tmdb", "imdb", "tvmaze":
				m.ProviderIDs[strings.ToLower(id.Type)] = strings.TrimSpace(id.Value)
			}
		}
	}
	return m, nil
}
