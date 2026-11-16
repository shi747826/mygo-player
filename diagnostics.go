package main

import (
	"fmt"
	"os"
)

// Opt-in diagnostics contain only phase names/counts, never media URLs or keys.
func trace(format string, args ...any) {
	if os.Getenv("MYGO_PLAYER_DIAGNOSTICS") != "" {
		fmt.Printf("GUI "+format+"\n", args...)
	}
}
