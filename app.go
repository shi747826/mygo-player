package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"mygo-player/internal/catalog"
	"mygo-player/internal/plugins"
	"mygo-player/internal/webplayer"
	"mygo-player/pluginapi"
)

type app struct {
	sourceSearch                              map[string]string
	navHistory, navForward                    []pageRoute
	searchedQuery                             string
	searchBox                                 ui.Handle
	pluginHostMu                              sync.RWMutex
	pluginManager                             *plugins.Manager
	pluginClients                             map[string]*plugins.Client
	pluginMessage                             string
	client                                    *catalog.Client
	source                                    string
	sourceStores                              map[string]*catalog.Client
	activeMedia                               *preparedEpisode
	pluginPath                                string
	pluginBusy                                bool
	pluginInstallCancel                       context.CancelFunc
	accShelfCancel                            context.CancelFunc
	hgSortSel                                 int
	hgSort                                    string
	win                                       *mygo.Window
	ctx                                       context.Context
	cancel                                    context.CancelFunc
	listCancel, detailCancel, playCancel      context.CancelFunc
	prefetchCancel                            context.CancelFunc
	prefetchGen                               int
	listID, detailID, playID                  int
	page, query, channel, cursor              string
	channelSel                                int
	items                                     []catalog.Series
	hasMore, busy, detailBusy, preparing      bool
	canResetDevice, resetting, autoResetTried bool
	resetMu                                   sync.Mutex
	resetDone                                 chan struct{}
	resetErr                                  error
	prefetching                               bool
	message, playMessage                      string
	detailMessage                             string
	listScroll                                ui.ScrollState
	selected                                  catalog.Series
	episodes                                  []catalog.Episode
	seasons                                   []catalog.Series
	playingSeries                             catalog.Series
	playingEpisode                            catalog.Episode
	playingEpisodes                           []catalog.Episode
	library                                   catalog.Library
	player                                    *webplayer.Player
	playerFocusPending                        bool
	quality                                   string
	qualityChosen                             bool
	qualities                                 []string
	autoplay, pinned, portrait                bool
	fullscreenVideo                           bool
	windowFullscreen                          bool
	normalWidth, normalHeight                 int
	volume, rate                              float64
	resume                                    float64
	resumePending, observedPlaying            bool
	resumeIssued                              bool
	resumeStarted                             time.Time
	endedHandled                              bool
	lastSave                                  time.Time
	preloadCancel                             context.CancelFunc
	preloaded                                 map[string]*preparedEpisode
	preloadedSeries                           string
	preloadNote                               string
	accountProfile                            accountProfile
	reservations                              []accountShelfItem
	reservationsLoaded                        bool
	reservationBusy                           bool
	reservationError                          string
	reservationCancel                         context.CancelFunc
	reservationGen                            int
	accountLoggedIn, accountBusy              bool
	accountQR                                 []byte
	accountQRImage                            *ui.Bitmap
	accountStatus                             string
	accountCancel                             context.CancelFunc
	danmakuCancel                             context.CancelFunc
	danmakuVisible                            bool
	danmakuPosition                           string
	danmakuSize                               string
	danmakuSpeed                              string
	danmakuOpacity                            int
	prefsSaved                                preferences
	prefsTimer                                *time.Timer
	prefsMu                                   sync.Mutex
	danmakuNote                               string
	danmakuEpisode                            string
	accSyncNote                               string
	accShelfBusy                              bool
	calendar                                  map[string]catalog.UpcomingEntry
	calendarAt                                time.Time
	calendarBusy                              bool
	accountGen                                int
	dataDir                                   string
	diagnosticState                           string
	closed                                    bool
	showPlayer                                bool
	covers                                    map[string]*ui.Bitmap
	coverUsed                                 map[string]int64
	coverSeq                                  int64
	prefetchPaused                            bool
	prefetchSeeded                            bool
	prefetchY                                 float32
	pageSize                                  int
	gridWidth                                 float32
	gridViewport                              float32
	gridContent                               float32
	renderedCards                             int
	layout                                    []int
	layoutCols, layoutListID                  int
	coverPending                              map[string]bool
	coverSlots                                chan struct{}
	smoke, smokePassed                        bool
	smokeNext                                 bool
	smokeFirstEpisode                         string
	smokeStage                                int
	smokeSeek                                 float64
}

type preparedEpisode struct {
	media  pluginapi.Media
	source plugins.Source
}

func (p *preparedEpisode) close() {
	if p == nil || p.media.SessionID == "" || p.source == nil {
		return
	}
	source, id := p.source, p.media.SessionID
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = source.Release(ctx, id)
	}()
}
func newApp(c *catalog.Client, _ any) *app {
	ctx, cancel := context.WithCancel(context.Background())
	a := &app{client: c, ctx: ctx, cancel: cancel, page: "推荐", quality: "自动", autoplay: true, volume: 0.8, rate: 1, covers: map[string]*ui.Bitmap{}, coverUsed: map[string]int64{}, coverPending: map[string]bool{}, coverSlots: make(chan struct{}, 4), preloaded: map[string]*preparedEpisode{}, sourceStores: map[string]*catalog.Client{}, danmakuVisible: true, danmakuPosition: "full", danmakuSize: "medium", danmakuSpeed: "medium", danmakuOpacity: 80}
	a.prefsSaved = a.currentPreferences()
	a.refreshLibrary()
	return a
}
func (a *app) setPortrait(enabled bool) {
	if enabled == a.portrait {
		return
	}
	a.portrait = enabled
	if a.win == nil {
		return
	}
	if enabled {
		a.normalWidth, a.normalHeight = a.win.Size()
		a.win.SetMinimumSize(420, 620)
		a.win.SetSize(480, 880)
	} else {
		a.win.SetMinimumSize(900, 620)
		w, h := a.normalWidth, a.normalHeight
		if w < 900 {
			w = 1280
		}
		if h < 620 {
			h = 820
		}
		a.win.SetSize(w, h)
	}
}
func (a *app) refreshLibrary() {
	lib, e := a.client.Library()
	if e != nil {
		a.message = "读取本地记录失败"
		return
	}
	a.library = lib
}
func (a *app) loadPage(page string, more bool) {
	if a.closed {
		return
	}
	if a.listCancel != nil {
		a.listCancel()
	}
	a.cancelReservationRequest()
	if a.prefetchCancel != nil {
		a.prefetchCancel()
		a.prefetchCancel = nil
	}
	a.prefetching = false
	a.prefetchPaused = false
	a.prefetchSeeded = false
	a.pageSize = a.targetPageSize()
	a.prefetchY = 0
	a.prefetchGen++
	a.listScroll = ui.ScrollState{}
	a.clearPreloads()
	a.showPlayer = false
	if a.player != nil {
		a.player.Pause()
		a.saveProgress()
	}
	a.setPortrait(false)
	if a.detailCancel != nil {
		a.detailCancel()
	}
	a.detailID++
	a.detailBusy = false
	a.listID++
	id := a.listID
	a.page = page
	if page == "搜索" && !more {
		a.searchedQuery = strings.TrimSpace(a.query)
	}
	a.message = ""
	a.busy = true
	cursor := ""
	if more {
		cursor = a.cursor
	} else {
		a.items = nil
		a.cursor = ""
		a.hasMore = false
	}
	query, channel := strings.TrimSpace(a.query), a.channel
	if page == "搜索" {
		query = a.searchedQuery
	}
	source := a.sourceAPI()
	sortValue, pageSize := a.hgSort, a.pageSizeOr(24)
	ctx, cancel := context.WithTimeout(a.ctx, 45*time.Second)
	a.listCancel = cancel
	if page == "预约" {
		cancel()
		a.busy = false
		a.selected = catalog.Series{}
		a.episodes = nil
		if a.sourceHas("reservations") {
			a.loadReservations()
		}
		return
	}
	if page == "账号" || page == "插件" {
		// 账号页只呈现登录状态，不发起目录请求。
		cancel()
		a.busy = false
		a.items = nil
		return
	}
	if page == "收藏" || page == "历史" {
		cancel()
		a.refreshStore()
		a.items = libraryItems(a.library, page, query)
		a.busy = false
		return
	}
	if a.pluginManager != nil && a.pluginClients[a.source] == nil {
		cancel()
		a.busy = false
		a.page = "插件"
		return
	}
	go func() {
		defer cancel()
		trace("catalog start id=%d page=%s", id, page)
		var p catalog.Page
		var e error
		p, e = a.sourcePage(ctx, source, page, query, channel, sortValue, cursor, pageSize)
		trace("catalog finished id=%d items=%d error=%v", id, len(p.Items), e != nil)
		a.win.Update(func() {
			trace("catalog callback id=%d current=%d closed=%v", id, a.listID, a.closed)
			if a.closed || id != a.listID {
				return
			}
			a.busy = false
			if e != nil {
				a.message = friendly(e)
				if a.smoke {
					a.failSmoke("catalog request failed")
				}
				return
			}
			if a.smoke {
				fmt.Printf("SMOKE catalog: %d results\n", len(p.Items))
			}
			a.items = mergeSeries(a.items, p.Items)
			a.cursor = p.Cursor
			a.hasMore = p.HasMore && p.Cursor != "" && page != "榜单"
			if len(a.items) == 0 {
				a.message = "暂时没有找到短剧，试试其他关键词。"
			}
			if a.smoke && len(a.items) > 0 {
				a.selectSeries(a.items[0], true)
			}
		})
	}()
}

// prefetch quietly loads the next two pages before the user scrolls into
// them, so the feed keeps scrolling without a gap or a 加载更多 wait. It
// appends when ready and re-arms while more remains.
func (a *app) prefetch() {
	if a.prefetching || a.busy || !a.hasMore || a.cursor == "" || a.closed || a.prefetchPaused {
		return
	}
	// 追加内容后滚动指标可能还是旧的，若用户没有实际继续下滑，
	// 就不要再触发下一批，否则几批会在同一位置连续落地，看起来像一次性多出上百条。
	if a.prefetchSeeded && a.listScroll.Y == a.prefetchY {
		return
	}
	a.prefetchSeeded = true
	a.prefetchY = a.listScroll.Y
	if a.prefetchCancel != nil {
		a.prefetchCancel()
	}
	a.prefetching = true
	a.prefetchGen++
	gen := a.prefetchGen
	page, query, channel := a.page, strings.TrimSpace(a.query), a.channel
	if page == "搜索" {
		query = a.searchedQuery
	}
	ctx, cancel := context.WithTimeout(a.ctx, 60*time.Second)
	a.prefetchCancel = cancel
	listID := a.listID
	source := a.sourceAPI()
	sortValue, pageSize := a.hgSort, a.pageSizeOr(24)
	cursorStart := a.cursor
	go func() {
		defer cancel()
		var all []catalog.Series
		failed := false
		cursor := cursorStart
		defer a.win.Update(func() {
			if listID == a.listID && gen == a.prefetchGen {
				a.prefetching = false
				a.prefetchCancel = nil
			}
		})
		for i := 0; i < 2 && cursor != ""; i++ {
			if ctx.Err() != nil {
				return
			}
			var p catalog.Page
			var e error
			p, e = a.sourcePage(ctx, source, page, query, channel, sortValue, cursor, pageSize)
			if e != nil {
				failed = true
				break
			}
			// cursor 必须前进，否则同一页会被反复请求。
			if p.Cursor == cursor {
				failed = true
				break
			}
			all = mergeSeries(all, p.Items)
			cursor = p.Cursor
			if !p.HasMore {
				break
			}
		}
		a.win.Update(func() {
			if a.closed || listID != a.listID || gen != a.prefetchGen {
				return
			}
			a.prefetching = false
			a.prefetchCancel = nil
			// 已经拿到的那部分先追加，不因为下一页失败就整批丢弃。
			if len(all) > 0 {
				a.items = mergeSeries(a.items, all)
				a.cursor = cursor
				a.hasMore = cursor != "" && page != "榜单"
				a.prefetchY = a.listScroll.Y
			}
			// 失败后不再原地自动重试同一页，等用户刷新或重新加载。
			if failed {
				a.prefetchPaused = true
			}
		})
	}()
}

// searchPage loads a search page and, when the server returns an
// under-filled first screen, keeps fetching following pages so the user
// does not have to press 加载更多 for a handful of results. It also backfills
// covers, which the search endpoint omits, from the series detail endpoint.
func (a *app) searchPage(ctx context.Context, query, cursor string) (catalog.Page, error) {
	page, err := a.sourceAPI().Search(ctx, query, cursor)
	if err != nil {
		return page, err
	}
	// First page only: keep pulling until the screen is reasonably filled.
	const bareScreen = 18
	const maxAutoPages = 6
	if cursor == "" {
		pulled := 1
		for page.HasMore && len(page.Items) < bareScreen && pulled < maxAutoPages {
			if ctx.Err() != nil {
				break
			}
			next, err := a.sourceAPI().Search(ctx, query, page.Cursor)
			if err != nil {
				break
			}
			page.Items = mergeSeries(page.Items, next.Items)
			page.Cursor = next.Cursor
			page.HasMore = next.HasMore
			pulled++
		}
	}
	_ = a.backfillCovers(ctx, page.Items)
	return page, nil
}

// backfillCovers fills missing Cover URLs on the in-memory items using the
// series-detail batch endpoint. Failures are ignored so search still works
// without art; those cards show their placeholder.
func (a *app) backfillCovers(ctx context.Context, items []catalog.Series) error {
	var missing []string
	for _, s := range items {
		if s.Cover == "" && s.ID != "" {
			missing = append(missing, s.ID)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	covers, err := a.sourceAPI().Covers(ctx, missing)
	if err != nil {
		return err
	}
	for i := range items {
		if items[i].Cover == "" {
			if c, ok := covers[items[i].ID]; ok {
				items[i].Cover = c
			}
		}
	}
	return nil
}
func libraryItems(lib catalog.Library, page, query string) []catalog.Series {
	out := []catalog.Series{}
	if page == "收藏" {
		for _, s := range lib.Favorites {
			if query == "" || strings.Contains(strings.ToLower(s.Title), strings.ToLower(query)) {
				out = append(out, s)
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	} else {
		ps := []catalog.Progress{}
		for _, p := range lib.History {
			if query == "" || strings.Contains(strings.ToLower(p.Series.Title), strings.ToLower(query)) {
				ps = append(ps, p)
			}
		}
		sort.Slice(ps, func(i, j int) bool { return ps[i].UpdatedAt.After(ps[j].UpdatedAt) })
		for _, p := range ps {
			out = append(out, p.Series)
		}
	}
	return out
}
func mergeSeries(old, more []catalog.Series) []catalog.Series {
	seen := map[string]bool{}
	out := make([]catalog.Series, 0, len(old)+len(more))
	for _, s := range append(append([]catalog.Series{}, old...), more...) {
		if s.ID != "" && !seen[s.ID] {
			seen[s.ID] = true
			out = append(out, s)
		}
	}
	return out
}
func (a *app) selectSeries(s catalog.Series, play bool) {
	if a.detailCancel != nil {
		a.detailCancel()
	}
	a.detailID++
	id := a.detailID
	// 直接播放时不打开详情面板：否则右侧面板会先弹出来挤压列表，几秒后
	// 才进入播放器。详情只在「选集」路径打开。
	if !play {
		a.selected = s
	}
	a.episodes = nil
	a.seasons = nil
	a.detailBusy = true
	a.detailMessage = ""
	a.message = ""
	ctx, cancel := context.WithTimeout(a.ctx, 90*time.Second)
	a.detailCancel = cancel
	source := a.sourceAPI()
	go func() {
		defer cancel()
		type episodesResult struct {
			detail catalog.Series
			eps    []catalog.Episode
			err    error
		}
		epCh := make(chan episodesResult, 1)
		seasonCh := make(chan []catalog.Series, 1)
		go func() {
			detail, eps, err := source.Detail(ctx, s.ID)
			epCh <- episodesResult{detail, eps, err}
		}()
		go func() {
			seasons, _ := source.Seasons(ctx, s.ID)
			seasonCh <- seasons
		}()
		epRes := <-epCh
		var seasons []catalog.Series
		select {
		case seasons = <-seasonCh:
		case <-ctx.Done():
		}
		a.win.Update(func() {
			if a.closed || id != a.detailID {
				return
			}
			a.detailBusy = false
			if epRes.err != nil {
				a.detailMessage = friendly(epRes.err)
				if a.smoke {
					a.failSmoke("episode list failed")
				}
				return
			}
			if a.smoke {
				fmt.Printf("SMOKE episodes: %d\n", len(epRes.eps))
			}
			// 详情结果是发布状态与集数的权威来源；标题/季号沿用列表或收藏
			// 传入的展示值，避免多季场景被详情里的默认值覆盖。
			merged := epRes.detail
			if s.Title != "" {
				merged.Title = s.Title
			}
			if s.Season > 0 {
				merged.Season = s.Season
			}
			if merged.Cover == "" {
				merged.Cover = s.Cover
			}
			a.episodes = epRes.eps
			a.seasons = seasons
			if !play {
				a.selected = merged
			}
			switch {
			case merged.ReleaseState == catalog.ReleaseUpcoming:
				a.detailMessage = upcomingText(merged.ReleaseNote)
				// 角标只说「更新至0集」时，用日历里的真实上线时间补强。
				a.applyCalendarHint(id, merged.ID)
			case len(epRes.eps) == 0:
				a.detailMessage = "分集暂不可用，请稍后再试"
				// 用上新日历做第二重证据：日历预告了上线时间才改判待上线。
				a.applyCalendarHint(id, merged.ID)
			}
			if play && len(epRes.eps) > 0 {
				idx, pos := resumeTarget(a.library, s.ID, epRes.eps)
				playing := merged
				if s.Title != "" {
					playing.Title = s.Title
				}
				a.startEpisode(playing, epRes.eps, idx, pos)
			}
		})
	}()
}

// upcomingText 给出待上线说明。服务端角标形如「更新至0集」时读起来别扭，
// 统一换成自然措辞，并保留「可能」的余地：零分集也可能因地区限制导致。
func upcomingText(note string) string {
	note = strings.TrimSpace(note)
	if note == "" || strings.Contains(note, "0集") || strings.Contains(note, "0 集") {
		return "这一季暂无可用分集，可能尚未上线"
	}
	return note
}

func resumeTarget(lib catalog.Library, id string, eps []catalog.Episode) (int, float64) {
	p, ok := lib.History[id]
	if ok {
		for i, e := range eps {
			if e.ID == p.Episode.ID {
				return i, math.Max(0, p.Position)
			}
		}
	}
	return 0, 0
}
func (a *app) startEpisode(s catalog.Series, eps []catalog.Episode, idx int, pos float64) {
	a.startEpisodeWithMode(s, eps, idx, pos, true)
}
func (a *app) startEpisodeWithMode(s catalog.Series, eps []catalog.Episode, idx int, pos float64, playWhenReady bool) {
	if idx < 0 || idx >= len(eps) || a.closed {
		return
	}
	if a.detailCancel != nil {
		a.detailCancel()
	}
	a.detailID++
	a.detailBusy = false
	a.stopPlayer()
	a.showPlayer = true
	a.canResetDevice = false
	a.autoResetTried = false
	a.preloadNote = ""
	a.playID++
	id := a.playID
	a.playingSeries = s
	a.playingEpisodes = append([]catalog.Episode{}, eps...)
	a.playingEpisode = eps[idx]
	a.endedHandled = false
	a.observedPlaying = false
	ep := eps[idx]

	if pos == 0 {
		if prep, ok := a.preloaded[ep.ID]; ok {
			delete(a.preloaded, ep.ID)
			a.adoptMedia(id, prep, pos, playWhenReady)
			return
		}
	}
	a.preparing = true
	a.playMessage = "正在准备视频…"
	quality := a.quality
	if quality == "自动" {
		quality = ""
	}
	ctx, cancel := context.WithTimeout(a.ctx, 4*time.Minute)
	a.playCancel = cancel
	source := a.sourceAPI()
	go func() {
		defer cancel()
		media, err := source.Resolve(ctx, ep.ID, quality)
		if err != nil {
			a.playError(id, err)
			return
		}
		prep := &preparedEpisode{media: media, source: source}
		a.win.Update(func() {
			if a.closed || id != a.playID {
				prep.close()
				return
			}
			a.adoptMedia(id, prep, pos, playWhenReady)
		})
	}()
}

// All sources enter the same native playback, quality, controls and episode flow.
func (a *app) adoptMedia(id int, prep *preparedEpisode, pos float64, playWhenReady bool) {
	if a.closed || id != a.playID {
		prep.close()
		return
	}
	p, err := webplayer.OpenURLInWindow(a.win, prep.media.URL, a.playerChanged)
	if err != nil {
		prep.close()
		a.preparing = false
		a.playMessage = "系统无法打开视频，请检查媒体组件和解码支持。"
		if a.smoke {
			a.failSmoke("native video open failed")
		}
		return
	}
	a.activeMedia = prep
	a.player = p
	a.playerFocusPending = true
	a.qualities = []string{"自动"}
	for _, q := range prep.media.Qualities {
		if q != "" && !contains(a.qualities, q) {
			a.qualities = append(a.qualities, q)
		}
	}
	if !a.qualityChosen {
		a.quality = highestQuality(a.qualities, prep.media.Definition)
	}
	a.preparing = false
	a.playMessage = ""
	a.resume = pos
	a.resumePending = pos > 0
	a.resumeIssued = false
	a.lastSave = time.Now()
	p.SetVolume(a.volume)
	p.SetRate(a.rate)
	if a.showPlayer && playWhenReady {
		p.Play()
	} else {
		p.Pause()
	}
	a.preloadNext()
	a.startDanmakuLoad(p, a.playingSeries.ID, a.playingEpisode.ID)
}
func contains(xs []string, s string) bool {
	for _, v := range xs {
		if v == s {
			return true
		}
	}
	return false
}

// preloadNext prepares one episode through the same source-neutral resolver.
func (a *app) preloadNext() {
	if a.closed || a.player == nil || !a.autoplay {
		return
	}
	var next catalog.Episode
	for i, e := range a.playingEpisodes {
		if e.ID == a.playingEpisode.ID && i+1 < len(a.playingEpisodes) {
			next = a.playingEpisodes[i+1]
			break
		}
	}
	if next.ID == "" {
		return
	}
	if _, ok := a.preloaded[next.ID]; ok {
		return
	}
	if a.preloadedSeries != a.playingSeries.ID {
		a.clearPreloads()
		a.preloadedSeries = a.playingSeries.ID
	}
	if a.preloadCancel != nil {
		a.preloadCancel()
	}
	ctx, cancel := context.WithTimeout(a.ctx, 4*time.Minute)
	a.preloadCancel = cancel
	source, id, quality := a.sourceAPI(), a.playID, a.quality
	if quality == "自动" {
		quality = ""
	}
	go func() {
		defer cancel()
		media, err := source.Resolve(ctx, next.ID, quality)
		if err != nil {
			if ctx.Err() == nil {
				a.win.Update(func() {
					if !a.closed && id == a.playID {
						a.preloadNote = friendly(err)
					}
				})
			}
			return
		}
		prep := &preparedEpisode{source: source, media: media}
		a.win.Update(func() {
			if a.closed || id != a.playID {
				prep.close()
				return
			}
			if old := a.preloaded[next.ID]; old != nil {
				old.close()
			}
			a.preloaded[next.ID] = prep
		})
	}()
}
func (a *app) playError(id int, e error) {

	a.win.Update(func() {
		if a.closed || id != a.playID {
			return
		}
		a.preparing = false
		a.playMessage = friendly(e)
		// 只有匿名设备被限制时，换一台设备才有意义。
		a.canResetDevice = errors.Is(e, pluginapi.ErrAnonymousLimited) && a.sourceHas("resetDevice")
		if a.smoke {
			a.failSmoke("playback preparation failed: " + friendly(e) + " [" + e.Error() + "]")
		}
	})
}
func (a *app) openLocal(path string) {
	a.showPlayer = true
	a.stopPlayer()
	p, e := webplayer.Open(path, a.playerChanged)
	if e != nil {
		a.playMessage = "无法打开测试视频"
		if a.smoke {
			a.failSmoke("local media open failed")
		}
		return
	}
	a.player = p
	a.playerFocusPending = true
	// 本地验证视频同样沿用已保存的音量与倍速，避免状态回报把设置改回去。
	p.SetVolume(a.volume)
	p.SetRate(a.rate)
	p.Play()
	a.playingSeries = catalog.Series{Title: "本地视频验证"}
	a.playingEpisode = catalog.Episode{Title: "测试视频", Number: 1}
	a.lastSave = time.Now()
	a.endedHandled = false
	if a.win != nil {
		a.win.Invalidate()
	}
}
func (a *app) jump(delta int) {
	for i, e := range a.playingEpisodes {
		if e.ID == a.playingEpisode.ID {
			a.startEpisode(a.playingSeries, a.playingEpisodes, i+delta, 0)
			return
		}
	}
}

// deviceRotating 报告是否有换设备正在进行，用于避免主流程重复触发。
func (a *app) deviceRotating() bool {
	a.resetMu.Lock()
	defer a.resetMu.Unlock()
	return a.resetDone != nil
}

// rotateDevice 换一台匿名设备，并保证同一时间只有一次注册在飞：
// 预加载和主播放同时遇到空响应时，共用同一次结果，不会连续注册好几台。
func (a *app) rotateDevice(ctx context.Context) error {
	a.resetMu.Lock()
	if a.resetDone != nil {
		ch := a.resetDone
		a.resetMu.Unlock()
		select {
		case <-ch:
			a.resetMu.Lock()
			err := a.resetErr
			a.resetMu.Unlock()
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	ch := make(chan struct{})
	a.resetDone = ch
	a.resetMu.Unlock()

	err := a.sourceAPI().ResetDevice(ctx)

	a.resetMu.Lock()
	a.resetErr = err
	a.resetDone = nil
	a.resetMu.Unlock()
	close(ch)
	return err
}

// resetDeviceAndRetry 丢弃当前匿名设备、注册一台新设备，再重试当前分集。
// 用于服务端对匿名设备限流后的手动换一台。
func (a *app) resetDeviceAndRetry() {
	if a.resetting || a.closed {
		return
	}
	a.stopPlayer()
	a.clearPreloads()
	a.playID++
	id := a.playID
	a.resetting = true
	a.canResetDevice = false
	a.playMessage = "正在更换匿名设备…"
	a.preparing = true
	if a.win != nil {
		a.win.Invalidate()
	}
	ctx, cancel := context.WithTimeout(a.ctx, 60*time.Second)
	a.playCancel = cancel
	go func() {
		defer cancel()
		err := a.rotateDevice(ctx)
		if a.win == nil {
			return
		}
		a.win.Update(func() {
			if a.closed {
				return
			}
			a.resetting = false
			if id != a.playID {
				return
			}
			a.preparing = false
			if err != nil {
				a.playMessage = friendly(err)
				a.canResetDevice = true
				return
			}
			a.playMessage = ""
			a.retry()
		})
	}()
}

func (a *app) retry() {
	pos := a.effectivePosition()
	if a.player == nil && !a.resumePending {
		if p, ok := a.library.History[a.playingSeries.ID]; ok && p.Episode.ID == a.playingEpisode.ID {
			pos = p.Position
		}
	}
	for i, e := range a.playingEpisodes {
		if e.ID == a.playingEpisode.ID {
			a.startEpisode(a.playingSeries, a.playingEpisodes, i, pos)
			return
		}
	}
}
func (a *app) saveProgress() {
	if a.player == nil || !a.observedPlaying || a.playingSeries.ID == "" {
		return
	}
	pos := a.effectivePosition()
	if !validPosition(pos) {
		return
	}
	p := catalog.Progress{Series: a.playingSeries, Episode: a.playingEpisode, Position: pos, UpdatedAt: time.Now()}
	if e := a.store().SaveProgress(p); e != nil {
		a.message = "播放进度保存失败，请检查数据目录权限。"
	} else {
		a.library.History[p.Series.ID] = p
	}
	a.lastSave = time.Now()
}
func (a *app) effectivePosition() float64 {
	if a.resumePending {
		return a.resume
	}
	if a.player != nil {
		return a.player.Position().Seconds()
	}
	return 0
}
func (a *app) seek(pos float64) {
	if a.player == nil || !validPosition(pos) {
		return
	}
	a.resume = pos
	a.resumePending = true
	a.resumeIssued = true
	a.resumeStarted = time.Now()
	a.player.Seek(time.Duration(pos * float64(time.Second)))
}
func validPosition(v float64) bool { return v >= 0 && !math.IsNaN(v) && !math.IsInf(v, 0) }
func seekAcknowledged(pos, target float64) bool {
	return validPosition(pos) && validPosition(target) && math.Abs(pos-target) <= 2
}
func (a *app) stopPlayer() {
	a.playerFocusPending = false
	if a.playCancel != nil {
		a.playCancel()
		a.playCancel = nil
	}
	a.saveProgress()
	if a.player != nil {
		a.player.Close()
		a.player = nil
	}
	if a.activeMedia != nil {
		a.activeMedia.close()
		a.activeMedia = nil
	}
	a.resumePending = false
	a.observedPlaying = false
	a.preparing = false
}

// clearPreloads drops any buffered next-episode session; used when leaving
// the player or the app so nothing keeps preparing files in the background.
func (a *app) clearPreloads() {
	if a.preloadCancel != nil {
		a.preloadCancel()
		a.preloadCancel = nil
	}
	for _, v := range a.preloaded {
		v.close()
	}
	a.preloaded = map[string]*preparedEpisode{}
	a.preloadedSeries = ""
}

// enterFullscreen expands only the video to fill the screen: it renders the
// player full-window and asks macOS to make that window fullscreen, so the
// sidebar, header and library stay out of the picture.
func (a *app) enterFullscreen() {
	if a.win == nil {
		return
	}
	a.fullscreenVideo = true
	a.win.SetFullScreen(true)
}

// exitFullscreen returns to the normal app layout and leaves macOS fullscreen.
func (a *app) exitFullscreen() {
	a.fullscreenVideo = false
	if a.win != nil {
		a.win.SetFullScreen(false)
	}
}

func (a *app) shutdown() {
	if a.closed {
		return
	}
	a.cancelReservationRequest()
	if a.accountCancel != nil {
		a.accountCancel()
		a.accountCancel = nil
	}
	a.stopPlayer()
	a.clearPreloads()
	a.flushPrefs()
	a.closed = true
	a.cancel()
	if a.pluginManager != nil {
		_ = a.pluginManager.Close()
	}
}
func (a *app) toggleFavorite(s catalog.Series) {
	if _, e := a.store().ToggleFavorite(s); e != nil {
		a.message = "收藏保存失败"
	}
	a.refreshStore()
	if a.page == "收藏" {
		a.items = libraryItems(a.library, "收藏", strings.TrimSpace(a.query))
	}
}
func (a *app) playerChanged(fn func()) {
	a.win.Update(func() {
		fn()
		if !a.closed {
			a.advancePlayback()
		}
	})
}
func (a *app) tick() {
	if a.player != nil {
		a.advancePlayback()
	}
}
func (a *app) advancePlayback() {
	p := a.player
	if p == nil {
		return
	}
	if e := p.Err(); e != nil {
		a.playMessage = "视频播放失败：系统可能不支持该视频编码，或网络连接已中断。可换画质后重试。"
		if a.smoke {
			a.failSmoke("native decoder error")
		}
		return
	}
	if !p.Ready() {
		return
	}
	// Each episode owns a new WebView. Give it the keyboard once it has
	// mounted and is ready, so auto-advance does not require another click.
	if a.playerFocusPending && a.showPlayer && a.win != nil && a.win.IsFocused() {
		a.playerFocusPending = false
		p.Focus()
	}
	if rate := p.Rate(); rate > 0 {
		a.rate = rate
		a.volume = p.Volume()
		a.savePrefs()
	}
	if a.resumePending {
		if !a.resumeIssued {
			if duration := p.Duration().Seconds(); duration > 0 && a.resume >= duration-2 {
				a.resume = 0
			}
			p.Seek(time.Duration(a.resume * float64(time.Second)))
			a.resumeIssued = true
			a.resumeStarted = time.Now()
			return
		}
		if !seekAcknowledged(p.Position().Seconds(), a.resume) {
			if time.Since(a.resumeStarted) > 15*time.Second {
				a.playMessage = "未能恢复播放进度，请重试播放；原观看记录已保留。"
			}
			return
		}
		a.resumePending = false
		if a.smoke {
			fmt.Printf("SMOKE resume acknowledged: %.1fs\n", p.Position().Seconds())
		}
	}
	if p.Position().Seconds() > 0.2 {
		a.observedPlaying = true
	}
	if time.Since(a.lastSave) >= 5*time.Second {
		a.saveProgress()
	}
	if a.smoke {
		pos := p.Position().Seconds()
		if a.smokeStage == 0 && pos > 2 {
			a.smokeSeek = math.Min(12, p.Duration().Seconds()/2)
			p.Seek(time.Duration(a.smokeSeek * float64(time.Second)))
			p.Play()
			a.smokeStage = 1
		}
		if a.smokeStage == 1 && math.Abs(pos-a.smokeSeek) < 2 {
			a.smokeStage = 2
		}
		if a.smokeStage == 2 && pos >= a.smokeSeek+1 && a.smokeNext {
			a.smokeFirstEpisode = a.playingEpisode.ID
			a.smokeStage = 3
			p.Seek(time.Duration(math.Max(0, p.Duration().Seconds()-1) * float64(time.Second)))
			fmt.Println("SMOKE seek verified; waiting for automatic next episode")
		}
		if (a.smokeStage == 2 && pos >= a.smokeSeek+1 && !a.smokeNext) || (a.smokeStage == 3 && a.playingEpisode.ID != a.smokeFirstEpisode && pos > 1) {
			p.Pause()
			a.saveProgress()
			a.smokePassed = true
			if a.smokeNext {
				fmt.Printf("SMOKE automatic next verified: episode %d\n", a.playingEpisode.Number)
			}
			fmt.Printf("WebView media ready: duration=%.1fs seek=%.1fs position=%.1fs\n", p.Duration().Seconds(), a.smokeSeek, pos)
			mygo.App.Quit()
			return
		}
	}
	if !p.Ended() && p.Playing() {
		a.endedHandled = false
	}
	if p.Ended() && !a.endedHandled {
		a.endedHandled = true
		a.saveProgress()
		if a.autoplay {
			a.jump(1)
		}
	}
}
func (a *app) failSmoke(reason string) { fmt.Println("SMOKE:", reason); mygo.App.Quit() }
func friendly(e error) string {
	if errors.Is(e, context.Canceled) {
		return "已取消"
	}
	if errors.Is(e, context.DeadlineExceeded) {
		return "请求超时，请检查网络后重试。"
	}
	switch {
	case errors.Is(e, pluginapi.ErrAnonymousLimited):
		return "数据源暂时限制了播放，请稍后重试。"
	case errors.Is(e, pluginapi.ErrRateLimited):
		return "请求过于频繁，请稍后重试。"
	case errors.Is(e, pluginapi.ErrAuthRequired):
		return "这部内容需要登录，请在该来源的账号页登录。"
	case errors.Is(e, pluginapi.ErrContentUnavailable):
		return "内容已下架或当前不可访问。"
	case errors.Is(e, pluginapi.ErrTransport):
		return "无法连接数据源，请检查网络后重试。"
	case errors.Is(e, pluginapi.ErrUnsupported):
		return "当前插件未提供此功能。"
	}

	if strings.Contains(strings.ToLower(e.Error()), "ffmpeg") {
		return "视频准备失败，请确认已安装 FFmpeg，并检查视频编码和网络连接。"
	}
	return "操作未完成，请稍后重试或换一部短剧。"
}

// highestQuality 返回可选档位里分辨率最高的一档，用作未手动选择时的默认值。
// fallback 是片源实际选中的档位，优先采用。
func highestQuality(choices []string, fallback string) string {
	fb := strings.TrimSpace(fallback)
	if fb != "" && contains(choices, fb) {
		return fb
	}
	best, bestRank := "", -1
	for _, c := range choices {
		if c == "" || c == "自动" {
			continue
		}
		if rank := qualityRank(c); rank > bestRank {
			best, bestRank = c, rank
		}
	}
	if best == "" {
		return "自动"
	}
	return best
}

// qualityRank 从 "1080P 高清" 这类标签里取出分辨率数字用于比较。
func qualityRank(label string) int {
	rank, digits := 0, 0
	for _, r := range label {
		if r < '0' || r > '9' {
			if digits > 0 {
				break
			}
			continue
		}
		rank = rank*10 + int(r-'0')
		digits++
		if digits > 4 {
			break
		}
	}
	if digits == 0 {
		return 0
	}
	return rank
}
