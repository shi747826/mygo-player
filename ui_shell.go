// SPDX-License-Identifier: AGPL-3.0-only
// Adapted from egoist/mygo-music internal/app/ui_main.go, ui_suggest.go,
// and ui_kit.go. Copyright mygo-music contributors.
// Modified for MyGo Player (2026): video sources and desktop library.
package main

import (
	"github.com/egoist/mygo/ui"
	"time"
)

const musicSidebarWidth = 232

func shellIconButton(c *ui.Context, icon, label string) ui.Element {
	b := ui.ButtonBase(c).Size(34, 34).Radius(999).Center().Border(0, ui.Transparent).Label(label).Cursor(ui.CursorPointer)
	bg := ui.Transparent
	if b.Hovered() {
		bg = ui.RGBA(128, 128, 128, 0.14)
	}
	b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond}))
	pose := ui.Pose{}
	if b.Pressed() {
		pose.Scale = .88
	}
	b.AnimatePose(pose, ui.Snappy)
	return b.Children(func() { symbol(c, icon, 17, appleMuted) })
}
func (a *app) sourceSwitcher(c *ui.Context) {
	chosen := a.source
	sel := ui.SelectBase(c.Key("source-toggle"), &chosen)
	b := sel.Trigger.FillWidth().Height(30).PaddingX(8).Gap(8).Radius(7).AlignItems(ui.Center).FocusRing(false).Label("选择来源").Cursor(ui.CursorPointer)
	if b.Hovered() || sel.Open() {
		b.Background(ui.RGBA(0, 0, 0, .04))
	} else {
		b.Background(ui.Transparent)
	}
	b.Children(func() {
		ui.Text(c, a.sourceDisplayName()).FontSize(12.5).FontWeight(600).Grow(1).MinWidth(0).SingleLine()
		symbol(c, "chevron.down", 10, appleMuted)
	})
	sel.Popup(func(panel ui.Element) {
		panel.Margin(6, 0, 0, 0).Padding(5).Gap(1).MinWidth(204).Radius(12).Background(appleSurface).Border(.5, appleHairline).Shadow(0, 8, 24, -4, ui.RGBA(0, 0, 0, .2))
		if a.pluginManager != nil {
			for _, m := range a.pluginManager.Manifests() {
				if !a.pluginManager.Enabled(m.ID) {
					continue
				}
				item := sel.Item(m.ID).Label(m.Name).Height(30).PaddingX(8).Gap(8).Radius(7).AlignItems(ui.Center)
				if item.Highlighted() {
					item.Background(ui.RGBA(0, 0, 0, .08))
				}
				item.Children(func() {
					mark := ui.Box(c).Size(14, 14).Center()
					mark.Children(func() {
						if a.source == m.ID {
							symbol(c, "checkmark", 12, appleInk)
						}
					})
					ui.Text(c, m.Name).FontSize(13).SingleLine()
				})
			}
		} else {
			sel.Item(a.source).Label(a.brandName()).Height(30).PaddingX(8).Children(func() { ui.Text(c, a.brandName()).FontSize(13) })
		}
		ui.Box(c).FillWidth().Height(.5).Margin(4, 8).Background(appleHairline)
		sel.Item("__plugins").Label("管理插件…").Height(30).PaddingX(8).Radius(7).Children(func() { ui.Text(c, "管理插件…").FontSize(13).TextColor(appleMuted) })
	})
	if b.Changed() {
		if chosen == "__plugins" {
			a.navigate("插件", "")
		} else if chosen != a.source {
			a.chooseSource(chosen)
		}
	}
}
func (a *app) sourceDisplayName() string {
	if a.pluginManager != nil {
		for _, m := range a.pluginManager.Manifests() {
			if m.ID == a.source {
				return m.Name
			}
		}
		return "选择来源"
	}
	return "选择来源"
}
func (a *app) sidebarSearch(c *ui.Context) {
	field := ui.Row(c).Height(32).FillWidth().Radius(9).AlignItems(ui.Stretch).Margin(0, 0, 14, 0)
	if c.Focused(a.searchBox) {
		field.Background(appleSurface).Border(1, appleBlue.Alpha(.6)).Shadow(0, 0, 0, 3, appleBlue.Alpha(.15))
	} else {
		field.Background(ui.RGBA(0, 0, 0, .04)).Border(.5, appleHairline)
	}
	field.Children(func() {
		end := float32(10)
		if a.query != "" {
			end = 28
		}
		in := ui.TextInput(c.Key("search"), &a.query).Bind(&a.searchBox).Grow(1).MinWidth(0).PaddingStart(31).PaddingEnd(end).FontSize(13).Placeholder("搜索短剧").Label("搜索短剧").Border(0, ui.Transparent).Background(ui.Transparent)
		ui.Box(c).Absolute().Left(10).Top(9).Size(14, 14).IgnoreInput(true).Children(func() { symbol(c, "magnifyingglass", 14, appleMuted) })
		if a.query != "" {
			shellIconButton(c.Key("clear-search"), "xmark.circle.fill", "清除搜索").Size(24, 24).Absolute().Right(2).Top(3).OnClick(a.clearSearch)
		}
		if in.Submitted() {
			a.submitSearch()
		}
	})
}
func (a *app) musicSidebar(c *ui.Context) {
	ui.Column(c).Width(musicSidebarWidth).FillHeight().Padding(14).Gap(4).Background(appleCanvas).Children(func() {
		ui.Box(c).Height(max(float32(48), c.TitleBar().Height+8) - 14).Shrink(0)
		if a.sourceHas("search") {
			a.sidebarSearch(c)
		}
		ui.Scroll(c.Key("sidebar-nav")).Grow(1).FillWidth().Gap(4).Children(func() {
			ui.Text(c, "资料库").FontSize(11).FontWeight(600).TextColor(appleMuted).Padding(8, 10, 4, 10)
			if a.pluginManager != nil {
				for _, m := range a.pluginManager.Manifests() {
					if m.ID == a.source && a.pluginManager.Enabled(m.ID) {
						for _, n := range m.Navigation {
							if n.Value == "收藏" || n.Value == "历史" || n.Value == "预约" || n.Value == "账号" || n.Value == "插件" {
								continue
							}
							a.sidebarItem(c, n.Value, n.Label)
						}
					}
				}
			}
			if a.source != "" {
				a.sidebarItem(c, "收藏", "收藏")
				a.sidebarItem(c, "历史", "历史")
			}
			if a.sourceHas("reservations") {
				a.sidebarItem(c, "预约", "预约")
			}

			if a.searchedQuery != "" {
				a.sidebarItem(c, "搜索", "搜索结果")
			}
		})
		ui.Box(c).FillWidth().Height(.5).Margin(6, 8).Background(appleHairline)
		a.sidebarItem(c, "插件", "插件与来源")
		if a.sourceHas("account") {
			label := "账号"
			if a.accountLoggedIn {
				label = a.accountProfile.Display()
			}
			a.sidebarItem(c, "账号", label)
		}
	})
}

// Window controls remain native; this transparent drag region reserves their
// actual platform insets instead of adding a second application toolbar.
func (a *app) windowControls(c *ui.Context) {
	bar := c.TitleBar()
	height := max(float32(40), bar.Height)
	row := ui.Row(c.Key("window-controls")).Absolute().Top(0).Left(0).FillWidth().Height(height).AlignItems(ui.Center).PaddingStart(bar.Left + 8).PaddingEnd(bar.Right + 8).DragWindow()
	row.Children(func() {
		if bar.Right > 0 && bar.Left == 0 {
			ui.Spacer(c)
		}
		width := float32(144)
		if bar.Left > 0 {
			width = max(float32(96), musicSidebarWidth-bar.Left-20)
		}
		ui.Box(c).Width(width).Children(func() { a.sourceSwitcher(c) })
	})
}

// Page headings belong to the white content surface, without a toolbar or
// source subtitle between the window edge and the catalog.
func (a *app) musicHeader(c *ui.Context) {
	top := max(float32(52), c.TitleBar().Height+12)
	ui.Column(c).FillWidth().Padding(top, 28, 16, 28).Children(func() {
		title := a.page
		if a.page == "插件" {
			title = "插件与来源"
		}
		if a.page == "搜索" && a.searchedQuery != "" {
			title = "搜索结果"
		}
		ui.Text(c, title).FontSize(30).FontWeight(700).TextColor(appleInk)
	})
}
