package core

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func coverService(t *testing.T) *Service {
	t.Helper()
	svc, err := NewWithRegistrations(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	t.Cleanup(svc.Close)
	return svc
}

func coverGet(svc *Service, raw string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/cover?k="+svc.Token+"&u="+url.QueryEscape(raw), nil)
	rec := httptest.NewRecorder()
	svc.ServeHTTP(rec, req)
	return rec
}

var pngBytes = []byte("\x89PNG\r\n\x1a\n" + "cover-body")

func TestCoverProxyCachesAndServes(t *testing.T) {
	var hits atomic.Int32
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer origin.Close()

	svc := coverService(t)
	svc.coverClient = origin.Client()
	svc.SetCoverConverter(nil)

	rec := coverGet(svc, origin.URL+"/poster.png")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("content type = %q", got)
	}
	if rec.Body.String() != string(pngBytes) {
		t.Fatalf("body = %q", rec.Body.String())
	}

	rec = coverGet(svc, origin.URL+"/poster.png")
	if rec.Code != http.StatusOK || rec.Body.String() != string(pngBytes) {
		t.Fatalf("second response = %d %q", rec.Code, rec.Body.String())
	}
	if hits.Load() != 1 {
		t.Fatalf("origin hits = %d, want 1 (cache)", hits.Load())
	}
}

func TestCoverProxyConvertsUnknownFormat(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("heif-payload"))
	}))
	defer origin.Close()

	svc := coverService(t)
	svc.coverClient = origin.Client()
	svc.SetCoverConverter(func(path string) (string, error) {
		if data, err := os.ReadFile(path); err != nil || string(data) != "heif-payload" {
			t.Fatalf("converter got %q, %v", data, err)
		}
		out := filepath.Join(filepath.Dir(path), "converted.jpg")
		return out, os.WriteFile(out, []byte("\xff\xd8\xffjpeg"), 0o600)
	})

	rec := coverGet(svc, origin.URL+"/cover.heic")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Fatalf("content type = %q", got)
	}
	if rec.Body.String() != "\xff\xd8\xffjpeg" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestCoverProxyRejectsBadRequests(t *testing.T) {
	svc := coverService(t)
	svc.coverClient = http.DefaultClient

	req := httptest.NewRequest(http.MethodGet, "/api/cover?k=wrong&u=https%3A%2F%2Fexample.test%2Fa.png", nil)
	rec := httptest.NewRecorder()
	svc.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("wrong key status = %d", rec.Code)
	}
	for _, raw := range []string{"", "file:///etc/passwd", "javascript:alert(1)", "https://user:pw@example.test/a.png"} {
		rec = coverGet(svc, raw)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%q status = %d, want 400", raw, rec.Code)
		}
	}
}

func TestCoverProxyReportsUpstreamFailure(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer origin.Close()

	svc := coverService(t)
	svc.coverClient = origin.Client()
	svc.SetCoverConverter(func(string) (string, error) { return "", os.ErrInvalid })

	if rec := coverGet(svc, origin.URL+"/missing.heic"); rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
}
