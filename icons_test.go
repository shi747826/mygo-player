package main

import (
	"image/color"
	"testing"
)

func TestDesktopSymbolFallbacks(t *testing.T) {
	// Cover navigation and controls, including states chosen at runtime.
	names := []string{
		"play.rectangle", "play.circle.fill", "xmark", "heart.fill",
		"checkmark", "film", "chevron.down", "xmark.circle.fill", "sidebar.left",
		"play.fill", "pause.fill", "arrow.up.left.and.arrow.down.right",
	}
	for _, page := range []string{"推荐", "榜单", "收藏", "历史", "账号", "插件", "搜索", "预约", "other-plugin-page"} {
		names = append(names, navIcon(page))
	}
	if got := navIcon("other-plugin-page"); got != "list.bullet" {
		t.Errorf("custom plugin page icon = %q, want list.bullet", got)
	}

	tint := color.RGBA{G: 255, A: 255}
	seen := make(map[string]string)
	for _, name := range names {
		svg := symbolFallbacks[name]
		if svg == nil {
			t.Errorf("%s has no SVG fallback", name)
			continue
		}
		icon := svg.Image(17, 17, tint)
		visible := false
		for pixel := range 17 * 17 {
			c := icon.RGBAAt(pixel%17, pixel/17)
			if c.A != 0 {
				visible = true
				if c.R != 0 || c.G == 0 || c.B != 0 {
					t.Errorf("%s pixel ignored the requested tint: %v", name, c)
					break
				}
			}
		}
		if !visible {
			t.Errorf("%s fallback rendered blank", name)
		}
		pixels := string(icon.Pix)
		if other, exists := seen[pixels]; exists {
			t.Errorf("%s and %s have identical fallback artwork", name, other)
		}
		seen[pixels] = name
	}
}

func TestFilledControlFallbacksKeepCenterCutouts(t *testing.T) {
	for _, name := range []string{"play.circle.fill", "play.rectangle.fill", "xmark.circle.fill"} {
		svg := symbolFallbacks[name]
		if svg == nil {
			t.Errorf("%s has no SVG fallback", name)
			continue
		}
		icon := svg.Image(48, 48, nil)
		if center, body := icon.RGBAAt(24, 24).A, icon.RGBAAt(12, 24).A; center != 0 || body != 255 {
			t.Errorf("%s fallback center alpha = %d and body alpha = %d, want 0 and 255", name, center, body)
		}
	}
}
