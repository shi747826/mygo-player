package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mygo-player/internal/plugins"
)

// Opt-in check against real private Releases. Credentials are resolved by the
// same installer as the GUI and never included in test output or fixtures.
func TestGitHubPrivateReleaseInstall(t *testing.T) {
	if os.Getenv("MYGO_PLAYER_LIVE_GITHUB") != "1" {
		t.Skip("set MYGO_PLAYER_LIVE_GITHUB=1 to check real GitHub releases")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	dataDir := t.TempDir()
	manager, err := plugins.LoadManager(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	// Supply private integration targets outside the host repository.
	var cases []struct{ ID, URL, Version string }
	if raw := os.Getenv("MYGO_PLAYER_LIVE_PLUGINS"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &cases); err != nil {
			t.Fatal("MYGO_PLAYER_LIVE_PLUGINS must be a JSON array of ID, URL and Version objects")
		}
	}
	if len(cases) == 0 {
		t.Skip("set MYGO_PLAYER_LIVE_PLUGINS to select integration packages")
	}
	for _, tc := range cases {
		if tc.ID == "" || tc.URL == "" || tc.Version == "" {
			t.Fatal("integration packages require ID, URL and Version")
		}
	}
	for _, tc := range cases {
		manifest, err := manager.InstallFromWithProgress(ctx, tc.URL, func(stage string) {
			t.Logf("%s: %s", tc.ID, stage)
		})
		if err != nil {
			t.Fatalf("%s install: %v", tc.ID, err)
		}
		if manifest.ID != tc.ID || manifest.Version != tc.Version || manager.IsBundled(tc.ID) || manager.Enabled(tc.ID) {
			t.Fatalf("unexpected installed package: %+v", manifest)
		}
		if err := manager.SetEnabled(tc.ID, true); err != nil {
			t.Fatal(err)
		}
		client, err := manager.Open(ctx, tc.ID)
		if err != nil {
			t.Fatalf("%s handshake: %v", tc.ID, err)
		}
		if tc.ID == "local-library" {
			if _, err := client.Discover(ctx, "", ""); err != nil {
				t.Fatalf("local library discover: %v", err)
			}
		}
		if err := manager.SetEnabled(tc.ID, false); err != nil {
			t.Fatal(err)
		}
		t.Logf("private GitHub install and protocol handshake: %s v%s", manifest.ID, manifest.Version)
	}
	if path := os.Getenv("MYGO_PLAYER_LIVE_APP"); path != "" {
		absolute, err := filepath.Abs(path)
		if err != nil {
			t.Fatal(err)
		}
		source := os.Getenv("MYGO_PLAYER_LIVE_SOURCE")
		if source == "" {
			t.Fatal("set MYGO_PLAYER_LIVE_SOURCE for native playback verification")
		}
		if err := manager.SetEnabled(source, true); err != nil {
			t.Fatal(err)
		}
		if err := manager.Close(); err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(ctx, absolute, "-data", dataDir, "-source", source, "-smoke", "-smoke-next")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("native playback: %v\n%s", err, output)
		}
		if !strings.Contains(string(output), "SMOKE PASS") || !strings.Contains(string(output), "SMOKE automatic next verified") {
			t.Fatalf("missing native evidence: %s", output)
		}
		t.Logf("native app with GitHub-installed plugin:\n%s", output)
	}
}
