package main

import (
	"github.com/egoist/mygo/ui"
	"mygo-player/internal/catalog"
	"mygo-player/internal/plugins"
	"testing"
)

func navigationTestApp(t *testing.T) *app {
	t.Helper()
	client, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(client, nil)
	t.Cleanup(a.cancel)
	attachUITestPlugin(t, a, plugins.Manifest{ID: "library-source", Name: "测试来源", Version: "1", Capabilities: []string{"detail", "resolve", "search"}})
	a.page = "收藏"
	return a
}

// Back and forward must restore library pages, and a new visit discards forward history.
func TestLibraryNavigationBackForward(t *testing.T) {
	a := navigationTestApp(t)
	tt := ui.NewTester(a.view, 1280, 820)
	if err := tt.Click("历史"); err != nil {
		t.Fatal(err)
	}
	if a.page != "历史" || len(a.navHistory) != 1 {
		t.Fatal("visit did not enter history")
	}
	if err := tt.Click("返回"); err != nil {
		t.Fatal(err)
	}
	if a.page != "收藏" || len(a.navForward) != 1 {
		t.Fatal("back did not restore favorites")
	}
	tt.Key(ui.Cmd, ui.KeyBracketRight)
	if a.page != "历史" || len(a.navForward) != 0 {
		t.Fatal("keyboard forward did not restore history")
	}
	tt.Key(ui.Cmd, ui.KeyBracketLeft)
	if err := tt.Click("插件"); err != nil {
		t.Fatal(err)
	}
	if a.page != "插件" || len(a.navForward) != 0 {
		t.Fatal("new visit kept stale forward pages")
	}
}

// The source menu offers management even when every source has been turned off.
func TestSourceMenuOpensManagement(t *testing.T) {
	a := navigationTestApp(t)
	pm, err := plugins.LoadManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer pm.Close()
	a.pluginManager = pm
	a.source = ""
	a.pluginClients = map[string]*plugins.Client{}
	tt := ui.NewTester(a.view, 1280, 820)
	if err := tt.Click("选择来源"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("管理插件…"); err != nil {
		t.Fatal(err)
	}
	if a.page != "插件" || a.busy {
		t.Fatal("source menu did not open management locally")
	}
}

// Cmd+F focuses search even after the sidebar was hidden.
func TestSearchShortcutRestoresSidebar(t *testing.T) {
	for _, width := range []int{900, 1280} {
		a := navigationTestApp(t)
		a.sidebarCollapsed = true
		tt := ui.NewTester(a.view, width, 820)
		for _, label := range []string{"选择来源", "搜索短剧", "刷新"} {
			r, ok := tt.Find(label)
			if !ok || r.X < 0 || r.X+r.W > float32(width) {
				t.Fatalf("collapsed header control %q outside %dpx window: %+v", label, width, r)
			}
		}
		tt.Key(ui.Cmd, ui.KeyF)
		if a.sidebarCollapsed || !tt.Focused("搜索短剧") {
			t.Fatal("search shortcut did not focus the sidebar field")
		}
	}
}

// Editing the search field must not rewrite the query stored in page history.
func TestSearchHistoryUsesSubmittedQuery(t *testing.T) {
	a := navigationTestApp(t)
	a.page = "搜索"
	a.searchedQuery = "都市"
	a.query = "正在输入"
	a.navigate("收藏", "")
	if len(a.navHistory) != 1 || a.navHistory[0].Query != "都市" {
		t.Fatal("typing changed previous search route")
	}
}

// Back closes the video stage and then the detail panel before leaving the page.
func TestBackClosesVideoBeforeNavigation(t *testing.T) {
	a := navigationTestApp(t)
	a.showPlayer = true
	a.fullscreenVideo = true
	a.goBack()
	if a.fullscreenVideo || !a.showPlayer {
		t.Fatal("back skipped fullscreen")
	}
	a.selected = catalog.Series{ID: "one"}
	a.goBack()
	if a.showPlayer || a.selected.ID == "" {
		t.Fatal("back skipped the video stage")
	}
	a.goBack()
	if a.selected.ID != "" || a.page != "收藏" {
		t.Fatal("back skipped the detail panel")
	}
}
