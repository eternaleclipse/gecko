//go:build !windows

package daemon

import (
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
)

func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

// ignoreHangup keeps the daemon alive when its terminal goes away. It
// catches SIGHUP rather than ignoring it: an ignored signal stays ignored in
// every process the daemon starts, and then shells and agents would outlive
// their tabs.
func ignoreHangup() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	go func() {
		for range ch {
		}
	}()
}

// spawnService starts the daemon as a transient systemd user unit when the
// user has lingering enabled, so it also survives logind cleaning up the
// SSH session's scope on logout.
func spawnService(exe string) bool {
	if runtime.GOOS != "linux" || os.Getenv("GECKO_HOME") != "" {
		return false
	}
	if _, err := exec.LookPath("systemd-run"); err != nil {
		return false
	}
	user := os.Getenv("USER")
	out, err := exec.Command("loginctl", "show-user", user, "-p", "Linger").Output()
	if err != nil || strings.TrimSpace(string(out)) != "Linger=yes" {
		return false
	}
	_ = exec.Command("systemctl", "--user", "reset-failed", "gecko.service").Run()
	return exec.Command("systemd-run", "--user", "--unit=gecko", "--collect", "-q", exe, "daemon").Run() == nil
}
