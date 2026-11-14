// SPDX-License-Identifier: Elastic-2.0
package httpapi

import (
	"bytes"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"
)

var hashedAsset = regexp.MustCompile(`[-.][a-zA-Z0-9_-]{8,}\.(js|css|woff2|svg|png)$`)

func serveStatic(w http.ResponseWriter, r *http.Request, assets fs.FS) {
	if r.Method != "GET" && r.Method != "HEAD" {
		w.Header().Set("Allow", "GET, HEAD")
		apiError(w, 405, "method_not_allowed", "Method not allowed")
		return
	}
	if assets == nil {
		apiError(w, 404, "not_found", "Not found")
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/")
	if name == "" {
		name = "index.html"
	}
	data, err := fs.ReadFile(assets, name)
	if err != nil && path.Ext(name) == "" && !strings.HasPrefix(name, "assets/") {
		name = "index.html"
		data, err = fs.ReadFile(assets, name)
	}
	if err != nil {
		apiError(w, 404, "not_found", "Not found")
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	if strings.HasPrefix(name, "assets/") && hashedAsset.MatchString(name) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}
