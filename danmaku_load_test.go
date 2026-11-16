package main

import (
	"testing"

	"mygo-player/pluginapi"
)

func TestToPlayerCuesBounds(t *testing.T) {
	in := []pluginapi.Cue{{Time: 1.5, Text: "a"}, {Time: 2.5, Text: "b"}}
	out := toPlayerCues(in)
	if len(out) != 2 || out[0].Time != 1.5 || out[1].Text != "b" {
		t.Fatalf("out = %+v", out)
	}
	many := make([]pluginapi.Cue, 5000)
	for i := range many {
		many[i] = pluginapi.Cue{Time: float64(i), Text: "x"}
	}
	if got := len(toPlayerCues(many)); got != 1200 {
		t.Fatalf("limit = %d, want 1200", got)
	}
}

func TestUpcomingTextReadsNaturally(t *testing.T) {
	if got := upcomingText("更新至0集"); got != "这一季暂无可用分集，可能尚未上线" {
		t.Fatalf("got %q", got)
	}
	if got := upcomingText("预计10月26日上线"); got != "预计10月26日上线" {
		t.Fatalf("got %q", got)
	}
}
