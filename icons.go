package main

import (
	_ "embed"
	"sync"

	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"
)

// sidebarIconData is a reduced copy of the user's application artwork.
//
//go:embed packaging/sidebar-icon.png
var sidebarIconData []byte

var (
	markOnce  sync.Once
	markImage ui.ImageSource
)

// appMark retains the user's artwork and colors, without extra chrome.
func appMark(c *ui.Context, size float32) {
	if size <= 0 {
		return
	}
	markOnce.Do(func() { markImage, _ = ui.DecodeImage(sidebarIconData) })
	if markImage != nil {
		ui.Image(c, markImage).Size(size, size).Fit(ui.Contain).Shrink(0).Label("MyGo Player")
		return
	}
	// Keep the app identifiable if the embedded image ever becomes invalid.
	ui.Box(c).Size(size, size).Shrink(0).Center().Children(func() { ui.Text(c, "MG").FontSize(size / 3).TextColor(c.Theme().Text) })
}

// symbol uses the system artwork when available; its independently drawn SVG
// fallback covers other platforms and symbols absent from older macOS versions.
func symbol(c *ui.Context, name string, size float32, tint ui.Color) {
	if size <= 0 {
		return
	}
	fallback := symbolFallbacks[name]
	if fallback == nil {
		fallback = symbolFallbacks["circle"]
	}
	sfsymbols.Symbol(c, name).Size(size).Color(tint).Weight(sfsymbols.Regular).Fallback(fallback)
}

// These simple geometric fallback drawings are local assets, not copies of
// Apple's SF Symbols. They share a 24-unit box and inherit the requested tint.
var symbolFallbacks = func() map[string]*ui.SVG {
	outlines := map[string]string{
		"play.rectangle.fill":                `<rect x="3" y="5" width="18" height="14" rx="3"/><path d="m10 9 5 3-5 3Z"/>`,
		"chart.bar.xaxis":                    `<path d="M3 20h18M5 16v-5h3v5M11 16V7h3v9M17 16V3h3v13"/>`,
		"heart":                              `<path d="M20 5a5 5 0 0 0-8 1 5 5 0 0 0-8-1c-4 4 1 9 8 15 7-6 12-11 8-15Z"/>`,
		"clock.arrow.circlepath":             `<path d="M3 10a9 9 0 1 1 1 7M3 5v5h5M12 7v5l3 2"/>`,
		"magnifyingglass":                    `<circle cx="10.5" cy="10.5" r="6.5"/><path d="m16 16 5 5"/>`,
		"arrow.clockwise":                    `<path d="M20 9a8 8 0 1 0 0 7M20 4v5h-5"/>`,
		"xmark":                              `<path d="m6 6 12 12M18 6 6 18"/>`,
		"sidebar.right":                      `<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M15 4v16m3-12h1m-1 4h1m-1 4h1"/>`,
		"arrow.up.left.and.arrow.down.right": `<path d="M4 10V4h6M4 4l6 6m10 4v6h-6m6 0-6-6"/>`,
		"arrow.down.right.and.arrow.up.left": `<path d="M10 4v6H4m6 0L4 4m10 16v-6h6m-6 0 6 6"/>`,
		"rectangle.portrait":                 `<rect x="6" y="2.5" width="12" height="19" rx="2"/>`,
		"rectangle":                          `<rect x="2.5" y="5" width="19" height="14" rx="2"/>`,
		"pin":                                `<path d="m9 3 8 3-2 5 2 4-1 1-5-1-4 3-1-1 1-5 2-9Zm1 12-5 6"/>`,
		"speaker.wave.2":                     `<path d="M4 9h4l5-4v14l-5-4H4ZM16 9a5 5 0 0 1 0 6m3-9a9 9 0 0 1 0 12"/>`,
		"film":                               `<rect x="3" y="3" width="18" height="18" rx="2"/><path d="M7 3v18M17 3v18M7 12h10M3 8h4m10 0h4M3 16h4m10 0h4"/>`,
		"chevron.right":                      `<path d="m9 5 7 7-7 7"/>`,
		"chevron.left":                       `<path d="m15 5-7 7 7 7"/>`,
		"checkmark":                          `<path d="m5 12 4 4L19 6"/>`,
		"lock.shield":                        `<path d="m12 3 8 3v6c0 4-4 7-8 9-4-2-8-5-8-9V6Zm-3 8V9a3 3 0 0 1 6 0v2"/><rect x="8" y="11" width="8" height="6" rx="1"/>`,
		"circle":                             `<circle cx="12" cy="12" r="8"/>`,
		"ellipsis":                           `<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>`,
		"list.bullet":                        `<path d="M9 6h12M9 12h12M9 18h12M3 6h1M3 12h1M3 18h1"/>`,
	}
	filled := map[string]string{
		"play.fill":         `<path d="M7 4v16l13-8Z"/>`,
		"pause.fill":        `<rect x="6" y="4" width="4" height="16" rx="1"/><rect x="14" y="4" width="4" height="16" rx="1"/>`,
		"backward.end.fill": `<path d="M5 4h3v16H5Zm14 0v16L9 12Z"/>`,
		"forward.end.fill":  `<path d="M16 4h3v16h-3ZM5 4l10 8-10 8Z"/>`,
		"stop.fill":         `<rect x="5" y="5" width="14" height="14" rx="2"/>`,
	}
	result := make(map[string]*ui.SVG, len(outlines)+len(filled)+5)
	for name, body := range outlines {
		result[name] = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">` + body + `</svg>`))
	}
	for name, body := range filled {
		result[name] = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor">` + body + `</svg>`))
	}
	result["heart.fill"] = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor">` + outlines["heart"] + `</svg>`))
	result["pin.fill"] = result["pin"]
	result["speaker.wave.2.fill"] = result["speaker.wave.2"]
	result["clock"] = result["clock.arrow.circlepath"]
	result["play.rectangle"] = result["play.rectangle.fill"]
	return result
}()
