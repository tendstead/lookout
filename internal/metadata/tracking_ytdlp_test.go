// SPDX-License-Identifier: Elastic-2.0
package metadata

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestYTDLPInspectUsesMetadataOnlyAndCanonicalVideo(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "fixture-ytdlp")
	script := `#!/bin/sh
expected="--ignore-config --no-plugin-dirs --no-remote-components --no-cookies --no-cookies-from-browser --no-cache-dir --simulate --print %(.{id,title,extractor_key,availability})j --no-playlist --use-extractors ^youtube$ --socket-timeout 4 --retries 0 --extractor-retries 0 -- https://www.youtube.com/watch?v=YE7VzlLtp-4"
[ "$*" = "$expected" ] || exit 2
printf '%s' '{"id":"YE7VzlLtp-4","title":"Big Buck Bunny","extractor_key":"Youtube","availability":"public","formats":[{"url":"https://example.org/transient-stream"}]}'
`
	if err := os.WriteFile(executable, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	inspect := NewYTDLPInspector(executable)
	got, err := inspect(context.Background(), "https://youtu.be/YE7VzlLtp-4?si=discard")
	if err != nil || got.ID != "YE7VzlLtp-4" || got.Name != "Big Buck Bunny" ||
		got.URL != "https://www.youtube.com/watch?v=YE7VzlLtp-4" || len(got.ProviderIDs) != 0 {
		t.Fatalf("metadata: %+v, %v", got, err)
	}
	if _, err = inspect(context.Background(), "https://outside.example/YE7VzlLtp-4"); err == nil {
		t.Fatal("public worker accepted unsupported host")
	}
}
