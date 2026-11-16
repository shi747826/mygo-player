package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"mygo-player/pluginapi"
)

func TestAccountNeutralDTOJSON(t *testing.T) {
	var profile accountProfile
	if err := json.Unmarshal([]byte(`{"user_id":"42","nickname":"源用户","avatar":"https://example.test/avatar"}`), &profile); err != nil {
		t.Fatal(err)
	}
	if profile.UserID != "42" || profile.Nickname != "源用户" || profile.Avatar != "https://example.test/avatar" {
		t.Fatalf("profile JSON mismatch: %+v", profile)
	}
	var challenge accountChallenge
	if err := json.Unmarshal([]byte(`{"Token":"opaque","Image":"cXI=","Expires":"2026-11-16T12:00:00Z"}`), &challenge); err != nil {
		t.Fatal(err)
	}
	if challenge.Token != "opaque" || string(challenge.Image) != "qr" || !challenge.Expires.Equal(time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("challenge JSON mismatch: %+v", challenge)
	}
	var rows []accountShelfItem
	if err := json.Unmarshal([]byte(`[{"SeriesID":"series","Title":"标题","Cover":"cover","Episodes":24,"Online":true,"PublishAt":"2026-11-20"}]`), &rows); err != nil {
		t.Fatal(err)
	}
	series := shelfToSeries(rows)
	if len(series) != 1 || series[0].ID != "series" || series[0].Title != "标题" || series[0].Cover != "cover" || series[0].Episodes != 24 || !rows[0].Online || rows[0].PublishAt != "2026-11-20" {
		t.Fatalf("shelf JSON mismatch: %+v", rows)
	}
	if got := shelfToSeries([]accountShelfItem{{Title: "missing id"}}); len(got) != 0 {
		t.Fatal("rows without identifiers must not enter favorites")
	}
}

func TestAccountNeutralErrorMessages(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{fmt.Errorf("wrapped: %w", pluginapi.ErrAuthRequired), "登录已过期，请重新扫码"},
		{pluginapi.ErrUnsupported, "当前源不支持此账号功能"},
		{pluginapi.ErrUpstream, "账号接口暂时不可用，请稍后重试"},
		{context.DeadlineExceeded, "请求超时，请检查网络后重试"},
		{context.Canceled, "已取消"},
	} {
		if got := friendlyAccount(tc.err); got != tc.want {
			t.Fatalf("friendlyAccount(%v)=%q want %q", tc.err, got, tc.want)
		}
	}
	if got := syncErrorText(pluginapi.ErrAuthRequired); got != "登录后才能同步账号收藏" {
		t.Fatal(got)
	}
	if got := reservationErrorText(pluginapi.ErrAuthRequired); got != "当前登录状态未能读取个人预约，请到账号页刷新登录后重试。" {
		t.Fatal(got)
	}
}

func TestAccountCapabilityGuards(t *testing.T) {
	a := &app{}
	if _, err := a.accountPlugin("account"); err != pluginapi.ErrUnsupported {
		t.Fatalf("unsupported source error=%v", err)
	}
	a.accountLoggedIn = true
	a.reservationsLoaded = true
	a.reservations = []accountShelfItem{{SeriesID: "private", Title: "旧源私有预约"}}
	tt := reservationUITester(a)
	if !tt.HasText("当前源不支持个人预约") || tt.HasText("旧源私有预约") {
		t.Fatal("unsupported source leaked stale private reservations")
	}
}
