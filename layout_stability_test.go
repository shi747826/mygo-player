package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"mygo-player/internal/catalog"
)

func seriesList(n int) []catalog.Series {
	out := make([]catalog.Series, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, catalog.Series{ID: "s" + itoa(i+1), Title: "剧集" + itoa(i+1), Episodes: i + 1})
	}
	return out
}

// 追加下一页时，已经出现的卡片必须留在原来的列和顺序上。
func TestAppendKeepsExistingColumns(t *testing.T) {
	client, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(client, nil)
	defer a.cancel()
	a.page = "推荐"
	a.items = seriesList(24)
	tt := ui.NewTester(a.view, 1280, 820)
	snapshot := append([]int{}, a.layout...)
	if len(snapshot) != 24 {
		t.Fatalf("layout len=%d", len(snapshot))
	}
	_ = tt
	a.items = seriesList(72)
	ui.NewTester(a.view, 1280, 820)
	if len(a.layout) != 72 {
		t.Fatalf("layout after append=%d", len(a.layout))
	}
	for i, col := range snapshot {
		if a.layout[i] != col {
			t.Fatalf("card %d moved from column %d to %d after append", i, col, a.layout[i])
		}
	}
}

// 列数变化时才允许整体重排，且新布局必须覆盖全部条目。
func TestColumnCountChangeRebuildsLayout(t *testing.T) {
	client, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(client, nil)
	defer a.cancel()
	a.items = seriesList(30)
	ui.NewTester(a.view, 1280, 820)
	wide := a.layoutCols
	ui.NewTester(a.view, 700, 820)
	if a.layoutCols == wide {
		t.Fatalf("column count did not change with width (still %d)", a.layoutCols)
	}
	if len(a.layout) != 30 {
		t.Fatalf("layout len=%d after width change", len(a.layout))
	}
	for _, col := range a.layout {
		if col < 0 || col >= a.layoutCols {
			t.Fatalf("column %d out of range %d", col, a.layoutCols)
		}
	}
}

// 换列表（新 listID）时布局缓存必须失效，否则会沿用上一组的列分配。
func TestNewListResetsLayout(t *testing.T) {
	client, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(client, nil)
	defer a.cancel()
	a.items = seriesList(8)
	ui.NewTester(a.view, 1280, 820)
	a.listID++
	a.items = seriesList(3)
	ui.NewTester(a.view, 1280, 820)
	if len(a.layout) != 3 {
		t.Fatalf("layout len=%d after new list", len(a.layout))
	}
}

// 封面缓存超上限时只淘汰最久未使用的，不是整表清空。
func TestCoverCacheEvictsLeastRecentlyUsed(t *testing.T) {
	client, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(client, nil)
	defer a.cancel()
	for i := 0; i < coverCacheLimit; i++ {
		raw := "cover-" + itoa(i)
		a.covers[raw] = nil
		a.touchCover(raw)
	}
	// 最旧的一张被重新访问后应当保留下来。
	oldest := "cover-0"
	a.touchCover(oldest)
	a.trimCovers()
	if len(a.covers) != coverCacheLimit-coverCacheLimit/4 {
		t.Fatalf("cache size after trim = %d", len(a.covers))
	}
	if _, ok := a.covers[oldest]; !ok {
		t.Fatal("recently used cover must survive eviction")
	}
	if _, ok := a.covers["cover-1"]; ok {
		t.Fatal("least recently used cover should be evicted")
	}
	if len(a.coverUsed) != len(a.covers) {
		t.Fatal("usage bookkeeping leaked entries")
	}
}
