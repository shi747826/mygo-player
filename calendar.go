package main

import (
	"context"
	"time"

	"mygo-player/internal/catalog"
)

// 上新日历只用来给「待上线」提供第二重证据：详情返回零分集且没有明确角标
// 时，如果日历里说这部剧预告在以后上线，就可以判定为待上线；日历读不到
// 就保持未知，不猜。

const calendarTTL = 30 * time.Minute

// ensureCalendar 按需在后台刷新上新日历缓存。
func (a *app) ensureCalendar() (map[string]catalog.UpcomingEntry, bool) {
	if a.calendar != nil && time.Since(a.calendarAt) < calendarTTL {
		return a.calendar, true
	}
	if !a.sourceHas("calendar") {
		return nil, false
	}
	if a.calendarBusy {
		return nil, false
	}
	a.calendarBusy = true
	source, sourceID := a.sourceAPI(), a.source
	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, 60*time.Second)
		defer cancel()
		entries, err := source.Calendar(ctx, 7)
		a.win.Update(func() {
			if a.closed || a.source != sourceID || a.sourceAPI() != source {
				return
			}
			a.calendarBusy = false
			if err != nil || len(entries) == 0 {
				return
			}
			a.calendar = entries
			a.calendarAt = time.Now()
			// 日历是异步取回的：当前面板若正等着这个证据，立刻补上。
			if a.selected.ID != "" && len(a.episodes) == 0 {
				a.applyCalendarHint(a.detailID, a.selected.ID)
			}
		})
	}()
	return nil, false
}

// applyCalendarHint 用日历结果校正当前详情面板的待上线状态。只在当前仍是
// 同一部剧、且状态未知时才动手，避免覆盖已经确定的结论。
func (a *app) applyCalendarHint(id int, seriesID string) {
	entries, ok := a.ensureCalendar()
	if !ok {
		return
	}
	entry, found := entries[seriesID]
	if !found {
		return
	}
	if a.closed || id != a.detailID || a.selected.ID != seriesID {
		return
	}
	if a.selected.ReleaseState == catalog.ReleasePublished || len(a.episodes) > 0 {
		return
	}
	if entry.Online {
		// 日历说已上线，但详情没有分集：多半是分集接口暂不可用，
		// 保持中性提示，不改成待上线。
		return
	}
	a.selected.ReleaseState = catalog.ReleaseUpcoming
	switch {
	case !entry.PublishAt.IsZero() && entry.PublishAt.After(time.Now()):
		a.detailMessage = "上新日历预告：" + entry.PublishAt.Format("1 月 2 日") + "上线"
	case entry.Note != "":
		a.detailMessage = entry.Note
	default:
		a.detailMessage = upcomingText("")
	}
}
