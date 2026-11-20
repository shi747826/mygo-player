// SPDX-License-Identifier: AGPL-3.0-only
// Settings layout follows egoist/mygo-music; adapted for executable video plugins.
package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

// pluginSettingsView lives beneath the host header, which supplies the page title.
func (a *app) pluginSettingsView(c *ui.Context) {
	ui.Scroll(c.Key("plugin-settings-page")).Grow(1).FillWidth().MinHeight(0).
		Padding(24, 28, 32, 28).Gap(8).Background(appleSurface).Children(func() {
		a.playerUpdateControls(c)
		if a.pluginManager == nil {
			ui.Text(c, "插件暂不可用").FontSize(13.5).FontWeight(500).TextColor(appleInk)
			ui.Text(c, "重新打开应用后再试。").FontSize(12).TextColor(appleMuted)
			return
		}

		manifests := a.pluginManager.Manifests()
		currentName := "尚未选择来源"
		current := false
		for _, m := range manifests {
			if a.source == m.ID && a.pluginManager.Enabled(m.ID) {
				currentName, current = m.Name, true
				break
			}
		}

		pluginPageCaption(c, "当前来源")
		ui.Row(c).FillWidth().Padding(12, 16).Gap(12).AlignItems(ui.Center).
			Radius(12).Border(1, appleHairline).Children(func() {
			ui.Box(c).Size(32, 32).Shrink(0).Radius(8).Background(appleBlue.Alpha(0.08)).Center().Children(func() {
				symbol(c, "play.rectangle", 18, appleBlue)
			})
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, currentName).FontSize(13.5).FontWeight(600).TextColor(appleInk).MaxLines(1)
				desc := "开启下方插件，再点击「使用」选择来源。"
				if current {
					desc = "浏览与搜索使用此来源。"
				}
				ui.Text(c, desc).FontSize(12).TextColor(appleMuted).Wrap()
			})
			if current {
				symbol(c, "checkmark", 15, appleBlue)
			}
		})

		pluginPageCaption(c, "安装来源插件")
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			primaryButton(c, "选择插件 ZIP…").Key("pick-plugin-zip").Disabled(a.pluginBusy).OnClick(func() { a.choosePluginPackage(false) })
			button(c, "选择文件夹…").Key("pick-plugin-folder").Disabled(a.pluginBusy).OnClick(func() { a.choosePluginPackage(true) })
			if a.pluginBusy && a.pluginInstallCancel != nil && !a.pluginUpdating {
				button(c, "取消安装").Key("cancel-plugin-install").OnClick(a.cancelPluginInstall)
			} else {
				button(c, "刷新").Key("reload-plugins").Disabled(a.pluginBusy).OnClick(a.reloadPlugins)
			}
		})
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.TextInput(c.Key("plugin-package-path"), &a.pluginPath).Grow(1).MinWidth(0).Placeholder("粘贴 GitHub 仓库或 Release 链接").Label("GitHub 链接")
			button(c, "从链接安装").Key("install-plugin").Disabled(a.pluginBusy || strings.TrimSpace(a.pluginPath) == "").OnClick(func() { a.installPlugin(strings.TrimSpace(a.pluginPath)) })
		})
		ui.Text(c, "私有仓库使用已登录的 GitHub CLI 或 GH_TOKEN。").FontSize(12).TextColor(appleMuted).Wrap()

		if a.pluginMessage != "" {
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, a.pluginMessage).Grow(1).MinWidth(0).FontSize(12).TextColor(appleBlue).Wrap().Margin(4, 4, 0, 4)
				if a.pluginBusy && a.pluginUpdating && a.pluginInstallCancel != nil {
					button(c, "取消更新").Key("cancel-plugin-update").OnClick(a.cancelPluginInstall)
				}
			})
		}

		pluginPageCaption(c, "已安装插件")
		ui.Column(c).FillWidth().Radius(12).Background(appleSurface).
			Border(1, appleHairline).Dividers(1, appleHairline).Children(func() {
			if len(manifests) == 0 {
				ui.Text(c, "暂无已安装插件").Padding(12, 16).FontSize(13).TextColor(appleMuted)
			}
			for _, entry := range manifests {
				m := entry
				enabled := a.pluginManager.Enabled(m.ID)
				selected := enabled && a.source == m.ID
				ui.Column(c.Key("plugin-row-" + m.ID)).FillWidth().Children(func() {
					ui.Row(c).FillWidth().Padding(12, 16).Gap(12).AlignItems(ui.Center).MinHeight(64).Children(func() {
						tint := appleMuted
						if enabled {
							tint = appleBlue
						}
						ui.Box(c).Size(32, 32).Shrink(0).Radius(8).Background(appleCanvas).Center().Children(func() {
							symbol(c, "film", 18, tint)
						})
						ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
							ui.Text(c, m.Name).FontSize(13.5).FontWeight(500).TextColor(appleInk).MaxLines(1)
							status := "已关闭"
							if selected {
								status = "当前来源"
							} else if enabled {
								status = "已开启"
							}
							statusColor := appleMuted
							if selected {
								statusColor = appleBlue
							}
							ui.Text(c, fmt.Sprintf("v%s · %s", m.Version, status)).FontSize(12).TextColor(statusColor).MaxLines(1)
						})
						ui.Row(c).Gap(8).Shrink(0).AlignItems(ui.Center).Children(func() {
							if enabled {
								label := "使用"
								if selected {
									label = "使用中"
								}
								button(c, label).Key("use-plugin-" + m.ID).Disabled(selected || a.pluginBusy).OnClick(func() { a.chooseSource(m.ID) })
								button(c, "关闭").Key("disable-plugin-" + m.ID).Disabled(a.pluginBusy).OnClick(func() { a.setPluginEnabled(m.ID, false) })
							} else {
								primaryButton(c, "开启").Key("enable-plugin-" + m.ID).Disabled(a.pluginBusy).OnClick(func() { a.setPluginEnabled(m.ID, true) })
							}
							button(c, "更新…").Key("update-plugin-" + m.ID).Disabled(a.pluginBusy).OnClick(func() {
								a.pluginUpdateLink = ""
								if a.pluginUpdateID == m.ID {
									a.pluginUpdateID = ""
								} else {
									a.pluginUpdateID = m.ID
								}
							})
							if !a.pluginManager.IsBundled(m.ID) {
								button(c, "卸载").Key("uninstall-plugin-" + m.ID).Disabled(a.pluginBusy).OnClick(func() { a.uninstallPlugin(m.ID) })
							}
						})
					})
					if a.pluginUpdateID == m.ID {
						a.pluginUpdateOptions(c, m.ID)
					}
				})
			}
		})

		pluginPageCaption(c, "使用帮助")
		ui.Text(c, "开启插件后，点击「使用」切换来源。收藏与观看历史按来源分别保存。").
			FontSize(12).TextColor(appleMuted).LineHeight(1.4).Wrap().Margin(0, 4)
	})
}

func (a *app) pluginUpdateOptions(c *ui.Context, id string) {
	source := a.pluginManager.UpdateSource(id)
	ui.Row(c.Key("plugin-update-options-"+id)).FillWidth().Padding(0, 16, 12, 16).Gap(8).AlignItems(ui.Center).Children(func() {
		if source == "" {
			ui.TextInput(c.Key("plugin-update-link-"+id), &a.pluginUpdateLink).Grow(1).MinWidth(0).
				Placeholder("粘贴 GitHub 仓库或 Release 链接").Label("插件更新链接").Disabled(a.pluginBusy)
		} else {
			ui.Text(c, "使用已保存的 GitHub 来源").Grow(1).MinWidth(0).FontSize(12).TextColor(appleMuted).Wrap()
		}
		button(c, "检查并更新").Key("check-plugin-update-" + id).
			Disabled(a.pluginBusy || (source == "" && strings.TrimSpace(a.pluginUpdateLink) == "")).OnClick(func() {
			input := ""
			if source == "" {
				input = strings.TrimSpace(a.pluginUpdateLink)
			}
			a.updatePlugin(id, input)
		})
		button(c, "选择新版 ZIP…").Key("pick-plugin-update-" + id).Disabled(a.pluginBusy).OnClick(func() {
			a.choosePluginPackageFor(id, false)
		})
	})
}

func pluginPageCaption(c *ui.Context, text string) {
	ui.Text(c, text).FontSize(12).FontWeight(600).TextColor(appleMuted).Margin(14, 4, 4, 4)
}
