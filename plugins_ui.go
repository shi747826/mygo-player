package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"mygo-player/internal/plugins"
)

func (a *app) setPluginEnabled(id string, enabled bool) {
	message := "插件已关闭"
	if enabled {
		message = "插件已开启"
	}
	a.changePlugins(func() error { return a.pluginManager.SetEnabled(id, enabled) }, message)
}
func (a *app) installPlugin() {
	if a.pluginManager == nil || a.pluginBusy {
		return
	}
	path := strings.TrimSpace(a.pluginPath)
	ctx, cancel := context.WithCancel(a.ctx)
	a.pluginInstallCancel = cancel
	a.changePlugins(func() error {
		defer cancel()
		_, err := a.pluginManager.InstallFromWithProgress(ctx, path, func(stage string) {
			message := pluginInstallStage(stage)
			apply := func() {
				if !a.closed && a.pluginBusy && ctx.Err() == nil {
					a.pluginMessage = message
				}
			}
			if a.win != nil {
				a.win.Update(apply)
			} else {
				apply()
			}
		})
		return err
	}, "插件已安装，开启后即可使用")
}

func pluginInstallStage(stage string) string {
	switch stage {
	case "github:credentials":
		return "正在读取 GitHub 登录凭据…"
	case "github:release":
		return "正在连接 GitHub，获取插件版本…"
	case "github:download":
		return "正在下载插件安装包…"
	case "github:install":
		return "下载完成，正在校验并安装…"
	case "github:complete":
		return "插件安装完成，正在刷新列表…"
	default:
		return "正在安装插件…"
	}
}

func (a *app) cancelPluginInstall() {
	if a.pluginInstallCancel != nil {
		a.pluginInstallCancel()
		a.pluginMessage = "正在取消安装…"
	}
}
func (a *app) uninstallPlugin(id string) {
	a.changePlugins(func() error { return a.pluginManager.Uninstall(id) }, "插件已卸载，本地观看记录已保留")
}
func (a *app) reloadPlugins() {
	a.changePlugins(func() error { return a.pluginManager.Reload() }, "插件列表已刷新")
}

// Stop users of a process before replacing packages. Package IO and handshakes
// happen away from the UI thread; the complete new registry is applied together.
func (a *app) changePlugins(work func() error, message string) {
	if a.pluginManager == nil || a.pluginBusy {
		return
	}
	a.pluginBusy = true
	a.pluginMessage = "正在更新插件…"
	previous := a.source
	if a.sourceSearch == nil {
		a.sourceSearch = map[string]string{}
	}
	a.sourceSearch[previous] = a.searchedQuery
	a.resetSourceAccount()
	a.playID++
	a.stopPlayer()
	a.clearPreloads()
	if a.listCancel != nil {
		a.listCancel()
	}
	if a.detailCancel != nil {
		a.detailCancel()
	}
	if a.prefetchCancel != nil {
		a.prefetchCancel()
	}
	a.listID++
	a.detailID++
	a.prefetchGen++
	a.busy = false
	a.detailBusy = false
	a.prefetching = false
	a.calendar = nil
	a.calendarBusy = false
	a.selected.ID = ""
	a.items = nil
	a.episodes = nil
	a.seasons = nil
	run := func() {
		_ = a.pluginManager.Close()
		err := work()
		clients := map[string]*plugins.Client{}
		for _, m := range a.pluginManager.Manifests() {
			if !a.pluginManager.Enabled(m.ID) {
				continue
			}
			c, e := a.pluginManager.Open(a.ctx, m.ID)
			if e != nil {
				if err == nil {
					err = fmt.Errorf("%s: %w", m.Name, e)
				}
				continue
			}
			clients[m.ID] = c
		}
		apply := func() {
			if a.closed {
				return
			}
			a.pluginHostMu.Lock()
			a.pluginClients = clients
			a.source = ""
			if clients[previous] != nil {
				a.source = previous
			} else {
				for _, m := range a.pluginManager.Manifests() {
					if clients[m.ID] != nil {
						a.source = m.ID
						break
					}
				}
			}
			a.pluginHostMu.Unlock()
			if a.source != previous {
				a.query = ""
				a.searchedQuery = a.sourceSearch[a.source]
				a.channel, a.hgSort = "", ""
				a.channelSel, a.hgSortSel = 0, 0
			}
			if e := a.ensureSourceStore(a.source); e != nil && err == nil {
				err = e
			}
			a.refreshStore()
			a.pluginBusy = false
			a.pluginInstallCancel = nil
			a.pluginMessage = message
			if errors.Is(err, context.Canceled) {
				a.pluginMessage = "已取消安装，可重试或安装本地 ZIP"
			} else if errors.Is(err, context.DeadlineExceeded) {
				a.pluginMessage = "连接 GitHub 超时，请检查系统代理后重试，或下载插件 ZIP 后本地安装"
			} else if err != nil {
				a.pluginMessage = "插件操作失败：" + err.Error()
			}
			a.restoreAccount()
		}
		if a.win != nil {
			a.win.Update(apply)
		} else {
			apply()
		}
	}
	if a.win != nil {
		go run()
	} else {
		run()
	}
}

func (a *app) switchPlugin(id string) {
	if a.pluginBusy || a.pluginManager == nil || !a.pluginManager.Enabled(id) || a.pluginClients[id] == nil {
		return
	}
	if err := a.ensureSourceStore(id); err != nil {
		a.pluginMessage = err.Error()
		return
	}
	a.stopPlayer()
	a.clearPreloads()
	a.resetSourceAccount()
	a.playID++
	if a.prefetchCancel != nil {
		a.prefetchCancel()
	}
	a.prefetchGen++
	a.prefetching = false
	if a.detailCancel != nil {
		a.detailCancel()
	}
	a.detailID++
	a.calendar = nil
	a.calendarBusy = false
	if a.sourceSearch == nil {
		a.sourceSearch = map[string]string{}
	}
	a.sourceSearch[a.source] = a.searchedQuery
	a.searchedQuery = a.sourceSearch[id]
	a.pluginHostMu.Lock()
	a.source = id
	a.pluginHostMu.Unlock()
	a.query = ""
	a.channel = ""
	a.channelSel = 0
	a.hgSort = ""
	a.hgSortSel = 0
	a.calendar = nil
	a.calendarBusy = false
	a.selected.ID = ""
	a.episodes = nil
	a.seasons = nil
	a.showPlayer = false
	a.refreshStore()
	a.restoreAccount()
	a.loadPage("推荐", false)
}
