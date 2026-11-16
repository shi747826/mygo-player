package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"mygo-player/internal/plugins"
	"mygo-player/pluginapi"
)

// This is a small protocol fixture, not a bundled source plugin. A separate
// module is compiled after the host test binary, then installed without rebuild.
func TestExternalExecutableModule(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	moduleDir := t.TempDir()
	module := fmt.Sprintf("module example.com/protocol-fixture\n\ngo 1.27.3\n\nrequire mygo-player v0.0.0\nreplace mygo-player => %q\n", root)
	for name, data := range map[string]string{"go.mod": module, "main.go": protocolFixtureSource} {
		if err := os.WriteFile(filepath.Join(moduleDir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	goPath := filepath.Join(runtime.GOROOT(), "bin", "go")
	if value := os.Getenv("GO"); value != "" {
		goPath = value
	}
	executable := "fixture"
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	binary := filepath.Join(moduleDir, executable)
	cmd := exec.Command(goPath, "build", "-trimpath", "-o", binary, ".")
	cmd.Dir = moduleDir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("independent fixture build: %v\n%s", err, output)
	}
	manifest := plugins.Manifest{ID: "protocol-fixture", Name: "Protocol fixture", Version: "1.0.0", Runtime: plugins.Runtime{Type: "exec", ProtocolVersion: 2, Executable: executable}, Capabilities: []string{"discover", "search", "detail", "resolve"}}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "fixture.zip")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, content := range map[string][]byte{"plugin.json": raw, executable: data} {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(0755)
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	pm, err := plugins.LoadManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer pm.Close()
	installed, err := pm.Install(archive)
	if err != nil {
		t.Fatal(err)
	}
	if installed.ID != manifest.ID || pm.Enabled(installed.ID) {
		t.Fatalf("bad install: %+v", installed)
	}
	if err := pm.SetEnabled(installed.ID, true); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client, err := pm.Open(ctx, installed.ID)
	if err != nil {
		t.Fatal(err)
	}
	a := &app{pluginManager: pm, source: installed.ID, pluginClients: map[string]*plugins.Client{installed.ID: client}}
	if a.sourceHas("danmaku") || !a.sourceHas("resolve") {
		t.Fatal("incorrect optional capabilities")
	}
	page, err := a.sourceAPI().Discover(ctx, "", "")
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("Discover: %+v %v", page, err)
	}
	found, err := client.Search(ctx, "fixture", "")
	if err != nil || len(found.Items) != 1 {
		t.Fatalf("Search: %+v %v", found, err)
	}
	_, episodes, err := client.Detail(ctx, found.Items[0].ID)
	if err != nil || len(episodes) != 1 {
		t.Fatalf("Detail: %+v %v", episodes, err)
	}
	media, err := client.Resolve(ctx, episodes[0].ID, "")
	if err != nil || media.SessionID != "fixture-session" {
		t.Fatalf("Resolve: %+v %v", media, err)
	}
	if _, err := client.Danmaku(ctx, "fixture", "episode"); !errors.Is(err, pluginapi.ErrUnsupported) {
		t.Fatalf("Danmaku: %v", err)
	}
	if err := client.Release(ctx, media.SessionID); err != nil {
		t.Fatal(err)
	}
	if err := pm.Reload(); err != nil {
		t.Fatal(err)
	}
	if err := pm.SetEnabled(installed.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := pm.Open(ctx, installed.ID); err == nil {
		t.Fatal("disabled source opened")
	}
	if err := pm.Uninstall(installed.ID); err != nil {
		t.Fatal(err)
	}
	for _, m := range pm.Manifests() {
		if m.ID == installed.ID {
			t.Fatal("uninstalled source remains")
		}
	}
}

const protocolFixtureSource = `package main
import("context";"encoding/json";"os";api "mygo-player/pluginapi")
func main(){
 series:=api.Series{ID:"fixture",Title:"Fixture",Episodes:1}
 handler:=func(ctx context.Context, method string, raw json.RawMessage)(any,error){
  switch method{
  case "Discover","Search": return api.Page{Items:[]api.Series{series}},nil
  case "Detail": return api.DetailResult{Series:series,Episodes:[]api.Episode{{ID:"episode",Title:"Episode",Number:1}}},nil
  case "Resolve": return api.Media{URL:"https://example.invalid/fixture.mp4",SessionID:"fixture-session"},nil
  case "Release": return nil,nil
  default:return nil,api.ErrUnsupported
  }
 }
 if err:=api.Serve(os.Stdin,os.Stdout,handler);err!=nil{os.Exit(1)}
}
`
