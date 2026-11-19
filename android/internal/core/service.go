package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mygo-player/android/sourceapi"
	"mygo-player/internal/catalog"
)

type SourceInfo struct {
	sourceapi.Manifest
	Available bool   `json:"available"`
	Error     string `json:"error,omitempty"`
}
type entry struct {
	source sourceapi.Source
	store  *catalog.Client
}
type Service struct {
	Token        string
	ctx          context.Context
	cancel       context.CancelFunc
	mu           sync.RWMutex
	entries      map[string]entry
	coverDir     string
	prefetchMu   sync.Mutex
	ready        map[string]*prefetchEntry
	prefetching  map[string]*prefetchEntry
	coverClient  *http.Client
	convertCover func(string) (string, error)
	sources      []SourceInfo
	closeOnce    sync.Once
}

// New opens the registered sources and installs the optional platform proxy.
func New(dir string, proxy func(*http.Request) (*url.URL, error)) (*Service, error) {
	return newWithProxy(dir, sourceapi.Registered(), proxy)
}

func NewWithRegistrations(dir string, registrations []sourceapi.Registration) (*Service, error) {
	return newWithProxy(dir, registrations, nil)
}

func newWithProxy(dir string, registrations []sourceapi.Registration, proxy func(*http.Request) (*url.URL, error)) (*Service, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	var token [24]byte
	if _, err := rand.Read(token[:]); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{Token: hex.EncodeToString(token[:]), ctx: ctx, cancel: cancel, entries: map[string]entry{}, coverDir: filepath.Join(dir, "cache", "covers"), ready: map[string]*prefetchEntry{}, prefetching: map[string]*prefetchEntry{}, coverClient: &http.Client{Timeout: 25 * time.Second}}
	if proxy != nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = proxy
		s.coverClient.Transport = transport
	}
	for _, r := range registrations {
		info := SourceInfo{Manifest: r.Manifest}
		// Registered manifests are validated before reaching this filesystem path.
		if filepath.Base(r.Manifest.ID) != r.Manifest.ID || r.Manifest.ID == "" || r.Manifest.ID == "." || r.Manifest.ID == ".." {
			s.Close()
			return nil, errors.New("invalid mobile source ID")
		}
		src, err := r.Open(ctx, filepath.Join(dir, "sources", r.Manifest.ID))
		if err != nil {
			info.Error = err.Error()
			s.sources = append(s.sources, info)
			continue
		}
		if configurable, ok := src.(interface {
			SetNetworkProxy(func(*http.Request) (*url.URL, error))
		}); ok && proxy != nil {
			configurable.SetNetworkProxy(proxy)
		}
		store, err := catalog.New(filepath.Join(dir, "library", r.Manifest.ID))
		if err != nil {
			_ = src.Close()
			s.Close()
			return nil, err
		}
		info.Available = true
		s.entries[r.Manifest.ID] = entry{src, store}
		s.sources = append(s.sources, info)
	}
	return s, nil
}
func (s *Service) Close() {
	s.closeOnce.Do(func() {
		s.releasePrefetched()
		s.cancel()
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, e := range s.entries {
			_ = e.source.Close()
		}
	})
}

// sessionHeader carries the per-launch secret the frontend sends with every API call.
const sessionHeader = "X-MyGo-Mobile"

var methods = map[string]bool{"Info": true, "Discover": true, "Search": true, "Rank": true, "Detail": true, "Seasons": true, "Covers": true, "Calendar": true, "Resolve": true, "Release": true, "Danmaku": true}

type command struct {
	Source   string          `json:"source"`
	Method   string          `json:"method"`
	Params   json.RawMessage `json:"params"`
	Series   catalog.Series  `json:"series"`
	Episode  catalog.Episode `json:"episode"`
	Position float64         `json:"position"`
}

// sessionToken reads the app session header. The Android host forwards the
// header names exactly as the WebView reported them, which is usually lower
// case, so a plain Header.Get would miss it.
func sessionToken(h http.Header) string {
	for k, vs := range h {
		if strings.EqualFold(k, sessionHeader) && len(vs) > 0 {
			return vs[0]
		}
	}
	return ""
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Covers are page subresources: they carry the session in the URL instead.
	if r.URL.Path == "/api/cover" {
		s.serveCover(w, r)
		return
	}
	// App protocol handlers can also receive requests from untrusted pages. The
	// secret is injected only into our own index, never exposed through the API.
	if sessionToken(r.Header) != s.Token {
		fail(w, http.StatusForbidden, errors.New("应用会话已失效，请重新打开页面"))
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.ctx.Err() != nil {
		fail(w, http.StatusServiceUnavailable, errors.New("应用已关闭"))
		return
	}
	if r.URL.Path == "/api/sources" && r.Method == http.MethodGet {
		sources := s.sources
		if sources == nil {
			sources = []SourceInfo{}
		}
		writeJSON(w, map[string]any{"sources": sources, "runtime": "inprocess/mobile-v1", "version": "0.1.0"})
		return
	}
	if r.URL.Path == "/api/prefetch" && r.Method == http.MethodPost {
		var cmd command
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&cmd); err != nil {
			fail(w, 400, errors.New("无效请求"))
			return
		}
		var p struct {
			ID      string `json:"id"`
			Quality string `json:"quality"`
		}
		_ = json.Unmarshal(cmd.Params, &p)
		if cmd.Source == "" || p.ID == "" {
			fail(w, 400, errors.New("缺少剧集"))
			return
		}
		if _, ok := s.entries[cmd.Source]; !ok {
			fail(w, 404, errors.New("来源不可用"))
			return
		}
		s.prefetch(cmd.Source, p.ID, p.Quality)
		writeJSON(w, map[string]bool{"queued": true})
		return
	}
	if r.URL.Path == "/api/library" && r.Method == http.MethodGet {
		e, ok := s.entries[r.URL.Query().Get("source")]
		if !ok {
			fail(w, 404, errors.New("来源不可用"))
			return
		}
		lib, err := e.store.Library()
		if err != nil {
			fail(w, 500, err)
			return
		}
		writeJSON(w, lib)
		return
	}
	if r.Method != http.MethodPost {
		fail(w, http.StatusMethodNotAllowed, errors.New("不支持此请求"))
		return
	}
	var cmd command
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10))
	if err := dec.Decode(&cmd); err != nil {
		fail(w, 400, errors.New("无效请求"))
		return
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		fail(w, 400, errors.New("无效请求内容"))
		return
	}
	e, ok := s.entries[cmd.Source]
	if !ok {
		fail(w, 404, errors.New("来源不可用"))
		return
	}
	switch r.URL.Path {
	case "/api/favorite":
		value, err := e.store.ToggleFavorite(cmd.Series)
		if err != nil {
			fail(w, 400, err)
			return
		}
		writeJSON(w, map[string]bool{"favorite": value})
	case "/api/progress":
		err := e.store.SaveProgress(catalog.Progress{Series: cmd.Series, Episode: cmd.Episode, Position: cmd.Position, UpdatedAt: time.Now()})
		if err != nil {
			fail(w, 400, err)
			return
		}
		writeJSON(w, map[string]bool{"saved": true})
	case "/api/call":
		if !methods[cmd.Method] {
			fail(w, 400, errors.New("手机端不支持该操作"))
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
		defer cancel()
		stop := context.AfterFunc(s.ctx, cancel)
		defer stop()
		if len(cmd.Params) == 0 {
			cmd.Params = json.RawMessage(`{}`)
		}
		var raw json.RawMessage
		var err error
		if cmd.Method == "Resolve" {
			if cached, ok, perr := s.takePrefetched(ctx, cmd.Source, cmd.Params); ok {
				raw, err = cached, perr
			} else {
				raw, err = e.source.Call(ctx, cmd.Method, cmd.Params)
			}
		} else {
			raw, err = e.source.Call(ctx, cmd.Method, cmd.Params)
		}
		if ctx.Err() != nil {
			// A source may finish Resolve just as the user swipes away. Release its
			// session rather than leaving the proxy alive for an abandoned response.
			if cmd.Method == "Resolve" {
				releaseReturned(e.source, raw)
			}
			fail(w, http.StatusGatewayTimeout, errors.New("请求已取消或超时，请重试"))
			return
		}
		if err != nil {
			fail(w, http.StatusBadGateway, err)
			return
		}
		if len(raw) == 0 {
			raw = json.RawMessage(`null`)
		}
		if !json.Valid(raw) || len(raw) > 8<<20 {
			fail(w, 502, errors.New("来源返回无效数据"))
			return
		}
		_, _ = w.Write(raw)
	default:
		fail(w, 404, errors.New("未找到操作"))
	}
}
func releaseReturned(src sourceapi.Source, raw json.RawMessage) {
	var media struct{ SessionID string }
	if json.Unmarshal(raw, &media) != nil || media.SessionID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	params, _ := json.Marshal(map[string]string{"SessionID": media.SessionID})
	_, _ = src.Call(ctx, "Release", params)
}
func writeJSON(w http.ResponseWriter, v any) { _ = json.NewEncoder(w).Encode(v) }
func fail(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)
	writeJSON(w, map[string]string{"error": fmt.Sprint(err)})
}
