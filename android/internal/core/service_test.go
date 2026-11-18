package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"mygo-player/android/sourceapi"
)

type fakeSource struct {
	call   func(context.Context, string, json.RawMessage) (json.RawMessage, error)
	mu     sync.Mutex
	closed bool
}

func (f *fakeSource) Call(ctx context.Context, m string, p json.RawMessage) (json.RawMessage, error) {
	if f.call != nil {
		return f.call(ctx, m, p)
	}
	return json.RawMessage(`null`), nil
}
func (f *fakeSource) Close() error { f.mu.Lock(); defer f.mu.Unlock(); f.closed = true; return nil }
func registration(id string, f *fakeSource) sourceapi.Registration {
	return sourceapi.Registration{Manifest: sourceapi.Manifest{ID: id, Name: id, Version: "1.0.0"}, Open: func(context.Context, string) (sourceapi.Source, error) { return f, nil }}
}
func request(s *Service, method, path string, body any) *httptest.ResponseRecorder {
	var data bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&data).Encode(body)
	}
	req := httptest.NewRequest(method, path, &data)
	req.Header.Set("X-MyGo-Mobile", s.Token)
	out := httptest.NewRecorder()
	s.ServeHTTP(out, req)
	return out
}
func newService(t *testing.T, rs ...sourceapi.Registration) *Service {
	t.Helper()
	s, err := NewWithRegistrations(t.TempDir(), rs)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestAPIAuthorizationAndAllowlist(t *testing.T) {
	f := &fakeSource{call: func(_ context.Context, m string, _ json.RawMessage) (json.RawMessage, error) {
		if m != "Discover" {
			t.Fatalf("unexpected source call: %s", m)
		}
		return json.RawMessage(`{"Items":[],"Cursor":"next","HasMore":true}`), nil
	}}
	s := newService(t, registration("sample", f))
	denied := httptest.NewRecorder()
	s.ServeHTTP(denied, httptest.NewRequest("GET", "/api/sources", nil))
	if denied.Code != 403 {
		t.Fatalf("unauthorized status %d", denied.Code)
	}
	allowed := request(s, "GET", "/api/sources", nil)
	if allowed.Code != 200 || !strings.Contains(allowed.Body.String(), `"available":true`) {
		t.Fatal(allowed.Body.String())
	}
	call := request(s, "POST", "/api/call", map[string]any{"source": "sample", "method": "Discover", "params": map[string]string{"Channel": "recommended"}})
	if call.Code != 200 || !strings.Contains(call.Body.String(), `"Cursor":"next"`) {
		t.Fatal(call.Body.String())
	}
	for _, tc := range []struct {
		body   map[string]string
		status int
	}{{map[string]string{"source": "missing", "method": "Discover"}, 404}, {map[string]string{"source": "sample", "method": "ResetDevice"}, 400}} {
		if got := request(s, "POST", "/api/call", tc.body); got.Code != tc.status {
			t.Fatalf("status %d: %s", got.Code, got.Body)
		}
	}
}

func TestLibraryPersistsAndIsolatesSources(t *testing.T) {
	dir := t.TempDir()
	rs := []sourceapi.Registration{registration("one", &fakeSource{}), registration("two", &fakeSource{})}
	s, err := NewWithRegistrations(dir, rs)
	if err != nil {
		t.Fatal(err)
	}
	series := map[string]any{"id": "shared", "title": "A saved story", "episodes": 3}
	favorite := request(s, "POST", "/api/favorite", map[string]any{"source": "one", "series": series})
	if favorite.Code != 200 || !strings.Contains(favorite.Body.String(), `"favorite":true`) {
		t.Fatal(favorite.Body.String())
	}
	progress := request(s, "POST", "/api/progress", map[string]any{"source": "one", "series": series, "episode": map[string]any{"id": "episode2", "title": "Episode 2", "number": 2}, "position": 12.25})
	if progress.Code != 200 {
		t.Fatal(progress.Body.String())
	}
	other := request(s, "GET", "/api/library?source=two", nil)
	if other.Code != 200 || strings.Contains(other.Body.String(), "shared") {
		t.Fatal("source library leaked: ", other.Body.String())
	}
	invalid := request(s, "POST", "/api/progress", map[string]any{"source": "one", "series": series, "episode": map[string]any{"id": "episode2", "number": 2}, "position": -1})
	if invalid.Code != 400 {
		t.Fatal("negative playback position accepted")
	}
	s.Close()
	reopened, err := NewWithRegistrations(dir, []sourceapi.Registration{registration("one", &fakeSource{})})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	actual := request(reopened, "GET", "/api/library?source=one", nil)
	var lib struct {
		Favorites map[string]any
		History   map[string]struct{ Position float64 }
	}
	if err := json.Unmarshal(actual.Body.Bytes(), &lib); err != nil {
		t.Fatal(err)
	}
	if len(lib.Favorites) != 1 || lib.History["shared"].Position != 12.25 {
		t.Fatal(actual.Body.String())
	}
}

func TestCancelledResolveReleasesLateSession(t *testing.T) {
	var released string
	ctx, cancel := context.WithCancel(context.Background())
	f := &fakeSource{call: func(_ context.Context, m string, p json.RawMessage) (json.RawMessage, error) {
		if m == "Resolve" {
			cancel()
			return json.RawMessage(`{"URL":"http://127.0.0.1/media","SessionID":"late-session"}`), nil
		}
		if m == "Release" {
			var v struct{ SessionID string }
			_ = json.Unmarshal(p, &v)
			released = v.SessionID
			return nil, nil
		}
		t.Fatalf("unexpected method %s", m)
		return nil, nil
	}}
	s := newService(t, registration("sample", f))
	req := httptest.NewRequest("POST", "/api/call", strings.NewReader(`{"source":"sample","method":"Resolve","params":{"ID":"episode"}}`)).WithContext(ctx)
	req.Header.Set("X-MyGo-Mobile", s.Token)
	out := httptest.NewRecorder()
	s.ServeHTTP(out, req)
	if out.Code != 504 || released != "late-session" {
		t.Fatalf("status %d, released %q", out.Code, released)
	}
}

func TestCloseCancelsPendingCallsBeforeClosingSource(t *testing.T) {
	started := make(chan struct{})
	f := &fakeSource{call: func(ctx context.Context, _ string, _ json.RawMessage) (json.RawMessage, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	s := newService(t, registration("sample", f))
	done := make(chan struct{})
	go func() {
		request(s, "POST", "/api/call", map[string]string{"source": "sample", "method": "Discover"})
		close(done)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request did not start")
	}
	s.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("pending call was not cancelled")
	}
	f.mu.Lock()
	closed := f.closed
	f.mu.Unlock()
	if !closed {
		t.Fatal("source not closed")
	}
	if got := request(s, "GET", "/api/sources", nil); got.Code != 503 {
		t.Fatalf("closed service returned %d", got.Code)
	}
}

func TestFailedSourceRemainsVisibleAndInvalidPayloadRejected(t *testing.T) {
	bad := sourceapi.Registration{Manifest: sourceapi.Manifest{ID: "broken", Name: "Broken"}, Open: func(context.Context, string) (sourceapi.Source, error) {
		return nil, errors.New("initialization failed")
	}}
	f := &fakeSource{call: func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`not-json`), nil
	}}
	s := newService(t, bad, registration("sample", f))
	info := request(s, "GET", "/api/sources", nil)
	if !strings.Contains(info.Body.String(), `"available":false`) || !strings.Contains(info.Body.String(), "initialization failed") {
		t.Fatal(info.Body.String())
	}
	if got := request(s, "POST", "/api/call", map[string]string{"source": "sample", "method": "Discover"}); got.Code != 502 {
		t.Fatal(got.Body.String())
	}
	for _, body := range []string{`{`, `{} {}`, strings.Repeat("x", (128<<10)+1)} {
		req := httptest.NewRequest(http.MethodPost, "/api/call", strings.NewReader(body))
		req.Header.Set("X-MyGo-Mobile", s.Token)
		out := httptest.NewRecorder()
		s.ServeHTTP(out, req)
		if out.Code != 400 {
			t.Fatalf("bad request returned %d", out.Code)
		}
	}
}
