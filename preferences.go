package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// preferences 保存播放器里由用户调过的选项，下次打开应用时恢复，
// 免得每次都要重设音量、弹幕范围这些。
type preferences struct {
	Volume          float64 `json:"volume"`
	Rate            float64 `json:"rate"`
	Autoplay        bool    `json:"autoplay"`
	Pinned          bool    `json:"pinned"`
	DanmakuEnabled  bool    `json:"danmakuEnabled"`
	DanmakuPosition string  `json:"danmakuPosition"`
	DanmakuSize     string  `json:"danmakuSize"`
	DanmakuSpeed    string  `json:"danmakuSpeed"`
	DanmakuOpacity  int     `json:"danmakuOpacity"`
	// Quality 只在用户主动选过画质后保存；留空表示跟随每个片源自动选最高档。
	Quality string `json:"quality"`
}

func defaultPreferences() preferences {
	return preferences{
		Volume: 0.8, Rate: 1, Autoplay: true,
		DanmakuEnabled: true, DanmakuPosition: "full",
		DanmakuSize: "medium", DanmakuSpeed: "medium", DanmakuOpacity: 80,
	}
}

func preferencesPath(dataDir string) string {
	return filepath.Join(dataDir, "preferences.json")
}

// sanitized 把越界或未知的取值拉回可用范围，坏文件不会带坏播放器。
func (p preferences) sanitized() preferences {
	out := defaultPreferences()
	if p.Volume > 0 && p.Volume <= 1 {
		out.Volume = p.Volume
	} else if p.Volume == 0 {
		out.Volume = 0
	}
	if p.Rate >= .25 && p.Rate <= 4 {
		out.Rate = p.Rate
	}
	out.Autoplay = p.Autoplay
	out.Pinned = p.Pinned
	out.DanmakuEnabled = p.DanmakuEnabled
	switch p.DanmakuPosition {
	case "mega", "full", "half", "top":
		out.DanmakuPosition = p.DanmakuPosition
	}
	switch p.DanmakuSize {
	case "large", "medium", "small":
		out.DanmakuSize = p.DanmakuSize
	}
	switch p.DanmakuSpeed {
	case "slow", "medium", "fast":
		out.DanmakuSpeed = p.DanmakuSpeed
	}
	if p.DanmakuOpacity >= 10 && p.DanmakuOpacity <= 100 {
		out.DanmakuOpacity = p.DanmakuOpacity
	}
	if q, ok := cleanQuality(p.Quality); ok {
		out.Quality = q
	}
	return out
}

// preferencesDoc 用指针区分「文件里没写这个字段」和「写成了 0 / false」，
// 缺失的字段保留默认值，显式的 0 音量则保持静音。
type preferencesDoc struct {
	Volume          *float64 `json:"volume"`
	Rate            *float64 `json:"rate"`
	Autoplay        *bool    `json:"autoplay"`
	Pinned          *bool    `json:"pinned"`
	DanmakuEnabled  *bool    `json:"danmakuEnabled"`
	DanmakuPosition *string  `json:"danmakuPosition"`
	DanmakuSize     *string  `json:"danmakuSize"`
	DanmakuSpeed    *string  `json:"danmakuSpeed"`
	DanmakuOpacity  *int     `json:"danmakuOpacity"`
	Quality         *string  `json:"quality"`
}

func decodePreferences(data []byte) (preferences, error) {
	var doc preferencesDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return defaultPreferences(), err
	}
	p := defaultPreferences()
	if doc.Volume != nil {
		p.Volume = *doc.Volume
	}
	if doc.Rate != nil {
		p.Rate = *doc.Rate
	}
	if doc.Autoplay != nil {
		p.Autoplay = *doc.Autoplay
	}
	if doc.Pinned != nil {
		p.Pinned = *doc.Pinned
	}
	if doc.DanmakuEnabled != nil {
		p.DanmakuEnabled = *doc.DanmakuEnabled
	}
	if doc.DanmakuPosition != nil {
		p.DanmakuPosition = *doc.DanmakuPosition
	}
	if doc.DanmakuSize != nil {
		p.DanmakuSize = *doc.DanmakuSize
	}
	if doc.DanmakuSpeed != nil {
		p.DanmakuSpeed = *doc.DanmakuSpeed
	}
	if doc.DanmakuOpacity != nil {
		p.DanmakuOpacity = *doc.DanmakuOpacity
	}
	if doc.Quality != nil {
		p.Quality = *doc.Quality
	}
	return p, nil
}

// loadPreferences 读取已保存的选项；文件不存在时用默认值，内容损坏时
// 也退回默认值而不覆盖原文件，方便用户自己修。
func loadPreferences(dataDir string) preferences {
	if dataDir == "" {
		return defaultPreferences()
	}
	data, err := os.ReadFile(preferencesPath(dataDir))
	if err != nil {
		return defaultPreferences()
	}
	p, err := decodePreferences(data)
	if err != nil {
		return defaultPreferences()
	}
	return p.sanitized()
}

// savePreferences 原子写入：先写临时文件再改名，避免半截文件。
func savePreferences(dataDir string, p preferences) error {
	if dataDir == "" {
		return errors.New("没有数据目录")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	p = p.sanitized()
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	target := preferencesPath(dataDir)
	temp := target + ".tmp"
	if err := os.WriteFile(temp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(temp, target); err != nil {
		os.Remove(temp)
		return err
	}
	return nil
}

// currentPreferences 取一份当前设置的快照，供去重和落盘使用。
func (a *app) currentPreferences() preferences {
	quality := ""
	if a.qualityChosen {
		quality = a.quality
	}
	return preferences{
		Volume: a.volume, Rate: a.rate,
		Autoplay: a.autoplay, Pinned: a.pinned,
		DanmakuEnabled: a.danmakuVisible, DanmakuPosition: a.danmakuPosition,
		DanmakuSize: a.danmakuSize, DanmakuSpeed: a.danmakuSpeed, DanmakuOpacity: a.danmakuOpacity,
		Quality: quality,
	}.sanitized()
}

func (a *app) applyPreferences(p preferences) {
	p = p.sanitized()
	a.volume, a.rate = p.Volume, p.Rate
	a.autoplay, a.pinned = p.Autoplay, p.Pinned
	a.danmakuVisible, a.danmakuPosition = p.DanmakuEnabled, p.DanmakuPosition
	a.danmakuSize, a.danmakuOpacity = p.DanmakuSize, p.DanmakuOpacity
	a.danmakuSpeed = p.DanmakuSpeed
	if p.Quality != "" {
		a.quality = p.Quality
		a.qualityChosen = true
	}
	a.prefsSaved = p
	if a.win != nil && a.pinned {
		a.win.SetAlwaysOnTop(true)
	}
}

// savePrefs 在设置变动后延迟落盘：拖音量滑杆时只写一次，
// 而且与上次写过的内容相同就不再写。
func (a *app) savePrefs() {
	if a.closed || a.dataDir == "" {
		return
	}
	next := a.currentPreferences()
	if next == a.prefsSaved {
		return
	}
	a.prefsSaved = next
	dir := a.dataDir
	a.prefsMu.Lock()
	if a.prefsTimer != nil {
		a.prefsTimer.Stop()
	}
	a.prefsTimer = time.AfterFunc(600*time.Millisecond, func() {
		_ = savePreferences(dir, next)
	})
	a.prefsMu.Unlock()
}

// flushPrefs 退出前立刻落盘，避免最后一次改动卡在去抖里丢掉。
func (a *app) flushPrefs() {
	if a.dataDir == "" {
		return
	}
	a.prefsMu.Lock()
	if a.prefsTimer != nil {
		a.prefsTimer.Stop()
		a.prefsTimer = nil
	}
	a.prefsMu.Unlock()
	current := a.currentPreferences()
	if current == a.prefsSaved {
		return
	}
	_ = savePreferences(a.dataDir, current)
	a.prefsSaved = current
}

// cleanQuality 只接受播放器可能给出的短标签，避免坏文件把任意字符串带进请求。
func cleanQuality(raw string) (string, bool) {
	q := strings.TrimSpace(raw)
	if q == "" || len([]rune(q)) > 32 {
		return "", false
	}
	for _, r := range q {
		if unicode.IsControl(r) {
			return "", false
		}
	}
	return q, true
}
