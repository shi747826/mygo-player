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
	"runtime"
	"strings"
	"testing"
)

func updateFixture(t *testing.T, id, version, script string) string {
	t.Helper()
	root := packageFixture(t, id, script)
	manifestPath := filepath.Join(root, "plugin.json")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := ParseManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Version = version
	raw, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return root
}

func updateZIP(t *testing.T, root string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zip := zip.NewWriter(&buf)
	for _, name := range []string{"plugin.json", "run"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		entry, err := zip.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zip.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPluginUpdateLifecycle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a shell executable")
	}
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("enabled=%v", enabled), func(t *testing.T) {
			ctx := context.Background()
			m := githubManager(t)
			const id = "update-fixture"
			if _, err := m.Install(updateFixture(t, id, "1.9.0", "")); err != nil {
				t.Fatal(err)
			}
			if err := m.SetEnabled(id, enabled); err != nil {
				t.Fatal(err)
			}
			state := filepath.Join(m.dir, "plugin-data", id, "history")
			if err := os.MkdirAll(filepath.Dir(state), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(state, []byte("saved history"), 0600); err != nil {
				t.Fatal(err)
			}
			if enabled {
				if _, err := m.Open(ctx, id); err != nil {
					t.Fatal(err)
				}
			}
			pkg := filepath.Join(t.TempDir(), "update.zip")
			if err := os.WriteFile(pkg, updateZIP(t, updateFixture(t, id, "v1.10.0+build.5", "")), 0600); err != nil {
				t.Fatal(err)
			}
			if got, err := m.UpdateFrom(ctx, id, pkg); err != nil || got.Version != "v1.10.0+build.5" {
				t.Fatalf("update: %+v, %v", got, err)
			}
			const corruptThenExit = "#!/bin/sh\nprintf damaged > \"$MYGO_PLUGIN_DATA_DIR/history\"\nprintf new > \"$MYGO_PLUGIN_DATA_DIR/new-file\"\nexit 1\n"
			if _, err := m.UpdateFrom(ctx, id, updateFixture(t, id, "2.0.0", corruptThenExit)); err == nil || !strings.Contains(err.Error(), "updated executable failed to start") {
				t.Fatalf("broken update accepted: %v", err)
			}
			for _, version := range []string{"1.10.0", "1.10.0-rc.2", "1.9.9"} {
				if _, err := m.UpdateFrom(ctx, id, updateFixture(t, id, version, corruptThenExit)); !errors.Is(err, ErrNoPluginUpdate) {
					t.Fatalf("non-upgrade %s: %v", version, err)
				}
			}
			if _, err := m.UpdateFrom(ctx, id, updateFixture(t, "wrong-id", "3.0.0", "")); err == nil || !strings.Contains(err.Error(), "does not match installed plugin") {
				t.Fatalf("different plugin accepted: %v", err)
			}
			if got, err := os.ReadFile(state); err != nil || string(got) != "saved history" {
				t.Fatalf("history changed: %q, %v", got, err)
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(state), "new-file")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("failed candidate data remains: %v", err)
			}
			if enabled {
				client, err := m.Open(ctx, id)
				if err != nil {
					t.Fatal(err)
				}
				result, _, err := client.Detail(ctx, "series")
				if err != nil || result.Title != "Third party" {
					t.Fatalf("restored worker unavailable: %+v, %v", result, err)
				}
			}
			reloaded, err := LoadManager(m.dir)
			if err != nil {
				t.Fatal(err)
			}
			defer reloaded.Close()
			if found := reloaded.Manifests(); len(found) != 1 || found[0].Version != "v1.10.0+build.5" || reloaded.Enabled(id) != enabled {
				t.Fatalf("version or enabled state changed: %+v, enabled=%v", found, reloaded.Enabled(id))
			}
		})
	}
}

func TestGitHubUpdateUsesSavedSourceAndLatestRelease(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a shell executable")
	}
	const id = "github-update"
	first := updateZIP(t, updateFixture(t, id, "1.0.0", ""))
	latest := updateZIP(t, updateFixture(t, id, "1.1.0", ""))
	ctx := context.Background()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-test-token" {
			t.Error("missing credential on private update request")
		}
		const releases = "/repos/owner/repo/releases/"
		switch r.URL.Path {
		case releases + "tags/v1.0.0":
			_ = json.NewEncoder(w).Encode(githubRelease{Assets: []githubAsset{{Name: "test-darwin-arm64.zip", URL: server.URL + releases + "assets/1"}}})
		case releases + "latest":
			_ = json.NewEncoder(w).Encode(githubRelease{Assets: []githubAsset{
				{Name: "another-darwin-arm64.zip", URL: server.URL + releases + "assets/3"},
				{Name: "test-darwin-arm64.zip", URL: server.URL + releases + "assets/2"},
			}})
		case releases + "assets/1":
			_, _ = w.Write(first)
		case releases + "assets/2":
			if r.Header.Get("Accept") != "application/octet-stream" {
				t.Error("missing binary asset Accept header")
			}
			_, _ = w.Write(latest)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	installer := githubTestInstaller(server)
	m := githubManager(t)
	if _, err := installer.install(ctx, m, "https://github.com/owner/repo/releases/download/v1.0.0/test-darwin-arm64.zip"); err != nil {
		t.Fatal(err)
	}
	reloaded, err := LoadManager(m.dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reloaded.Close()
	const source = "https://github.com/owner/repo/releases/latest/download/test-darwin-arm64.zip"
	if got := reloaded.UpdateSource(id); got != source {
		t.Fatalf("saved source: %q", got)
	}
	got, err := installer.update(ctx, reloaded, id, "", nil)
	if err != nil || got.Version != "1.1.0" || reloaded.Enabled(id) {
		t.Fatalf("update: %+v, %v, enabled=%v", got, err, reloaded.Enabled(id))
	}
	if _, err := reloaded.UpdateFrom(ctx, id, updateFixture(t, id, "1.2.0", "")); err != nil {
		t.Fatal(err)
	}
	if got := reloaded.UpdateSource(id); got != source {
		t.Fatalf("local update lost saved source: %q", got)
	}
	if _, err := installer.update(ctx, reloaded, id, "", nil); !errors.Is(err, ErrNoPluginUpdate) {
		t.Fatalf("older GitHub release accepted: %v", err)
	}
}

func TestGitHubUpdateLearnsSourceForLegacyPackage(t *testing.T) {
	const id = "legacy-plugin"
	pkg := updateFixture(t, id, "1.2", "")
	data := updateZIP(t, pkg)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/releases/latest":
			_ = json.NewEncoder(w).Encode(githubRelease{Assets: []githubAsset{{Name: "latest-darwin-arm64.zip", URL: server.URL + "/repos/owner/repo/releases/assets/1"}}})
		case "/repos/owner/repo/releases/assets/1":
			_, _ = w.Write(data)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	m := githubManager(t)
	if _, err := m.Install(pkg); err != nil {
		t.Fatal(err)
	}
	if m.UpdateSource(id) != "" {
		t.Fatal("local plugin unexpectedly has a GitHub source")
	}
	const source = "https://github.com/owner/repo"
	if _, err := githubTestInstaller(server).update(context.Background(), m, id, source, nil); !errors.Is(err, ErrNoPluginUpdate) {
		t.Fatalf("equal release: %v", err)
	}
	if got := m.UpdateSource(id); got != source {
		t.Fatalf("legacy source not remembered: %q", got)
	}
}

func TestPluginVersionPrecedence(t *testing.T) {
	versions := []string{"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta", "1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0", "v1.0.1", "1.2", "1.10.0", "1000000000000000000000.0.0"}
	for i := 1; i < len(versions); i++ {
		if got, err := comparePluginVersions(versions[i-1], versions[i]); err != nil || got >= 0 {
			t.Errorf("%s should precede %s: %d %v", versions[i-1], versions[i], got, err)
		}
	}
	if got, err := comparePluginVersions("v1.0.0+build.11", "1"); err != nil || got != 0 {
		t.Errorf("equivalent versions: %d %v", got, err)
	}
	for _, invalid := range []string{"1.0.0-01", "1.02.0", "nightly", "1.0.1.2"} {
		if _, err := comparePluginVersions(invalid, "1"); err == nil {
			t.Errorf("accepted version %q", invalid)
		}
	}
}
