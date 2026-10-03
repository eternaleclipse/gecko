package daemon

import (
	"os/exec"
	"syscall"
)

func detach(cmd *exec.Cmd) {
	const detachedProcess = 0x00000008
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | detachedProcess}
}

func ignoreHangup() {}

func spawnService(string) bool { return false }
