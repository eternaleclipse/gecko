//go:build !windows

package execx

import "os/exec"

// hide is a no-op outside Windows, where child processes have no windows.
func hide(*exec.Cmd) {}
