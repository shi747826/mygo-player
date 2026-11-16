package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"mygo-player/internal/plugins"
	"mygo-player/pluginapi"
)

// 账号页使用当前源的可选账户能力；登录态由插件保管。
func (a *app) accountView(c *ui.Context) {
	ui.Column(c).Grow(1).FillWidth().Padding(0, 24, 12, 24).Gap(14).MinHeight(0).Children(func() {
		if !a.sourceHas("account") {
			ui.Text(c, "当前源不支持账号登录").TextColor(appleMuted)
			return
		}
		ui.Column(c).Padding(20).Gap(12).Radius(18).Border(1, appleHairline).Background(appleSurface).MaxWidth(420).Children(func() {
			if a.accountLoggedIn {
				ui.Text(c, a.accountProfile.Display()).FontSize(18).FontWeight(600).TextColor(appleInk)
				if a.accountProfile.UserID != "" {
					ui.Text(c, "用户编号 "+a.accountProfile.UserID).FontSize(12).TextColor(appleMuted)
				}
				ui.Text(c, "已登录当前源账号，可查看账号资料。").FontSize(12).TextColor(appleMuted).Wrap()
				ui.Row(c).Gap(8).Children(func() {
					button(c, "刷新资料").Disabled(a.accountBusy).OnClick(func() { a.refreshAccountProfile() })
					button(c, "退出登录").OnClick(func() { a.signOutAccount() })
				})
				a.accountSyncSection(c)
				return
			}
			ui.Text(c, "扫码登录当前源账号").FontSize(18).FontWeight(600).TextColor(appleInk)
			ui.Text(c, "使用当前源对应的 App 扫描二维码。登录状态保存在该插件的本机私有目录。").FontSize(12).TextColor(appleMuted).Wrap()
			if len(a.accountQR) > 0 {
				if b := a.accountQRBitmap(); b != nil {
					ui.Image(c, b).Size(200, 200).Label("登录二维码")
				}
				ui.Text(c, "二维码有效期约 1 分钟").FontSize(11).TextColor(appleMuted)
			} else {
				ui.Box(c).Size(200, 200).Radius(12).Background(appleParchment).Center().Children(func() {
					if a.accountBusy {
						ui.Spinner(c)
					} else {
						ui.Text(c, "未获取二维码").FontSize(12).TextColor(appleMuted)
					}
				})
			}
			ui.Row(c).Gap(8).Children(func() {
				label := "获取二维码"
				if len(a.accountQR) > 0 {
					label = "重新获取"
				}
				button(c, label).Disabled(a.accountBusy).OnClick(func() { a.startAccountLogin() })
				if len(a.accountQR) > 0 {
					button(c, "取消").OnClick(func() { a.cancelAccountLogin() })
				}
			})
			if a.accountStatus != "" {
				ui.Text(c, a.accountStatus).FontSize(12).TextColor(appleMuted).Wrap()
			}
		})
	})
}

// accountSyncSection 说明账号收藏会并入本地「收藏」页，并提供手动同步。
func (a *app) accountSyncSection(c *ui.Context) {
	if !a.sourceHas("favoritesSync") {
		return
	}
	ui.Text(c, "账号收藏会同步到「收藏」页").FontSize(12).TextColor(appleMuted).Wrap()
	button(c, "立即同步收藏").Disabled(a.accShelfBusy).OnClick(func() { a.syncAccountFavorites() })
	if a.accSyncNote != "" {
		ui.Text(c, a.accSyncNote).FontSize(12).TextColor(appleMuted).Wrap()
	}
}

// accountQRBitmap 解码二维码 PNG；成功时缓存，失败返回 nil。
func (a *app) accountQRBitmap() *ui.Bitmap {
	if a.accountQR == nil {
		return nil
	}
	if a.accountQRImage != nil {
		return a.accountQRImage
	}
	b, err := ui.DecodeBitmap(a.accountQR)
	if err != nil {
		return nil
	}
	a.accountQRImage = b
	return b
}

// startAccountLogin requests an opaque login challenge from the active worker.
func (a *app) startAccountLogin() {
	if a.accountBusy || a.closed {
		return
	}
	client, err := a.accountPlugin("account")
	if err != nil {
		a.accountStatus = friendlyAccount(err)
		return
	}
	a.cancelAccountLogin()
	a.accountBusy = true
	a.accountStatus = "正在获取二维码…"
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Minute)
	a.accountCancel = cancel
	gen := a.accountGen
	go func() {
		defer cancel()
		var ch accountChallenge
		err := client.Invoke(ctx, "AccountLoginStart", struct{}{}, &ch)
		a.win.Update(func() {
			if a.closed || gen != a.accountGen {
				return
			}
			a.accountBusy = false
			if err != nil {
				a.accountStatus = friendlyAccount(err)
				return
			}
			a.accountQR, a.accountQRImage = ch.Image, nil
			a.accountStatus = "请用当前源对应的 App 扫码确认"
		})
		if err == nil {
			a.pollAccountLogin(ctx, gen, client, ch)
		}
	}()
}

func (a *app) pollAccountLogin(ctx context.Context, gen int, client *plugins.Client, ch accountChallenge) {
	deadline := ch.Expires
	if deadline.IsZero() || deadline.Before(time.Now()) {
		deadline = time.Now().Add(time.Minute)
	}
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	defer a.win.Update(func() {
		if !a.closed && gen == a.accountGen {
			a.accountCancel = nil
			a.accountBusy = false
		}
	})
	for {
		var status string
		err := client.Invoke(ctx, "AccountLoginWait", struct{ Token string }{ch.Token}, &status)
		if ctx.Err() != nil || status == "expired" {
			a.win.Update(func() {
				if a.closed || gen != a.accountGen {
					return
				}
				a.accountQR, a.accountQRImage = nil, nil
				if !errors.Is(ctx.Err(), context.Canceled) {
					a.accountStatus = "二维码已过期，请重新获取"
				}
			})
			return
		}
		if err != nil {
			a.win.Update(func() {
				if !a.closed && gen == a.accountGen {
					a.accountStatus = friendlyAccount(err)
				}
			})
			return
		}
		if status == "confirmed" {
			a.finishAccountLogin(ctx, gen, client)
			return
		}
		// Accept workers which return a pending status instead of long polling.
		timer := time.NewTimer(2 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (a *app) finishAccountLogin(ctx context.Context, gen int, client *plugins.Client) {
	profile, err := invokeAccountProfile(ctx, client)
	a.win.Update(func() {
		if a.closed || gen != a.accountGen {
			return
		}
		if err != nil {
			a.accountStatus = friendlyAccount(err)
			return
		}
		a.accountProfile, a.accountLoggedIn = profile, true
		a.accountQR, a.accountQRImage = nil, nil
		a.accountStatus, a.accountBusy = "", false
		a.syncAccountFavorites()
	})
}

// restoreAccount performs validation entirely off the UI thread. The worker
// restores its private session; the host never reads a cookie or device file.
func (a *app) restoreAccount() {
	if a.closed || a.win == nil {
		return
	}
	client, err := a.accountPlugin("account")
	if err != nil {
		return
	}
	a.requestAccountProfile(client, false)
}

func (a *app) refreshAccountProfile() {
	if a.accountBusy || a.closed || a.win == nil {
		return
	}
	client, err := a.accountPlugin("account")
	if err != nil {
		a.accountStatus = friendlyAccount(err)
		return
	}
	a.requestAccountProfile(client, true)
}

func (a *app) requestAccountProfile(client *plugins.Client, showError bool) {
	if a.accountCancel != nil {
		a.accountCancel()
	}
	a.accountGen++
	if a.accShelfCancel != nil {
		a.accShelfCancel()
		a.accShelfCancel = nil
	}
	a.accShelfBusy = false
	a.cancelReservationRequest()
	gen := a.accountGen
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	a.accountCancel = cancel
	a.accountBusy = true
	go func() {
		defer cancel()
		profile, err := invokeAccountProfile(ctx, client)
		a.win.Update(func() {
			if a.closed || gen != a.accountGen {
				return
			}
			a.accountBusy, a.accountCancel = false, nil
			if err != nil {
				if errors.Is(err, pluginapi.ErrAuthRequired) {
					a.accountLoggedIn = false
					a.accountProfile = accountProfile{}
					a.clearReservations()
				}
				if showError {
					a.accountStatus = friendlyAccount(err)
				}
				return
			}
			a.accountProfile, a.accountLoggedIn = profile, true
			a.accountStatus = ""
			a.syncAccountFavorites()
		})
	}()
}

// Clear private host state immediately; session deletion stays in the worker.
func (a *app) signOutAccount() {
	client, err := a.accountPlugin("account")
	a.resetSourceAccount()
	a.accountStatus = "已退出登录"
	if err != nil {
		return
	}
	gen := a.accountGen
	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
		defer cancel()
		err := client.Invoke(ctx, "AccountLogout", struct{}{}, nil)
		if err == nil || a.win == nil {
			return
		}
		// Preserve a useful failure notice without restoring any private data.
		a.win.Update(func() {
			if !a.closed && gen == a.accountGen && !a.accountLoggedIn {
				a.accountStatus = "已清除界面状态，但插件退出失败：" + friendlyAccount(err)
			}
		})
	}()
}

func (a *app) cancelAccountLogin() {
	if a.accountCancel != nil {
		a.accountCancel()
		a.accountCancel = nil
	}
	a.accountGen++
	a.accountBusy = false
	a.accountQR, a.accountQRImage = nil, nil
	a.accountStatus = "已取消扫码"
}

func friendlyAccount(err error) string {
	switch {
	case errors.Is(err, pluginapi.ErrAuthRequired):
		return "登录已过期，请重新扫码"
	case errors.Is(err, pluginapi.ErrUnsupported):
		return "当前源不支持此账号功能"
	case errors.Is(err, pluginapi.ErrUpstream):
		return "账号接口暂时不可用，请稍后重试"
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, pluginapi.ErrTimeout):
		return "请求超时，请检查网络后重试"
	case errors.Is(err, context.Canceled):
		return "已取消"
	}
	return fmt.Sprintf("账号操作未完成：%v", err)
}
