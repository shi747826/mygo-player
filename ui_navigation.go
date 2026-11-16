// SPDX-License-Identifier: AGPL-3.0-only
// Navigation adapted from egoist/mygo-music, internal/app/ui_main.go.
// Copyright mygo-music contributors. Modified for MyGo Player (2026).
package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"mygo-player/internal/catalog"
)

type pageRoute struct {
	Source, Page, Query, Channel, Sort string
	ChannelSel, SortSel                int
}

func (a *app) currentRoute() pageRoute {
	query := ""
	if a.page == "搜索" {
		query = a.searchedQuery
	}
	return pageRoute{a.source, a.page, query, a.channel, a.hgSort, a.channelSel, a.hgSortSel}
}
func (a *app) rememberRoute(next pageRoute) {
	if next == a.currentRoute() {
		return
	}
	a.navHistory = append(a.navHistory, a.currentRoute())
	if len(a.navHistory) > 80 {
		a.navHistory = a.navHistory[len(a.navHistory)-80:]
	}
	a.navForward = nil
}
func (a *app) navigate(page, query string) {
	next := a.currentRoute()
	next.Page = page
	next.Query = query
	a.rememberRoute(next)
	a.query = query
	a.selected = catalog.Series{}
	a.episodes = nil
	a.seasons = nil
	a.loadPage(page, false)
}
func (a *app) showRoute(r pageRoute) {
	if r.Source != "" && r.Source != a.source {
		if a.pluginManager == nil || !a.pluginManager.Enabled(r.Source) || a.pluginClients[r.Source] == nil {
			r = pageRoute{Source: a.source, Page: "推荐"}
		} else {
			a.switchPlugin(r.Source)
		}
	}
	a.query = r.Query
	a.channel = r.Channel
	a.hgSort = r.Sort
	a.channelSel = r.ChannelSel
	a.hgSortSel = r.SortSel
	a.selected = catalog.Series{}
	a.episodes = nil
	a.seasons = nil
	a.loadPage(r.Page, false)
}
func (a *app) goBack() {
	if a.fullscreenVideo || a.windowFullscreen {
		a.exitFullscreen()
		return
	}
	if a.showPlayer {
		a.showPlayer = false
		a.setPortrait(false)
		return
	}
	if a.selected.ID != "" {
		if a.detailCancel != nil {
			a.detailCancel()
		}
		a.detailID++
		a.detailBusy = false
		a.selected = catalog.Series{}
		return
	}
	if len(a.navHistory) == 0 {
		return
	}
	a.navForward = append(a.navForward, a.currentRoute())
	r := a.navHistory[len(a.navHistory)-1]
	a.navHistory = a.navHistory[:len(a.navHistory)-1]
	a.showRoute(r)
}
func (a *app) goForward() {
	if len(a.navForward) == 0 {
		return
	}
	a.navHistory = append(a.navHistory, a.currentRoute())
	r := a.navForward[len(a.navForward)-1]
	a.navForward = a.navForward[:len(a.navForward)-1]
	a.showRoute(r)
}
func (a *app) submitSearch() {
	q := strings.TrimSpace(a.query)
	if q == "" {
		return
	}
	a.navigate("搜索", q)
}
func (a *app) clearSearch() {
	a.query = ""
	if a.page == "搜索" {
		a.navigate("推荐", "")
	}
	a.searchedQuery = ""
}
func (a *app) shellShortcuts(c *ui.Context) {
	c.OnShortcut(0, ui.KeyBack, a.goBack)
	c.OnShortcut(0, ui.KeyForward, a.goForward)
	c.OnShortcut(ui.Cmd, ui.KeyBracketLeft, a.goBack)
	c.OnShortcut(ui.Cmd, ui.KeyBracketRight, a.goForward)
	c.OnShortcut(ui.Cmd, ui.KeyF, func() { a.sidebarCollapsed = false; a.searchBox.Focus(c) })
	if !c.Focused(a.searchBox) {
		c.OnShortcut(0, ui.KeySpace, func() {
			if a.player != nil {
				if a.player.Playing() {
					a.player.Pause()
				} else {
					a.player.Play()
				}
			}
		})
	}
	c.OnShortcut(0, ui.KeyEscape, func() {
		if a.fullscreenVideo {
			a.exitFullscreen()
		} else {
			a.goBack()
		}
	})
}

func (a *app) chooseSource(id string) {
	if a.pluginManager == nil || id == a.source || !a.pluginManager.Enabled(id) || a.pluginClients[id] == nil {
		return
	}
	a.rememberRoute(pageRoute{Source: id, Page: "推荐"})
	a.switchPlugin(id)
}
