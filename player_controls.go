package main

import (
	"strconv"

	"mygo-player/internal/webplayer"
)

// Keep app-owned choices in the video document; transport operations stay
// inside that document so mouse activity and auto-hide share one coordinate space.
func (a *app) configurePlayerControls(p *webplayer.Player) {
	p.ConfigureControls(a.playerControlConfig(), a.playerControlHandler(p))
}

// playerControlConfig 汇总应用侧保存的播放选项，交给播放器页面渲染。
func (a *app) playerControlConfig() webplayer.ControlConfig {
	episodes := make([]webplayer.ControlEpisode, 0, len(a.playingEpisodes))
	for _, ep := range a.playingEpisodes {
		episodes = append(episodes, webplayer.ControlEpisode{ID: ep.ID, Number: ep.Number})
	}
	return webplayer.ControlConfig{
		Title: a.playingSeries.Title, Episode: a.playingEpisode.Number,
		Episodes: episodes, CurrentEpisode: a.playingEpisode.ID,
		Qualities: a.qualities, Quality: a.quality, Autoplay: a.autoplay,
		Portrait: a.portrait, Pinned: a.pinned, Fullscreen: a.fullscreenVideo, WindowFullscreen: a.windowFullscreen,
		CanPrevious: a.canJump(-1), CanNext: a.canJump(1),
		DanmakuAvailable: a.sourceHas("danmaku"), DanmakuEnabled: a.sourceHas("danmaku") && a.danmakuVisible, DanmakuPosition: a.danmakuPosition,
		DanmakuSize: a.danmakuSize, DanmakuSpeed: a.danmakuSpeed, DanmakuOpacity: a.danmakuOpacity,
	}
}

func (a *app) playerControlHandler(p *webplayer.Player) func(webplayer.ControlAction) {
	return func(action webplayer.ControlAction) {
		if a.closed || a.player != p {
			return
		}
		switch action.Name {
		case "previous":
			a.jump(-1)
		case "next":
			a.jump(1)
		case "quality":
			if action.Value != a.quality && contains(a.qualities, action.Value) {
				a.clearPreloads()
				a.quality = action.Value
				a.qualityChosen = true
				a.savePrefs()
				a.changeQuality()
			}
		case "episode":
			for i, ep := range a.playingEpisodes {
				if ep.ID == action.Value && ep.ID != a.playingEpisode.ID {
					a.startEpisode(a.playingSeries, a.playingEpisodes, i, 0)
					break
				}
			}
		case "autoplay":
			a.autoplay = action.Value == "true"
			a.savePrefs()
			if a.autoplay {
				a.preloadNext()
			} else {
				a.clearPreloads()
			}
		case "portrait":
			if !a.fullscreenVideo && !a.windowFullscreen {
				a.setPortrait(action.Value == "true")
			}
		case "pin":
			a.pinned = action.Value == "true"
			a.savePrefs()
			if a.win != nil {
				a.win.SetAlwaysOnTop(a.pinned)
			}
		case "danmaku":
			a.danmakuVisible = action.Value == "true"
			a.configurePlayerControls(p)
			a.savePrefs()
		case "danmakuPosition":
			if validDanmakuPosition(action.Value) {
				a.danmakuPosition = action.Value
				a.configurePlayerControls(p)
				a.savePrefs()
			}
		case "danmakuSize":
			if validDanmakuSize(action.Value) {
				a.danmakuSize = action.Value
				a.configurePlayerControls(p)
				a.savePrefs()
			}
		case "danmakuSpeed":
			if validDanmakuSpeed(action.Value) {
				a.danmakuSpeed = action.Value
				a.configurePlayerControls(p)
				a.savePrefs()
			}
		case "danmakuOpacity":
			if v, err := strconv.Atoi(action.Value); err == nil && v >= 10 && v <= 100 {
				a.danmakuOpacity = v
				a.configurePlayerControls(p)
				a.savePrefs()
			}
		case "pip":
			p.TogglePictureInPicture()
		case "windowFullscreen":
			// Only change the app layout; never alter the native window state.
			if !a.fullscreenVideo {
				a.windowFullscreen = action.Value == "true"
			}
		case "fullscreen":
			if action.Value == "true" && !a.fullscreenVideo {
				a.enterFullscreen()
			} else if action.Value == "false" && a.fullscreenVideo {
				a.exitFullscreen()
			}
		}
	}
}

func validDanmakuPosition(v string) bool {
	switch v {
	case "mega", "full", "half", "top":
		return true
	}
	return false
}

func validDanmakuSpeed(v string) bool {
	switch v {
	case "slow", "medium", "fast":
		return true
	}
	return false
}

func validDanmakuSize(v string) bool {
	switch v {
	case "large", "medium", "small":
		return true
	}
	return false
}
