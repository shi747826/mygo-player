package main

import (
	"context"
	"encoding/json"
	"image/png"
	"mygo-player/pluginapi"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"mygo-player/internal/catalog"
	"mygo-player/internal/plugins"
)

func TestPluginManagementWithNoEnabledSources(t *testing.T) {
	dir := t.TempDir()
	pm, err := plugins.LoadManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer pm.Close()
	catalogClient, err := catalog.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(catalogClient, nil)
	defer a.cancel()
	a.pluginManager = pm
	a.pluginClients = map[string]*plugins.Client{}
	a.source = ""
	a.loadPage("推荐", false)
	if a.page != "插件" || a.busy {
		t.Fatalf("disabled sources started catalog: page=%s busy=%v", a.page, a.busy)
	}
	tt := ui.NewTester(a.view, 1280, 820)
	if len(a.pluginManager.Manifests()) != 0 {
		t.Fatal("render enabled a source")
	}
	if preview := os.Getenv("MYGO_PLAYER_PLUGIN_PREVIEW"); preview != "" {
		tt = ui.NewTester(a.view, 1280, 820)
		f, err := os.Create(preview)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPluginNavigationUsesManifest(t *testing.T) {
	cl, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(cl, nil)
	defer a.cancel()
	attachUITestPlugin(t, a, plugins.Manifest{ID: "third-source", Name: "第三个来源", Version: "1", Capabilities: []string{"detail", "resolve", "discover", "search"}, Navigation: []plugins.Navigation{{Label: "自定义推荐", Value: "推荐"}}, Filters: []plugins.Filter{{Label: "频道", Value: "channel", Options: []plugins.Option{{Label: "测试频道", Value: "test"}}}}})
	a.page = "推荐"
	tt := ui.NewTester(a.view, 1280, 820)
	for _, text := range []string{"自定义推荐", "测试频道"} {
		if !tt.HasText(text) {
			t.Fatalf("manifest ignored %q: %v", text, tt.Texts())
		}
	}
	if tt.HasText("真人剧") {
		t.Fatal("hardcoded filters leaked into plugin navigation")
	}
}

func TestPluginPageHasNoCatalogRequest(t *testing.T) {
	cl, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(cl, nil)
	defer a.cancel()
	ctx, cancel := context.WithCancel(a.ctx)
	a.ctx = ctx
	defer cancel()
	a.loadPage("插件", false)
	if a.busy || a.page != "插件" {
		t.Fatal("plugin page triggered remote load")
	}
}

func TestClearHLSPreloadsWithoutPreparedSession(t *testing.T) {
	a := &app{preloaded: map[string]*preparedEpisode{"s:1": {media: pluginapi.Media{URL: "http://127.0.0.1/example"}}}}
	a.clearPreloads()
	if len(a.preloaded) != 0 {
		t.Fatal("preloads retained")
	}
}

func TestDisablingLastSourceResetsSourceRoute(t *testing.T) {
	cl, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(cl, nil)
	defer a.cancel()
	attachUITestPlugin(t, a, plugins.Manifest{ID: "third-source", Name: "测试来源", Version: "1", Capabilities: []string{"detail", "resolve"}})
	a.page = "插件"
	a.searchedQuery = "old query"
	a.query = "draft"
	a.channel = "old-channel"
	a.hgSort = "old-sort"
	a.channelSel = 2
	a.hgSortSel = 1
	a.setPluginEnabled("third-source", false)
	if a.pluginBusy || a.source != "" || a.channel != "" || a.hgSort != "" || a.query != "" || a.searchedQuery != "" || a.page != "插件" {
		t.Fatalf("stale route after disable: source=%q channel=%q sort=%q query=%q", a.source, a.channel, a.hgSort, a.query)
	}
	if a.sourceSearch["third-source"] != "old query" {
		t.Fatal("source search memory lost")
	}
}

func TestPluginInstallCanBeCanceledFromPage(t *testing.T) {
	cl, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(cl, nil)
	defer a.cancel()
	pm, err := plugins.LoadManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer pm.Close()
	a.pluginManager = pm
	a.page = "插件"
	a.pluginBusy = true
	ctx, cancel := context.WithCancel(a.ctx)
	defer cancel()
	a.pluginInstallCancel = cancel
	a.pluginMessage = pluginInstallStage("github:release")
	tt := ui.NewTester(a.view, 1280, 820)
	if !tt.HasText("正在连接 GitHub，获取插件版本…") {
		t.Fatal("installation stage is hidden")
	}
	if err := tt.Click("取消安装"); err != nil {
		t.Fatal(err)
	}
	if ctx.Err() != context.Canceled || a.pluginMessage != "正在取消安装…" {
		t.Fatal("cancel button did not cancel installation")
	}
}

func TestLegacyPluginUpdateOptionsAndCancellation(t *testing.T) {
	cl, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(cl, nil)
	defer a.cancel()
	const id = "legacy-source"
	attachUITestPlugin(t, a, plugins.Manifest{ID: id, Name: "测试来源", Version: "1.0.0", Capabilities: []string{"detail", "resolve"}})
	a.page = "插件"
	tt := ui.NewTester(a.view, 1280, 820)
	if tt.HasText("选择新版 ZIP…") {
		t.Fatal("plugin update options were open before selecting a plugin")
	}
	if err := tt.Click("更新…"); err != nil {
		t.Fatal(err)
	}
	if a.pluginUpdateID != id || !tt.HasText("检查并更新") || !tt.HasText("选择新版 ZIP…") {
		t.Fatalf("missing legacy update options: %v", tt.Texts())
	}
	ctx, cancel := context.WithCancel(a.ctx)
	defer cancel()
	a.pluginBusy, a.pluginUpdating = true, true
	a.pluginInstallCancel = cancel
	a.pluginMessage = "正在更新插件…"
	tt = ui.NewTester(a.view, 1280, 820)
	if err := tt.Click("取消更新"); err != nil {
		t.Fatal(err)
	}
	if ctx.Err() != context.Canceled || a.pluginMessage != "正在取消更新…" {
		t.Fatal("cancel button did not cancel the plugin update")
	}
}

// UI fixtures register a manifest without launching an executable or source SDK.
func attachUITestPlugin(t *testing.T, a *app, m plugins.Manifest) {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, "plugins", m.ID)
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	m.Runtime = plugins.Runtime{Type: "exec", ProtocolVersion: pluginapi.ProtocolVersion, Executable: "fixture"}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "fixture"), []byte("fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	pm, err := plugins.LoadManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pm.Close() })
	if err := pm.SetEnabled(m.ID, true); err != nil {
		t.Fatal(err)
	}
	a.pluginManager = pm
	a.source = m.ID
}
