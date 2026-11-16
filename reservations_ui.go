package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"mygo-player/internal/catalog"
)

var reservationShanghai = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return loc
}()

var reservationChineseDate = regexp.MustCompile(`(?:\d{4}年)?\d{1,2}月\d{1,2}[日号]`)

// reservationDateText formats account-provided dates only. A missing release
// date is unknown; it must not be inferred from the public release calendar.
func reservationDateText(raw string) string {
	const unknown = "上线时间待公布"
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return unknown
	}
	if stamp, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if stamp <= 0 {
			return unknown
		}
		// Eight digits are commonly a calendar date, rather than Unix seconds.
		if len(raw) == 8 {
			if d, err := time.ParseInLocation("20060102", raw, reservationShanghai); err == nil {
				return d.Format("2006-01-02")
			}
		}
		var d time.Time
		if stamp >= 1_000_000_000_000 {
			d = time.UnixMilli(stamp)
		} else {
			d = time.Unix(stamp, 0)
		}
		if d.Year() < 2000 || d.Year() > 2200 {
			return unknown
		}
		return d.In(reservationShanghai).Format("2006-01-02")
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02", "2006/01/02"} {
		if d, err := time.ParseInLocation(layout, raw, reservationShanghai); err == nil {
			return d.In(reservationShanghai).Format("2006-01-02")
		}
	}
	// Keep the service's wording (e.g. "预计11月18日上线") rather than
	// guessing a year for a date that did not include one.
	if reservationChineseDate.MatchString(raw) {
		return raw
	}
	return unknown
}

func reservationScheduleText(raw string) string {
	text := reservationDateText(raw)
	if text == "上线时间待公布" || strings.Contains(text, "上线") || strings.Contains(text, "开播") {
		return text
	}
	return "预计上线：" + text
}

// reservationsView displays only the signed-in account's own reservations.
// The surrounding page already supplies navigation, a header and a footer.
func (a *app) reservationsView(c *ui.Context) {
	ui.Column(c).Grow(1).FillWidth().Padding(0, 24, 12, 24).Gap(14).MinHeight(0).Children(func() {
		if !a.sourceHas("reservations") {
			ui.Text(c, "当前源不支持个人预约").TextColor(appleMuted)
			return
		}
		if !a.accountLoggedIn {
			ui.Column(c).FillWidth().Grow(1).Center().Gap(12).Children(func() {
				symbol(c, "person.crop.circle", 32, appleMuted)
				ui.Text(c, "登录后查看个人预约").FontSize(20).FontWeight(600).TextColor(appleInk)
				ui.Text(c, "查看你在当前源账号中预约的短剧及上线时间。").FontSize(13).TextColor(appleMuted).Wrap()
				primaryButton(c, "前往登录").OnClick(func() { a.loadPage("账号", false) })
			})
			return
		}

		ui.Text(c, a.accountProfile.Display()+"的个人预约").FontSize(16).FontWeight(600).TextColor(appleInk).MaxLines(1).MinWidth(0)
		if a.reservationBusy {
			ui.Column(c).Grow(1).FillWidth().Center().Gap(12).Children(func() {
				ui.Spinner(c)
				ui.Text(c, "正在读取个人预约…").FontSize(14).TextColor(appleMuted)
			})
			return
		}
		if a.reservationError != "" {
			ui.Column(c).FillWidth().Padding(20).Gap(12).Radius(14).Background(appleSurface).Border(1, appleHairline).Children(func() {
				ui.Text(c, "个人预约读取失败").FontSize(18).FontWeight(600).TextColor(appleInk)
				ui.Text(c, a.reservationError).FontSize(13).TextColor(appleMuted).Wrap()
				primaryButton(c, "重试").OnClick(func() { a.loadReservations() })
			})
			return
		}
		if !a.reservationsLoaded {
			ui.Column(c).Grow(1).FillWidth().Center().Gap(12).Children(func() {
				ui.Text(c, "尚未读取个人预约").FontSize(18).FontWeight(600).TextColor(appleInk)
				primaryButton(c, "读取个人预约").OnClick(func() { a.loadReservations() })
			})
			return
		}
		if len(a.reservations) == 0 {
			ui.Column(c).Grow(1).FillWidth().Center().Gap(10).Children(func() {
				ui.Text(c, "你的账号还没有预约短剧").FontSize(20).FontWeight(600).TextColor(appleInk)
				ui.Text(c, "在当前源对应的 App 中预约后，可以在这里刷新查看。").FontSize(13).TextColor(appleMuted).Wrap()
			})
			return
		}
		ui.Text(c, fmt.Sprintf("共 %d 部个人预约", len(a.reservations))).FontSize(12).TextColor(appleMuted)
		ui.Scroll(c.Key("personal-reservations-list")).Grow(1).FillWidth().MinHeight(0).Children(func() {
			ui.Column(c).FillWidth().Gap(12).Children(func() {
				for _, item := range a.reservations {
					a.reservationRow(c, item)
				}
			})
		})
	})
}

func (a *app) reservationRow(c *ui.Context, item accountShelfItem) {
	title := strings.TrimSpace(item.Title)
	if title == "" {
		title = "未命名短剧"
	}
	ui.Row(c.Key("reservation-"+item.SeriesID)).FillWidth().Padding(12).Gap(16).AlignItems(ui.Center).
		Radius(14).Background(appleSurface).Border(1, appleHairline).Children(func() {
		ui.Box(c).Width(72).Height(96).Radius(8).Background(appleParchment).Center().Children(func() {
			if b := a.cover(item.Cover); b != nil {
				ui.Image(c, b).Fill().Fit(ui.Cover).Radius(8)
			} else {
				ui.Text(c, posterText(title)).FontSize(15).FontWeight(600).TextColor(appleMuted).MaxLines(2).Wrap()
			}
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(8).Children(func() {
			ui.Text(c, title).FontSize(16).FontWeight(600).TextColor(appleInk).MaxLines(2).Wrap()
			if item.Online {
				ui.Text(c, "已上线").FontSize(12).FontWeight(600).TextColor(appleBlue)
				if date := reservationDateText(item.PublishAt); date != "上线时间待公布" {
					ui.Text(c, "上线日期："+date).FontSize(13).TextColor(appleMuted).Wrap()
				}
			} else {
				ui.Text(c, "待上线").FontSize(12).FontWeight(600).TextColor(appleMuted)
				ui.Text(c, reservationScheduleText(item.PublishAt)).FontSize(13).TextColor(appleMuted).Wrap()
			}
			if item.Episodes > 0 {
				ui.Text(c, fmt.Sprintf("%d 集", item.Episodes)).FontSize(12).TextColor(appleMuted)
			}
		})
		if item.Online && strings.TrimSpace(item.SeriesID) != "" {
			primaryButton(c, "播放").Label("播放预约 " + title).OnClick(func() {
				a.selectSeries(catalog.Series{ID: item.SeriesID, Title: title, Cover: item.Cover, Episodes: item.Episodes}, true)
			})
		}
	})
}
