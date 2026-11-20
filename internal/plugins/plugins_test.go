package plugins

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mygo-player/pluginapi"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerHelper(t *testing.T) {
	if os.Getenv("MYGO_PLUGIN_TEST") == "" {
		return
	}
	var canceled atomic.Int32
	err := pluginapi.Serve(os.Stdin, os.Stdout, func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		var p params
		_ = json.Unmarshal(raw, &p)
		switch method {
		case "Echo":
			return p.Days, nil
		case "Block":
			<-ctx.Done()
			canceled.Add(1)
			return nil, ctx.Err()
		case "Probe":
			return canceled.Load(), nil
		case "Crash":
			os.Exit(7)
		case "Large":
			return strings.Repeat("x", MaxMessageSize+100), nil
		case "Malformed":
			fmt.Fprintln(os.Stdout, "not json")
			return nil, nil
		case "OversizedLine":
			fmt.Fprintln(os.Stdout, strings.Repeat("x", MaxMessageSize+100))
			return nil, nil
		case "Detail":
			return pluginapi.DetailResult{Series: Series{ID: p.ID, Title: "Third party"}, Episodes: []Episode{{ID: "episode"}}}, nil
		case "Resolve":
			return pluginapi.Media{URL: "https://example.invalid/video", Definition: p.Quality, SessionID: "session"}, nil
		case "Release":
			return nil, nil
		case "Danmaku":
			return []pluginapi.Cue{{Time: 1.25, Text: p.SeriesID + ":" + p.ID}}, nil
		case "State":
			return os.Getenv("MYGO_PLUGIN_DATA_DIR"), nil
		case "Limited":
			return nil, fmt.Errorf("wrapped: %w", pluginapi.ErrAnonymousLimited)
		}
		return nil, ErrUnsupported
	})
	if err != nil {
		os.Exit(3)
	}
	if path := os.Getenv("MYGO_PLUGIN_CLEANUP"); path != "" {
		if err = os.WriteFile(path, []byte("cleaned"), 0600); err != nil {
			os.Exit(4)
		}
	}
	os.Exit(0)
}
func helperClient(t *testing.T) *Client {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestWorkerHelper$")
	cmd.Env = append(os.Environ(), "MYGO_PLUGIN_TEST=1")
	c, err := startCommand(cmd)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}
func TestGracefulClose(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "cleanup")
	cmd := exec.Command(os.Args[0], "-test.run=^TestWorkerHelper$")
	cmd.Env = append(os.Environ(), "MYGO_PLUGIN_TEST=1", "MYGO_PLUGIN_CLEANUP="+marker)
	c, err := startCommand(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	var version int
	if err = c.Invoke(context.Background(), "Ping", nil, &version); err != nil {
		t.Fatal(err)
	}
	if err = c.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(marker)
	if err != nil || string(b) != "cleaned" {
		t.Fatalf("plugin cleanup did not run: %s %v", b, err)
	}
	if !cmd.ProcessState.Success() {
		t.Fatalf("plugin did not exit gracefully: %v", cmd.ProcessState)
	}
}
func TestConcurrentProtocol(t *testing.T) {
	c := helperClient(t)
	var wg sync.WaitGroup
	for i := 0; i < 80; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var out int
			if err := c.call(ctx, "Echo", params{Days: i}, &out); err != nil || out != i {
				t.Errorf("request %d: %d %v", i, out, err)
			}
		}(i)
	}
	wg.Wait()
}
func TestCancellation(t *testing.T) {
	c := helperClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := c.call(ctx, "Block", params{}, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		var n int
		if err := c.call(context.Background(), "Probe", params{}, &n); err != nil {
			t.Fatal(err)
		}
		if n == 1 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("worker did not receive cancellation")
}
func TestCrashFailsPending(t *testing.T) {
	c := helperClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pending := make(chan error, 1)
	go func() { pending <- c.call(ctx, "Block", params{}, nil) }()
	if err := c.call(ctx, "Crash", params{}, nil); !errors.Is(err, ErrWorkerExited) {
		t.Fatal(err)
	}
	if err := <-pending; !errors.Is(err, ErrWorkerExited) {
		t.Fatal(err)
	}
	c.Close()
	if c.cmd.ProcessState == nil {
		t.Fatal("worker was not reaped")
	}
}
func TestProtocolLimits(t *testing.T) {
	for _, method := range []string{"Large", "Malformed", "OversizedLine"} {
		t.Run(method, func(t *testing.T) {
			c := helperClient(t)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := c.call(ctx, method, params{}, nil); !errors.Is(err, ErrProtocol) {
				t.Fatal(err)
			}
		})
	}
}
func TestNeutralErrors(t *testing.T) {
	c := helperClient(t)
	if err := c.Invoke(context.Background(), "Limited", nil, nil); !errors.Is(err, pluginapi.ErrAnonymousLimited) {
		t.Fatal(err)
	}
	if err := c.Invoke(context.Background(), "Unknown", nil, nil); !errors.Is(err, pluginapi.ErrUnsupported) {
		t.Fatal(err)
	}
}
func packageFixture(t *testing.T, id, script string) string {
	t.Helper()
	p := t.TempDir()
	v := Manifest{ID: id, Name: "External fixture", Version: "1", Runtime: Runtime{Type: "exec", ProtocolVersion: 2, Executable: "run"}, Capabilities: []string{"detail", "resolve", "danmaku"}, DefaultEnabled: true}
	b, _ := json.Marshal(v)
	if err := os.WriteFile(filepath.Join(p, "plugin.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	if script == "" {
		script = fmt.Sprintf("#!/bin/sh\nexport MYGO_PLUGIN_TEST=1\nexec '%s' -test.run='^TestWorkerHelper$'\n", strings.ReplaceAll(os.Args[0], "'", "'\\''"))
	}
	if err := os.WriteFile(filepath.Join(p, "run"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	return p
}
func TestThirdPartyLifecycle(t *testing.T) {
	data := t.TempDir()
	m, err := LoadManager(data)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if len(m.Manifests()) != 0 {
		t.Fatal("embedded sources were synthesized")
	}
	v, err := m.Install(packageFixture(t, "third-party", ""))
	if err != nil {
		t.Fatal(err)
	}
	if m.Enabled(v.ID) || m.IsBundled(v.ID) {
		t.Fatal("installation enabled plugin")
	}
	if _, err = m.Open(context.Background(), v.ID); err == nil {
		t.Fatal("disabled plugin opened")
	}
	if err = m.Reload(); err != nil {
		t.Fatal(err)
	}
	if err = m.SetEnabled(v.ID, true); err != nil {
		t.Fatal(err)
	}
	c, err := m.Open(context.Background(), v.ID)
	if err != nil {
		t.Fatal(err)
	}
	s, eps, err := c.Detail(context.Background(), "s")
	if err != nil || s.Title != "Third party" || len(eps) != 1 {
		t.Fatal(s, eps, err)
	}
	media, err := c.Resolve(context.Background(), "episode", "HD")
	if err != nil || media.SessionID != "session" || media.Definition != "HD" {
		t.Fatal(media, err)
	}
	if err = c.Release(context.Background(), media.SessionID); err != nil {
		t.Fatal(err)
	}
	cues, err := c.Danmaku(context.Background(), "s", "e")
	if err != nil || len(cues) != 1 || cues[0].Text != "s:e" {
		t.Fatal(cues, err)
	}
	var state string
	if err = c.Invoke(context.Background(), "State", nil, &state); err != nil || state != filepath.Join(data, "plugin-data", v.ID) {
		t.Fatal(state, err)
	}
	if err = m.SetEnabled(v.ID, false); err != nil {
		t.Fatal(err)
	}
	if err = c.Invoke(context.Background(), "Echo", nil, nil); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
	m2, err := LoadManager(data)
	if err != nil || m2.Enabled(v.ID) {
		t.Fatal(err)
	}
	defer m2.Close()
	if err = m.Uninstall(v.ID); err != nil {
		t.Fatal(err)
	}
	if len(m.Manifests()) != 0 {
		t.Fatal("uninstall did not reload")
	}
}
func TestHandshakeVersion(t *testing.T) {
	m, err := LoadManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	_, err = m.Install(packageFixture(t, "wrong-version", "#!/bin/sh\nread line\nprintf '%s\\n' '{\"version\":2,\"id\":1,\"result\":1}'\nread line\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err = m.SetEnabled("wrong-version", true); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Open(context.Background(), "wrong-version"); !errors.Is(err, ErrProtocol) {
		t.Fatal(err)
	}
}
func TestManifestValidation(t *testing.T) {
	v := Manifest{ID: "independent-source", Name: "name", Version: "1", Runtime: Runtime{Type: "exec", ProtocolVersion: 2, Executable: "bin/source"}, Capabilities: []string{"detail", "resolve"}}
	if err := v.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../source", "/bin/sh", "bin/../../bad", "bin\\source", "C:/source", "bin/source:stream", "bin/.. /source", "bin/source."} {
		v.Runtime.Executable = path
		if v.Validate() == nil {
			t.Fatalf("accepted %s", path)
		}
	}
	v.Runtime.Executable = "source"
	v.Runtime.Type = "builtin"
	if v.Validate() == nil {
		t.Fatal("builtin accepted")
	}
}
func TestInstallSecurity(t *testing.T) {
	m, err := LoadManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	p := packageFixture(t, "symlink-source", "")
	if err = os.Symlink("/bin/sh", filepath.Join(p, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Install(p); err == nil {
		t.Fatal("accepted symlink")
	}
	for _, name := range []string{"../escape", "/absolute", "bin/../../escape"} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "bad.zip")
			f, err := os.Create(p)
			if err != nil {
				t.Fatal(err)
			}
			z := zip.NewWriter(f)
			w, err := z.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = w.Write([]byte("bad"))
			z.Close()
			f.Close()
			if _, err = m.Install(p); err == nil {
				t.Fatal("accepted traversal")
			}
		})
	}
}
func TestInstallSizeLimit(t *testing.T) {
	m, err := LoadManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	src := packageFixture(t, "oversized-source", "")
	f, err := os.Create(filepath.Join(src, "huge"))
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(MaxPackageSize + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err = m.Install(src); err == nil {
		t.Fatal("accepted oversized package")
	}
	if len(m.Manifests()) != 0 {
		t.Fatal("partially installed oversized package")
	}
}
func TestZipInstall(t *testing.T) {
	m, err := LoadManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	src := packageFixture(t, "zip-source", "")
	p := filepath.Join(t.TempDir(), "source.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	for _, name := range []string{"plugin.json", "run"} {
		b, e := os.ReadFile(filepath.Join(src, name))
		if e != nil {
			t.Fatal(e)
		}
		w, e := z.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		_, _ = w.Write(b)
	}
	z.Close()
	f.Close()
	if _, err = m.Install(p); err != nil {
		t.Fatal(err)
	}
	if err = m.SetEnabled("zip-source", true); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Open(context.Background(), "zip-source"); err != nil {
		t.Fatal(err)
	}
}
