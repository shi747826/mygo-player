package webplayer

import (
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// ControlAction is a player UI request handled by the Go host. Boolean values
// use "true"/"false"; previous/next carry an empty value.
type ControlAction struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

//go:embed controls.html
var controlsHTML string

//go:embed controls.css
var controlsCSS string

//go:embed controls.js
var controlsJS string

type videoState struct {
	Volume float64 `json:"volume"`
	Rate   float64 `json:"rate"`
	// MediaGeneration 标记页面当前加载的是第几路片源，换源时递增；
	// 宿主用它丢弃上一集迟到的状态上报。
	MediaGeneration uint64  `json:"mediaGeneration"`
	Sequence        uint64  `json:"sequence"`
	CurrentTime     float64 `json:"currentTime"`
	Duration        float64 `json:"duration"`
	ReadyState      int     `json:"readyState"`
	Error           int     `json:"error"`
	Paused          bool    `json:"paused"`
	Ended           bool    `json:"ended"`
}
type bridge struct {
	token         string
	host          string
	origin        string
	source        string
	file          *os.File
	server        *http.Server
	onState       func(videoState)
	onAction      []func(ControlAction)
	documentReady chan struct{}
	readyOnce     sync.Once
	closeOnce     sync.Once
	closeErr      error
}

func newBridge(source string, file *os.File, onState func(videoState), onAction ...func(ControlAction)) (*bridge, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, errors.New("无法生成播放器会话，请重试")
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, errors.New("无法启动本机视频服务，请检查本机网络权限")
	}
	b := &bridge{token: hex.EncodeToString(raw[:]), host: listener.Addr().String(), source: source, file: file, onState: onState, onAction: append([]func(ControlAction){}, onAction...), documentReady: make(chan struct{})}
	b.origin = "http://" + b.host
	if file != nil {
		b.source = b.origin + "/media?token=" + b.token
	}
	b.server = &http.Server{Handler: b, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, ErrorLog: log.New(io.Discard, "", 0)}
	go func() { _ = b.server.Serve(listener) }()
	return b, nil
}
func (b *bridge) playerURL() string { return b.origin + "/player?token=" + b.token }
func (b *bridge) close() error {
	b.closeOnce.Do(func() {
		if b.server != nil {
			b.closeErr = b.server.Close()
		}
		if b.file != nil {
			if err := b.file.Close(); b.closeErr == nil {
				b.closeErr = err
			}
		}
	})
	return b.closeErr
}
func (b *bridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Host != b.host || subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("token")), []byte(b.token)) != 1 {
		http.Error(w, "禁止访问", http.StatusForbidden)
		return
	}
	switch r.URL.Path {
	case "/player":
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, playerHTML(b.source, "/state?token="+b.token))
	case "/state":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Origin") != b.origin {
			http.Error(w, "来源无效", http.StatusForbidden)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "格式无效", http.StatusUnsupportedMediaType)
			return
		}
		var state videoState
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&state); err != nil {
			http.Error(w, "状态无效", http.StatusBadRequest)
			return
		}
		if err := dec.Decode(new(any)); err != io.EOF || !validState(state) {
			http.Error(w, "状态无效", http.StatusBadRequest)
			return
		}
		b.readyOnce.Do(func() { close(b.documentReady) })
		// Window.Update 只安排主线程执行。这里不持锁，也不等待回调完成。
		if b.onState != nil {
			b.onState(state)
		}
		w.WriteHeader(http.StatusNoContent)
	case "/action":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Origin") != b.origin {
			http.Error(w, "来源无效", http.StatusForbidden)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "格式无效", http.StatusUnsupportedMediaType)
			return
		}
		var action ControlAction
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&action); err != nil {
			http.Error(w, "操作无效", http.StatusBadRequest)
			return
		}
		if err := dec.Decode(new(any)); err != io.EOF || !validAction(action) {
			http.Error(w, "操作无效", http.StatusBadRequest)
			return
		}
		for _, callback := range b.onAction {
			if callback != nil {
				callback(action)
			}
		}
		w.WriteHeader(http.StatusNoContent)
	case "/media":
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if b.file == nil {
			http.NotFound(w, r)
			return
		}
		info, err := b.file.Stat()
		if err != nil {
			http.Error(w, "文件不可读取", http.StatusGone)
			return
		}
		// 每个请求独立游标，同时保留用户打开时的文件，支持并发 Range。
		http.ServeContent(w, r, info.Name(), info.ModTime(), io.NewSectionReader(b.file, 0, info.Size()))
	default:
		http.NotFound(w, r)
	}
}
func validAction(a ControlAction) bool {
	switch a.Name {
	case "previous", "next", "pip":
		return a.Value == ""
	case "autoplay", "portrait", "pin", "fullscreen", "windowFullscreen", "danmaku":
		return a.Value == "true" || a.Value == "false"
	case "danmakuPosition":
		return a.Value == "mega" || a.Value == "full" || a.Value == "half" || a.Value == "top"
	case "danmakuSize":
		return a.Value == "large" || a.Value == "medium" || a.Value == "small"
	case "danmakuSpeed":
		return a.Value == "slow" || a.Value == "medium" || a.Value == "fast"
	case "danmakuOpacity":
		n, err := strconv.Atoi(a.Value)
		return err == nil && n >= 10 && n <= 100
	case "quality", "episode":
		return len(a.Value) <= 256 && utf8.ValidString(a.Value) && strings.TrimSpace(a.Value) != "" && !strings.ContainsFunc(a.Value, unicode.IsControl)
	default:
		return false
	}
}

func validState(s videoState) bool {
	// duration 超出 time.Duration 表示范围时拒绝，避免秒数转换溢出。
	const maxSeconds = float64(int64(1<<63-1)) / float64(time.Second)
	return finite(s.Volume) && s.Volume >= 0 && s.Volume <= 1 && finite(s.Rate) && (s.Rate == 0 || (s.Rate >= .25 && s.Rate <= 4)) && s.Sequence > 0 && finite(s.CurrentTime) && finite(s.Duration) && s.CurrentTime >= 0 && s.Duration >= 0 && s.CurrentTime < maxSeconds && s.Duration < maxSeconds && s.ReadyState >= 0 && s.ReadyState <= 4 && s.Error >= 0 && s.Error <= 4
}

func playerHTML(source, stateURL string) string {
	// encoding/json 默认转义 < > & 及 U+2028/2029，URL 不可退出 script 上下文。
	src, _ := json.Marshal(source)
	state, _ := json.Marshal(stateURL)
	return `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>视频播放</title><style>` + controlsCSS + `</style></head><body><video id="video" playsinline preload="auto"></video><div id="hint" role="status" aria-live="polite"></div>` + controlsHTML + `<script>
(()=>{'use strict';const hint=document.getElementById('hint');const video=document.getElementById('video');
const source=` + string(src) + `, endpoint=` + string(state) + `;
let sequence=0,sending=false,pending=null,wantPlay=false,lastReport='',loadFailure=false;
// 单元素播放：video 元素永不替换，换集只换 src，所以控制条的 timeupdate/click
// 监听始终有效（进度条时间不再停在上一集）。generation 标识当前片源代号。
let generation=0,switching=false,playToken=0,pendingStart=0,pendingSeek=false,desiredRate=1,applyingRate=false,silenced=false,resumeVolume=1;
function snapshot(){return {sequence:++sequence,mediaGeneration:generation,currentTime:Number.isFinite(video.currentTime)?video.currentTime:0,duration:Number.isFinite(video.duration)?video.duration:0,readyState:video.readyState,error:loadFailure?4:(video.error?video.error.code:0),paused:video.paused,ended:video.ended,volume:video.muted?0:video.volume,rate:video.playbackRate};}
async function flush(){if(sending)return;sending=true;try{while(pending){const body=pending;pending=null;try{const response=await fetch(endpoint,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body),cache:'no-store'});if(response.ok)lastReport=JSON.stringify({...body,sequence:0});}catch(_){}}}finally{sending=false;}}
function report(){const next=snapshot();if(!sending&&JSON.stringify({...next,sequence:0})===lastReport)return;pending=next;void flush();}
// playToken 让换集时尚未完成的 play() 失效，避免旧片源在静音后又出声。
async function play(){if(!wantPlay||switching)return;const token=++playToken;let started=false;try{await video.play();started=true;}catch(error){if(token!==playToken)return;if(!wantPlay||error.name==='AbortError')return;video.muted=true;try{await video.play();started=true;}catch(_){hint.textContent='自动播放被阻止';}}if(token!==playToken){if(switching)try{video.pause();}catch(_){}return;}if(started)report();}
// video.load() 会把 playbackRate 重置成 defaultPlaybackRate，两者一起设置才保得住 2 倍速。
function applyRate(){applyingRate=true;try{video.defaultPlaybackRate=desiredRate;if(video.playbackRate!==desiredRate)video.playbackRate=desiredRate;}catch(_){}applyingRate=false;}
function restoreSound(){if(!silenced)return;silenced=false;video.muted=false;video.volume=resumeVolume;}
const events=['loadedmetadata','loadeddata','canplay','canplaythrough','play','playing','pause','waiting','stalled','seeking','seeked','ended','durationchange','ratechange','volumechange','error'];
for(const event of events)video.addEventListener(event,()=>{
  if(event==='error'){const code=video.error?video.error.code:0;if(code<=1)return;/*换源打断不算失败*/loadFailure=true;hint.textContent='视频加载失败，请刷新地址或更换清晰度。';}
  if(event==='ratechange'&&!applyingRate&&video.playbackRate!==desiredRate)applyRate();
  if(event==='loadedmetadata'){if(pendingSeek){pendingSeek=false;if(pendingStart>0){try{video.currentTime=Number.isFinite(video.duration)&&video.duration>0?Math.min(pendingStart,video.duration):pendingStart;}catch(_){}}}applyRate();}
  if((event==='loadedmetadata'||event==='canplay')&&wantPlay&&!switching)void play();
  report();
});
window.playerCommand=(command,value)=>{switch(command){case 'play':wantPlay=true;void play();break;case 'pause':wantPlay=false;video.pause();break;case 'seek':video.currentTime=Number.isFinite(video.duration)?Math.min(value,video.duration):value;break;case 'rate':if(Number.isFinite(value)){desiredRate=Math.max(.25,Math.min(4,value));applyRate();}break;case 'volume':video.volume=value;video.muted=value===0;if(value>0){resumeVolume=value;silenced=false;}break;}report();return true;};
// 宿主开始解析下一集时调用：立刻静音并暂停旧片源，保留最后一帧和全屏状态，
// 同时让未完成的 play() 失效。之后的 playerLoad 换源并恢复声音与播放。
window.playerBeginSwitch=()=>{if(switching)return true;switching=true;playToken+=1;if(!video.muted){silenced=true;resumeVolume=video.volume;video.muted=true;}try{video.pause();}catch(_){}report();return true;};
// latest-wins：连续换源只有最后一次生效，前一次由 load() 打断，不再创建第二个
// video 元素，因此同一时刻最多只有一路音频。
window.playerLoad=(url,start)=>{if(typeof url!=='string'||!/^https?:\/\//.test(url))return false;generation+=1;loadFailure=false;playToken+=1;switching=false;pendingStart=Number.isFinite(start)&&start>0?start:0;pendingSeek=pendingStart>0;try{video.pause();}catch(_){}video.src=url;applyRate();video.load();applyRate();restoreSound();if(wantPlay)void play();report();return true;};
applyRate();video.src=source;video.load();applyRate();report();const timer=setInterval(report,500);window.addEventListener('pagehide',()=>{clearInterval(timer);video.pause();});
` + controlsJS + `
})();</script></body></html>`
}
