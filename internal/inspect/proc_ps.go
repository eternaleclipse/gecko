//go:build darwin || freebsd || openbsd || netbsd

package inspect

import (
	"os/exec"
	"strconv"
	"strings"
)

// Snapshot reads the process table using ps(1).
func Snapshot() (*Table, error) {
	out, err := exec.Command("ps", "-axww", "-o", "pid=,ppid=,pgid=,tpgid=,tty=,args=").Output()
	if err != nil {
		return nil, err
	}
	var ps []*Proc
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) < 6 {
			continue
		}
		p := &Proc{}
		p.Pid, _ = strconv.Atoi(f[0])
		p.PPid, _ = strconv.Atoi(f[1])
		p.Pgid, _ = strconv.Atoi(f[2])
		p.Tpgid, _ = strconv.Atoi(f[3])
		if f[4] != "??" && f[4] != "-" {
			p.TTY = "/dev/" + f[4]
			if !strings.HasPrefix(f[4], "tty") {
				p.TTY = "/dev/tty" + f[4]
			}
		}
		p.Args = f[5:]
		ps = append(ps, p)
	}
	return newTable(ps), nil
}

// Cwd returns the working directory of pid.
func Cwd(pid int) string {
	out, err := exec.Command("lsof", "-a", "-d", "cwd", "-Fn", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return ""
	}
	for _, l := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(l, "n") {
			return l[1:]
		}
	}
	return ""
}
