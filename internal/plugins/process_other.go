//go:build !windows

package plugins

import "os/exec"

func configureBackgroundCommand(*exec.Cmd) {}
