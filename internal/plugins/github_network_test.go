package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestGitHubMacProxyParsing(t *testing.T) {
	values := githubParseProxyValues(`<dictionary> {
  HTTPEnable : 1
  HTTPProxy : 127.0.0.1
  HTTPPort : 7897
  HTTPSEnable : 1
  HTTPSProxy : ::1
  HTTPSPort : 7898
}`)
	for scheme, want := range map[string]string{"http": "http://127.0.0.1:7897", "https": "http://[::1]:7898"} {
		if got := githubMacProxy(scheme, values); got == nil || got.String() != want {
			t.Fatalf("%s proxy mismatch", scheme)
		}
	}
	for _, tc := range []struct{ key, value string }{{"HTTPSEnable", "0"}, {"HTTPSPort", "0"}, {"HTTPSPort", "65536"}, {"HTTPSPort", "bad"}, {"HTTPSProxy", ""}, {"HTTPSProxy", "user@host"}} {
		copy := make(map[string]string)
		for k, v := range values {
			copy[k] = v
		}
		copy[tc.key] = tc.value
		if githubMacProxy("https", copy) != nil {
			t.Errorf("accepted invalid %s", tc.key)
		}
	}
	if githubMacProxy("ftp", values) != nil {
		t.Fatal("unsupported scheme")
	}
	values["SOCKSEnable"], values["SOCKSProxy"], values["SOCKSPort"] = "1", "127.0.0.1", "7899"
	if proxy := githubMacProxy("https", values); proxy == nil || proxy.Scheme != "http" {
		t.Fatal("HTTPS system proxy must take precedence over SOCKS")
	}
	values["HTTPSEnable"] = "0"
	if proxy := githubMacProxy("https", values); proxy == nil || proxy.String() != "socks5://127.0.0.1:7899" {
		t.Fatal("SOCKS fallback missing")
	}
}

func TestGitHubSystemProxyWithoutEnvironment(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS system proxy")
	}
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy", "NO_PROXY", "no_proxy"} {
		t.Setenv(key, "")
	}
	githubProxyCache.Lock()
	oldAt, oldValues := githubProxyCache.at, githubProxyCache.values
	githubProxyCache.at = time.Now()
	githubProxyCache.values = githubParseProxyValues("HTTPSEnable : 1\nHTTPSProxy : 127.0.0.1\nHTTPSPort : 7897")
	githubProxyCache.Unlock()
	t.Cleanup(func() {
		githubProxyCache.Lock()
		githubProxyCache.at, githubProxyCache.values = oldAt, oldValues
		githubProxyCache.Unlock()
	})
	req, _ := http.NewRequest("GET", "https://api.github.com/repos/o/r", nil)
	proxy, err := githubProxy(req)
	if err != nil || proxy == nil || proxy.String() != "http://127.0.0.1:7897" {
		t.Fatal("system proxy was not selected")
	}
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:9898")
	if proxy, err = githubProxy(req); err != nil || proxy == nil || proxy.String() != "http://127.0.0.1:9898" {
		t.Fatal("explicit environment proxy did not take precedence")
	}
	t.Setenv("NO_PROXY", ".github.com")
	if proxy, err = githubProxy(req); err != nil || proxy != nil {
		t.Fatal("NO_PROXY did not bypass explicit proxy")
	}
	t.Setenv("HTTPS_PROXY", "")
	if proxy, err = githubProxy(req); err != nil || proxy != nil {
		t.Fatal("NO_PROXY did not bypass system proxy")
	}
	for _, target := range []string{"http://localhost/x", "http://127.0.0.1/x", "http://[::1]/x"} {
		req, _ = http.NewRequest("GET", target, nil)
		if proxy, err = githubProxy(req); err != nil || proxy != nil {
			t.Fatal("local request used proxy")
		}
	}
}

func TestGitHubCommandTimeoutAndInheritedPipe(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	for _, tc := range []struct {
		name, command string
		timeout       time.Duration
		want          error
	}{
		{"deadline", "exec sleep 2", 40 * time.Millisecond, context.DeadlineExceeded},
		{"inherited-output", "sleep 2 & exit 0", time.Second, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			_, err := githubCommand(context.Background(), tc.timeout, "/bin/sh", "-c", tc.command)
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.want == nil && err == nil {
				t.Fatal("expected bounded pipe drain error")
			}
			if time.Since(start) > time.Second {
				t.Fatal("command or inherited pipe was not bounded")
			}
		})
	}
}

func TestGitHubCLIFinderPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix desktop PATH")
	}
	home := t.TempDir()
	path := filepath.Join(home, ".local", "bin", "gh")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "/usr/bin:/bin")
	if githubCLIPath(home) != path {
		t.Fatal("Finder PATH did not find user-local GitHub CLI")
	}
	// PATH remains authoritative when the caller explicitly supplies one.
	t.Setenv("PATH", filepath.Dir(path))
	if githubCLIPath("") != path {
		t.Fatal("PATH GitHub CLI was ignored")
	}
}

func TestGitHubCredentialShortTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte("#!/bin/sh\nexec /bin/sleep 10\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	start := time.Now()
	if githubCredential(context.Background()) != "" {
		t.Fatal("failed command returned a credential")
	}
	if elapsed := time.Since(start); elapsed < githubCommandTimeout || elapsed > githubCommandTimeout+time.Second {
		t.Fatalf("credential timeout: %v", elapsed)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start = time.Now()
	if githubCredential(ctx) != "" || time.Since(start) > time.Second {
		t.Fatal("credential ignored cancellation")
	}
}

func TestGitHubTransportTimeouts(t *testing.T) {
	client := newGitHubHTTPClient()
	transport := client.Transport.(*http.Transport)
	defer transport.CloseIdleConnections()
	if transport.DialContext == nil || transport.TLSHandshakeTimeout <= 0 || transport.ResponseHeaderTimeout <= 0 {
		t.Fatal("missing transport timeout")
	}
	transport.Proxy = nil
	transport.ResponseHeaderTimeout = 40 * time.Millisecond
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	g := newGitHubInstaller()
	g.client, g.apiBase = client, server.URL
	start := time.Now()
	_, err := g.request(context.Background(), server.URL+"/repos/o/r", "secret-test-token", "application/json")
	if !errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "secret-test-token") || time.Since(start) > time.Second {
		t.Fatalf("header timeout: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	defer close(done)
	go func() {
		conn, e := listener.Accept()
		if e == nil {
			defer conn.Close()
			<-done
		}
	}()
	transport.TLSHandshakeTimeout = 40 * time.Millisecond
	g.apiBase = "https://" + listener.Addr().String()
	_, err = g.request(context.Background(), g.apiBase+"/repos/o/r", "", "application/json")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("TLS timeout: %v", err)
	}
}

func TestGitHubProgressAndStageCancellation(t *testing.T) {
	stages := []string{"github:credentials", "github:release", "github:download", "github:install", "github:complete"}
	for _, cancelAt := range append([]string{""}, stages[:4]...) {
		t.Run("cancel-"+cancelAt, func(t *testing.T) {
			data := githubZIP(t, false)
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/latest") {
					_ = json.NewEncoder(w).Encode(githubRelease{Assets: []githubAsset{{Name: "p-darwin-arm64.zip", URL: server.URL + "/repos/owner/repo/releases/assets/1"}}})
				} else {
					_, _ = w.Write(data)
				}
			}))
			defer server.Close()
			m := githubManager(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var got []string
			_, err := githubTestInstaller(server).installWithProgress(ctx, m, "https://github.com/owner/repo", func(stage string) {
				got = append(got, stage)
				if stage == cancelAt {
					cancel()
				}
			})
			want := stages
			if cancelAt != "" {
				if !errors.Is(err, context.Canceled) || len(m.Manifests()) != 0 {
					t.Fatalf("canceled install changed manager: %v", err)
				}
				for i, stage := range stages {
					if stage == cancelAt {
						want = stages[:i+1]
						break
					}
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("stages: %v; want %v", got, want)
			}
		})
	}
}
