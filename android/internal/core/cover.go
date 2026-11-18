package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Covers come from the sources as remote URLs. The app page cannot read them
// directly: some sources serve HEIF, which the WebView cannot decode, and
// others refuse cross-origin loads. The service fetches them once, converts
// formats the platform can show, and serves the bytes from the app origin.

const (
	maxCoverBytes  = 8 << 20
	maxCoverFiles  = 400
	coverFetchTime = 20 * time.Second
)

// SetCoverConverter installs the platform image converter. It receives the
// path of a downloaded image and returns the path of a JPEG it wrote. Nil
// leaves HEIF covers unconverted, which only matters on platforms without a
// decoder the Go side can reach.
func (s *Service) SetCoverConverter(convert func(string) (string, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.convertCover = convert
}

func (s *Service) serveCover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		fail(w, http.StatusMethodNotAllowed, errors.New("不支持此请求"))
		return
	}
	// Images load as page subresources, which cannot carry the session header,
	// so the per-launch key in the URL stands in for it.
	if r.URL.Query().Get("k") != s.Token {
		fail(w, http.StatusForbidden, errors.New("应用会话已失效，请重新打开页面"))
		return
	}
	raw, err := parseCoverURL(r.URL.Query().Get("u"))
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	sum := sha256.Sum256([]byte(raw))
	name := filepath.Join(s.coverDir, hex.EncodeToString(sum[:])+".img")
	body, err := os.ReadFile(name)
	if err != nil {
		var kind string
		body, kind, err = s.fetchCover(raw, name)
		if err != nil {
			fail(w, http.StatusBadGateway, errors.New("封面暂时无法加载"))
			return
		}
		serveCoverBytes(w, r, body, kind)
		return
	}
	_ = os.Chtimes(name, time.Now(), time.Now())
	serveCoverBytes(w, r, body, sniffImage(body))
}

func serveCoverBytes(w http.ResponseWriter, r *http.Request, body []byte, kind string) {
	if kind == "" {
		fail(w, http.StatusBadGateway, errors.New("封面暂时无法加载"))
		return
	}
	h := w.Header()
	h.Set("Content-Type", kind)
	h.Set("Cache-Control", "public, max-age=604800")
	h.Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write(body)
	}
}

func (s *Service) fetchCover(raw, name string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(s.ctx, coverFetchTime)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "image/*")
	client := s.coverClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, "", errors.New("cover status " + resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCoverBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(body) > maxCoverBytes {
		return nil, "", errors.New("cover too large")
	}
	kind := sniffImage(body)
	if kind == "" {
		converted, err := s.convertImage(name, body)
		if err != nil {
			return nil, "", err
		}
		body, kind = converted, "image/jpeg"
	}
	if err := os.MkdirAll(s.coverDir, 0o700); err != nil {
		return nil, "", err
	}
	tmp := name + ".tmp"
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		return nil, "", err
	}
	if err := os.Rename(tmp, name); err != nil {
		_ = os.Remove(tmp)
		return nil, "", err
	}
	s.pruneCovers()
	return body, kind, nil
}

func (s *Service) convertImage(name string, body []byte) ([]byte, error) {
	s.mu.RLock()
	convert := s.convertCover
	s.mu.RUnlock()
	if convert == nil {
		return nil, errors.New("no image converter")
	}
	if err := os.MkdirAll(s.coverDir, 0o700); err != nil {
		return nil, err
	}
	src := name + ".src"
	if err := os.WriteFile(src, body, 0o600); err != nil {
		return nil, err
	}
	defer os.Remove(src)
	out, err := convert(src)
	if err != nil {
		return nil, err
	}
	defer os.Remove(out)
	jpeg, err := os.ReadFile(out)
	if err != nil {
		return nil, err
	}
	if len(jpeg) > maxCoverBytes {
		return nil, errors.New("converted cover too large")
	}
	return jpeg, nil
}

func (s *Service) pruneCovers() {
	entries, err := os.ReadDir(s.coverDir)
	if err != nil || len(entries) <= maxCoverFiles {
		return
	}
	type item struct {
		name string
		when time.Time
	}
	items := make([]item, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, item{filepath.Join(s.coverDir, e.Name()), info.ModTime()})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].when.Before(items[j].when) })
	for i := 0; i < len(items)-maxCoverFiles; i++ {
		_ = os.Remove(items[i].name)
	}
}

func parseCoverURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 4096 {
		return "", errors.New("无效封面地址")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil {
		return "", errors.New("无效封面地址")
	}
	return u.String(), nil
}

func sniffImage(body []byte) string {
	switch {
	case len(body) >= 3 && body[0] == 0xFF && body[1] == 0xD8 && body[2] == 0xFF:
		return "image/jpeg"
	case len(body) >= 8 && bytes.Equal(body[:8], []byte("\x89PNG\r\n\x1a\n")):
		return "image/png"
	case len(body) >= 12 && bytes.Equal(body[:4], []byte("RIFF")) && bytes.Equal(body[8:12], []byte("WEBP")):
		return "image/webp"
	case len(body) >= 6 && (bytes.Equal(body[:6], []byte("GIF87a")) || bytes.Equal(body[:6], []byte("GIF89a"))):
		return "image/gif"
	}
	return ""
}
