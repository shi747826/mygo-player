package main

import (
	"context"
	"errors"
	"time"

	"mygo-player/internal/catalog"
	"mygo-player/pluginapi"
)

// Synchronization runs against a captured source worker. The generation is
// checked before any local write so late results cannot enter another source.
func (a *app) syncAccountFavorites() {
	if a.accShelfBusy || a.closed || !a.accountLoggedIn || a.win == nil {
		return
	}
	client, err := a.accountPlugin("favoritesSync")
	if err != nil {
		return
	}
	a.accShelfBusy = true
	a.accSyncNote = "正在同步账号收藏…"
	ctx, cancel := context.WithTimeout(a.ctx, 40*time.Second)
	a.accShelfCancel = cancel
	gen := a.accountGen
	go func() {
		defer cancel()
		var items []accountShelfItem
		err := client.Invoke(ctx, "AccountFavorites", struct{}{}, &items)
		series := shelfToSeries(items)
		if err == nil {
			ids := make([]string, 0, len(series))
			for _, s := range series {
				if s.Cover == "" {
					ids = append(ids, s.ID)
				}
			}
			if len(ids) > 0 {
				if covers, e := client.Covers(ctx, ids); e == nil {
					for i := range series {
						if series[i].Cover == "" {
							series[i].Cover = covers[series[i].ID]
						}
					}
				}
			}
		}
		a.win.Update(func() {
			if a.closed || gen != a.accountGen || !a.accountLoggedIn {
				return
			}
			a.accShelfBusy, a.accShelfCancel = false, nil
			if err != nil {
				a.accSyncNote = syncErrorText(err)
				return
			}
			added, mergeErr := a.store().MergeFavorites(series)
			switch {
			case mergeErr != nil:
				a.accSyncNote = "同步失败，本地收藏未改变"
			case len(series) == 0:
				a.accSyncNote = "账号里没有收藏"
			case added == 0:
				a.accSyncNote = "账号收藏已是最新"
			default:
				a.accSyncNote = "已同步 " + itoa(added) + " 部到收藏"
			}
			a.refreshStore()
			if a.page == "收藏" {
				a.items = libraryItems(a.library, "收藏", a.query)
			}
		})
	}()
}

// shelfToSeries 把账号收藏条目转成本地收藏使用的短剧结构。
func shelfToSeries(items []accountShelfItem) []catalog.Series {
	out := make([]catalog.Series, 0, len(items))
	for _, item := range items {
		if item.SeriesID == "" {
			continue
		}
		out = append(out, catalog.Series{
			ID:       item.SeriesID,
			Title:    item.Title,
			Cover:    item.Cover,
			Episodes: item.Episodes,
		})
	}
	return out
}

func syncErrorText(err error) string {
	switch {
	case errors.Is(err, pluginapi.ErrAuthRequired):
		return "登录后才能同步账号收藏"
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, pluginapi.ErrTimeout):
		return "同步超时，请稍后重试"
	}
	return "暂时同步不到账号收藏"
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
