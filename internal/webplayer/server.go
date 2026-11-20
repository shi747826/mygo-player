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
	Volume      float64 `json:"volume"`
	Rate        float64 `json:"rate"`
	Sequence    uint64  `json:"sequence"`
	CurrentTime float64 `json:"currentTime"`
	Duration    float64 `json:"duration"`
	ReadyState  int     `json:"readyState"`
	Error       int     `json:"error"`
	Paused      bool    `json:"paused"`
	Ended       bool    `json:"ended"`
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
(()=>{'use strict';const hint=document.getElementById('hint');let video=document.getElementById('video');
const source=` + string(src) + `, endpoint=` + string(state) + `;let sequence=0,sending=false,pending=null,wantPlay=false,lastReport='',switching=false,loadFailure=false;
function snapshot(){return {sequence:++sequence,currentTime:Number.isFinite(video.currentTime)?video.currentTime:0,duration:Number.isFinite(video.duration)?video.duration:0,readyState:video.readyState,error:loadFailure?4:(video.error?video.error.code:0),paused:video.paused,ended:video.ended,volume:video.muted?0:video.volume,rate:video.playbackRate};}
async function flush(){if(sending)return;sending=true;try{while(pending){const body=pending;pending=null;try{const response=await fetch(endpoint,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body),cache:'no-store'});if(response.ok)lastReport=JSON.stringify({...body,sequence:0});}catch(_){}}}finally{sending=false;}}
function report(){const next=snapshot();if(!sending&&JSON.stringify({...next,sequence:0})===lastReport)return;pending=next;void flush();}
async function play(el){if(!wantPlay)return;try{await el.play();}catch(error){if(!wantPlay||error.name==='AbortError')return;el.muted=true;try{await el.play();}catch(_){hint.textContent='自动播放被阻止';}}report();}
const events=['loadedmetadata','loadeddata','canplay','canplaythrough','play','playing','pause','waiting','stalled','seeking','seeked','ended','durationchange','ratechange','volumechange','error'];
function attach(el){for(const event of events)el.addEventListener(event,()=>{if(event==='error'&&el===video)hint.textContent='视频加载失败，请刷新地址或更换清晰度。';report();});}
attach(video);
window.playerCommand=(command,value)=>{switch(command){case 'play':wantPlay=true;void play(video);break;case 'pause':wantPlay=false;video.pause();break;case 'seek':video.currentTime=Number.isFinite(video.duration)?Math.min(value,video.duration):value;break;case 'rate':video.playbackRate=value;break;case 'volume':video.volume=value;video.muted=value===0;break;}report();return true;};
window.playerLoad=(url,start)=>{if(typeof url!=='string'||!/^https?:\/\//.test(url)||switching)return false;switching=true;const next=document.createElement('video');next.className='video-layer';next.setAttribute('playsinline','');next.preload='auto';next.style.opacity='0';video.after(next);next.volume=video.volume;next.muted=video.muted;next.playbackRate=video.playbackRate;
const abort=()=>{loadFailure=true;next.remove();switching=false;report();};
next.addEventListener('error',abort,{once:true});
next.addEventListener('loadeddata',()=>{loadFailure=false;if(Number.isFinite(start)&&start>0){try{next.currentTime=start;}catch(_){}}const finish=()=>{next.style.opacity='1';video.style.opacity='0';const old=video;video=next;attach(next);old.pause();setTimeout(()=>old.remove(),260);switching=false;report();};if(wantPlay){next.play().then(finish).catch(()=>{next.muted=true;next.play().then(finish).catch(abort);});}else{finish();}},{once:true});
next.src=url;next.load();report();return true;};
video.addEventListener('loadedmetadata',()=>{if(wantPlay)void play(video);},{once:true});video.src=source;video.load();report();const timer=setInterval(report,500);window.addEventListener('pagehide',()=>{clearInterval(timer);video.pause();});
` + controlsJS + `
})();</script></body></html>`
}
