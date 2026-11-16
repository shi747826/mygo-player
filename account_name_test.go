package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"mygo-player/internal/plugins"
	"mygo-player/pluginapi"
)

func installAccountTestSource(t *testing.T, a *app) {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, "plugins", "fixture-account")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := plugins.Manifest{ID: "fixture-account", Name: "测试源", Version: "1", Runtime: plugins.Runtime{Type: "exec", ProtocolVersion: pluginapi.ProtocolVersion, Executable: "worker"}, Capabilities: []string{"detail", "resolve", "account", "reservations", "favoritesSync"}}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "worker"), []byte("fixture only"), 0700); err != nil {
		t.Fatal(err)
	}
	manager, err := plugins.LoadManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Close() })
	a.pluginManager, a.source = manager, "fixture-account"
}

func TestAccountProfileDisplay(t *testing.T) {
	for _, tc := range []struct {
		profile accountProfile
		want    string
	}{
		{accountProfile{Nickname: "测试用户", UserID: "42"}, "测试用户"},
		{accountProfile{UserID: "42"}, "用户42"},
		{accountProfile{}, "已登录用户"},
	} {
		if got := tc.profile.Display(); got != tc.want {
			t.Fatalf("Display()=%q want %q", got, tc.want)
		}
	}
}

func TestAccountSourceResetClearsPrivateStateAndCancelsRequests(t *testing.T) {
	login, cancelLogin := context.WithCancel(context.Background())
	shelf, cancelShelf := context.WithCancel(context.Background())
	reservations, cancelReservations := context.WithCancel(context.Background())
	a := &app{accountCancel: cancelLogin, accShelfCancel: cancelShelf, reservationCancel: cancelReservations, accountLoggedIn: true, accountBusy: true, accShelfBusy: true, accountProfile: accountProfile{Nickname: "私有用户"}, accountQR: []byte("qr"), accSyncNote: "old", reservations: []accountShelfItem{{SeriesID: "private"}}, reservationsLoaded: true}
	a.resetSourceAccount()
	for _, ctx := range []context.Context{login, shelf, reservations} {
		if ctx.Err() != context.Canceled {
			t.Fatal("source reset must cancel all private requests")
		}
	}
	if a.accountLoggedIn || a.accountBusy || a.accShelfBusy || a.accountProfile != (accountProfile{}) || len(a.accountQR) > 0 || len(a.reservations) > 0 || a.reservationsLoaded || a.accSyncNote != "" {
		t.Fatalf("source reset leaked private state: %+v", a)
	}
	if a.accountGen == 0 || a.reservationGen == 0 {
		t.Fatal("source reset must invalidate queued callbacks")
	}
}
