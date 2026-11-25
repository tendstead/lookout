// SPDX-License-Identifier: Elastic-2.0
package store

import (
	"context"
	"testing"
	"time"

	"github.com/tendstead/lookout/internal/library"
)

func TestChangedFilesystemObservationClearsTechnicalProfile(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	root := library.Root{ID: "linked", Path: "/fixtures", Category: "movie"}
	if err := s.EnsureSource(ctx, root); err != nil {
		t.Fatal(err)
	}
	file := library.FileObservation{SourceID: root.ID, Group: "Movie", RelativePath: "Movie.mkv", Category: "movie", Size: 100, Modified: time.Unix(1, 0), Title: library.Metadata{Title: "Movie"}}
	scan, err := s.StartScan(ctx, root.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Observe(ctx, scan, file); err != nil {
		t.Fatal(err)
	}
	profile := &library.TechnicalProfile{Source: "jellyfin", VideoCodec: "hevc", ObservedAt: time.Now()}
	if err = s.AttachTechnicalProfile(ctx, root.ID, "legacy", file.RelativePath, profile); err != nil {
		t.Fatal(err)
	}
	page, err := s.ListTitles(ctx, TitleQuery{})
	if err != nil || len(page.Items) != 1 {
		t.Fatal(page, err)
	}
	getFile := func() MediaFile {
		t.Helper()
		detail, e := s.GetTitle(ctx, page.Items[0].ID)
		if e != nil || len(detail.Files) != 1 {
			t.Fatal(detail, e)
		}
		return detail.Files[0]
	}
	before := getFile()
	if before.Technical == nil || before.Technical.VideoCodec != "hevc" {
		t.Fatal("missing attached profile", before)
	}
	if err = s.Observe(ctx, scan, file); err != nil {
		t.Fatal(err)
	}
	if again := getFile(); again.Technical == nil || again.CopyRevision != before.CopyRevision {
		t.Fatal("unchanged file lost profile or revision", again)
	}
	file.Modified = file.Modified.Add(time.Hour)
	if err = s.Observe(ctx, scan, file); err != nil {
		t.Fatal(err)
	}
	if err = s.AttachTechnicalProfile(ctx, root.ID, "legacy", file.RelativePath, profile); err != nil {
		t.Fatal(err)
	}
	if after := getFile(); after.Technical != nil || after.CopyRevision == before.CopyRevision {
		t.Fatal("changed file kept obsolete profile or revision", after)
	}
}

func TestComparisonIgnoresUnknownValuesAndObservationTimes(t *testing.T) {
	german := []library.AudioTrack{{Language: "Korean", Codec: "aac"}}
	otherGerman := []library.AudioTrack{{Language: "ko"}}
	movie := MediaFile{ServerID: "a", Modified: time.Unix(10, 0), Technical: &library.TechnicalProfile{Audio: german, VideoCodec: "hevc"}}
	copy := MediaFile{ServerID: "b", Modified: time.Unix(11, 0), Technical: &library.TechnicalProfile{Audio: otherGerman}}
	if filesDiffer(movie, copy) {
		t.Fatal("unknown values and language aliases produced a difference")
	}
	copy.Technical.Audio = []library.AudioTrack{{Language: "ko"}, {Language: "en"}}
	if !filesDiffer(movie, copy) {
		t.Fatal("additional audio language did not produce a difference")
	}
}

func TestFileDifferencesPageGroupsConfirmedEpisodesAcrossServers(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	for index, server := range []string{"home", "friend"} {
		if err := s.EnsureServer(ctx, server, server); err != nil {
			t.Fatal(err)
		}
		root := library.Root{ID: server, ServerID: server, Label: server, Path: "/fixtures", Category: "series"}
		if err := s.EnsureSource(ctx, root); err != nil {
			t.Fatal(err)
		}
		scan, err := s.StartScan(ctx, root.ID)
		if err != nil {
			t.Fatal(err)
		}
		title := library.Metadata{Title: "Fictional show", ProviderIDs: map[string]string{"tvdb": "7100"}}
		for _, file := range []library.FileObservation{
			{RelativePath: "Two episodes.mkv", Season: 1, Episodes: []int{2, 3}, Numbering: "nfo", Size: int64(3000 + index)},
			{RelativePath: "No difference.mkv", Season: 1, Episodes: []int{1}, Numbering: "season_episode", Size: 3000},
			{RelativePath: "Ambiguous.mkv", Season: 1, Episodes: []int{4}, Numbering: "absolute", Size: int64(4000 + index)},
		} {
			file.SourceID, file.Group, file.Category, file.Title = root.ID, "Fictional show", "series", title
			file.Modified = time.Unix(int64(index), 0)
			if err := s.Observe(ctx, scan, file); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.FinishScan(ctx, scan, root.ID, library.ScanStats{Media: 3}, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ReconcileIdentities(ctx); err != nil {
		t.Fatal(err)
	}
	for offset, wanted := range []int{2, 3} {
		page, err := s.ListFileDifferences(ctx, 1, offset)
		if err != nil || len(page.Items) != 1 || page.HasMore != (offset == 0) {
			t.Fatal(page, err)
		}
		group := page.Items[0]
		if group.Episode == nil || *group.Episode != wanted || len(group.Files) != 2 || len(group.Files[0].Episodes) != 2 {
			t.Fatalf("episode %d did not include both copies and full coverage: %+v", wanted, group)
		}
	}
}

func TestFileComparisonDetectsChannelsThatChangedCodec(t *testing.T) {
	stereo, surround := 2, 6
	left := MediaFile{Technical: &library.TechnicalProfile{Audio: []library.AudioTrack{
		{Language: "en", Codec: "aac", Channels: &stereo},
		{Language: "en", Codec: "ac3", Channels: &surround},
	}}}
	right := MediaFile{Technical: &library.TechnicalProfile{Audio: []library.AudioTrack{
		{Language: "en", Codec: "ac3", Channels: &stereo},
		{Language: "en", Codec: "aac", Channels: &surround},
	}}}
	if !filesDiffer(left, right) {
		t.Fatal("changed associations of codecs and channels were ignored")
	}
}

func TestProviderRevisionChangesTheCopyKeyWithoutChangingTechnicalDifferences(t *testing.T) {
	profile := &library.TechnicalProfile{Source: "jellyfin", VideoCodec: "hevc"}
	before := MediaFile{Adapter: "jellyfin", Technical: profile, providerRevision: "revision-a"}
	after := MediaFile{Adapter: "jellyfin", Technical: profile, providerRevision: "revision-b"}
	if fileRevision(before) == fileRevision(after) || filesDiffer(before, after) {
		t.Fatal("provider replacement must invalidate hide preference without creating a technical difference")
	}
}
