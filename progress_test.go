package main

import (
	"testing"

	"mygo-player/internal/catalog"
)

func TestGridColumnsThinWithWidth(t *testing.T) {
	cases := map[float32]int{
		1200: 4, 1400: 4, 1000: 3, 900: 3, 700: 2, 600: 2, 500: 1, 300: 1,
		// 更宽的窗口继续加列，卡片尺寸保持稳定。
		1504: 5, 1808: 6, 2112: 7, 2416: 8, 4000: 10, 6000: 10,
	}
	for w, want := range cases {
		if got := gridColumns(w); got != want {
			t.Fatalf("gridColumns(%v)=%d want %d", w, got, want)
		}
	}
}
func TestCardHeightGrowsWithColumnWidth(t *testing.T) {
	// 列表封面用固定比例，卡片高度只随列宽变化，与图片何时加载无关。
	narrow := cardHeight(180)
	wide := cardHeight(320)
	if wide <= narrow || narrow <= 0 {
		t.Fatalf("card height must grow with width: narrow=%v wide=%v", narrow, wide)
	}
}
func TestSeasonLabel(t *testing.T) {
	if seasonLabel(catalogSeries(2, "")) != "第 2 季" {
		t.Fatal("numbered season")
	}
	if seasonLabel(catalogSeries(0, "特别篇")) != "特别篇" {
		t.Fatal("named season")
	}
	if seasonLabel(catalogSeries(0, "")) != "第 1 季" {
		t.Fatal("fallback season")
	}
}
func catalogSeries(season int, title string) catalog.Series {
	return catalog.Series{ID: "x", Title: title, Season: season}
}

func TestSeekRequiresAcknowledgement(t *testing.T) {
	if seekAcknowledged(.3, 120) {
		t.Fatal("old playback state acknowledged pending resume")
	}
	if !seekAcknowledged(120.5, 120) {
		t.Fatal("confirmed seek was ignored")
	}
}
func TestPendingSeekIsEffectivePosition(t *testing.T) {
	a := &app{resumePending: true, resume: 120}
	if a.effectivePosition() != 120 {
		t.Fatal("lost original resume target before player state arrived")
	}
	a.resume = 10
	if a.effectivePosition() != 10 {
		t.Fatal("manual seek target was replaced by cached position")
	}
	a.resumePending = false
	if a.effectivePosition() != 0 {
		t.Fatal("missing player should return zero")
	}
}
