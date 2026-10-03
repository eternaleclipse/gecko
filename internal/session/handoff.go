package session

import (
	"errors"
	"time"

	"github.com/gecko-term/gecko/internal/proto"
	"github.com/gecko-term/gecko/internal/ptyx"
	"github.com/gecko-term/gecko/internal/vt"
)

// State is everything needed to continue a session in a new gecko process
// (`gecko upgrade`): the PTY master stays open across exec, the rest is
// carried in this struct.
type State struct {
	Info       proto.SessionInfo `json:"info"`
	Fd         uintptr           `json:"fd"`
	Pid        int               `json:"pid"`
	TTY        string            `json:"tty"`
	RingStart  int64             `json:"ringStart"`
	Ring       []byte            `json:"-"` // stored next to the state file
	LastOutput time.Time         `json:"lastOutput"`
	LastInput  time.Time         `json:"lastInput"`
	PendingCmd string            `json:"pendingCmd,omitempty"`
}

type fdPty interface{ Fd() uintptr }

// Freeze stops all sessions at a byte boundary and returns their state.
// Sessions stay locked: the caller either execs (handing everything over)
// or calls the returned thaw function to resume.
func (m *Manager) Freeze() (states []State, thaw func(), err error) {
	m.mu.Lock()
	ss := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		ss = append(ss, s)
	}
	locked := []*Session{}
	thaw = func() {
		for _, s := range locked {
			s.mu.Unlock()
		}
		m.mu.Unlock()
	}
	for _, s := range ss {
		s.mu.Lock()
		locked = append(locked, s)
		if s.closed {
			continue
		}
		fp, ok := s.pty.(fdPty)
		if !ok {
			thaw()
			return nil, nil, errors.New("keeping sessions across a restart isn't supported on this platform")
		}
		data, start := s.ring.ReadFrom(0)
		states = append(states, State{
			Info: s.infoLocked(), Fd: fp.Fd(), Pid: s.pty.Pid(), TTY: s.info.TTY,
			RingStart: start, Ring: data, LastOutput: s.lastOutput, LastInput: s.lastInput,
			PendingCmd: s.pendingCmd,
		})
	}
	return states, thaw, nil
}

// Adopt continues sessions handed over by a previous process.
func (m *Manager) Adopt(states []State) []error {
	var errs []error
	for _, st := range states {
		p, err := ptyx.Adopt(st.Fd, st.Pid, st.TTY)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		info := st.Info
		info.Host = m.opts.Host
		ring := NewRing(m.opts.ScrollbackBytes)
		ring.Restore(st.RingStart, st.Ring)
		s := &Session{
			m: m, pty: p, ring: ring, vt: vt.New(max(info.Cols, 1), max(info.Rows, 1), 5000),
			subs: map[Sink]struct{}{}, info: info,
			lastOutput: st.LastOutput, lastInput: st.LastInput, pendingCmd: st.PendingCmd,
		}
		// Rebuild the screen from the buffered output before wiring up the
		// OSC/bell handlers, so replaying history doesn't re-fire them.
		data, _ := ring.ReadFrom(0)
		_, _ = s.vt.Write(data)
		s.vt.OnOSC = s.osc
		s.vt.OnBell = s.bell
		m.mu.Lock()
		m.sessions[info.ID] = s
		m.mu.Unlock()
		go s.readLoop()
	}
	return errs
}
