package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"mygo-player/internal/catalog"
)

// 应用品牌已移除；插件选择仍提供可点击的手型指针。
func TestBrandDoesNotShowPointerCursor(t *testing.T) {
	client, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(client, nil)
	defer a.cancel()
	a.page = "推荐"
	a.items = seriesList(4)
	tt := ui.NewTester(a.view, 1280, 820)
	if tt.HasText("MyGo Player") {
		t.Fatal("application brand must not appear in the sidebar")
	}
	// 侧栏空白仍使用默认指针。
	tt.Move(220, 44)
	if got := tt.Cursor(); got != ui.CursorDefault {
		t.Fatalf("brand cursor = %v, want default arrow", got)
	}
	// 普通按钮仍应显示手型，确认没有影响其它控件。
	r, ok := tt.Find("选择来源")
	if !ok {
		t.Fatal("source switcher missing")
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	if got := tt.Cursor(); got != ui.CursorPointer {
		t.Fatalf("button cursor = %v, want pointer", got)
	}
}
