// SPDX-License-Identifier: Elastic-2.0
package metadata

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
	"unicode"
)

type Candidate struct {
	Provider    string            `json:"provider"`
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Year        int               `json:"year,omitempty"`
	Overview    string            `json:"overview"`
	ProviderIDs map[string]string `json:"provider_ids"`
	URL         string            `json:"url,omitempty"`
	ImageURL    string            `json:"image_url,omitempty"`
}
type Episode struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Season   int    `json:"season"`
	Number   *int   `json:"number"`
	AirDate  string `json:"air_date"`
	AirStamp string `json:"air_stamp"`
	URL      string `json:"url"`
}

func normalName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
func ExactCandidate(items []Candidate, name string, year int) (Candidate, bool) {
	var matched []Candidate
	for _, c := range items {
		if normalName(c.Name) == normalName(name) && (year == 0 || year == c.Year) {
			matched = append(matched, c)
		}
	}
	if len(matched) == 1 {
		return matched[0], true
	}
	return Candidate{}, false
}
func readJSON(ctx context.Context, client *http.Client, address string, headers map[string]string, out any) error {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	r, e := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if e != nil {
		return errors.New("invalid provider request")
	}
	r.Header.Set("User-Agent", "Tendstead-Lookout/0.1")
	r.Header.Set("Accept", "application/json")
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	response, e := client.Do(r)
	if e != nil {
		var dns *net.DNSError
		var cert *tls.CertificateVerificationError
		if errors.As(e, &dns) {
			return errors.New("provider DNS resolution failed")
		}
		if errors.As(e, &cert) {
			return errors.New("provider TLS certificate verification failed")
		}
		return errors.New("provider connection failed")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("provider request rejected (HTTP %d)", response.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024+1))
	if e != nil || len(b) > 8*1024*1024 {
		return errors.New("provider response exceeds limit")
	}
	if e = json.Unmarshal(b, out); e != nil {
		return errors.New("invalid provider response")
	}
	return nil
}
func validateBaseURL(value string) (string, error) {
	u, e := url.Parse(value)
	if e != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("server URL must be HTTP(S) without credentials or query")
	}
	return strings.TrimRight(u.String(), "/"), nil
}

type PathMapping struct {
	ServerPrefix string `json:"server_prefix"`
	SourceID     string `json:"source_id"`
	SourcePrefix string `json:"source_prefix"`
}

func MapPath(remote string, mappings []PathMapping) (string, string, bool) {
	if !path.IsAbs(remote) || path.Clean(remote) != remote {
		return "", "", false
	}
	var best *PathMapping
	for i := range mappings {
		m := &mappings[i]
		prefix := strings.TrimRight(m.ServerPrefix, "/")
		if remote == prefix || strings.HasPrefix(remote, prefix+"/") {
			if best == nil || len(prefix) > len(best.ServerPrefix) {
				best = m
			}
		}
	}
	if best == nil {
		return "", "", false
	}
	relative := strings.TrimPrefix(strings.TrimPrefix(remote, strings.TrimRight(best.ServerPrefix, "/")), "/")
	if relative == "" {
		return "", "", false
	}
	return best.SourceID, relative, true
}
