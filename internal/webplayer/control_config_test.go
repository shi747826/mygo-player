package webplayer

import (
	"strings"
	"testing"
)

func TestControlsConfigurationIsEscapedAndDeduplicated(t *testing.T) {
	p := &Player{commands: make(chan string, 8)}
	cfg := ControlConfig{Title: "</script><script>alert(1)</script>", Episodes: []ControlEpisode{{ID: "123", Number: 1}}}
	p.ConfigureControls(cfg, nil)
	js := <-p.commands
	if strings.Contains(js, "</script>") || !strings.HasPrefix(js, "window.playerConfigure(") {
		t.Fatal("control configuration is not safely encoded")
	}
	p.ConfigureControls(cfg, nil)
	if len(p.commands) != 0 {
		t.Fatal("unchanged config flooded command queue")
	}
	cfg.Fullscreen = true
	p.ConfigureControls(cfg, nil)
	if len(p.commands) != 1 {
		t.Fatal("fullscreen state was not delivered")
	}
	p.closed.Store(true)
	cfg.Fullscreen = false
	p.ConfigureControls(cfg, nil)
	if len(p.commands) != 1 {
		t.Fatal("closed player accepted config")
	}
}
