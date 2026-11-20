package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/egoist/mygo"

	"mygo-player/internal/plugins"
)

func (a *app) setPluginEnabled(id string, enabled bool) {
	message := "插件已关闭"
	if enabled {
		message = "插件已开启"
	}
	a.changePlugins(func() error { return a.pluginManager.SetEnabled(id, enabled) }, message)
}

func (a *app) choosePluginPackage(directory bool) {
	a.choosePluginPackageFor("", directory)
}

func (a *app) choosePluginPackageFor(id string, directory bool) {
	if a.win == nil || a.closed || a.pluginBusy {
		return
	}
	opts := mygo.OpenDialogOptions{Parent: a.win, Title: "选择插件 ZIP", ButtonLabel: "安装"}
	if id != "" {
		opts.ButtonLabel = "更新"
	}
	if directory {
		opts.Title = "选择插件文件夹"
		opts.Directory = true
	} else {
		opts.Filters = []mygo.FileFilter{{Name: "插件 ZIP", Extensions: []string{"zip"}}}
	}
	paths, err := mygo.Dialog.Open(opts)
	if a.closed {
		return
	}
	if err != nil {
		a.pluginMessage = "无法打开文件选择器：" + err.Error()
		return
	}
	if len(paths) > 0 {
		if id == "" {
			a.installPlugin(paths[0])
		} else {
			a.updatePlugin(id, paths[0])
		}
	}
}

func (a *app) installPlugin(path string) {
	if path == "" {
		return
	}
	a.installOrUpdatePlugin("", path)
}

func (a *app) updatePlugin(id, input string) {
	a.installOrUpdatePlugin(id, input)
}

func (a *app) installOrUpdatePlugin(id, input string) {
	if a.pluginManager == nil || a.closed || a.pluginBusy || a.playerUpdate.busy {
		return
	}
	updating := id != ""
	ctx, cancel := context.WithCancel(a.ctx)
	a.pluginInstallCancel = cancel
	a.pluginUpdating = updating
	message := "插件已安装，开启后即可使用"
	if updating {
		message = "插件已更新"
	}
	a.changePlugins(func() error {
		defer cancel()
		progress := func(stage string) {
			message := pluginInstallStage(stage)
			if updating && stage == "github:install" {
				message = "下载完成，正在校验并更新插件…"
			} else if updating && stage == "github:complete" {
				message = "插件更新完成，正在刷新列表…"
			}
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
		}
		if updating {
			_, err := a.pluginManager.UpdateFromWithProgress(ctx, id, input, progress)
			return err
		}
		_, err := a.pluginManager.InstallFromWithProgress(ctx, input, progress)
		return err
	}, message)
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
		if a.pluginUpdating {
			a.pluginMessage = "正在取消更新…"
		}
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
	if a.pluginManager == nil || a.pluginBusy || a.playerUpdate.busy {
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
			updating := a.pluginUpdating
			a.pluginUpdating = false
			a.pluginMessage = message
			switch {
			case errors.Is(err, plugins.ErrNoPluginUpdate):
				a.pluginMessage = "未找到比当前版本更新的插件"
			case updating && errors.Is(err, context.Canceled):
				a.pluginMessage = "已取消更新"
			case errors.Is(err, context.Canceled):
				a.pluginMessage = "已取消安装，可重试或安装本地 ZIP"
			case updating && errors.Is(err, context.DeadlineExceeded):
				a.pluginMessage = "插件更新超时，可重试或选择新版 ZIP"
			case errors.Is(err, context.DeadlineExceeded):
				a.pluginMessage = "连接 GitHub 超时，请检查系统代理后重试，或下载插件 ZIP 后本地安装"
			case err != nil:
				a.pluginMessage = "插件操作失败：" + err.Error()
			case updating:
				a.pluginUpdateID, a.pluginUpdateLink = "", ""
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
