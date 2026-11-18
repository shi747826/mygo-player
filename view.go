// SPDX-License-Identifier: AGPL-3.0-only
// UI palette and navigation adapted from egoist/mygo-music; see THIRD_PARTY_NOTICES.md.
package main

import (
	"fmt"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"mygo-player/internal/catalog"
)

// MyGo Music palette: white content, a quiet sidebar, and a red accent.
var palette = func() *ui.Theme {
	t := ui.LightTheme()
	t.Background = ui.Hex("#ffffff")
	t.Surface = ui.Hex("#ffffff")
	t.SurfaceHover = ui.Hex("#f0f0f2")
	t.SurfacePressed = ui.Hex("#e8e8ea")
	t.Border = ui.Hex("#d2d2d7")
	t.Text = ui.Hex("#1d1d1f")
	t.TextMuted = ui.Hex("#6e6e73")
	t.Accent = ui.Hex("#d42f45")
	t.AccentHover = ui.Hex("#be293e")
	t.AccentPressed = ui.Hex("#a92337")
	t.AccentText = ui.Hex("#ffffff")
	t.InactiveAccent = ui.Hex("#d42f45")
	t.Radius = 10
	t.Spacing = 4
	return t
}()

var (
	appleInk       = ui.Hex("#1d1d1f")
	appleMuted     = ui.Hex("#6e6e73")
	appleCanvas    = ui.Hex("#f4f4f6")
	appleSurface   = ui.Hex("#ffffff")
	appleBlue      = ui.Hex("#d42f45")
	appleBlueHov   = ui.Hex("#be293e")
	appleHairline  = ui.RGBA(0, 0, 0, .07)
	appleParchment = ui.Hex("#fafafc")
	videoSurface   = ui.Hex("#000000")
)

// button / primaryButton 统一给可点击控件加手型光标。
func button(c *ui.Context, label string) ui.Element {
	return ui.Button(c, label).Cursor(ui.CursorPointer)
}
func primaryButton(c *ui.Context, label string) ui.Element {
	return ui.PrimaryButton(c, label).Cursor(ui.CursorPointer)
}

func (a *app) view(c *ui.Context) {
	state := fmt.Sprintf("page=%s busy=%v items=%d detail=%v player=%v", a.page, a.busy, len(a.items), a.detailBusy, a.player != nil)
	if state != a.diagnosticState {
		a.diagnosticState = state
		trace("view %s", state)
	}
	c.SetTheme(palette)
	a.shellShortcuts(c)
	a.tick()
	playbackLayout := (a.fullscreenVideo || a.windowFullscreen) && (a.player != nil || a.preparing || a.playMessage != "")
	if a.win != nil && a.windowButtonsDark != playbackLayout {
		theme := mygo.ThemeLight
		if playbackLayout {
			theme = mygo.ThemeDark
		}
		a.win.SetWindowButtonsTheme(theme)
		a.windowButtonsDark = playbackLayout
	}
	if playbackLayout {
		a.fullscreenVideoView(c)
		return
	}
	ui.Row(c).Fill().Gap(0).Background(palette.Background).Children(func() {
		if !a.portrait && !a.sidebarCollapsed {
			a.sidebar(c)
		}
		ui.Column(c).Grow(1).FillHeight().MinWidth(0).Children(func() {
			if !a.portrait {
				a.header(c)
			}
			if a.showPlayer && (a.player != nil || a.preparing || a.playMessage != "") {
				a.playerView(c)
			} else {
				a.catalogView(c)
			}
			if !a.portrait {
				a.footer(c)
			}
		})
		if a.selected.ID != "" && !a.portrait {
			a.detailView(c)
		}
		if !a.portrait {
			a.windowControls(c)
		}
	})
}
func navIcon(name string) string {
	switch name {
	case "推荐":
		return "play.rectangle.fill"
	case "榜单":
		return "chart.bar.xaxis"
	case "收藏":
		return "heart"
	case "历史":
		return "clock.arrow.circlepath"
	case "账号":
		return "person.crop.circle"
	case "插件":
		return "puzzlepiece.extension"
	case "搜索":
		return "magnifyingglass"
	case "预约":
		return "calendar"
	}
	return "circle"
}
func navTextColor(active bool) ui.Color {
	if active {
		return appleBlue
	}
	return appleMuted
}
func (a *app) sidebar(c *ui.Context) { a.musicSidebar(c) }

func (a *app) sidebarItem(c *ui.Context, page, label string) {
	active := a.page == page
	item := ui.ButtonBase(c).Key(page).FillWidth().Height(34).Radius(8).PaddingX(12).Justify(ui.Start).AlignItems(ui.Center).Border(0, ui.Transparent).Cursor(ui.CursorPointer)
	switch {
	case active:
		item.Background(ui.RGBA(0, 0, 0, .08))
	case item.Hovered():
		item.Background(ui.RGBA(0, 0, 0, .04))
	default:
		item.Background(ui.Transparent)
	}
	item.Children(func() {
		ui.Row(c).FillWidth().MinWidth(0).AlignItems(ui.Center).Gap(10).Children(func() {
			symbol(c, navIcon(page), 16, navTextColor(active))
			weight := 400
			if active {
				weight = 600
			}
			ui.Text(c, label).FontSize(13.5).FontWeight(weight).TextColor(appleInk).MinWidth(0).MaxLines(1)
		})
	}).Label(page).OnClick(func() {
		query := ""
		if page == "搜索" {
			query = a.searchedQuery
		}
		a.navigate(page, query)
		a.portrait = false
	})
}

func (a *app) header(c *ui.Context) { a.musicHeader(c) }
func (a *app) catalogView(c *ui.Context) {
	if a.page == "插件" {
		a.pluginSettingsView(c)
		return
	}
	if a.page == "预约" && a.sourceHas("reservations") {
		a.reservationsView(c)
		return
	}
	if a.page == "账号" {
		a.accountView(c)
		return
	}
	ui.Column(c).Grow(1).FillWidth().Padding(0, 28, 12, 28).Gap(14).MinHeight(0).Children(func() {
		if a.page == "推荐" && a.pluginManager != nil {
			a.pluginFilters(c)
		}
		if a.busy {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c)
				ui.Text(c, "正在加载短剧…").TextColor(appleMuted)
			})
		}
		if a.message != "" {
			ui.Text(c, a.message).Wrap().TextColor(appleInk).Padding(12).Radius(8).Background(appleParchment).Border(1, appleHairline)
		}
		if !a.busy && len(a.items) == 0 && a.message == "" {
			ui.Column(c).Grow(1).Center().Gap(12).Children(func() {
				ui.Text(c, "这里还没有短剧").FontSize(24).FontWeight(600).TextColor(appleInk)
				ui.Text(c, "去推荐页发现故事，或搜索一部喜欢的剧。").TextColor(appleMuted)
			})
		}
		ui.Scroll(c.Key(a.page + "-list")).Grow(1).FillWidth().MinHeight(0).TrackScroll(&a.listScroll).Children(func() {
			a.masonryGrid(c)
			if a.hasMore {
				ui.Row(c).Margin(18, 0).AlignItems(ui.Center).Justify(ui.Center).Gap(8).Children(func() {
					ui.Spinner(c)
					ui.Text(c, "正在加载更多…").TextColor(appleMuted)
				})
			}
		})
		if a.hasMore && !a.busy && !a.prefetching {
			if a.listScroll.MaxY > 0 && a.listScroll.Y >= a.listScroll.MaxY-500 {
				a.prefetch()
			}
		}
	})
}

// gridColumns 按可用宽度决定列数：窄窗口逐级变少（4 → 3 → 2 → 1），
// 宽窗口继续增列。1200 宽时是 4 列，每列约 288，之后每多约 300 像素再加一列，
// 这样卡片尺寸基本不变，只靠加列利用多出来的宽度。
func gridColumns(width float32) int {
	switch {
	case width >= 1200:
		// (288 + 16 间距) 约等于一列占用的宽度。
		cols := 4 + int((width-1200)/304)
		if cols > 10 {
			cols = 10
		}
		return cols
	case width >= 900:
		return 3
	case width >= 600:
		return 2
	default:
		return 1
	}
}

// gridAspect 是列表里封面的固定宽高比。封面是异步加载的，如果让真实比例参与
// 布局，图片到达时列归属就会变、滚动位置会跳；因此列表一律按同一比例排版，
// 图片只在卡片内部裁切填充。
const gridAspect = 3.0 / 4.0

// loaderRowHeight 是底部「正在加载更多…」一行的高度估算，用于视口换算。
const loaderRowHeight = 54

// masonryGrid 只渲染可视区域内的行。
//
// 卡片高度统一（封面固定比例、标题固定高度），所以第 i 个条目落在第 i/cols 行、
// 第 i%cols 列。信息流可以无限追加，但每帧只为可见的若干行创建控件，上下用等高
// 占位撑开滚动范围；否则条目上千时每帧要建几千个控件，滚动会越来越卡。
func (a *app) masonryGrid(c *ui.Context) {
	w, _ := c.Size()
	cols := gridColumns(w)
	if cols < 1 {
		cols = 1
	}
	gap := float32(16)
	a.gridWidth = w
	colWidth := (w - gap*float32(cols-1)) / float32(cols)
	if colWidth < 160 {
		colWidth = 160
	}
	rowH := cardHeight(colWidth) + gap
	if a.layoutCols != cols || a.layoutListID != a.listID || len(a.layout) != len(a.items) {
		a.layout = make([]int, len(a.items))
		for i := range a.layout {
			a.layout[i] = i % cols
		}
		a.layoutCols = cols
		a.layoutListID = a.listID
	}
	rows := (len(a.items) + cols - 1) / cols
	// 内容总高包含底部的「正在加载更多」提示行。
	content := float32(rows) * rowH
	if a.hasMore {
		content += loaderRowHeight
	}
	a.gridContent = content
	// 视口高度由窗口高度决定（扣除标题与底部），始终有上界：
	// 不能由 MaxY 反推，否则列表刚变长时旧的 MaxY 会把视口算成整个内容高度。
	viewport := float32(800)
	if a.win != nil {
		if _, h := a.win.Size(); h > 0 {
			viewport = float32(h) - 160
		}
	}
	if viewport < 300 {
		viewport = 300
	}
	a.gridViewport = viewport
	first := int(a.listScroll.Y / rowH)
	if first < 0 {
		first = 0
	}
	first-- // 上方多留一行，快速滚动时不留空白
	if first < 0 {
		first = 0
	}
	// Include a partial viewport row as well as overscan above and below.
	visible := int(viewport/rowH) + 3
	last := first + visible
	if last > rows {
		last = rows
	}
	if first > last {
		first = last
	}
	if top := float32(first) * rowH; top > 0 {
		ui.Box(c).FillWidth().Height(top)
	}
	a.renderedCards = 0
	if last > first {
		ui.Row(c).Gap(gap).AlignItems(ui.Start).Children(func() {
			for j := 0; j < cols; j++ {
				var column []catalog.Series
				for r := first; r < last; r++ {
					if i := r*cols + j; i < len(a.items) {
						column = append(column, a.items[i])
					}
				}
				if len(column) == 0 {
					continue
				}
				ui.Column(c).Grow(1).Gap(gap).Children(func() {
					for _, s := range column {
						a.renderedCards++
						a.seriesCard(c, s)
					}
				})
			}
		})
	}
	if bottom := float32(rows-last) * rowH; bottom > 0 {
		ui.Box(c).FillWidth().Height(bottom)
	}
}

// cardHeight matches the fixed cover, single-line title and action row.
// Progress belongs to the cover so every grid row keeps the same height.
func cardHeight(colWidth float32) float32 {
	cover := colWidth / gridAspect
	return cover + 62
}
func (a *app) seriesCard(c *ui.Context, s catalog.Series) {
	ui.Column(c.Key(s.ID)).Padding(0).Gap(8).MinWidth(0).Children(func() {
		cover := ui.Box(c).FillWidth().AspectRatio(gridAspect).Radius(8).Background(appleCanvas).Center().Cursor(ui.CursorPointer).Label("播放 " + s.Title).OnClick(func() { a.selectSeries(s, true) })
		hovered := cover.Hovered()
		cover.Children(func() {
			if b := a.cover(s.Cover); b != nil {
				ui.Image(c, b).Fill().Fit(ui.Cover).Radius(8)
			} else {
				ui.Text(c, posterText(s.Title)).FontSize(26).FontWeight(600).TextColor(appleMuted)
			}
			if hovered {
				ui.Box(c).Absolute().Top(0).Bottom(0).Left(0).Right(0).Radius(8).Background(ui.RGBA(0, 0, 0, .18)).IgnoreInput(true).Center().Children(func() { symbol(c, "play.circle.fill", 44, ui.Hex("#ffffff")) })
			}
			if s.Episodes > 0 {
				ui.Box(c).Label("总集数").Absolute().Top(8).Left(8).Padding(3, 6).Radius(4).Background(ui.RGBA(0, 0, 0, .65)).IgnoreInput(true).Children(func() {
					ui.Text(c, fmt.Sprintf("%d 集", s.Episodes)).FontSize(11).FontWeight(500).TextColor(ui.Hex("#ffffff")).SingleLine()
				})
			}
			if p, ok := a.library.History[s.ID]; ok {
				ui.Box(c).Label("观看进度").Absolute().Bottom(8).Left(8).Right(8).Padding(4, 6).Radius(4).Background(ui.RGBA(0, 0, 0, .65)).IgnoreInput(true).Children(func() {
					ui.Text(c, fmt.Sprintf("看到第 %d 集 · %s", p.Episode.Number, clock(p.Position))).FontSize(11).TextColor(ui.Hex("#ffffff")).FillWidth().MinWidth(0).SingleLine()
				})
			}
		})
		ui.Column(c).FillWidth().MinWidth(0).Gap(4).Children(func() {
			ui.Text(c, s.Title).FontSize(14).FontWeight(500).FillWidth().MinWidth(0).SingleLine().Height(20)
			ui.Row(c).FillWidth().Height(30).Gap(6).AlignItems(ui.Center).Children(func() {
				button(c, "播放").Height(28).PaddingX(10).OnClick(func() { a.selectSeries(s, true) })
				button(c, "选集").Height(28).PaddingX(10).OnClick(func() { a.selectSeries(s, false) })
				ui.Spacer(c)
				fav := "heart"
				if _, ok := a.library.Favorites[s.ID]; ok {
					fav = "heart.fill"
				}
				shellIconButton(c, fav, "收藏 "+s.Title).Size(28, 28).OnClick(func() { a.toggleFavorite(s) })
			})
		})
	})
}
func posterText(s string) string {
	r := []rune(s)
	if len(r) > 4 {
		return string(r[:4])
	}
	return s
}

// coverAspect returns the cover's width/height ratio so 竖屏 and 横屏
// covers keep their natural shape in the grid; until the art loads it
// uses a portrait placeholder (3:4).
func coverAspect(b *ui.Bitmap) float32 {
	if b != nil {
		w, h := b.Size()
		if w > 0 && h > 0 {
			r := float32(w) / float32(h)
			// Clamp extremes so one aberrant image can't dominate a row.
			if r < 0.25 {
				r = 0.25
			} else if r > 2.5 {
				r = 2.5
			}
			return r
		}
	}
	return 3.0 / 4.0
}
func seasonLabel(s catalog.Series) string {
	if s.Season > 0 {
		return fmt.Sprintf("第 %d 季", s.Season)
	}
	if s.Title != "" {
		return s.Title
	}
	return "第 1 季"
}
func (a *app) detailView(c *ui.Context) {
	ui.Column(c).Width(300).FillHeight().Background(appleCanvas).BorderWidth(0, 0, 0, 0).BorderColor(appleHairline).Padding(max(float32(48), c.TitleBar().Height+8), 16, 20, 16).Gap(12).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "剧集详情").FontWeight(600).FontSize(17).TextColor(appleInk).Grow(1)
			button(c, "").Label("收起").OnClick(func() {
				if a.detailCancel != nil {
					a.detailCancel()
				}
				a.detailID++
				a.detailBusy = false
				a.selected = catalog.Series{}
			}).Children(func() { symbol(c, "xmark", 15, appleMuted) })
		})
		ui.Text(c, a.selected.Title).FontSize(20).FontWeight(600).TextColor(appleInk).Wrap().MaxLines(4)
		if a.selected.Description != "" {
			ui.Text(c, a.selected.Description).FontSize(13).TextColor(appleMuted).Wrap().MaxLines(5)
		}
		label := "加入收藏"
		if _, ok := a.library.Favorites[a.selected.ID]; ok {
			label = "已收藏"
		}
		button(c, label).OnClick(func() { a.toggleFavorite(a.selected) })
		if len(a.seasons) > 1 {
			ui.Text(c, fmt.Sprintf("全部季 · %d 季", len(a.seasons))).FontWeight(600).FontSize(13).TextColor(appleInk)
			ui.Scroll(c.Key("seasons-list")).MaxHeight(112).MinHeight(0).Children(func() {
				ui.Grid(c).Columns(5).Gap(8).FillWidth().Children(func() {
					for i, season := range a.seasons {
						sn := season
						active := sn.ID == a.selected.ID
						bg := appleParchment
						tc := appleInk
						if active {
							bg = appleBlue
							tc = appleSurface
						}
						number := sn.Season
						if number <= 0 {
							number = i + 1
						}
						button(c.Key("season-"+sn.ID), fmt.Sprint(number)).Height(32).Radius(8).Padding(4, 8).Justify(ui.Center).AlignItems(ui.Center).Background(bg).TextColor(tc).Label(fmt.Sprintf("选择第%d季", number)).OnClick(func() { a.selectSeries(sn, false) })
					}
				})
			})
		}
		ui.Text(c, fmt.Sprintf("选集 · %d 集", len(a.episodes))).FontWeight(600).FontSize(13).TextColor(appleInk)
		if a.detailBusy {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c)
				ui.Text(c, "正在读取选集…").TextColor(appleMuted)
			})
		}
		if !a.detailBusy && a.detailMessage != "" {
			status := "待上线"
			if a.selected.ReleaseState != catalog.ReleaseUpcoming {
				status = "提示"
			}
			ui.Column(c).Padding(12).Gap(6).Radius(10).Background(appleParchment).Border(1, appleHairline).Children(func() {
				ui.Text(c, status).FontSize(12).FontWeight(600).TextColor(appleBlue)
				ui.Text(c, a.detailMessage).FontSize(12).TextColor(appleMuted).Wrap()
			})
		}
		ui.Scroll(c.Key(a.selected.ID + "-episodes")).Grow(1).MinHeight(0).Children(func() {
			ui.Grid(c).Columns(4).Gap(8).FillWidth().Children(func() {
				for i, ep := range a.episodes {
					idx := i
					bg := appleParchment
					text := appleInk
					if ep.ID == a.playingEpisode.ID {
						bg = appleBlue
						text = appleSurface
					}
					button(c.Key(ep.ID), fmt.Sprint(ep.Number)).Height(36).Radius(8).Background(bg).TextColor(text).Label(fmt.Sprintf("播放第%d集", ep.Number)).OnClick(func() { a.startEpisode(a.selected, a.episodes, idx, 0) })
				}
			})
		})
	})
}
func (a *app) playerView(c *ui.Context) {
	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Padding(0, 18, 12, 18).Gap(10).Children(func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, a.playingSeries.Title).FontWeight(600).MaxLines(1).Grow(1)
			ui.Text(c, fmt.Sprintf("第 %d 集", a.playingEpisode.Number)).TextColor(appleMuted)
			if a.playingSeries.ID != "" && a.selected.ID == "" {
				button(c, "选集").OnClick(func() { a.selectSeries(a.playingSeries, false) })
			}
			if a.preloadNote != "" {
				ui.Text(c, a.preloadNote).FontSize(11).TextColor(appleMuted)
			}
			if a.danmakuNote != "" {
				ui.Text(c, a.danmakuNote).FontSize(11).TextColor(appleMuted)
			}
			button(c, "返回剧库").OnClick(func() {
				a.showPlayer = false
				if a.player != nil {
					a.player.Pause()
					a.saveProgress()
				}
				a.setPortrait(false)
			})
		})
		a.videoSurface(c)
	})
}

// videoSurface draws the video with the transport controls overlaid on top,
// the way a native player shows them.
func (a *app) videoSurface(c *ui.Context) {
	ui.Box(c).Grow(1).FillWidth().MinHeight(0).Radius(14).Background(videoSurface).Children(func() {
		if a.player != nil {
			ui.WebView(c, a.player.Page().WebView()).Fill().Label("短剧画面")
		}
		if a.preparing || a.playMessage != "" {
			ui.Column(c).Absolute().Top(0).Bottom(0).Left(0).Right(0).Center().Padding(30).Gap(14).Background(ui.Hex("#131317")).Children(func() {
				ui.Text(c, a.playMessage).Wrap().MaxWidth(480).TextColor(ui.Hex("#f5f5f7"))
				if a.preparing {
					button(c, "取消加载").OnClick(func() { a.stopPlayer(); a.playID++; a.playMessage = "" })
				} else {
					ui.Row(c).Gap(8).Children(func() {
						primaryButton(c, "重试播放").OnClick(a.retry)
						if a.canResetDevice {
							button(c, "换设备重试").OnClick(func() { a.resetDeviceAndRetry() })
						}
					})
				}
			})
		}
		if p := a.player; p != nil {
			a.configurePlayerControls(p)
		}
	})
}

// Window playback retains a dark native drag region outside the WebView.
// System fullscreen leaves the entire surface to the video and its controls.
func (a *app) playbackTitlebar(c *ui.Context) {
	bar := c.TitleBar()
	ui.Row(c.Key("playback-titlebar")).Label("播放窗口标题栏").FillWidth().Height(max(float32(40), bar.Height)).Shrink(0).
		PaddingStart(bar.Left + 12).PaddingEnd(bar.Right + 16).Gap(12).AlignItems(ui.Center).
		Background(ui.Hex("#151518")).DragWindow().Children(func() {
		title := a.playingSeries.Title
		if title == "" {
			title = "正在播放"
		}
		ui.Text(c, title).FontSize(13).FontWeight(500).TextColor(ui.Hex("#e5e5e7")).Grow(1).MinWidth(0).SingleLine().IgnoreInput(true)
		if a.playingEpisode.Number > 0 {
			ui.Text(c, fmt.Sprintf("第 %d 集", a.playingEpisode.Number)).FontSize(12).TextColor(ui.Hex("#96969e")).SingleLine().IgnoreInput(true)
		}
	})
}

func (a *app) fullscreenVideoView(c *ui.Context) {
	ui.Column(c).Fill().Background(videoSurface).Children(func() {
		if a.windowFullscreen && !a.fullscreenVideo {
			a.playbackTitlebar(c)
		}
		ui.Box(c).Grow(1).FillWidth().MinHeight(0).Background(videoSurface).Children(func() {
			if a.player != nil {
				ui.WebView(c, a.player.Page().WebView()).Fill().Label("短剧画面")
			}
			if a.preparing || a.playMessage != "" {
				ui.Column(c).Absolute().Top(0).Bottom(0).Left(0).Right(0).Center().Padding(30).Gap(14).Background(ui.Hex("#131317")).Children(func() {
					ui.Text(c, a.playMessage).Wrap().MaxWidth(480).TextColor(ui.Hex("#f5f5f7"))
					if a.preparing {
						button(c, "取消加载").OnClick(func() { a.stopPlayer(); a.playID++; a.playMessage = "" })
					} else {
						ui.Row(c).Gap(8).Children(func() {
							primaryButton(c, "重试播放").OnClick(a.retry)
							if a.canResetDevice {
								button(c, "换设备重试").OnClick(func() { a.resetDeviceAndRetry() })
							}
						})
					}
				})
			}
			if p := a.player; p != nil {
				a.configurePlayerControls(p)
			}
		})
	})
}
func (a *app) canJump(delta int) bool {
	for i, e := range a.playingEpisodes {
		if e.ID == a.playingEpisode.ID {
			return i+delta >= 0 && i+delta < len(a.playingEpisodes)
		}
	}
	return false
}
func (a *app) changeQuality() {
	pos := a.effectivePosition()
	playWhenReady := a.player != nil && a.player.Playing()
	for i, ep := range a.playingEpisodes {
		if ep.ID == a.playingEpisode.ID {
			a.startEpisodeWithMode(a.playingSeries, a.playingEpisodes, i, pos, playWhenReady)
			return
		}
	}
}
func (a *app) footer(c *ui.Context) {
	if a.player == nil && !a.preparing && a.playMessage == "" {
		return
	}
	ui.Row(c).FillWidth().Height(76).Padding(10, 24).Gap(12).AlignItems(ui.Center).Background(appleCanvas).BorderWidth(.5, 0, 0, 0).BorderColor(appleHairline).Children(func() {
		ui.Box(c).Size(38, 52).Radius(5).Background(appleSurface).Center().Children(func() {
			if b := a.cover(a.playingSeries.Cover); b != nil {
				ui.Image(c, b).Fill().Fit(ui.Cover).Radius(5)
			} else {
				symbol(c, "play.rectangle", 22, appleMuted)
			}
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
			ui.Text(c, a.playingSeries.Title).FontSize(13).FontWeight(600).SingleLine()
			ui.Text(c, fmt.Sprintf("%s · 第 %d 集", a.sourceDisplayName(), a.playingEpisode.Number)).FontSize(12).TextColor(appleMuted).SingleLine()
		})
		if a.player != nil {
			icon := "play.fill"
			if a.player.Playing() {
				icon = "pause.fill"
			}
			shellIconButton(c, icon, "播放或暂停").OnClick(func() {
				if a.player.Playing() {
					a.player.Pause()
				} else {
					a.player.Play()
				}
			})
		}
		shellIconButton(c, "arrow.up.left.and.arrow.down.right", "打开正在播放").Disabled(a.showPlayer).OnClick(func() { a.showPlayer = true })
		shellIconButton(c, "xmark", "停止播放").OnClick(func() { a.stopPlayer(); a.playID++; a.playMessage = "" })
	})
}
func clock(v float64) string {
	if !validPosition(v) {
		return "00:00"
	}
	n := int(v)
	if n >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", n/3600, n/60%60, n%60)
	}
	return fmt.Sprintf("%02d:%02d", n/60, n%60)
}
