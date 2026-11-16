package main

import (
	"context"
	"strings"
	"time"

	"mygo-player/internal/plugins"
	"mygo-player/pluginapi"
)

// These DTOs contain display data only. Authentication and device state belong
// to the source worker, including its private persistent data directory.
type accountProfile struct {
	UserID   string `json:"user_id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

func (p accountProfile) Display() string {
	if name := strings.TrimSpace(p.Nickname); name != "" {
		return name
	}
	if p.UserID != "" {
		return "用户" + p.UserID
	}
	return "已登录用户"
}

type accountChallenge struct {
	Token   string    `json:"Token"`
	Image   []byte    `json:"Image"`
	Expires time.Time `json:"Expires"`
}
type accountShelfItem struct {
	SeriesID  string `json:"SeriesID"`
	Title     string `json:"Title"`
	Cover     string `json:"Cover"`
	Episodes  int    `json:"Episodes"`
	Online    bool   `json:"Online"`
	PublishAt string `json:"PublishAt"`
}

func (a *app) accountPlugin(capability string) (*plugins.Client, error) {
	if !a.sourceHas(capability) {
		return nil, pluginapi.ErrUnsupported
	}
	client, ok := a.sourceAPI().(*plugins.Client)
	if !ok || client == nil {
		return nil, pluginapi.ErrUnsupported
	}
	return client, nil
}

// resetSourceAccount must run before changing the active source. Generation
// checks discard callbacks already queued by a worker that is being replaced.
func (a *app) resetSourceAccount() {
	a.cancelAccountLogin()
	if a.accShelfCancel != nil {
		a.accShelfCancel()
		a.accShelfCancel = nil
	}
	a.accShelfBusy = false
	a.accSyncNote = ""
	a.accountLoggedIn = false
	a.accountProfile = accountProfile{}
	a.accountStatus = ""
	a.clearReservations()
}

func invokeAccountProfile(ctx context.Context, client *plugins.Client) (accountProfile, error) {
	var profile accountProfile
	err := client.Invoke(ctx, "AccountProfile", struct{}{}, &profile)
	return profile, err
}
