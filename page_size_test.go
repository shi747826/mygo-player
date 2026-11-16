package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"mygo-player/internal/catalog"
)

// 每页条数应随列数变化：一页约 5 行。
func TestPageSizeFollowsColumnCount(t *testing.T) {
	client, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(client, nil)
	defer a.cancel()
	cases := []struct {
		width float32
		cols  int
		want  int
	}{
		{700, 2, 10},
		{1000, 3, 15},
		{1280, 4, 20},
		{1808, 6, 30},
		{2416, 8, 40},
		{4000, 10, 50}, // 站点上限 50
	}
	for _, tc := range cases {
		a.items = seriesList(6)
		ui.NewTester(a.view, int(tc.width), 820)
		if a.layoutCols != tc.cols {
			t.Fatalf("width %v gave %d columns want %d", tc.width, a.layoutCols, tc.cols)
		}
		if got := a.targetPageSize(); got != tc.want {
			t.Fatalf("width %v size=%d want %d", tc.width, got, tc.want)
		}
	}
}

// 列表开始加载时锁定页大小；窗口在列表中途变宽不应改变后续页的尺寸，
// 否则同一页会重复或漏项。
func TestPageSizeLockedForList(t *testing.T) {
	client, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(client, nil)
	defer a.cancel()
	a.items = seriesList(6)
	ui.NewTester(a.view, 1280, 820)
	a.loadPage("推荐", false)
	if a.pageSize != 20 {
		t.Fatalf("initial page size = %d want 20", a.pageSize)
	}
	a.listCancel()
	// 窗口变宽，但未重新加载列表：页大小保持。
	ui.NewTester(a.view, 1808, 820)
	if a.pageSize != 20 {
		t.Fatalf("page size changed mid-list: %d", a.pageSize)
	}
	a.loadPage("推荐", false)
	if a.pageSize != 30 {
		t.Fatalf("reload should adopt new size: %d", a.pageSize)
	}
	a.listCancel()
}
