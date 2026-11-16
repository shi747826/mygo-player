package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"mygo-player/internal/catalog"
)

// 宽窗口应当真正铺满更多列，而不是固定 4 列留白。
func TestWideWindowUsesMoreColumns(t *testing.T) {
	client, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(client, nil)
	defer a.cancel()
	a.items = seriesList(60)
	amounts := map[[2]int]int{}
	_ = amounts
	ui.NewTester(a.view, 1280, 820)
	normal := a.layoutCols
	ui.NewTester(a.view, 2200, 900)
	wide := a.layoutCols
	if wide <= normal {
		t.Fatalf("wide window kept %d columns (normal %d)", wide, normal)
	}
	if wide != 7 {
		t.Fatalf("2200px should fit 7 columns, got %d", wide)
	}
	// 每个条目仍然只属于一列，且列号不越界。
	for i, col := range a.layout {
		if i >= 60 {
			break
		}
		if col < 0 || col >= a.layoutCols {
			t.Fatalf("item %d column %d out of range", i, col)
		}
	}
}
