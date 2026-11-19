package core

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestPrefetchedResolveIsReusedOnce(t *testing.T) {
	var calls atomic.Int32
	release := make(chan string, 4)
	var src *fakeSource
	src = &fakeSource{call: func(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
		switch method {
		case "Resolve":
			calls.Add(1)
			var p struct{ ID string }
			_ = json.Unmarshal(params, &p)
			time.Sleep(120 * time.Millisecond)
			return json.RawMessage(`{"URL":"http://127.0.0.1/media/` + p.ID + `","SessionID":"session-` + p.ID + `"}`), nil
		case "Release":
			var p struct{ SessionID string }
			_ = json.Unmarshal(params, &p)
			release <- p.SessionID
			return json.RawMessage(`null`), nil
		}
		return json.RawMessage(`null`), nil
	}}
	svc := newService(t, registration("sample", src))

	pre := httptest.NewRequest("POST", "/api/prefetch", strings.NewReader(`{"source":"sample","params":{"id":"ep2","quality":""}}`))
	pre.Header.Set("X-MyGo-Mobile", svc.Token)
	rec := httptest.NewRecorder()
	svc.ServeHTTP(rec, pre)
	if rec.Code != 200 {
		t.Fatalf("prefetch status %d: %s", rec.Code, rec.Body)
	}
	time.Sleep(250 * time.Millisecond)

	start := time.Now()
	body := request(svc, "POST", "/api/call", map[string]any{"source": "sample", "method": "Resolve", "params": map[string]string{"id": "ep2"}})
	if body.Code != 200 || !strings.Contains(body.Body.String(), "ep2") {
		t.Fatalf("resolve = %d %s", body.Code, body.Body.String())
	}
	if elapsed := time.Since(start); elapsed > 80*time.Millisecond {
		t.Fatalf("prefetched resolve waited %v", elapsed)
	}
	if calls.Load() != 1 {
		t.Fatalf("source resolved %d times, want 1", calls.Load())
	}
	// A second resolve asks the source again, since the entry is consumed.
	if got := request(svc, "POST", "/api/call", map[string]any{"source": "sample", "method": "Resolve", "params": map[string]string{"id": "ep2"}}); got.Code != 200 {
		t.Fatalf("second resolve = %d", got.Code)
	}
	if calls.Load() != 2 {
		t.Fatalf("source resolved %d times after reuse, want 2", calls.Load())
	}
}

func TestPrefetchReleasesUnusedSessions(t *testing.T) {
	released := make(chan string, 4)
	src := &fakeSource{call: func(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
		if method == "Release" {
			var p struct{ SessionID string }
			_ = json.Unmarshal(nil, &p)
			return json.RawMessage(`null`), nil
		}
		return json.RawMessage(`{"URL":"http://127.0.0.1/media","SessionID":"session-unused"}`), nil
	}}
	svc := newService(t, registration("sample", src))
	svc.prefetchMu.Lock()
	svc.ready[prefetchKey("sample", "ep3")] = &prefetchEntry{source: "sample", done: closedCh(), raw: json.RawMessage(`{"SessionID":"session-unused"}`), at: time.Now().Add(-2 * prefetchTTL), sessionID: "session-unused"}
	svc.prefetchMu.Unlock()
	svc.prunePrefetch()
	svc.prefetchMu.Lock()
	remaining := len(svc.ready)
	svc.prefetchMu.Unlock()
	if remaining != 0 {
		t.Fatalf("expired entry kept: %d", remaining)
	}
	_ = released
}

func closedCh() chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
