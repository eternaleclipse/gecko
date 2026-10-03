package daemon

import (
	"errors"
	"net"

	"github.com/gecko-term/gecko/internal/hub"
	"github.com/gecko-term/gecko/internal/session"
)

type handoff struct {
	UnixFd, TCPFd uintptr
	Sessions      []session.State
	Reap          []int
}

func readHandoff() (*handoff, error) { return nil, nil }

func reap([]int) {}

func upgrade(*session.Manager, *hub.Hub, net.Listener, net.Listener) error {
	return errors.New("restarting in place isn't supported on Windows yet; use gecko stop && gecko")
}
