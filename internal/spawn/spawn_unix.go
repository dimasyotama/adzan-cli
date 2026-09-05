//go:build !windows

// Package spawn starts the daemon as a detached background process.
package spawn

import (
	"os/exec"
	"syscall"
)

// detach puts the child in a new session so it survives the terminal closing.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
