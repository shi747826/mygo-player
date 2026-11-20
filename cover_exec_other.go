//go:build !windows

package main

import "os/exec"

func configureCoverCommand(*exec.Cmd) {}
