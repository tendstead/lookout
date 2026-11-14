// SPDX-License-Identifier: Elastic-2.0
package library

import (
	"path"
	"regexp"
	"strconv"
	"strings"
)

var providerTag = regexp.MustCompile(`(?i)[\[\{](tvdb|tmdb|imdb)(?:id)?[-=: ]+([a-z0-9]+)[\]\}]`)
var titleYear = regexp.MustCompile(`(?:[\(\[]((?:19|20)\d{2})[\)\]]|\s((?:19|20)\d{2}))$`)
var sePattern = regexp.MustCompile(`(?i)\bS(\d{1,3})[ ._-]*E(\d{1,4})((?:[ ._-]*-?[ ._-]*E\d{1,4})*)`)
var moreEpisode = regexp.MustCompile(`(?i)(-?)E(\d{1,4})`)
var xPattern = regexp.MustCompile(`(?i)\b(\d{1,3})x(\d{1,4})\b`)
var seasonDir = regexp.MustCompile(`(?i)^(?:season|staffel)[ ._-]*(\d{1,3})$`)
var leadingEpisode = regexp.MustCompile(`^(\d{1,3})(?:\s*[-.]\s*|\s+)`)

func ParseTitle(name string) Metadata {
	m := Metadata{ProviderIDs: map[string]string{}, Provenance: "filename"}
	name = strings.TrimSpace(name)
	for _, v := range providerTag.FindAllStringSubmatch(name, -1) {
		m.ProviderIDs[strings.ToLower(v[1])] = v[2]
	}
	name = strings.TrimSpace(providerTag.ReplaceAllString(name, ""))
	if v := titleYear.FindStringSubmatch(name); len(v) > 0 {
		y := v[1]
		if y == "" {
			y = v[2]
		}
		m.Year, _ = strconv.Atoi(y)
		name = strings.TrimSpace(name[:len(name)-len(v[0])])
	}
	m.Title = strings.TrimSpace(strings.ReplaceAll(name, "_", " "))
	return m
}
func ParseNumbering(file, dir string) (int, []int) {
	if m := sePattern.FindStringSubmatch(file); m != nil {
		s, _ := strconv.Atoi(m[1])
		first, _ := strconv.Atoi(m[2])
		episodes := []int{first}
		for _, v := range moreEpisode.FindAllStringSubmatch(strings.ReplaceAll(m[3], " ", ""), -1) {
			n, _ := strconv.Atoi(v[2])
			if v[1] == "-" && n > episodes[len(episodes)-1] && n-episodes[len(episodes)-1] <= 30 {
				for j := episodes[len(episodes)-1] + 1; j <= n; j++ {
					episodes = append(episodes, j)
				}
			} else {
				episodes = append(episodes, n)
			}
		}
		return s, episodes
	}
	if m := xPattern.FindStringSubmatch(file); m != nil {
		s, _ := strconv.Atoi(m[1])
		e, _ := strconv.Atoi(m[2])
		return s, []int{e}
	}
	if m := seasonDir.FindStringSubmatch(path.Base(dir)); m != nil {
		if n := leadingEpisode.FindStringSubmatch(file); n != nil {
			s, _ := strconv.Atoi(m[1])
			e, _ := strconv.Atoi(n[1])
			return s, []int{e}
		}
	}
	return 0, nil
}
func IsMedia(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".mkv", ".mp4", ".avi", ".m4v", ".mov", ".wmv", ".webm", ".mpg", ".mpeg", ".ts", ".m2ts", ".vob", ".ogv", ".flv", ".iso":
		return true
	}
	return false
}

var movieYearAnywhere = regexp.MustCompile(`\b(?:19|20)\d{2}\b`)

func ParseMovieFilename(name string) Metadata {
	name = strings.TrimSuffix(name, path.Ext(name))
	name = strings.ReplaceAll(strings.ReplaceAll(name, ".", " "), "_", " ")
	for _, span := range movieYearAnywhere.FindAllStringIndex(name, -1) {
		if span[0] > 0 {
			m := ParseTitle(strings.TrimSpace(name[:span[0]]))
			m.Year, _ = strconv.Atoi(name[span[0]:span[1]])
			return m
		}
	}
	return ParseTitle(name)
}
