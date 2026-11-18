package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"mygo-player/internal/catalog"
)

// 条目很多时也只渲染可见行，控件数量不随总量增长。
func TestGridRendersOnlyVisibleRows(t *testing.T) {
	client, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(client, nil)
	defer a.cancel()
	a.items = seriesList(660)
	ui.NewTester(a.view, 1280, 820)
	small := a.renderedCards
	if small <= 0 {
		t.Fatal("no cards rendered")
	}
	if small > 120 {
		t.Fatalf("rendered %d cards for 660 items; expected a bounded window", small)
	}

	a.items = seriesList(2000)
	ui.NewTester(a.view, 1280, 820)
	if a.renderedCards > small*2 {
		t.Fatalf("rendered cards grew with list size: %d -> %d", small, a.renderedCards)
	}
}

// 滚动到列表末尾时渲染的应当是末尾的条目。
func TestGridRendersTailAtBottom(t *testing.T) {
	client, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(client, nil)
	defer a.cancel()
	a.items = seriesList(660)
	ui.NewTester(a.view, 1280, 820)
	// 用内容高度减去一屏，跳到末尾。
	a.listScroll.Y = a.gridContent
	tt := ui.NewTester(a.view, 1280, 820)
	if !tt.HasText("剧集660") {
		t.Fatalf("tail item not rendered at bottom: scroll=%v content=%v width=%v cols=%d rendered=%d", a.listScroll.Y, a.gridContent, a.gridWidth, a.layoutCols, a.renderedCards)
	}
}

// 视口高度由内容高与 MaxY 反推，占位高度必须撑起完整滚动范围。
func TestGridSpacersPreserveScrollRange(t *testing.T) {
	client, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(client, nil)
	defer a.cancel()
	a.items = seriesList(400)
	ui.NewTester(a.view, 1280, 820)
	if a.gridContent <= 0 {
		t.Fatal("content height not tracked")
	}
	// 每行高度一致时，内容高约等于行数 × 行高。
	rowH := a.gridContent / float32((len(a.items)+a.layoutCols-1)/a.layoutCols)
	if rowH < 100 {
		t.Fatalf("row height looks wrong: %v", rowH)
	}
}
