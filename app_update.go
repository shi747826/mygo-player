package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type playerUpdateState struct {
	available *mygo.Update
	busy      bool
	installed bool
	message   string
}

func (a *app) playerUpdateControls(c *ui.Context) {
	pluginPageCaption(c, "播放器更新")
	state := &a.playerUpdate
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "当前版本 · "+mygo.App.Version()).Grow(1).FontSize(13).TextColor(appleInk)
		switch {
		case state.installed:
			primaryButton(c, "立即重启").Disabled(a.pluginBusy).OnClick(func() {
				a.shutdown()
				mygo.App.Relaunch()
			})
		case state.available != nil:
			primaryButton(c, "下载并安装 "+state.available.Version).Disabled(state.busy || a.pluginBusy).OnClick(a.installPlayerUpdate)
		default:
			button(c, "检查更新").Disabled(state.busy || a.pluginBusy).OnClick(a.checkPlayerUpdate)
		}
	})
	if state.message != "" {
		ui.Text(c, state.message).FontSize(12).TextColor(appleMuted).Wrap()
	}
}

func (a *app) checkPlayerUpdate() {
	state := &a.playerUpdate
	if a.win == nil || state.busy || a.pluginBusy || a.pluginUpdating || state.installed {
		return
	}
	state.busy, state.message = true, "正在检查播放器更新…"
	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, 40*time.Second)
		defer cancel()
		update, err := mygo.Updater.Check(ctx)
		a.win.Update(func() {
			if a.closed {
				return
			}
			state.busy, state.available = false, update
			switch {
			case errors.Is(err, mygo.ErrUpdatesDisabled):
				state.message = "当前版本或安装位置不支持应用内更新。请从 GitHub Releases 安装正式版，并确认安装位置可写。"
			case err != nil && (strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "403")):
				state.message = "读取 GitHub 发布信息失败。私有仓库需要先安装并登录 GitHub CLI，或设置 GH_TOKEN。"
			case err != nil:
				state.message = "检查播放器更新失败 · " + err.Error()
			case update == nil:
				state.message = "已是最新版本。"
			default:
				state.message = "有新版本可安装。"
			}
		})
	}()
}

// checkPlayerUpdateOnLaunch looks for a newer release without interrupting the
// user; failures stay silent and the update page still offers a manual check.
func (a *app) checkPlayerUpdateOnLaunch() {
	if a.win == nil || a.closed || !mygo.Updater.Enabled() {
		return
	}
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()
	update, err := mygo.Updater.Check(ctx)
	if err != nil || update == nil {
		return
	}
	a.win.Update(func() {
		if a.closed || a.playerUpdate.available != nil || a.playerUpdate.installed {
			return
		}
		a.playerUpdate.available = update
		a.playerUpdate.message = "发现新版本 " + update.Version + "，可在这里更新。"
	})
}

func (a *app) installPlayerUpdate() {
	state := &a.playerUpdate
	if a.win == nil || state.busy || a.pluginBusy || a.pluginUpdating || state.installed || state.available == nil {
		return
	}
	update := state.available
	state.busy, state.message = true, "正在下载播放器更新…"
	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, 10*time.Minute)
		defer cancel()
		var lastPercent int64 = -1
		err := update.Install(ctx, func(downloaded, total int64) {
			percent := 100 * downloaded / total
			if percent == lastPercent || (percent%5 != 0 && percent < 100) {
				return
			}
			lastPercent = percent
			a.win.Update(func() {
				if !a.closed && state.busy {
					state.message = fmt.Sprintf("正在下载播放器更新 · %d%%", min(100, percent))
				}
			})
		})
		a.win.Update(func() {
			if a.closed {
				return
			}
			state.busy = false
			if err != nil {
				state.message = "安装播放器更新失败 · " + err.Error()
				return
			}
			state.installed = true
			state.message = "更新已准备好，重启播放器后生效。"
		})
	}()
}
