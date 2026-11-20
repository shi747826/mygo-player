package plugins

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseGitHubLink(t *testing.T) {
	for _, input := range []string{"github.com/owner/repo", "https://github.com/owner/repo/releases/latest/download/plugin-darwin-arm64.zip", "https://github.com/owner/repo", "https://github.com/owner/repo.git/", "https://github.com/owner/repo/releases/latest", "https://github.com/owner/repo/releases/tag/v0.3.0", "https://github.com/owner/repo/releases/download/v0.3.0/plugin-darwin-arm64.zip"} {
		got, err := parseGitHubLink(input)
		if err != nil || got.owner != "owner" || got.repo != "repo" {
			t.Fatalf("%s: %+v %v", input, got, err)
		}
	}
	for _, input := range []string{"http://github.com/a/b", "https://evil.test/a/b", "https://github.com.evil.test/a/b", "https://github.com:443/a/b", "https://user@github.com/a/b", "https://github.com/a/b?token=x", "https://github.com/a/b#x", "https://github.com/a/b?", "https://github.com/a/../b", "https://github.com/a/b/archive/main.zip", "https://github.com/a/b/releases/download/v1/x%2f.zip", "https://github.com/a//b", "https://github.com/a/..git", "https://github.com/a/b/releases/latest/download", "gitlab.com/a/b"} {
		if _, err := parseGitHubLink(input); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
}
func TestGitHubPlatformSelection(t *testing.T) {
	for _, tc := range []struct {
		name, os, arch string
		ok             bool
	}{
		{"plugin-darwin-arm64.zip", "darwin", "arm64", true},
		{"plugin-macos-aarch64.ZIP", "darwin", "arm64", true},
		{"plugin-linux-x86_64.zip", "linux", "amd64", true},
		{"plugin-windows-x64.zip", "windows", "amd64", true},
		{"plugin-darwin-amd64.zip", "darwin", "arm64", false},
		{"plugin-linux-arm64.zip", "darwin", "arm64", false},
		{"plugin-darwin-arm64-linux.zip", "darwin", "arm64", false},
		{"plugin-darwin-arm64-amd64.zip", "darwin", "arm64", false},
		{"plugin-darwin-arm64.zip.exe", "darwin", "arm64", false},
		{"source.zip", "darwin", "arm64", false},
		{"plugin-darwin.zip", "darwin", "arm64", false},
	} {
		if got := platformAsset(tc.name, tc.os, tc.arch); got != tc.ok {
			t.Errorf("%s: %v", tc.name, got)
		}
	}
}
func githubZIP(t *testing.T, unsafe bool) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	manifest := Manifest{ID: "github-test", Name: "GitHub test", Version: "0.3.0", Runtime: Runtime{Type: "exec", ProtocolVersion: ProtocolVersion, Executable: "bin/plugin"}, Capabilities: []string{"detail", "resolve"}}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		"plugin.json": raw, "bin/plugin": []byte("binary fixture"),
		"licenses/": nil, "licenses/NOTICE.txt": []byte("plugin notice"),
	} {
		w, e := z.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = w.Write(data); e != nil {
			t.Fatal(e)
		}
	}
	if unsafe {
		w, e := z.Create("../escape")
		if e != nil {
			t.Fatal(e)
		}
		_, _ = w.Write([]byte("bad"))
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func githubManager(t *testing.T) *Manager {
	t.Helper()
	m, err := LoadManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close() })
	return m
}
func githubTestInstaller(server *httptest.Server) githubInstaller {
	g := newGitHubInstaller()
	g.client, g.apiBase = server.Client(), server.URL
	g.credential = func(context.Context) string { return "secret-test-token" }
	g.goos, g.goarch = "darwin", "arm64"
	return g
}
func TestGitHubInstallReleaseForms(t *testing.T) {
	for _, input := range []string{"github.com/owner/repo", "https://github.com/owner/repo", "https://github.com/owner/repo/releases/latest", "https://github.com/owner/repo/releases/tag/v0.3.0", "https://github.com/owner/repo/releases/download/v0.3.0/test-darwin-arm64.zip", "https://github.com/owner/repo/releases/latest/download/test-darwin-arm64.zip"} {
		t.Run(input, func(t *testing.T) {
			data := githubZIP(t, false)
			var server *httptest.Server
			var apiCalls, assetCalls int
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer secret-test-token" {
					t.Error("missing private repo credential")
				}
				switch r.URL.Path {
				case "/repos/owner/repo/releases/latest", "/repos/owner/repo/releases/tags/v0.3.0":
					apiCalls++
					if strings.Contains(input, "v0.3.0") && !strings.HasSuffix(r.URL.Path, "/tags/v0.3.0") {
						t.Error("tag ignored")
					}
					_ = json.NewEncoder(w).Encode(githubRelease{Assets: []githubAsset{{Name: "test-linux-arm64.zip", URL: server.URL + "/repos/owner/repo/releases/assets/2"}, {Name: "test-darwin-arm64.zip", URL: server.URL + "/repos/owner/repo/releases/assets/1", Size: int64(len(data))}}})
				case "/repos/owner/repo/releases/assets/1":
					assetCalls++
					if r.Header.Get("Accept") != "application/octet-stream" {
						t.Error("private asset must use octet-stream API")
					}
					_, _ = w.Write(data)
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			m := githubManager(t)
			got, err := githubTestInstaller(server).install(context.Background(), m, input)
			if err != nil || got.ID != "github-test" {
				t.Fatalf("install: %+v %v", got, err)
			}
			if apiCalls != 1 || assetCalls != 1 {
				t.Fatalf("requests: %d %d", apiCalls, assetCalls)
			}
			installed, err := os.ReadFile(filepath.Join(m.dir, "plugins", "github-test", "bin", "plugin"))
			if err != nil || string(installed) != "binary fixture" || m.Enabled(got.ID) {
				t.Fatalf("installation incorrect: %s %v", installed, err)
			}
			if _, err = githubTestInstaller(server).install(context.Background(), m, input); err == nil || !strings.Contains(err.Error(), "already installed") {
				t.Fatalf("replacement unexpectedly accepted: %v", err)
			}
			after, _ := os.ReadFile(filepath.Join(m.dir, "plugins", "github-test", "bin", "plugin"))
			if !bytes.Equal(installed, after) {
				t.Fatal("original changed after failure")
			}
		})
	}
}
func TestGitHubErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		assets []githubAsset
		data   []byte
		limit  int64
		want   string
	}{
		{name: "unauthorized", status: 401, want: "gh auth login"},
		{name: "missing-release", status: 404, want: "published release not found"},
		{name: "forbidden", status: 403, want: "access denied"},
		{name: "server-error", status: 500, want: "HTTP 500"},
		{name: "no-platform", assets: []githubAsset{{Name: "p-linux-arm64.zip"}}, want: "darwin/arm64"},
		{name: "explicit-incompatible", assets: []githubAsset{{Name: "p-linux-arm64.zip"}}, want: "darwin/arm64"},
		{name: "ambiguous", assets: []githubAsset{{Name: "p-darwin-arm64.zip"}, {Name: "q-macos-aarch64.zip"}}, want: "multiple ZIP"},
		{name: "declared-large", assets: []githubAsset{{Name: "p-darwin-arm64.zip", Size: MaxPackageSize + 1}}, want: "size limit"},
		{name: "non-zip", assets: []githubAsset{{Name: "p-darwin-arm64.zip"}}, data: []byte("<html>login</html>"), want: "not a valid ZIP"},
		{name: "size-mismatch", assets: []githubAsset{{Name: "p-darwin-arm64.zip", Size: 20}}, data: []byte("short"), want: "size mismatch"},
		{name: "body-large", assets: []githubAsset{{Name: "p-darwin-arm64.zip"}}, data: []byte(strings.Repeat("x", 65)), limit: 64, want: "size limit"},
		{name: "unsafe", assets: []githubAsset{{Name: "p-darwin-arm64.zip"}}, data: githubZIP(t, true), want: "unsafe"},
		{name: "external-api", assets: []githubAsset{{Name: "p-darwin-arm64.zip", URL: "https://evil.test/repos/owner/repo/releases/assets/1"}}, want: "untrusted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/latest") || strings.Contains(r.URL.Path, "/tags/") {
					if tc.status != 0 {
						w.WriteHeader(tc.status)
						return
					}
					assets := append([]githubAsset(nil), tc.assets...)
					for i := range assets {
						if assets[i].URL == "" {
							assets[i].URL = server.URL + fmt.Sprintf("/repos/owner/repo/releases/assets/%d", i+1)
						}
					}
					_ = json.NewEncoder(w).Encode(githubRelease{Assets: assets})
				} else {
					_, _ = w.Write(tc.data)
				}
			}))
			defer server.Close()
			g := githubTestInstaller(server)
			if tc.limit != 0 {
				g.maxBytes = tc.limit
			}
			input := "https://github.com/owner/repo"
			if tc.name == "explicit-incompatible" {
				input += "/releases/download/v1/p-linux-arm64.zip"
			}
			m := githubManager(t)
			_, err := g.install(context.Background(), m, input)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %s, got %v", tc.want, err)
			}
			if len(m.Manifests()) != 0 {
				t.Fatal("failed install left manifest")
			}
		})
	}
}
func TestGitHubRedirectCredentialIsolation(t *testing.T) {
	data := githubZIP(t, false)
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("credential leaked to CDN")
		}
		_, _ = w.Write(data)
	}))
	defer cdn.Close()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-test-token" {
			t.Error("API credential missing")
		}
		if strings.HasSuffix(r.URL.Path, "/latest") {
			_ = json.NewEncoder(w).Encode(githubRelease{Assets: []githubAsset{{Name: "p-darwin-arm64.zip", URL: server.URL + "/repos/owner/repo/releases/assets/1"}}})
		} else {
			http.Redirect(w, r, cdn.URL+"/signed", http.StatusFound)
		}
	}))
	defer server.Close()
	if _, err := githubTestInstaller(server).install(context.Background(), githubManager(t), "https://github.com/owner/repo"); err != nil {
		t.Fatal(err)
	}
}
func TestGitHubCancellation(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	g := githubTestInstaller(server)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	m := githubManager(t)
	go func() { _, err := g.install(ctx, m, "https://github.com/owner/repo"); done <- err }()
	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancellation did not stop request")
	}
}
func TestGitHubCredentialEnvironment(t *testing.T) {
	t.Setenv("GH_TOKEN", "first-token")
	t.Setenv("GITHUB_TOKEN", "second-token")
	if got := githubCredential(context.Background()); got != "first-token" {
		t.Fatal("GH_TOKEN precedence")
	}
	t.Setenv("GH_TOKEN", "")
	if got := githubCredential(context.Background()); got != "second-token" {
		t.Fatal("GITHUB_TOKEN fallback")
	}
}
func TestInstallFromLocalAndInvalidURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "local.zip")
	if err := os.WriteFile(path, githubZIP(t, false), 0600); err != nil {
		t.Fatal(err)
	}
	m := githubManager(t)
	if got, err := m.InstallFrom(context.Background(), path); err != nil || got.ID != "github-test" {
		t.Fatalf("local install: %+v %v", got, err)
	}
	if _, err := m.InstallFrom(context.Background(), "https://evil.test/p.zip"); err == nil || !strings.Contains(err.Error(), "github.com") {
		t.Fatalf("invalid URL: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.InstallFrom(ctx, path); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestGitHubAnonymousStreamingAndCleanup(t *testing.T) {
	for _, mode := range []string{"anonymous", "stream-large", "cancel-download", "timeout-download"} {
		t.Run(mode, func(t *testing.T) {
			m := githubManager(t)
			temp := t.TempDir()
			t.Setenv("TMPDIR", temp)
			data := githubZIP(t, false)
			started := make(chan struct{})
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "" {
					t.Error("anonymous request sent token")
				}
				if strings.HasSuffix(r.URL.Path, "/latest") {
					_ = json.NewEncoder(w).Encode(githubRelease{Assets: []githubAsset{{Name: "p-darwin-arm64.zip", URL: server.URL + "/repos/owner/repo/releases/assets/1"}}})
					return
				}
				w.WriteHeader(200)
				w.(http.Flusher).Flush()
				close(started)
				switch mode {
				case "stream-large":
					_, _ = w.Write(bytes.Repeat([]byte("a"), 65))
				case "cancel-download", "timeout-download":
					<-r.Context().Done()
				default:
					_, _ = w.Write(data)
				}
			}))
			defer server.Close()
			g := githubTestInstaller(server)
			g.credential = func(context.Context) string { return "" }
			if mode == "stream-large" {
				g.maxBytes = 64
			}
			if mode == "timeout-download" {
				g.timeout = 200 * time.Millisecond
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { _, err := g.install(ctx, m, "https://github.com/owner/repo"); done <- err }()
			<-started
			if mode == "cancel-download" {
				cancel()
			}
			select {
			case err := <-done:
				switch mode {
				case "anonymous":
					if err != nil {
						t.Fatal(err)
					}
				case "stream-large":
					if err == nil || !strings.Contains(err.Error(), "size limit") {
						t.Fatal(err)
					}
				case "cancel-download":
					if !errors.Is(err, context.Canceled) {
						t.Fatal(err)
					}
				case "timeout-download":
					if !errors.Is(err, context.DeadlineExceeded) {
						t.Fatal(err)
					}
				}
			case <-time.After(3 * time.Second):
				t.Fatal("download did not stop")
			}
			entries, err := os.ReadDir(temp)
			if err != nil || len(entries) != 0 {
				t.Fatalf("temporary download not removed: %v %v", entries, err)
			}
		})
	}
}
