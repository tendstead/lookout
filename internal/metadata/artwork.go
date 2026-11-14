// SPDX-License-Identifier: Elastic-2.0
package metadata

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

func (p *MediaServer) Image(ctx context.Context, relative string) ([]byte, string, error) {
	if e := p.Validate(); e != nil {
		return nil, "", e
	}
	u, e := url.Parse(relative)
	if e != nil || u.IsAbs() || u.Host != "" || !strings.HasPrefix(u.Path, "/") || path.Clean(u.Path) != u.Path {
		return nil, "", errors.New("invalid image reference")
	}
	allowed := p.Kind == "jellyfin" && strings.HasPrefix(u.Path, "/Items/") && strings.Contains(u.Path, "/Images/") || p.Kind == "plex" && strings.HasPrefix(u.Path, "/library/metadata/")
	if !allowed {
		return nil, "", errors.New("image outside metadata scope")
	}
	return fetchImage(ctx, p.client(), p.BaseURL+u.String(), p.headers())
}
func PublicImage(ctx context.Context, address string) ([]byte, string, error) {
	u, e := url.Parse(address)
	if e != nil || u.Scheme != "https" || u.Host != "static.tvmaze.com" || !strings.HasPrefix(u.Path, "/uploads/images/") {
		return nil, "", errors.New("unsupported official artwork host")
	}
	c := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if req.URL.Host != "static.tvmaze.com" || req.URL.Scheme != "https" || len(via) > 2 {
			return errors.New("artwork redirect rejected")
		}
		return nil
	}}
	return fetchImage(ctx, c, address, nil)
}
func fetchImage(ctx context.Context, c *http.Client, address string, headers map[string]string) ([]byte, string, error) {
	r, e := http.NewRequestWithContext(ctx, "GET", address, nil)
	if e != nil {
		return nil, "", errors.New("invalid image request")
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	response, e := c.Do(r)
	if e != nil {
		return nil, "", errors.New("image source unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, "", errors.New("image request rejected")
	}
	data, e := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024+1))
	if e != nil || len(data) > 2*1024*1024 {
		return nil, "", errors.New("image exceeds limit")
	}
	mime := http.DetectContentType(data)
	switch mime {
	case "image/jpeg", "image/png", "image/webp":
	default:
		return nil, "", errors.New("unsupported image content")
	}
	return data, mime, nil
}
