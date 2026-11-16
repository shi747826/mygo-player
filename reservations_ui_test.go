package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"mygo-player/internal/catalog"
)

func TestReservationDateText(t *testing.T) {
	local := time.Date(2026, 11, 15, 0, 30, 0, 0, reservationShanghai)
	for _, tc := range []struct {
		raw, want string
	}{
		{"", "上线时间待公布"},
		{"  ", "上线时间待公布"},
		{"0", "上线时间待公布"},
		{"-1", "上线时间待公布"},
		{"not-a-date", "上线时间待公布"},
		{"999999999999999999999999", "上线时间待公布"},
		{"2026-02-30", "上线时间待公布"},
		{"2026-11-15", "2026-11-15"},
		{" 2026-11-15 ", "2026-11-15"},
		{"20261115", "2026-11-15"},
		{"2026-11-15 08:00:00", "2026-11-15"},
		{"2026-11-14T16:30:00Z", "2026-11-15"},
		{"2026-11-15T00:30:00.123+08:00", "2026-11-15"},
		{fmt.Sprint(local.Unix()), "2026-11-15"},
		{fmt.Sprint(local.UnixMilli()), "2026-11-15"},
		{"预计11月18日上线", "预计11月18日上线"},
		{"2026年11月18日上线", "2026年11月18日上线"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			if got := reservationDateText(tc.raw); got != tc.want {
				t.Fatalf("reservationDateText(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
	if got := reservationScheduleText("预计11月18日上线"); got != "预计11月18日上线" {
		t.Fatalf("announcement should not get a duplicate prefix: %q", got)
	}
	if got := reservationScheduleText("2026-11-15"); got != "预计上线：2026-11-15" {
		t.Fatalf("calendar date needs a release label: %q", got)
	}
}

// No login, media URLs or network requests are needed for these page states.
func reservationUITestApp(t *testing.T) *app {
	t.Helper()
	client, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(client, nil)
	t.Cleanup(a.cancel)
	installAccountTestSource(t, a)
	a.accountProfile = accountProfile{UserID: "fixture-user", Nickname: "测试用户"}
	return a
}

func reservationUITester(a *app) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) {
		c.SetTheme(palette)
		ui.Column(c).Fill().Children(func() { a.reservationsView(c) })
	}, 1000, 760)
}

func TestReservationsUILoginGate(t *testing.T) {
	a := reservationUITestApp(t)
	// Stale account data must not leak into a signed-out page.
	a.reservationsLoaded = true
	a.reservations = []accountShelfItem{{SeriesID: "private", Title: "旧账号预约"}}
	tt := reservationUITester(a)
	if !tt.HasText("登录后查看个人预约") || !tt.HasText("前往登录") {
		t.Fatal("signed-out page must offer login")
	}
	if tt.HasText("旧账号预约") || tt.HasText("你的账号还没有预约短剧") {
		t.Fatal("signed-out page must not show private data or a successful empty state")
	}
}

func TestReservationsUIOnlySuccessfulEmptyIsEmpty(t *testing.T) {
	a := reservationUITestApp(t)
	a.accountLoggedIn = true
	tt := reservationUITester(a)
	if !tt.HasText("尚未读取个人预约") || tt.HasText("你的账号还没有预约短剧") {
		t.Fatal("not-yet-loaded must not be presented as an empty account")
	}
	a.reservationBusy = true
	tt = reservationUITester(a)
	if !tt.HasText("正在读取个人预约…") || tt.HasText("你的账号还没有预约短剧") {
		t.Fatal("loading must not be presented as an empty account")
	}
	a.reservationBusy = false
	a.reservationError = "账号身份暂时不可用"
	tt = reservationUITester(a)
	if !tt.HasText("个人预约读取失败") || !tt.HasText("账号身份暂时不可用") || !tt.HasText("重试") {
		t.Fatal("error page must explain the failure and offer retry")
	}
	if tt.HasText("你的账号还没有预约短剧") {
		t.Fatal("request failure must not look like an empty account")
	}
	a.reservationError = ""
	a.reservationsLoaded = true
	tt = reservationUITester(a)
	if !tt.HasText("你的账号还没有预约短剧") || !tt.HasText("测试用户的个人预约") {
		t.Fatal("successful empty account must be clear and account-scoped")
	}
}

func TestReservationsUIShowsPersonalStatusAndDates(t *testing.T) {
	a := reservationUITestApp(t)
	a.accountLoggedIn = true
	a.reservationsLoaded = true
	a.reservations = []accountShelfItem{
		{SeriesID: "released", Title: "已上线预约示例", Episodes: 24, Online: true, PublishAt: "2026-11-14"},
		{SeriesID: "announced", Title: "有日期的预约示例", PublishAt: "2026-11-18"},
		{SeriesID: "unknown", Title: "尚未公布日期的示例"},
	}
	tt := reservationUITester(a)
	for _, label := range []string{"共 3 部个人预约", "已上线预约示例", "已上线", "上线日期：2026-11-14", "24 集", "有日期的预约示例", "待上线", "预计上线：2026-11-18", "上线时间待公布"} {
		if !tt.HasText(label) {
			t.Fatalf("missing personal reservation information %q", label)
		}
	}
	if _, ok := tt.Access("播放预约 已上线预约示例"); !ok {
		t.Fatal("released reservation must offer a play action")
	}
	for _, label := range []string{"播放预约 有日期的预约示例", "播放预约 尚未公布日期的示例"} {
		if _, ok := tt.Access(label); ok {
			t.Fatalf("unreleased reservation must not offer playback: %s", label)
		}
	}
	for _, label := range []string{"上新日历", "即将上线日历", "公共上新日历"} {
		if tt.HasText(label) {
			t.Fatalf("personal page must not substitute a public calendar: %s", label)
		}
	}
}
