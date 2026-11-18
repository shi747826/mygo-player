package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"mygo-player/internal/catalog"
	"mygo-player/internal/webplayer"
)

func TestPlaybackTitlebarClearsNativeButtons(t *testing.T) {
	for _, bar := range []ui.TitleBar{{Height: 40, Left: 74}, {Height: 40, Right: 138}, {Height: 48, Left: 100, Right: 46}} {
		client, err := catalog.New(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		a := newApp(client, nil)
		t.Cleanup(a.cancel)
		a.windowFullscreen = true
		a.preparing = true
		a.playMessage = "正在准备视频"
		title := "一个很长的剧名，用来验证窗口标题不会盖住系统的关闭、最小化和放大按钮，以及播放集数"
		a.playingSeries = catalog.Series{ID: "series", Title: title}
		a.playingEpisode = catalog.Episode{ID: "episode", Number: 999}
		tt := ui.NewTester(a.view, 900, 620)
		tt.SetTitleBar(bar)
		header, ok := tt.Find("播放窗口标题栏")
		if !ok || header.Y != 0 || header.H != max(float32(40), bar.Height) {
			t.Fatalf("unexpected titlebar: %+v", header)
		}
		text, ok := tt.Find(title)
		if !ok || text.X < bar.Left || text.X+text.W > 900-bar.Right || text.Y+text.H > header.H {
			t.Fatalf("title overlaps window controls: %+v", text)
		}
		episode, ok := tt.Find("第 999 集")
		if !ok || episode.X < text.X+text.W || episode.X+episode.W > 900-bar.Right {
			t.Fatalf("episode overlaps title/native buttons: %+v", episode)
		}
		message, ok := tt.Find("正在准备视频")
		if !ok || message.Y < header.H {
			t.Fatalf("loading surface covers titlebar: %+v", message)
		}
	}
}

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
	a.playingSeries = a.selected
	a.playingEpisode = catalog.Episode{ID: "episode", Number: 3}
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
	if _, ok := tt.Find("播放窗口标题栏"); !ok || !tt.HasText("示例剧") || !tt.HasText("第 3 集") {
		t.Fatal("window playback needs a draggable series title outside the video")
	}
	// Dragging the native header gives focus back to the app; Escape must
	// still leave window playback, without relying on the WebView shortcut.
	tt.Key(0, ui.KeyEscape)
	if a.windowFullscreen || !a.sidebarCollapsed || a.selected.ID != "example" {
		t.Fatal("native Escape must restore previous app layout")
	}
	handler(webplayer.ControlAction{Name: "windowFullscreen", Value: "true"})
	handler(webplayer.ControlAction{Name: "windowFullscreen", Value: "false"})
	if a.windowFullscreen || !a.sidebarCollapsed || a.selected.ID != "example" {
		t.Fatal("exit must preserve previous sidebar and detail state")
	}
	tt = ui.NewTester(a.view, 1280, 820)
	if !tt.HasText("剧集详情") {
		t.Fatal("detail must return on exit")
	}
	a.fullscreenVideo = true
	tt = ui.NewTester(a.view, 1280, 820)
	if _, ok := tt.Find("播放窗口标题栏"); ok {
		t.Fatal("system fullscreen must use the full video surface")
	}
	handler(webplayer.ControlAction{Name: "windowFullscreen", Value: "true"})
	if a.windowFullscreen {
		t.Fatal("window fullscreen button must not change layout while system fullscreen is active")
	}
}
