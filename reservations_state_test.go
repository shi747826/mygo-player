package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"mygo-player/internal/catalog"
)

func TestReservationsNavigationAndSignOut(t *testing.T) {
	client, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(client, nil)
	defer a.cancel()
	installAccountTestSource(t, a)
	a.accountLoggedIn = true
	a.accountProfile = accountProfile{UserID: "fixture-user", Nickname: "测试用户"}
	a.reservations = []accountShelfItem{{SeriesID: "fixture", Title: "私有预约", PublishAt: "2026-11-20"}}
	a.reservationsLoaded = true
	tt := ui.NewTester(a.view, 1280, 820)
	if !tt.HasText("预约") {
		t.Fatal("personal source must expose reservations menu")
	}
	if err := tt.Click("预约"); err != nil {
		t.Fatal(err)
	}
	if a.page != "预约" || a.busy {
		t.Fatal("reservation menu must use account reader, not catalog discovery")
	}
	if !tt.HasText("私有预约") || !tt.HasText("预计上线：2026-11-20") {
		t.Fatal("menu did not display personal list")
	}
	a.signOutAccount()
	if len(a.reservations) != 0 || a.reservationsLoaded {
		t.Fatal("signout must clear private list")
	}
	tt = ui.NewTester(a.view, 1280, 820)
	if tt.HasText("私有预约") || !tt.HasText("登录后查看个人预约") {
		t.Fatal("signed-out view leaked private reservations")
	}
	a.source = "fixture-anonymous"
	a.page = "推荐"
	tt = ui.NewTester(a.view, 1280, 820)
	if tt.HasText("预约") {
		t.Fatal("other data source must not expose account reservations")
	}
}

func TestReservationsUpcomingDatesSortBeforeReleased(t *testing.T) {
	rows := []accountShelfItem{
		{SeriesID: "released", Online: true},
		{SeriesID: "unknown"},
		{SeriesID: "later", PublishAt: "2026-11-22"},
		{SeriesID: "soon", PublishAt: "2026-11-17"},
	}
	sortReservations(rows)
	want := []string{"soon", "later", "unknown", "released"}
	for i, id := range want {
		if rows[i].SeriesID != id {
			t.Fatalf("position %d=%s want %s", i, rows[i].SeriesID, id)
		}
	}
}
