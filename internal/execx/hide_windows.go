//go:build windows

package execx

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// hide keeps a console helper from flashing a window: the daemon runs
// detached, with no console of its own, so Windows would give every child
// a brand-new console window on screen.
func hide(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_NO_WINDOW
}
