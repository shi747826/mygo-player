package main

import (
	"context"
	"math"
	"mygo-player/internal/webplayer"
	"mygo-player/pluginapi"
	"time"
)

// The timeline renderer is shared. Only plugins advertising danmaku are queried.
func (a *app) startDanmakuLoad(p *webplayer.Player, seriesID, episodeID string) {
	if a.danmakuCancel != nil {
		a.danmakuCancel()
		a.danmakuCancel = nil
	}
	a.danmakuEpisode = episodeID
	a.danmakuNote = ""
	if !a.sourceHas("danmaku") {
		p.SetDanmaku(nil)
		p.SetDanmakuVisible(false)
		return
	}
	a.danmakuNote = "弹幕加载中…"
	source := a.sourceAPI()
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	a.danmakuCancel = cancel
	go func() {
		defer cancel()
		cues, err := source.Danmaku(ctx, seriesID, episodeID)
		a.win.Update(func() {
			if a.closed || a.player != p || a.danmakuEpisode != episodeID {
				return
			}
			if err != nil && len(cues) == 0 {
				a.danmakuNote = "弹幕暂时读不到"
				return
			}
			if len(cues) == 0 {
				a.danmakuNote = "本集暂无弹幕"
				return
			}
			a.danmakuNote = ""
			p.SetDanmaku(toPlayerCues(cues))
			p.SetDanmakuVisible(a.danmakuVisible)
		})
	}()
}
func toPlayerCues(cues []pluginapi.Cue) []webplayer.DanmakuCue {
	const limit = 1200
	out := make([]webplayer.DanmakuCue, 0, min(len(cues), limit))
	for _, c := range cues {
		if len(out) >= limit {
			break
		}
		if c.Time < 0 || math.IsNaN(c.Time) || math.IsInf(c.Time, 0) || c.Text == "" {
			continue
		}
		out = append(out, webplayer.DanmakuCue{Time: c.Time, Text: c.Text})
	}
	return out
}
func (a *app) setDanmakuVisible(visible bool) {
	a.danmakuVisible = visible
	if a.player != nil {
		a.configurePlayerControls(a.player)
	}
}
