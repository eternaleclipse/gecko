package hub

import (
	"runtime"

	"github.com/gecko-term/gecko/internal/proto"
	"github.com/gecko-term/gecko/internal/session"
)

// Local is the backend for this machine.
type Local struct{ m *session.Manager }

func (l *Local) Info() proto.HostInfo {
	return proto.HostInfo{Name: l.m.Host(), Local: true, Status: "connected", OS: runtime.GOOS, Version: proto.Version}
}

func (l *Local) Sessions() []proto.SessionInfo { return l.m.List() }

func (l *Local) Create(req proto.CreateReq) (proto.SessionInfo, error) {
	s, err := l.m.Create(req)
	if err != nil {
		return proto.SessionInfo{}, err
	}
	return s.Info(), nil
}

func (l *Local) Attach(id string, offset int64, sink session.Sink) (func(), error) {
	s, err := l.m.Get(id)
	if err != nil {
		return nil, err
	}
	return s.Attach(offset, sink), nil
}

func (l *Local) Input(id string, data []byte) error {
	s, err := l.m.Get(id)
	if err != nil {
		return err
	}
	return s.Write(data)
}

func (l *Local) Resize(id string, cols, rows int) error {
	s, err := l.m.Get(id)
	if err != nil {
		return err
	}
	return s.Resize(cols, rows)
}

func (l *Local) Kill(id string) error {
	s, err := l.m.Get(id)
	if err != nil {
		return err
	}
	return s.Kill()
}

func (l *Local) Patch(id string, p proto.Patch) error {
	s, err := l.m.Get(id)
	if err != nil {
		return err
	}
	s.Patch(p)
	return nil
}

func (l *Local) Capture(id string) (string, error) {
	s, err := l.m.Get(id)
	if err != nil {
		return "", err
	}
	return s.Capture(), nil
}

func (l *Local) Search(q string) ([]proto.Match, error) {
	var out []proto.Match
	for _, info := range l.m.List() {
		s, err := l.m.Get(info.ID)
		if err != nil {
			continue
		}
		lines, n := s.Search(q, 1) // one result per tab: its most recent match
		for _, line := range lines {
			out = append(out, proto.Match{ID: info.ID, Line: line, Count: n})
		}
	}
	return out, nil
}

func (l *Local) ListDir(dir string) (string, []proto.DirEntry, string, error) {
	return session.ListDir(dir)
}

func (l *Local) TmuxDo(action, target, arg, tabID string) error {
	return l.m.TmuxDo(action, target, arg, tabID)
}

func (l *Local) Tmux() []proto.TmuxSession { return l.m.Tmux() }

func (l *Local) Close() {}
