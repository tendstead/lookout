package library

import (
	"reflect"
	"testing"
)

func TestParseTitle(t *testing.T) {
	for _, tc := range []struct {
		in, title string
		year      int
		ids       map[string]string
	}{
		{"Breaking Bad [tvdbid-81189]", "Breaking Bad", 0, map[string]string{"tvdb": "81189"}},
		{"A Silent Voice (2016)", "A Silent Voice", 2016, map[string]string{}},
		{"Fallout 2024", "Fallout", 2024, map[string]string{}},
		{"Dr. Stone", "Dr. Stone", 0, map[string]string{}},
	} {
		got := ParseTitle(tc.in)
		if got.Title != tc.title || got.Year != tc.year || !reflect.DeepEqual(got.ProviderIDs, tc.ids) {
			t.Fatalf("%s: %#v", tc.in, got)
		}
	}
}
func TestParseNumbering(t *testing.T) {
	for _, tc := range []struct {
		file, dir string
		season    int
		episodes  []int
	}{
		{"Show.S02E03E04.1080p.mkv", "Season 2", 2, []int{3, 4}},
		{"Show S01E01-E03.mkv", "Season 1", 1, []int{1, 2, 3}},
		{"Show 1x02.mp4", "", 1, []int{2}},
		{"02 - title.mkv", "Season 3", 3, []int{2}},
		{"[Group] Anime - 128 [1080p].mkv", "", 0, nil},
		{"unknown.mkv", "Specials", 0, nil},
	} {
		s, e := ParseNumbering(tc.file, tc.dir)
		if s != tc.season || !reflect.DeepEqual(e, tc.episodes) {
			t.Fatalf("%s: %d %#v", tc.file, s, e)
		}
	}
}
func TestNFO(t *testing.T) {
	m, err := ParseNFO([]byte(`<tvshow><title>Feuer &amp; Flamme</title><year>2017</year><plot>Example</plot><uniqueid type="tvdb" default="true">327414</uniqueid></tvshow>`))
	if err != nil || m.Title != "Feuer & Flamme" || m.ProviderIDs["tvdb"] != "327414" {
		t.Fatalf("%#v %v", m, err)
	}
	episode, err := ParseNFO([]byte(`<episodedetails><title>Episode</title><season>2</season><episode>3</episode><uniqueid type="tvdb">888</uniqueid></episodedetails>`))
	if err != nil || episode.Season == nil || *episode.Season != 2 || len(episode.ProviderIDs) != 0 {
		t.Fatalf("episode IDs confused with show: %#v %v", episode, err)
	}
}
