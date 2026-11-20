// Package webplayer 在 MyGo 原生界面中以 WebView 承载 HTML5 视频。
// 除内部 HTTP 回调外，所有公共方法均应在主线程调用。
package webplayer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
)

type Player struct {
	page          *mygo.Page
	view          *mygo.WebView
	bridge        *bridge
	state         videoState
	err           error
	closed        atomic.Bool
	cancel        context.CancelFunc
	commands      chan string
	controlConfig string
	onControl     func(ControlAction)
}

// OpenURL 创建当前焦点窗口内的播放器；没有焦点时使用唯一窗口。
// URL 仅接受 HTTP(S)。update 应为所属窗口的 Window.Update。
func OpenURL(rawURL string, update func(func())) (*Player, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
		return nil, errors.New("视频地址无效，请使用 HTTP 或 HTTPS 视频地址")
	}
	return OpenURLInWindow(defaultWindow(), rawURL, update)
}

// OpenURLInWindow 显式选择承载播放器的原生界面窗口。
func OpenURLInWindow(win *mygo.Window, rawURL string, update func(func())) (*Player, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
		return nil, errors.New("视频地址无效，请使用 HTTP 或 HTTPS 视频地址")
	}
	return open(win, rawURL, nil, update)
}

// Open 提供本地文件的带 Range 支持的临时播放地址，不暴露其它目录。
func Open(path string, update func(func())) (*Player, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("无法打开本地视频，请检查文件路径和读取权限")
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, errors.New("请选择可读取的普通视频文件")
	}
	return open(defaultWindow(), "", f, update)
}

func defaultWindow() *mygo.Window {
	win := mygo.FocusedWindow()
	if win == nil {
		wins := mygo.Windows()
		if len(wins) == 1 {
			win = wins[0]
		}
	}
	return win
}

func open(win *mygo.Window, source string, file *os.File, update func(func())) (*Player, error) {
	fail := func(err error) (*Player, error) {
		if file != nil {
			file.Close()
		}
		return nil, err
	}
	if update == nil {
		return fail(errors.New("创建播放器失败，请提供窗口 Update 回调"))
	}
	if win == nil {
		return fail(errors.New("创建播放器失败，请先打开并聚焦应用窗口"))
	}
	p := &Player{commands: make(chan string, 64)}
	b, err := newBridge(source, file, func(s videoState) {
		update(func() {
			if p.closed.Load() || s.Sequence <= p.state.Sequence {
				return
			}
			p.state = s
			if s.Error != 0 {
				p.err = mediaError(s.Error)
			} else if s.ReadyState >= 2 {
				// A later episode may play fine after an earlier one failed.
				p.err = nil
			}
		})
	}, func(action ControlAction) {
		update(func() {
			if !p.closed.Load() && p.onControl != nil {
				p.onControl(action)
			}
		})
	})
	if err != nil {
		return fail(err)
	}
	p.bridge = b
	view, err := win.NewWebView(mygo.WebViewOptions{URL: b.playerURL(), BackgroundColor: "#080a10", Page: mygo.PageOptions{Autoplay: mygo.AutoplayAllow, Untrusted: true, PictureInPicture: true}})
	if err != nil {
		b.close()
		return nil, errors.New("创建网页播放器失败，请检查系统 WebView 是否可用")
	}
	p.view, p.page = view, view.Page()
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	go p.runCommands(ctx, p.page, update)
	return p, nil
}

// ControlConfig describes app-owned options for the HTML video controls.
type ControlConfig struct {
	Title            string           `json:"title"`
	Episode          int              `json:"episode"`
	Episodes         []ControlEpisode `json:"episodes"`
	CurrentEpisode   string           `json:"currentEpisode"`
	Qualities        []string         `json:"qualities"`
	Quality          string           `json:"quality"`
	Autoplay         bool             `json:"autoplay"`
	Portrait         bool             `json:"portrait"`
	Pinned           bool             `json:"pinned"`
	Fullscreen       bool             `json:"fullscreen"`
	WindowFullscreen bool             `json:"windowFullscreen"`
	CanPrevious      bool             `json:"canPrevious"`
	CanNext          bool             `json:"canNext"`
	// 弹幕显示设置：位置、字号与不透明度都由应用保存在本机，随配置下发。
	DanmakuAvailable bool   `json:"danmakuAvailable"`
	DanmakuEnabled   bool   `json:"danmakuEnabled"`
	DanmakuPosition  string `json:"danmakuPosition"`
	DanmakuSize      string `json:"danmakuSize"`
	DanmakuSpeed     string `json:"danmakuSpeed"`
	DanmakuOpacity   int    `json:"danmakuOpacity"`
}
type ControlEpisode struct {
	ID     string `json:"id"`
	Number int    `json:"number"`
}

// SwitchSource replaces the playing media without replacing the WebView, so a
// fullscreen window keeps its state and the previous frame stays visible until
// the new video has decoded its first frame.
func (p *Player) SwitchSource(rawURL string, start time.Duration) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
		return false
	}
	if p.closed.Load() {
		return false
	}
	if start < 0 {
		start = 0
	}
	p.err = nil
	p.enqueue("window.playerLoad(" + strconv.Quote(rawURL) + "," + strconv.FormatFloat(start.Seconds(), 'f', -1, 64) + ")")
	return true
}

// ConfigureControls updates the HTML UI only when app-owned options change.
func (p *Player) ConfigureControls(config ControlConfig, handler func(ControlAction)) {
	p.onControl = handler
	data, err := json.Marshal(config)
	if err != nil || p.closed.Load() || string(data) == p.controlConfig {
		return
	}
	p.controlConfig = string(data)
	p.enqueue("window.playerConfigure(" + string(data) + ")")
}

// DanmakuCue 是一条弹幕：Time 相对本集开头的秒数，Text 是要显示的文本。
type DanmakuCue struct {
	Time float64 `json:"time"`
	Text string  `json:"text"`
}

// SetDanmaku 把整集弹幕交给播放器页面，由它按视频时间轴投放。
func (p *Player) SetDanmaku(cues []DanmakuCue) {
	if p.closed.Load() {
		return
	}
	if cues == nil {
		cues = []DanmakuCue{}
	}
	data, err := json.Marshal(cues)
	if err != nil {
		return
	}
	p.enqueue("window.playerDanmaku(" + string(data) + ")")
}

// TogglePictureInPicture 让本集视频进入或退出画中画。
func (p *Player) TogglePictureInPicture() {
	if p.closed.Load() || p.page == nil {
		return
	}
	p.page.TogglePictureInPicture()
}

// PictureInPictureActive 报告当前是否处于画中画。
func (p *Player) PictureInPictureActive() bool {
	if p.closed.Load() || p.page == nil {
		return false
	}
	return p.page.IsPictureInPictureActive()
}

// SetDanmakuVisible 控制弹幕层是否显示。
func (p *Player) SetDanmakuVisible(visible bool) {
	if visible {
		p.enqueue(`window.playerDanmakuEnabled(true)`)
		return
	}
	p.enqueue(`window.playerDanmakuEnabled(false)`)
}

func (p *Player) Volume() float64 { return p.state.Volume }
func (p *Player) Rate() float64   { return p.state.Rate }

// Focus gives the mounted video WebView keyboard input. Call on the UI thread
// after the player is ready, rather than on every frame.
func (p *Player) Focus() {
	if !p.closed.Load() && p.view != nil {
		p.view.Focus()
	}
}

func (p *Player) Page() *mygo.Page {
	if p.closed.Load() {
		return nil
	}
	return p.page
}
func (p *Player) Ready() bool {
	return !p.closed.Load() && p.state.ReadyState >= 2 && p.state.Error == 0
}
func (p *Player) Playing() bool {
	return !p.closed.Load() && p.state.Sequence > 0 && !p.state.Paused && !p.state.Ended && p.state.Error == 0
}
func (p *Player) Buffering() bool {
	return !p.closed.Load() && !p.state.Ended && p.state.Error == 0 && p.state.ReadyState < 3
}
func (p *Player) Ended() bool             { return p.state.Ended }
func (p *Player) Position() time.Duration { return seconds(p.state.CurrentTime) }
func (p *Player) Duration() time.Duration { return seconds(p.state.Duration) }
func (p *Player) Err() error              { return p.err }
func seconds(v float64) time.Duration     { return time.Duration(v * float64(time.Second)) }
func (p *Player) Play()                   { p.enqueue(`window.playerCommand("play",0)`) }
func (p *Player) Pause()                  { p.enqueue(`window.playerCommand("pause",0)`) }
func (p *Player) Seek(d time.Duration) {
	if d < 0 {
		d = 0
	}
	p.command("seek", d.Seconds())
}
func (p *Player) SetRate(rate float64) {
	if !finite(rate) || rate <= 0 {
		return
	}
	p.command("rate", math.Max(.25, math.Min(rate, 4)))
}
func (p *Player) SetVolume(volume float64) {
	if !finite(volume) {
		return
	}
	p.command("volume", math.Max(0, math.Min(volume, 1)))
}
func (p *Player) command(name string, value float64) {
	p.enqueue("window.playerCommand(" + strconv.Quote(name) + "," + strconv.FormatFloat(value, 'f', -1, 64) + ")")
}
func (p *Player) enqueue(js string) {
	if p.closed.Load() {
		return
	}
	select {
	case p.commands <- js:
	default:
		p.err = errors.New("播放器控制请求过多，请稍后重试")
	}
}
func (p *Player) runCommands(ctx context.Context, page *mygo.Page, update func(func())) {
	select {
	case <-ctx.Done():
		return
	case <-p.bridge.documentReady:
	}
	for {
		select {
		case <-ctx.Done():
			return
		case command := <-p.commands:
			if ctx.Err() != nil {
				return
			}
			callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			_, err := page.EvalContext(callCtx, command)
			cancel()
			if err != nil && ctx.Err() == nil {
				update(func() {
					if !p.closed.Load() {
						p.err = errors.New("播放器控制失败，请使用视频画面内的播放控件重试")
					}
				})
			}
		}
	}
}

// Close 不等待需要主线程的 JS 或 UI 回调；取消命令后销毁 WebView。
func (p *Player) Close() error {
	if p.closed.Swap(true) {
		return nil
	}
	if p.cancel != nil {
		p.cancel()
	}
	if p.view != nil {
		p.view.Destroy()
	}
	if p.bridge != nil {
		return p.bridge.close()
	}
	return nil
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func mediaError(code int) error {
	switch code {
	case 1:
		return errors.New("视频加载已中止，请重新选择分集")
	case 2:
		return errors.New("视频网络加载失败，请检查网络或刷新播放地址")
	case 3:
		return errors.New("视频解码失败，当前 WebView 可能不支持此编码")
	case 4:
		return errors.New("视频格式或地址不受支持，请刷新播放地址或更换清晰度")
	default:
		return fmt.Errorf("视频播放失败（错误 %d），请重试", code)
	}
}
