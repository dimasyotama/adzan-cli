//go:build !windows

package audio

import (
	"os/exec"
	"syscall"
)

// setProcAttr puts the player in its own process group so that killing it
// also kills any helper children it spawned.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killProcess(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	// Negative pid targets the whole process group.
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		_ = cmd.Process.Kill()
	}
}
