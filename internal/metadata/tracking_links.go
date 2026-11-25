// SPDX-License-Identifier: Elastic-2.0
package metadata

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type TrackingResolution struct {
	Title     Candidate
	Season    *int
	Episode   *int
	Category  string
	SourceURL string
	Inferred  bool
}

type TrackingSeason struct {
	Number       int    `json:"number"`
	Name         string `json:"name"`
	EpisodeOrder int    `json:"episodeOrder"`
	EndDate      string `json:"endDate"`
}

var (
	externalHostname = regexp.MustCompile(`^[a-zA-Z0-9-]+(\.[a-zA-Z0-9-]+)+$`)
	tvdbSlug         = regexp.MustCompile(`^[a-zA-Z0-9-]+$`)
	youtubeVideoID   = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
)

func TrackingSourceURL(input string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(input))
	if err != nil || len(input) > 2048 || u.Scheme != "https" && u.Scheme != "http" ||
		u.User != nil || u.Port() != "" || !externalHostname.MatchString(u.Hostname()) ||
		net.ParseIP(u.Hostname()) != nil {
		return nil, errors.New("provide a public HTTP(S) link")
	}
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	return u, nil
}

func (p *TVmaze) ResolveTracking(ctx context.Context, input string) (TrackingResolution, error) {
	u, err := TrackingSourceURL(input)
	if err != nil {
		return TrackingResolution{}, err
	}
	result := TrackingResolution{SourceURL: u.String()}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	switch u.Host {
	case "tvmaze.com", "www.tvmaze.com", "api.tvmaze.com":
		result.Category = "series"
		return p.resolveMazeTracking(ctx, result, parts)
	case "thetvdb.com", "www.thetvdb.com":
		result.Category = "series"
		return p.resolveTVDBTracking(ctx, result, parts)
	case "www.youtube.com", "youtube.com", "youtu.be", "m.youtube.com":
		result.Category = "movie"
		id := youtubeTrackingID(u)
		if id == "" {
			break
		}
		result.SourceURL = canonicalYouTubeTrackingURL(id)
		if p.TrackingMedia != nil {
			result.Title, err = p.TrackingMedia(ctx, result.SourceURL)
			if err == nil {
				return result, nil
			}
		}
		result.Title, err = p.youtubeTrackingTitle(ctx, id)
		return result, err
	}
	return result, errors.New("automatic metadata is unavailable for this link")
}

type mazeTrackingEpisode struct {
	ID       int  `json:"id"`
	Season   int  `json:"season"`
	Number   *int `json:"number"`
	Embedded struct {
		Show mazeShow `json:"show"`
	} `json:"_embedded"`
}

func (p *TVmaze) resolveMazeTracking(ctx context.Context, result TrackingResolution, parts []string) (TrackingResolution, error) {
	if len(parts) < 2 {
		return result, errors.New("TVmaze link does not identify a title")
	}
	id, err := strconv.Atoi(parts[1])
	if err != nil || id <= 0 {
		return result, errors.New("invalid TVmaze link ID")
	}
	key := strconv.Itoa(id)
	switch parts[0] {
	case "shows":
		result.Title, err = p.Show(ctx, key)
		if err == nil && result.Title.ID == key && result.Title.Name != "" {
			return result, nil
		}
	case "episodes":
		var episode mazeTrackingEpisode
		err = p.get(ctx, "/episodes/"+key+"?embed=show", &episode)
		if err == nil && episode.ID == id && episode.Embedded.Show.ID > 0 && episode.Embedded.Show.Name != "" {
			result.Title = episode.Embedded.Show.candidate()
			result.Season, result.Episode = &episode.Season, episode.Number
			return result, nil
		}
	case "seasons":
		var episodes []mazeTrackingEpisode
		err = p.get(ctx, "/seasons/"+key+"/episodes?embed=show", &episodes)
		if err == nil {
			for _, episode := range episodes {
				if episode.Embedded.Show.ID > 0 && episode.Embedded.Show.Name != "" {
					result.Title = episode.Embedded.Show.candidate()
					result.Season = &episode.Season
					return result, nil
				}
			}
		}
	}
	if err == nil {
		err = errors.New("TVmaze could not identify this link")
	}
	result.Title = Candidate{}
	return result, err
}

func (p *TVmaze) resolveTVDBTracking(ctx context.Context, result TrackingResolution, parts []string) (TrackingResolution, error) {
	if len(parts) > 0 && parts[0] == "index.php" {
		parts = parts[1:]
	}
	if len(parts) == 3 && parts[0] == "dereferrer" && parts[1] == "series" {
		parts = parts[1:]
	}
	if len(parts) != 2 && len(parts) != 5 || parts[0] != "series" || !tvdbSlug.MatchString(parts[1]) {
		return result, errors.New("TheTVDB link does not identify a series")
	}
	if len(parts) == 5 {
		number, err := strconv.Atoi(parts[4])
		if parts[2] != "seasons" || parts[3] != "official" || err != nil || number < 0 {
			return result, errors.New("unsupported TheTVDB season link")
		}
		result.Season = &number
	}
	if id, err := strconv.Atoi(parts[1]); err == nil && id > 0 {
		result.Title, err = p.Lookup(ctx, map[string]string{"tvdb": strconv.Itoa(id)})
		return result, err
	}

	// A series slug offers a search term. The exact result still needs confirmation
	// because its title may match another series that uses a different TVDB URL.
	name := strings.ReplaceAll(parts[1], "-", " ")
	items, err := p.Search(ctx, name)
	if err != nil {
		return result, err
	}
	candidate, ok := ExactCandidate(items, name, 0)
	if !ok || candidate.ProviderIDs["tvdb"] == "" {
		return result, errors.New("TheTVDB link needs a title match")
	}
	result.Title = candidate
	result.Inferred = true
	return result, nil
}

func (p *TVmaze) Seasons(ctx context.Context, id string) ([]TrackingSeason, error) {
	n, err := strconv.Atoi(id)
	if err != nil || n <= 0 {
		return nil, errors.New("invalid TVmaze identity")
	}
	var seasons []TrackingSeason
	err = p.get(ctx, "/shows/"+strconv.Itoa(n)+"/seasons", &seasons)
	return seasons, err
}

func youtubeTrackingID(u *url.URL) string {
	switch u.Host {
	case "www.youtube.com", "youtube.com", "youtu.be", "m.youtube.com":
	default:
		return ""
	}
	var id string
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	switch {
	case u.Host == "youtu.be" && len(parts) == 1:
		id = parts[0]
	case u.Host == "youtu.be":
		return ""
	case u.Path == "/watch":
		id = u.Query().Get("v")
	case len(parts) == 2 && (parts[0] == "shorts" || parts[0] == "live" || parts[0] == "embed"):
		id = parts[1]
	}
	if youtubeVideoID.MatchString(id) {
		return id
	}
	return ""
}

func canonicalYouTubeTrackingURL(id string) string {
	return "https://www.youtube.com/watch?v=" + id
}

func (p *TVmaze) youtubeTrackingTitle(ctx context.Context, id string) (Candidate, error) {
	address := "https://www.youtube.com/oembed?" + url.Values{
		"format": {"json"},
		"url":    {canonicalYouTubeTrackingURL(id)},
	}.Encode()
	client := http.Client{Timeout: 8 * time.Second}
	if p.HTTP != nil {
		client.Transport = p.HTTP.Transport
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	var response struct {
		Title string `json:"title"`
	}
	if err := readJSON(ctx, &client, address, nil, &response); err != nil {
		return Candidate{}, err
	}
	title := strings.TrimSpace(response.Title)
	if title == "" {
		return Candidate{}, errors.New("public video title is unavailable")
	}
	return Candidate{Provider: "youtube", ID: id, Name: title, URL: canonicalYouTubeTrackingURL(id)}, nil
}
