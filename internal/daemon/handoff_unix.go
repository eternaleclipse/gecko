//go:build !windows

package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/gecko-term/gecko/internal/config"
	"github.com/gecko-term/gecko/internal/hub"
	"github.com/gecko-term/gecko/internal/session"
)

// handoff is what an upgrading daemon passes to its replacement. File
// descriptors stay open across exec; everything else is in this file.
type handoff struct {
	UnixFd   uintptr         `json:"unixFd"`
	TCPFd    uintptr         `json:"tcpFd,omitempty"`
	Sessions []session.State `json:"sessions"`
	Reap     []int           `json:"reap,omitempty"` // helper processes to wait for
}

const handoffEnv = "GECKO_HANDOFF"

func handoffDir() string { return filepath.Join(config.StateDir(), "handoff") }

// executable is the binary to exec: the file at the path we were started
// from, which `make install` may have replaced since.
func executable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe = strings.TrimSuffix(exe, " (deleted)")
	if _, err := os.Stat(exe); err != nil {
		return "", fmt.Errorf("can't find the gecko binary to restart into (%s)", exe)
	}
	return exe, nil
}

// upgrade replaces this process with the binary on disk. On success it
// never returns; sessions, their programs and both listening sockets carry
// over, and clients reconnect and resume where they were.
func upgrade(m *session.Manager, h *hub.Hub, ln, web net.Listener) error {
	exe, err := executable()
	if err != nil {
		return err
	}
	dir := handoffDir()
	_ = os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	hand := handoff{Reap: h.Children()}
	var keep []*os.File // duplicated listener fds, kept open until exec
	dup := func(l net.Listener) (uintptr, error) {
		fl, ok := l.(interface{ File() (*os.File, error) })
		if !ok {
			return 0, errors.New("listener can't be handed over")
		}
		f, err := fl.File()
		if err != nil {
			return 0, err
		}
		keep = append(keep, f)
		return f.Fd(), nil
	}
	if hand.UnixFd, err = dup(ln); err != nil {
		return err
	}
	if web != nil {
		if hand.TCPFd, err = dup(web); err != nil {
			return err
		}
	}
	h.Close() // remote links reconnect from the new process

	states, thaw, err := m.Freeze()
	if err != nil {
		return err
	}
	fail := func(err error) error {
		thaw()
		for _, f := range keep {
			f.Close()
		}
		return err
	}
	for _, st := range states {
		if err := os.WriteFile(filepath.Join(dir, st.Info.ID+".ring"), st.Ring, 0o600); err != nil {
			return fail(err)
		}
	}
	hand.Sessions = states
	fds := []uintptr{hand.UnixFd}
	if hand.TCPFd != 0 {
		fds = append(fds, hand.TCPFd)
	}
	for _, st := range states {
		fds = append(fds, st.Fd)
	}
	for _, fd := range fds {
		if _, err := unix.FcntlInt(fd, unix.F_SETFD, 0); err != nil { // survive exec
			return fail(err)
		}
	}
	b, err := json.Marshal(hand)
	if err != nil {
		return fail(err)
	}
	statePath := filepath.Join(dir, "state.json")
	if err := os.WriteFile(statePath, b, 0o600); err != nil {
		return fail(err)
	}
	env := []string{handoffEnv + "=" + statePath}
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, handoffEnv+"=") {
			env = append(env, e)
		}
	}
	log.Printf("upgrading in place: %s (%d sessions)", exe, len(states))
	err = syscall.Exec(exe, append([]string{exe}, os.Args[1:]...), env)
	return fail(fmt.Errorf("exec %s: %w", exe, err)) // only reached if exec failed
}

// readHandoff loads the state a previous process left for us, if any.
func readHandoff() (*handoff, error) {
	path := os.Getenv(handoffEnv)
	if path == "" {
		return nil, nil
	}
	os.Unsetenv(handoffEnv)
	defer os.RemoveAll(filepath.Dir(path))
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var hand handoff
	if err := json.Unmarshal(b, &hand); err != nil {
		return nil, err
	}
	for i := range hand.Sessions {
		st := &hand.Sessions[i]
		st.Ring, _ = os.ReadFile(filepath.Join(filepath.Dir(path), st.Info.ID+".ring"))
	}
	for _, fd := range append([]uintptr{hand.UnixFd, hand.TCPFd}, fdsOf(hand.Sessions)...) {
		if fd != 0 {
			unix.CloseOnExec(int(fd)) // don't leak them into shells we start
		}
	}
	return &hand, nil
}

func fdsOf(ss []session.State) []uintptr {
	out := make([]uintptr, len(ss))
	for i, s := range ss {
		out[i] = s.Fd
	}
	return out
}

// reap waits for helper processes the previous image started (ssh bridges),
// so they don't linger as zombies. They exit once their pipes close.
func reap(pids []int) {
	for _, pid := range pids {
		go func(pid int) {
			var ws syscall.WaitStatus
			for i := 0; i < 600; i++ {
				if p, err := syscall.Wait4(pid, &ws, syscall.WNOHANG, nil); p == pid || err != nil {
					return
				}
				time.Sleep(100 * time.Millisecond)
			}
		}(pid)
	}
}
