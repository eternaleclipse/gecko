package inspect

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Snapshot reads the process table from /proc.
func Snapshot() (*Table, error) {
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	ps := make([]*Proc, 0, len(ents))
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		stat, err := os.ReadFile("/proc/" + e.Name() + "/stat")
		if err != nil {
			continue
		}
		// pid (comm) state ppid pgrp session tty_nr tpgid ...
		i := bytes.LastIndexByte(stat, ')')
		if i < 0 {
			continue
		}
		f := strings.Fields(string(stat[i+1:]))
		if len(f) < 6 {
			continue
		}
		p := &Proc{Pid: pid}
		p.PPid, _ = strconv.Atoi(f[1])
		p.Pgid, _ = strconv.Atoi(f[2])
		ttyNr, _ := strconv.Atoi(f[4])
		p.Tpgid, _ = strconv.Atoi(f[5])
		p.TTY = ttyName(ttyNr)
		cmd, _ := os.ReadFile("/proc/" + e.Name() + "/cmdline")
		cmd = bytes.TrimRight(cmd, "\x00")
		if len(cmd) == 0 {
			continue // kernel thread or zombie
		}
		p.Args = strings.Split(string(cmd), "\x00")
		ps = append(ps, p)
	}
	return newTable(ps), nil
}

func ttyName(nr int) string {
	if nr == 0 {
		return ""
	}
	major, minor := (nr>>8)&0xfff, (nr&0xff)|((nr>>12)&0xfff00)
	if major >= 136 && major <= 143 {
		return fmt.Sprintf("/dev/pts/%d", minor+(major-136)*256)
	}
	return ""
}

// Cwd returns the working directory of pid.
func Cwd(pid int) string {
	s, _ := os.Readlink("/proc/" + strconv.Itoa(pid) + "/cwd")
	return s
}
