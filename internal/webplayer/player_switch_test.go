package webplayer

import "testing"

// 换集期间宿主不能读到上一集的播放状态：只有新片源解出元数据才结束等待，
// 之后迟到的旧代际上报一律丢弃。
func TestMediaSwitchGeneration(t *testing.T) {
	var m mediaSwitch
	if m.switching() {
		t.Fatal("a fresh player must not report a pending switch")
	}
	m.begin()
	if !m.switching() {
		t.Fatal("begin must mark the switch as pending")
	}
	// 换集开始前已经在路上的旧状态既不能结束等待，也不能被当成新片源。
	if !m.observe(videoState{Sequence: 1, MediaGeneration: 0, ReadyState: 4}) {
		t.Fatal("state of the still-current generation must be accepted")
	}
	if !m.switching() {
		t.Fatal("the old generation must not end the switch")
	}
	// 新片源只加载到元数据之前仍算在切换中。
	m.observe(videoState{Sequence: 2, MediaGeneration: 1, ReadyState: 1})
	if !m.switching() {
		t.Fatal("the new generation must not end the switch before metadata")
	}
	m.observe(videoState{Sequence: 3, MediaGeneration: 1, ReadyState: 4})
	if m.switching() {
		t.Fatal("metadata of the new generation must end the switch")
	}
	if m.current() != 1 {
		t.Fatalf("generation = %d, want 1", m.current())
	}
	// 上一集迟到的上报必须被丢弃，否则会把 Ended/Rate 读成旧值。
	if m.observe(videoState{Sequence: 9, MediaGeneration: 0, ReadyState: 4}) {
		t.Fatal("a state from an older generation must be dropped")
	}
	if m.current() != 1 {
		t.Fatalf("stale state moved generation to %d", m.current())
	}
}

func TestMediaSwitchErrorAndCancel(t *testing.T) {
	var m mediaSwitch
	m.begin()
	// 新片源直接失败也要结束等待，宿主要看到错误而不是一直等着。
	m.observe(videoState{Sequence: 1, MediaGeneration: 1, Error: 4})
	if m.switching() {
		t.Fatal("a failed load of the new generation must end the switch")
	}
	m.begin()
	m.cancel()
	if m.switching() {
		t.Fatal("cancel must clear the pending switch")
	}
	// 打开播放器时页面还没上报过代际，第一次上报（代际 0）不能被丢弃。
	var fresh mediaSwitch
	if !fresh.observe(videoState{Sequence: 1, ReadyState: 4}) {
		t.Fatal("the first state of a new player must be accepted")
	}
}
