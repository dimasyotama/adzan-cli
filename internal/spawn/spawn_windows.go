//go:build windows

package spawn

import (
	"os/exec"
	"syscall"
)

const (
	detachedProcess     = 0x00000008
	createNewProcessGrp = 0x00000200
)

func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: detachedProcess | createNewProcessGrp,
		HideWindow:    true,
	}
}
