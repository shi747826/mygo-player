package main

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
	"mygo-player/internal/catalog"
)

// The virtual scroller must use the same row height for short/long titles,
// watched items and unknown episode counts, or it introduces gaps on scroll.
func TestCardHeightMatchesVirtualGrid(t *testing.T) {
	for _, width := range []float32{180, 240} {
		for _, watched := range []bool{false, true} {
			client, err := catalog.New(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			a := newApp(client, nil)
			t.Cleanup(a.cancel)
			for _, s := range []catalog.Series{{ID: "short", Title: "普通剧集标题"}, {ID: "long", Title: "一个非常非常长的剧集名称应该在一行以内显示并在末尾使用省略号", Episodes: 1000}} {
				if watched {
					a.library.History[s.ID] = catalog.Progress{Series: s, Episode: catalog.Episode{Number: 3}, Position: 26}
				}
				tt := ui.NewTester(func(c *ui.Context) {
					c.SetTheme(palette)
					ui.Column(c).Width(width).Label("卡片范围").Children(func() { a.seriesCard(c, s) })
				}, int(width), 600)
				cover, ok := tt.Find("播放 " + s.Title)
				if !ok {
					t.Fatal("missing cover")
				}
				title, ok := tt.Find(s.Title)
				if !ok || title.H > 20 || title.Y-cover.Y-cover.H > 12 {
					t.Fatalf("title adds blank lines: %+v", title)
				}
				card, ok := tt.Find("卡片范围")
				if !ok || math.Abs(float64(card.H-cardHeight(cover.W))) > .1 {
					t.Fatalf("virtual height mismatch: cover=%+v, card=%+v, expected=%v", cover, card, cardHeight(cover.W))
				}
				badge, hasBadge := tt.Find("总集数")
				if hasBadge != (s.Episodes > 0) {
					t.Fatal("incorrect total episode badge")
				}
				if hasBadge && (badge.X < cover.X || badge.Y < cover.Y || badge.Y+badge.H > cover.Y+40) {
					t.Fatalf("episode count must be inside cover top-left: %+v", badge)
				}
				progress, hasProgress := tt.Find("观看进度")
				if hasProgress != watched {
					t.Fatal("watch progress lost")
				}
				if hasProgress && (progress.Y < cover.Y || progress.Y+progress.H > cover.Y+cover.H) {
					t.Fatalf("progress must stay inside cover: %+v", progress)
				}
			}
		}
	}
}
