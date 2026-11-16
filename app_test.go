package main

import (
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"mygo-player/internal/catalog"
	"mygo-player/internal/plugins"
)

func TestResumeKeepsExactEpisodeAndPosition(t *testing.T) {
	lib := catalog.Library{History: map[string]catalog.Progress{"show": {Episode: catalog.Episode{ID: "ep2"}, Position: 42.5}}}
	eps := []catalog.Episode{{ID: "ep1"}, {ID: "ep2"}}
	i, pos := resumeTarget(lib, "show", eps)
	if i != 1 || pos != 42.5 {
		t.Fatalf("resume=(%d,%f)", i, pos)
	}
	i, pos = resumeTarget(lib, "other", eps)
	if i != 0 || pos != 0 {
		t.Fatal("unknown series must start at beginning")
	}
	i, pos = resumeTarget(lib, "show", eps[:1])
	if i != 0 || pos != 0 {
		t.Fatal("removed episode must fall back")
	}
}
func TestLibraryHistorySortAndFilter(t *testing.T) {
	lib := catalog.Library{History: map[string]catalog.Progress{"a": {Series: catalog.Series{ID: "a", Title: "旧故事"}, UpdatedAt: time.Unix(1, 0)}, "b": {Series: catalog.Series{ID: "b", Title: "新故事"}, UpdatedAt: time.Unix(2, 0)}}}
	items := libraryItems(lib, "历史", "")
	if len(items) != 2 || items[0].ID != "b" {
		t.Fatal(items)
	}
	items = libraryItems(lib, "历史", "旧")
	if len(items) != 1 || items[0].ID != "a" {
		t.Fatal(items)
	}
}
func TestMergePagesDoesNotDuplicateSeries(t *testing.T) {
	items := mergeSeries([]catalog.Series{{ID: "a"}}, []catalog.Series{{ID: "a"}, {ID: "b"}, {ID: ""}})
	if len(items) != 2 || items[1].ID != "b" {
		t.Fatal(items)
	}
}
func TestPositionsRejectInvalidValues(t *testing.T) {
	for _, v := range []float64{-1, math.NaN(), math.Inf(1)} {
		if validPosition(v) {
			t.Fatal(v)
		}
	}
	if clock(65) != "01:05" {
		t.Fatal(clock(65))
	}
}
func TestNativeLibraryAndEpisodeLayout(t *testing.T) {
	client, e := catalog.New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a := newApp(client, nil)
	defer a.cancel()
	a.page = "收藏"
	a.items = []catalog.Series{{ID: "s1", Title: "长街故事", Episodes: 24}, {ID: "s2", Title: "时光里的我们", Episodes: 36}, {ID: "s3", Title: "山海之间", Episodes: 18}}
	a.selected = a.items[0]
	a.episodes = []catalog.Episode{{ID: "e1", Number: 1}, {ID: "e2", Number: 2}}
	tt := ui.NewTester(a.view, 1280, 820)
	if !tt.HasText("长街故事") || !tt.HasText("选集 · 2 集") {
		t.Fatal(tt.Texts())
	}
	if e := tt.Click("收藏 长街故事"); e != nil {
		t.Fatal(e)
	}
	if _, ok := a.library.Favorites["s1"]; !ok {
		t.Fatal("favorite did not persist")
	}
	if e := tt.Click("收起"); e != nil {
		t.Fatal(e)
	}
	if a.selected.ID != "" {
		t.Fatal("detail remained open")
	}
}
func TestClickableElementsUsePointerCursor(t *testing.T) {
	client, e := catalog.New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a := newApp(client, nil)
	defer a.cancel()
	a.page = "推荐"
	a.items = []catalog.Series{{ID: "a", Title: "长街故事", Episodes: 24}}
	tt := ui.NewTester(a.view, 1280, 820)
	// 封面约在内容区 x 224..470，卡片顶部 y 116 起，封面高约 327。
	tt.Move(340, 260)
	if tt.Cursor() != ui.CursorPointer {
		t.Fatalf("cover cursor = %v, want pointer", tt.Cursor())
	}
	// 按钮「播放」在封面下方。
	tt.Move(280, 570)
	if tt.Cursor() != ui.CursorPointer {
		t.Fatalf("button cursor = %v, want pointer", tt.Cursor())
	}
}

func TestResetDeviceButtonOnlyForAnonymousLimit(t *testing.T) {
	client, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(client, nil)
	defer a.cancel()
	a.showPlayer = true
	a.playMessage = "来源暂时限制了匿名播放，请稍后重试。"
	a.canResetDevice = true
	tt := ui.NewTester(a.view, 1280, 820)
	if !tt.HasText("换设备重试") || !tt.HasText("重试播放") {
		t.Fatalf("missing retry buttons: %v", tt.Texts())
	}
	a.canResetDevice = false
	tt = ui.NewTester(a.view, 1280, 820)
	if tt.HasText("换设备重试") || !tt.HasText("重试播放") {
		t.Fatalf("reset button should be hidden: %v", tt.Texts())
	}
}

func TestRenderPreview(t *testing.T) {
	path := os.Getenv("MYGO_PLAYER_PREVIEW")
	if path == "" {
		t.Skip("set MYGO_PLAYER_PREVIEW to render an illustrative offline UI")
	}
	client, e := catalog.New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a := newApp(client, nil)
	defer a.cancel()
	a.page = "推荐"
	a.items = []catalog.Series{{ID: "a", Title: "长街故事", Episodes: 24}, {ID: "b", Title: "时光里的我们", Episodes: 36}, {ID: "c", Title: "山海之间", Episodes: 18}, {ID: "d", Title: "春日来信", Episodes: 60}, {ID: "e", Title: "灯火人间", Episodes: 32}, {ID: "f", Title: "重逢的夏天", Episodes: 40}}
	tt := ui.NewTester(a.view, 1280, 820)
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		t.Fatal(e)
	}
	f, e := os.Create(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	if e = png.Encode(f, tt.Image()); e != nil {
		t.Fatal(e)
	}
}

func accountProfileStub() accountProfile {
	return accountProfile{UserID: "42", Nickname: "阿红"}
}

func TestApplyCalendarHintMarksUpcoming(t *testing.T) {
	client, e := catalog.New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a := newApp(client, nil)
	defer a.cancel()
	a.selected = catalog.Series{ID: "s", Title: "第十一季"}
	a.detailID = 3
	a.calendar = map[string]catalog.UpcomingEntry{
		"s": {SeriesID: "s", Online: false, PublishAt: time.Now().AddDate(0, 0, 4)},
	}
	a.calendarAt = time.Now()
	a.applyCalendarHint(3, "s")
	if a.selected.ReleaseState != catalog.ReleaseUpcoming || a.detailMessage == "" {
		t.Fatalf("state=%q note=%q", a.selected.ReleaseState, a.detailMessage)
	}
	// 已上线但无分集时不能改成待上线。
	a.selected.ReleaseState = catalog.ReleaseUnknown
	a.calendar["s"] = catalog.UpcomingEntry{SeriesID: "s", Online: true}
	a.detailMessage = "中性"
	a.applyCalendarHint(3, "s")
	if a.selected.ReleaseState != catalog.ReleaseUnknown || a.detailMessage != "中性" {
		t.Fatalf("online entry changed state: %q %q", a.selected.ReleaseState, a.detailMessage)
	}
}

func TestSourceNavigationAndCollapsibleSidebar(t *testing.T) {
	client, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(client, nil)
	defer a.cancel()
	a.page = "推荐"
	for _, source := range []string{"third-source", "fourth-source"} {
		attachUITestPlugin(t, a, plugins.Manifest{ID: source, Name: source, Version: "1", Capabilities: []string{"detail", "resolve"}})
		a.sidebarCollapsed = false
		tt := ui.NewTester(a.view, 1280, 820)
		for _, label := range []string{"随时接着看", "收藏与进度保存在本机", "非官方客户端 · 首版", "匿名浏览", "· 本地收藏与续播"} {
			if tt.HasText(label) {
				t.Fatalf("%s contains removed label %q", source, label)
			}
		}
		for _, label := range []string{"榜单", "账号", "精选", "真人剧", "漫剧", "找剧"} {
			if tt.HasText(label) {
				t.Fatalf("%s unexpected visibility for %q", source, label)
			}
		}
		if err := tt.Click("收起侧栏"); err != nil {
			t.Fatal(err)
		}
		if !a.sidebarCollapsed {
			t.Fatal("sidebar did not collapse")
		}
		if tt.HasText("MyGo Player") {
			t.Fatal("brand must disappear when sidebar collapses")
		}
		if err := tt.Click("展开侧栏"); err != nil {
			t.Fatal(err)
		}
		if a.sidebarCollapsed {
			t.Fatal("sidebar did not expand")
		}
		if !tt.HasText("MyGo Player") {
			t.Fatal("sidebar brand did not return")
		}
	}
}
