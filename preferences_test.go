package main

import (
	"os"
	"runtime"
	"testing"
	"time"

	"mygo-player/internal/catalog"
)

// testApp 建一个只读写临时目录的应用实例。
func testApp(t *testing.T) *app {
	t.Helper()
	client, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(client, nil)
	t.Cleanup(a.cancel)
	return a
}

func TestPreferencesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := preferences{
		Volume: .35, Rate: 1.5, Autoplay: false, Pinned: true,
		DanmakuEnabled: false, DanmakuPosition: "top",
		DanmakuSize: "small", DanmakuSpeed: "fast", DanmakuOpacity: 45,
	}
	if err := savePreferences(dir, want); err != nil {
		t.Fatal(err)
	}
	if got := loadPreferences(dir); got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestPreferencesDefaultsWhenMissingOrBroken(t *testing.T) {
	dir := t.TempDir()
	if got := loadPreferences(dir); got != defaultPreferences() {
		t.Fatalf("missing file: %+v", got)
	}
	path := preferencesPath(dir)
	if err := os.WriteFile(path, []byte("{ this is not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := loadPreferences(dir); got != defaultPreferences() {
		t.Fatalf("broken file: %+v", got)
	}
	// 损坏的文件保留原样，方便用户自己修。
	if data, err := os.ReadFile(path); err != nil || string(data) != "{ this is not json" {
		t.Fatalf("broken file was overwritten: %q %v", data, err)
	}
}

func TestPreferencesMissingFieldsKeepDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(preferencesPath(dir), []byte(`{"volume":0,"danmakuOpacity":30}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got := loadPreferences(dir)
	if got.Volume != 0 {
		t.Fatalf("explicit zero volume must stay zero: %+v", got)
	}
	if got.DanmakuOpacity != 30 {
		t.Fatalf("opacity not applied: %+v", got)
	}
	def := defaultPreferences()
	if got.Rate != def.Rate || got.DanmakuPosition != def.DanmakuPosition || got.DanmakuSize != def.DanmakuSize || got.DanmakuSpeed != def.DanmakuSpeed || got.Autoplay != def.Autoplay {
		t.Fatalf("missing fields should keep defaults: %+v", got)
	}
}

func TestPreferencesClampOutOfRange(t *testing.T) {
	raw := preferences{
		Volume: 5, Rate: 99, Autoplay: true,
		DanmakuPosition: "middle", DanmakuSize: "huge", DanmakuSpeed: "warp", DanmakuOpacity: 500,
	}
	got, def := raw.sanitized(), defaultPreferences()
	if got.Volume != def.Volume || got.Rate != def.Rate {
		t.Fatalf("volume/rate not clamped: %+v", got)
	}
	if got.DanmakuPosition != def.DanmakuPosition || got.DanmakuSize != def.DanmakuSize || got.DanmakuSpeed != def.DanmakuSpeed || got.DanmakuOpacity != def.DanmakuOpacity {
		t.Fatalf("danmaku not clamped: %+v", got)
	}
	// 0 音量是合法状态，不能被拉回默认值。
	if (preferences{Volume: 0}).sanitized().Volume != 0 {
		t.Fatal("zero volume must stay zero")
	}
}

func TestPreferencesFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions only")
	}
	dir := t.TempDir()
	if err := savePreferences(dir, defaultPreferences()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(preferencesPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("perm = %o want 600", perm)
	}
	if _, err := os.Stat(preferencesPath(dir) + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file left behind: %v", err)
	}
}

func TestApplyPreferencesRestoresPlayerState(t *testing.T) {
	a := testApp(t)
	a.applyPreferences(preferences{
		Volume: .2, Rate: 2, Autoplay: false,
		DanmakuEnabled: false, DanmakuPosition: "half",
		DanmakuSize: "large", DanmakuSpeed: "slow", DanmakuOpacity: 30,
	})
	if a.volume != .2 || a.rate != 2 || a.autoplay || a.danmakuVisible {
		t.Fatalf("app not restored: %+v", a.currentPreferences())
	}
	if a.danmakuPosition != "half" || a.danmakuSize != "large" || a.danmakuSpeed != "slow" || a.danmakuOpacity != 30 {
		t.Fatalf("danmaku not restored: %+v", a.currentPreferences())
	}
}

func TestSavePrefsWritesChangedSettings(t *testing.T) {
	dir := t.TempDir()
	a := testApp(t)
	a.dataDir = dir
	a.volume = .55
	a.danmakuPosition = "mega"
	a.savePrefs()
	waitForFile(t, preferencesPath(dir))
	got := loadPreferences(dir)
	if got.Volume != .55 || got.DanmakuPosition != "mega" {
		t.Fatalf("not persisted: %+v", got)
	}
}

// 同一份设置重复保存不应反复写文件。
func TestSavePrefsSkipsUnchanged(t *testing.T) {
	dir := t.TempDir()
	a := testApp(t)
	a.dataDir = dir
	a.volume = .7
	a.savePrefs()
	waitForFile(t, preferencesPath(dir))
	first := modTime(t, preferencesPath(dir))
	time.Sleep(20 * time.Millisecond)
	a.savePrefs()
	if second := modTime(t, preferencesPath(dir)); !second.Equal(first) {
		t.Fatalf("unchanged settings rewrote the file: %v -> %v", first, second)
	}
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s was never written", path)
}

func modTime(t *testing.T, path string) time.Time {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.ModTime()
}

// 退出时应当立刻落盘，不等去抖。
func TestFlushPrefsWritesImmediately(t *testing.T) {
	dir := t.TempDir()
	a := testApp(t)
	a.dataDir = dir
	a.volume = .42
	a.danmakuOpacity = 55
	a.flushPrefs()
	got := loadPreferences(dir)
	if got.Volume != .42 || got.DanmakuOpacity != 55 {
		t.Fatalf("flush did not persist: %+v", got)
	}
}
