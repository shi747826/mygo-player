package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"mygo-player/internal/catalog"
	"mygo-player/internal/webplayer"
)

func TestWindowFullscreenOnlyChangesLayout(t *testing.T) {
	client, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(client, nil)
	defer a.cancel()
	a.showPlayer = true
	a.preparing = true
	a.playMessage = "准备中"
	a.selected = catalog.Series{ID: "example", Title: "示例剧"}
	a.sidebarCollapsed = true
	handler := a.playerControlHandler(nil)
	handler(webplayer.ControlAction{Name: "windowFullscreen", Value: "true"})
	if !a.windowFullscreen || a.fullscreenVideo {
		t.Fatal("must enter app-window layout without system fullscreen")
	}
	if !a.playerControlConfig().WindowFullscreen {
		t.Fatal("control config did not reflect layout")
	}
	tt := ui.NewTester(a.view, 1280, 820)
	if tt.HasText("搜索短剧") || tt.HasText("剧集详情") || tt.HasText("MYGO PLAYER") {
		t.Fatal("window fullscreen must hide app chrome")
	}
	handler(webplayer.ControlAction{Name: "windowFullscreen", Value: "false"})
	if a.windowFullscreen || !a.sidebarCollapsed || a.selected.ID != "example" {
		t.Fatal("exit must preserve previous sidebar and detail state")
	}
	tt = ui.NewTester(a.view, 1280, 820)
	if !tt.HasText("剧集详情") {
		t.Fatal("detail must return on exit")
	}
	a.fullscreenVideo = true
	handler(webplayer.ControlAction{Name: "windowFullscreen", Value: "true"})
	if a.windowFullscreen {
		t.Fatal("window fullscreen button must not change layout while system fullscreen is active")
	}
}
