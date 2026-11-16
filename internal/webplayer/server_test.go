package webplayer

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testBridge(t *testing.T, file *os.File, onState func(videoState)) *bridge {
	t.Helper()
	b, err := newBridge("https://example.invalid/video.mp4", file, onState)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := b.close(); err != nil {
			t.Error(err)
		}
	})
	return b
}
func TestHTMLSourceEscaping(t *testing.T) {
	source := `https://example.invalid/video?x=</script><script>alert("injected")</script>&line=` + "\u2028\u2029"
	html := playerHTML(source, "/state?token=secret")
	if strings.Contains(html, source) || strings.Count(html, "</script>") != 1 || strings.Count(html, "<script>") != 1 {
		t.Fatal("source escaped script context")
	}
	encoded, _ := json.Marshal(source)
	if !strings.Contains(html, "const source="+string(encoded)) {
		t.Fatal("source was not encoded as JSON")
	}
	for _, part := range []string{"playsinline preload", "if(wantPlay)void play()", "wantPlay=false;video.pause()", "setInterval(report,500)", "video.muted=true", "自动播放被阻止", "window.playerCommand", "currentTime", "duration", "readyState"} {
		if !strings.Contains(html, part) {
			t.Fatalf("missing %s", part)
		}
	}
	if strings.Contains(html, "<video") && strings.Contains(html, " controls") {
		t.Fatal("native video controls must be hidden; the app draws its own")
	}
}
func TestStateValidation(t *testing.T) {
	var states []videoState
	b := testBridge(t, nil, func(s videoState) { states = append(states, s) })
	good := `{"sequence":1,"currentTime":1.5,"duration":24,"readyState":4,"error":0,"paused":false,"ended":false}`
	cases := []struct {
		name, method, token, origin, contentType, body, host string
		status                                               int
	}{
		{"valid", "POST", b.token, b.origin, "application/json", good, b.host, 204},
		{"bad token", "POST", "wrong", b.origin, "application/json", good, b.host, 403},
		{"foreign origin", "POST", b.token, "https://attacker.invalid", "application/json", good, b.host, 403},
		{"missing origin", "POST", b.token, "", "application/json", good, b.host, 403},
		{"null origin", "POST", b.token, "null", "application/json", good, b.host, 403},
		{"wrong method", "GET", b.token, b.origin, "application/json", good, b.host, 405},
		{"wrong host", "POST", b.token, b.origin, "application/json", good, "attacker.invalid", 403},
		{"wrong type", "POST", b.token, b.origin, "text/plain", good, b.host, 415},
		{"bad json", "POST", b.token, b.origin, "application/json", "{bad", b.host, 400},
		{"trailing", "POST", b.token, b.origin, "application/json", good + good, b.host, 400},
		{"negative time", "POST", b.token, b.origin, "application/json", strings.Replace(good, "1.5", "-1", 1), b.host, 400},
		{"invalid readiness", "POST", b.token, b.origin, "application/json", strings.Replace(good, `"readyState":4`, `"readyState":9`, 1), b.host, 400},
		{"overflow time", "POST", b.token, b.origin, "application/json", strings.Replace(good, "1.5", "999999999999", 1), b.host, 400},
		{"extra field", "POST", b.token, b.origin, "application/json", strings.TrimSuffix(good, "}") + `,"url":"secret"}`, b.host, 400},
		{"large body", "POST", b.token, b.origin, "application/json", strings.Repeat(" ", 5000) + good, b.host, 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, b.origin+"/state?token="+tc.token, strings.NewReader(tc.body))
			req.Host = tc.host
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Content-Type", tc.contentType)
			response := httptest.NewRecorder()
			b.ServeHTTP(response, req)
			if response.Code != tc.status {
				t.Fatalf("got %d want %d: %s", response.Code, tc.status, response.Body.String())
			}
		})
	}
	if len(states) != 1 || states[0].CurrentTime != 1.5 {
		t.Fatalf("bad state dispatch: %#v", states)
	}
	select {
	case <-b.documentReady:
	default:
		t.Fatal("document readiness not signaled")
	}
}
func TestActionValidation(t *testing.T) {
	var actions []ControlAction
	b, err := newBridge("https://example.invalid/video.mp4", nil, nil, nil, func(a ControlAction) { actions = append(actions, a) })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.close() })
	good := `{"name":"fullscreen","value":"true"}`
	for _, tc := range []struct {
		name, method, token, origin, contentType, body, host string
		status                                               int
	}{
		{"valid", "POST", b.token, b.origin, "application/json", good, b.host, 204},
		{"token", "POST", "wrong", b.origin, "application/json", good, b.host, 403},
		{"host", "POST", b.token, b.origin, "application/json", good, "attacker.invalid", 403},
		{"origin", "POST", b.token, "https://attacker.invalid", "application/json", good, b.host, 403},
		{"missing origin", "POST", b.token, "", "application/json", good, b.host, 403},
		{"type", "POST", b.token, b.origin, "text/plain", good, b.host, 415},
		{"method", "GET", b.token, b.origin, "application/json", good, b.host, 405},
		{"trailing", "POST", b.token, b.origin, "application/json", good + good, b.host, 400},
		{"unknown field", "POST", b.token, b.origin, "application/json", `{"name":"next","url":"x"}`, b.host, 400},
		{"unknown action", "POST", b.token, b.origin, "application/json", `{"name":"exec","value":"true"}`, b.host, 400},
		{"invalid boolean", "POST", b.token, b.origin, "application/json", `{"name":"fullscreen","value":"toggle"}`, b.host, 400},
		{"null", "POST", b.token, b.origin, "application/json", `null`, b.host, 400},
		{"size", "POST", b.token, b.origin, "application/json", strings.Repeat(" ", 4096) + good, b.host, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, b.origin+"/action?token="+tc.token, strings.NewReader(tc.body))
			req.Host = tc.host
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Content-Type", tc.contentType)
			response := httptest.NewRecorder()
			b.ServeHTTP(response, req)
			if response.Code != tc.status {
				t.Fatalf("got %d want %d: %s", response.Code, tc.status, response.Body.String())
			}
		})
	}
	if len(actions) != 1 || actions[0] != (ControlAction{Name: "fullscreen", Value: "true"}) {
		t.Fatalf("dispatch: %#v", actions)
	}
	for _, a := range []ControlAction{{"previous", ""}, {"next", ""}, {"quality", "1080P 高清"}, {"episode", "episode-12"}, {"autoplay", "true"}, {"portrait", "false"}, {"pin", "true"}, {"fullscreen", "false"}, {"danmaku", "false"}, {"danmakuPosition", "top"}, {"danmakuPosition", "mega"}, {"danmakuSize", "small"}, {"danmakuSpeed", "slow"}, {"danmakuSpeed", "fast"}, {"danmakuOpacity", "65"}} {
		if !validAction(a) {
			t.Fatalf("valid action rejected: %#v", a)
		}
	}
	for _, a := range []ControlAction{{"previous", "1"}, {"episode", ""}, {"quality", "\n1080P"}, {"episode", strings.Repeat("x", 257)}, {"quality", "  "}, {"pin", "1"}, {"danmaku", "1"}, {"danmakuPosition", "middle"}, {"danmakuSize", "huge"}, {"danmakuSpeed", "warp"}, {"danmakuOpacity", "5"}, {"danmakuOpacity", "101"}, {"danmakuOpacity", "abc"}} {
		if validAction(a) {
			t.Fatalf("invalid action accepted: %#v", a)
		}
	}
}

func TestStateVolumeAndRate(t *testing.T) {
	base := videoState{Sequence: 1, Volume: 1, Rate: 1}
	if !validState(base) || !validState(videoState{Sequence: 1}) {
		t.Fatal("new and legacy states must validate")
	}
	for _, pair := range [][2]float64{{-1, 1}, {1.1, 1}, {1, -1}, {1, .1}, {1, 5}} {
		s := base
		s.Volume = pair[0]
		s.Rate = pair[1]
		if validState(s) {
			t.Fatalf("invalid volume/rate accepted: %v", pair)
		}
	}
}

func TestLocalMediaRangeAndToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clip.mp4")
	content := "0123456789abcdefghijklmnopqrstuvwxyz"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	b := testBridge(t, file, nil)
	for _, tc := range []struct {
		method, path, rangeHeader string
		status                    int
		body                      string
	}{
		{"GET", "/media?token=" + b.token, "bytes=3-7", 206, "34567"},
		{"GET", "/media?token=" + b.token, "bytes=-3", 206, "xyz"},
		{"GET", "/media?token=" + b.token, "", 200, content},
		{"HEAD", "/media?token=" + b.token, "", 200, ""},
		{"GET", "/media?token=wrong", "", 403, ""},
		{"GET", "/other?token=" + b.token, "", 404, ""},
		{"POST", "/media?token=" + b.token, "", 405, ""},
		{"GET", "/media?token=" + b.token, "bytes=500-700", 416, ""},
	} {
		t.Run(tc.method+tc.path+tc.rangeHeader, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, b.origin+tc.path, nil)
			req.Header.Set("Range", tc.rangeHeader)
			response := httptest.NewRecorder()
			b.ServeHTTP(response, req)
			if response.Code != tc.status {
				t.Fatalf("got %d want %d", response.Code, tc.status)
			}
			if tc.status < 300 && response.Body.String() != tc.body {
				t.Fatalf("body: %q", response.Body.String())
			}
			if tc.status == 206 && response.Header().Get("Accept-Ranges") != "bytes" {
				t.Fatal("missing ranges support")
			}
		})
	}
	req := httptest.NewRequest("GET", b.playerURL(), nil)
	response := httptest.NewRecorder()
	b.ServeHTTP(response, req)
	if response.Code != 200 || !strings.Contains(response.Body.String(), "/media?token=") {
		t.Fatal("local media not wired into page")
	}
	if err := b.close(); err != nil {
		t.Fatal(err)
	}
	if err := b.close(); err != nil {
		t.Fatal("close not idempotent", err)
	}
}
func TestFixtureRange(t *testing.T) {
	// 真实本机 fixture 仅通过 loopback 读取；不启动 GUI，不进行外部网络请求。
	path := filepath.Join("..", "..", "..", ".tools", "native-hevc.mp4")
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		t.Skip("optional local HEVC fixture unavailable")
	}
	if err != nil {
		t.Fatal(err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		t.Fatal(err)
	}
	if info.Size() < 32 {
		file.Close()
		t.Fatal("fixture too small")
	}
	b := testBridge(t, file, nil)
	req, err := http.NewRequest("GET", b.origin+"/media?token="+b.token, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Range", "bytes=0-31")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != 206 || len(body) != 32 {
		t.Fatalf("fixture range: status=%d len=%d err=%v", resp.StatusCode, len(body), err)
	}
	if !strings.Contains(string(body), "ftyp") {
		t.Fatalf("not an mp4 header: %q", body)
	}
}
func TestPlayerCommandsAndState(t *testing.T) {
	p := &Player{commands: make(chan string, 64)}
	if p.Playing() || p.Ready() {
		t.Fatal("uninitialized player reports playing/ready")
	}
	p.Play()
	p.Pause()
	p.Seek(-time.Second)
	p.SetRate(100)
	p.SetVolume(100)
	for _, want := range []string{`window.playerCommand("play",0)`, `window.playerCommand("pause",0)`, `window.playerCommand("seek",0)`, `window.playerCommand("rate",4)`, `window.playerCommand("volume",1)`} {
		if got := <-p.commands; got != want {
			t.Fatalf("command %q want %q", got, want)
		}
	}
	p.state = videoState{Sequence: 1, CurrentTime: 1.5, Duration: 24, ReadyState: 4}
	if !p.Ready() || !p.Playing() || p.Buffering() || p.Position() != 1500*time.Millisecond || p.Duration() != 24*time.Second {
		t.Fatal("invalid mapped player state")
	}
	p.state.Ended = true
	if !p.Ended() || p.Playing() {
		t.Fatal("ended state")
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	p.Play()
	if len(p.commands) != 0 || p.Page() != nil || p.Ready() {
		t.Fatal("closed player accepted commands")
	}
	if _, err := OpenURL("javascript:alert(1)", nil); err == nil {
		t.Fatal("unsafe URL accepted")
	}
	if _, err := Open(filepath.Join(t.TempDir(), "missing.mp4"), nil); err == nil {
		t.Fatal("missing path accepted")
	}
}
