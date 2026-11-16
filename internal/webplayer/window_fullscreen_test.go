package webplayer

import "testing"

func TestWindowFullscreenActionValidation(t *testing.T) {
	for _, value := range []string{"true", "false"} {
		if !validAction(ControlAction{Name: "windowFullscreen", Value: value}) {
			t.Fatalf("valid action rejected: %s", value)
		}
	}
	for _, value := range []string{"", "1", "yes"} {
		if validAction(ControlAction{Name: "windowFullscreen", Value: value}) {
			t.Fatalf("invalid action accepted: %s", value)
		}
	}
}
