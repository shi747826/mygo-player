package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"mygo-player/internal/catalog"
)

// 品牌文字不可点击；普通工具按钮仍使用手型指针。
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
	// 左上角标识约在 (20..54, 20..54)。
	tt.Move(37, 37)
	if got := tt.Cursor(); got != ui.CursorDefault {
		t.Fatalf("brand cursor = %v, want default arrow", got)
	}
	// 普通按钮仍应显示手型，确认没有影响其它控件。
	r, ok := tt.Find("刷新")
	if !ok {
		t.Fatal("refresh button missing")
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	if got := tt.Cursor(); got != ui.CursorPointer {
		t.Fatalf("button cursor = %v, want pointer", got)
	}
}
