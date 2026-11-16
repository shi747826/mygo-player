package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"mygo-player/internal/catalog"
	"mygo-player/internal/plugins"
)

// 推荐页按插件清单显示排序或频道选项。
func TestSortRowFollowsPluginManifest(t *testing.T) {
	client, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(client, nil)
	defer a.cancel()
	a.page = "推荐"
	a.items = seriesList(4)

	attachSortTestSource(t, a)
	tt := ui.NewTester(a.view, 1280, 820)
	for _, label := range []string{"热门", "最近更新", "当前热播"} {
		if !tt.HasText(label) {
			t.Fatalf("sorted source recommendation missing sort option %q", label)
		}
	}
	for _, label := range []string{"精选", "真人剧", "漫剧", "找剧"} {
		if tt.HasText(label) {
			t.Fatalf("sorted source should hide channel %q", label)
		}
	}

	attachUITestPlugin(t, a, plugins.Manifest{ID: "channels-source", Name: "频道来源", Version: "1", Capabilities: []string{"detail", "resolve"}, Filters: []plugins.Filter{{Label: "频道", Value: "channel", Options: []plugins.Option{{Label: "精选", Value: ""}}}}})
	tt = ui.NewTester(a.view, 1280, 820)
	if !tt.HasText("精选") || tt.HasText("当前热播") {
		t.Fatal("channel source should keep channels and hide sort options")
	}
}

// 切换排序会清空列表并重新从第一页加载。
func TestSwitchingSortReloadsList(t *testing.T) {
	client, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(client, nil)
	defer a.cancel()
	attachSortTestSource(t, a)
	a.page = "推荐"
	a.items = seriesList(8)
	a.cursor = "3"
	tt := ui.NewTester(a.view, 1280, 820)
	if err := tt.Click("最近更新"); err != nil {
		t.Fatal(err)
	}
	if a.hgSort != "new" || a.hgSortSel != 1 {
		t.Fatalf("sort not applied: %q sel=%d", a.hgSort, a.hgSortSel)
	}
	if len(a.items) != 0 || a.cursor != "" {
		t.Fatal("switching sort must reload from the first page")
	}
	a.listCancel()
}

func attachSortTestSource(t *testing.T, a *app) {
	attachUITestPlugin(t, a, plugins.Manifest{ID: "sorted-source", Name: "排序来源", Version: "1", Capabilities: []string{"detail", "resolve"}, Filters: []plugins.Filter{{Label: "排序", Value: "sort", Options: []plugins.Option{{Label: "热门", Value: ""}, {Label: "最近更新", Value: "new"}, {Label: "当前热播", Value: "hot_window"}}}}})
}
