package main

import (
	"context"
	"errors"
	"sort"
	"time"

	"mygo-player/pluginapi"
)

func (a *app) cancelReservationRequest() {
	if a.reservationCancel != nil {
		a.reservationCancel()
		a.reservationCancel = nil
	}
	a.reservationGen++
	a.reservationBusy = false
}

func (a *app) clearReservations() {
	a.cancelReservationRequest()
	a.reservations = nil
	a.reservationsLoaded = false
	a.reservationError = ""
}

func (a *app) loadReservations() {
	if a.closed || !a.sourceHas("reservations") {
		return
	}
	a.cancelReservationRequest()
	a.reservationError = ""
	if !a.accountLoggedIn {
		a.reservations = nil
		a.reservationsLoaded = false
		return
	}
	if a.win == nil {
		return
	}
	client, err := a.accountPlugin("reservations")
	if err != nil {
		a.reservationError = reservationErrorText(err)
		return
	}
	accountGen, gen, listID := a.accountGen, a.reservationGen, a.listID
	ctx, cancel := context.WithTimeout(a.ctx, 60*time.Second)
	a.reservationCancel = cancel
	a.reservationBusy = true
	go func() {
		defer cancel()
		var items []accountShelfItem
		err := client.Invoke(ctx, "AccountReservations", struct{}{}, &items)
		if err == nil {
			sortReservations(items)
		}
		a.win.Update(func() {
			if a.closed || a.accountGen != accountGen || a.reservationGen != gen || a.listID != listID || !a.accountLoggedIn || !a.sourceHas("reservations") {
				return
			}
			a.reservationBusy = false
			a.reservationCancel = nil
			if err != nil {
				a.reservationError = reservationErrorText(err)
				return
			}
			a.reservations = items
			a.reservationsLoaded = true
		})
	}()
}

func sortReservations(items []accountShelfItem) {
	// Upcoming shows go first; within that group announced dates are ascending,
	// followed by unknown dates. Preserve service order for already-released rows.
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.Online != b.Online {
			return !a.Online
		}
		if a.Online {
			return false
		}
		da, db := reservationDateText(a.PublishAt), reservationDateText(b.PublishAt)
		ta, ea := time.Parse("2006-01-02", da)
		tb, eb := time.Parse("2006-01-02", db)
		if ea == nil && eb != nil {
			return true
		}
		if ea != nil || eb != nil {
			return false
		}
		return ta.Before(tb)
	})
}

func reservationErrorText(err error) string {
	switch {
	case errors.Is(err, pluginapi.ErrAuthRequired):
		return "当前登录状态未能读取个人预约，请到账号页刷新登录后重试。"
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, pluginapi.ErrTimeout):
		return "读取个人预约超时，请稍后重试。"
	case errors.Is(err, context.Canceled):
		return "预约读取已取消，请重试。"
	}
	return "暂时无法读取个人预约，请稍后重试；原有账号状态未改变。"
}
