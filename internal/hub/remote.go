package hub

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gecko-term/gecko/internal/hostsetup"
	"github.com/gecko-term/gecko/internal/proto"
	"github.com/gecko-term/gecko/internal/session"
)

// Remote is a host reached by running `gecko bridge` on it (over SSH by
// default). The remote daemon owns the sessions, so they survive every
// network drop; Remote just reconnects and resumes each stream from the
// last byte it saw, like mosh but for every tab at once.
type Remote struct {
	name, target string
	argv         []string
	hub          *Hub

	mu       sync.Mutex
	conn     proto.Conn
	cmd      *exec.Cmd
	status   string
	errMsg   string
	os       string
	version  int
	sessions map[string]proto.SessionInfo
	tmux     []proto.TmuxSession
	pending  map[int]chan *proto.Msg
	rid      int
	subs     map[string]*rsub
	subN     int
	closed   bool
	stop     chan struct{}
}

type rsub struct {
	id     string
	offset int64
	sink   session.Sink
}

// NewRemote creates a remote backend; call Run to connect.
func NewRemote(name string, argv []string, target string, h *Hub) *Remote {
	return &Remote{name: name, argv: argv, target: target, hub: h, status: "connecting",
		sessions: map[string]proto.SessionInfo{}, pending: map[int]chan *proto.Msg{},
		subs: map[string]*rsub{}, stop: make(chan struct{})}
}

// Run keeps the connection up until Close.
func (r *Remote) Run() {
	backoff := time.Second
	for {
		start := time.Now()
		err := r.serve()
		r.mu.Lock()
		if r.closed {
			r.mu.Unlock()
			return
		}
		r.conn = nil
		r.status, r.errMsg = "error", err.Error()
		for rid, ch := range r.pending {
			close(ch)
			delete(r.pending, rid)
		}
		r.mu.Unlock()
		r.hub.stateChanged()
		if time.Since(start) > 30*time.Second {
			backoff = time.Second
		}
		select {
		case <-time.After(backoff):
		case <-r.stop:
			return
		}
		backoff = min(backoff*2, 30*time.Second)
		r.setStatus("connecting", "")
	}
}

func (r *Remote) setStatus(st, msg string) {
	r.mu.Lock()
	r.status, r.errMsg = st, msg
	r.mu.Unlock()
	r.hub.stateChanged()
}

type tailBuf struct {
	mu sync.Mutex
	b  []byte
}

func (t *tailBuf) Write(p []byte) (int, error) {
	t.mu.Lock()
	t.b = append(t.b, p...)
	if len(t.b) > 2048 {
		t.b = t.b[len(t.b)-2048:]
	}
	t.mu.Unlock()
	return len(p), nil
}

func (t *tailBuf) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.TrimSpace(string(t.b))
}

func (r *Remote) serve() error {
	cmd := exec.Command(r.argv[0], r.argv[1:]...)
	stderr := &tailBuf{}
	cmd.Stderr = stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	conn := proto.NewStreamConn(out, in, closerFunc(func() error {
		in.Close()
		_ = cmd.Process.Kill()
		return nil
	}))
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		conn.Close()
		return errors.New("closed")
	}
	r.cmd = cmd
	r.mu.Unlock()
	defer func() {
		conn.Close()
		_ = cmd.Wait()
	}()

	if err := proto.WriteJSON(conn, &proto.Msg{T: proto.THello, Scope: ScopeLocal, Client: "hub", Version: proto.Version}); err != nil {
		return r.fail(err, stderr)
	}
	// Handshake: the first message must be the hello reply.
	timer := time.AfterFunc(30*time.Second, func() { conn.Close() })
	_, b, err := conn.Read()
	timer.Stop()
	if err != nil {
		return r.fail(err, stderr)
	}
	var hello proto.Msg
	if json.Unmarshal(b, &hello) != nil || hello.T != proto.THello {
		return r.fail(errors.New("unexpected handshake from remote (is gecko installed there?)"), stderr)
	}
	r.mu.Lock()
	r.conn, r.os, r.version, r.status, r.errMsg = conn, hello.OS, hello.Version, "connected", ""
	r.mu.Unlock()
	r.hub.stateChanged()

	go r.resync()
	for {
		bin, b, err := conn.Read()
		if err != nil {
			return r.fail(err, stderr)
		}
		if bin {
			r.onData(b)
			continue
		}
		var m proto.Msg
		if err := json.Unmarshal(b, &m); err != nil {
			continue
		}
		r.onMsg(&m)
	}
}

func (r *Remote) fail(err error, stderr *tailBuf) error {
	if s := stderr.String(); s != "" {
		lines := strings.Split(s, "\n")
		return errors.New(hostsetup.Explain(lines[len(lines)-1], r.name))
	}
	if errors.Is(err, io.EOF) {
		return errors.New("connection closed")
	}
	return err
}

// resync refreshes the session list and resumes every attached stream.
func (r *Remote) resync() {
	reply, err := r.request(&proto.Msg{T: proto.TList})
	if err == nil {
		r.mu.Lock()
		r.sessions = map[string]proto.SessionInfo{}
		for _, s := range reply.Sessions {
			r.sessions[localID(s.ID)] = s
		}
		r.mu.Unlock()
		r.hub.hostReset(r.name, reply.Sessions)
	}
	r.mu.Lock()
	conn := r.conn
	subs := make(map[string]rsub, len(r.subs))
	for k, s := range r.subs {
		subs[k] = *s
	}
	r.mu.Unlock()
	for k, s := range subs {
		if conn != nil {
			_ = proto.WriteJSON(conn, &proto.Msg{T: proto.TAttach, ID: s.id, Sub: k, Offset: s.offset})
		}
	}
}

func (r *Remote) onData(b []byte) {
	kind, sub, off, data, err := proto.DecodeData(b)
	if err != nil || kind != proto.KindOutput {
		return
	}
	r.mu.Lock()
	s := r.subs[sub]
	if s != nil {
		s.offset = off + int64(len(data))
	}
	r.mu.Unlock()
	if s != nil {
		s.sink.Data(off, data)
	}
}

func (r *Remote) onMsg(m *proto.Msg) {
	switch m.T {
	case proto.TReply:
		r.mu.Lock()
		ch := r.pending[m.Rid]
		delete(r.pending, m.Rid)
		r.mu.Unlock()
		if ch != nil {
			ch <- m
		}
	case proto.TSession:
		if m.Session == nil {
			return
		}
		s := *m.Session
		r.mu.Lock()
		r.sessions[localID(s.ID)] = s
		r.mu.Unlock()
		r.hub.sessionEvent(r.name, &s, "")
	case proto.TClosed:
		id := localID(m.ID)
		r.mu.Lock()
		delete(r.sessions, id)
		r.mu.Unlock()
		r.hub.sessionEvent(r.name, nil, id)
	case proto.TState:
		r.mu.Lock()
		r.tmux = m.Tmux
		r.mu.Unlock()
		r.hub.stateChanged()
	case proto.TReset:
		r.mu.Lock()
		s := r.subs[m.Sub]
		if s != nil {
			s.offset = m.Offset
		}
		r.mu.Unlock()
		if s != nil {
			s.sink.Reset(m.Offset, m.Prefix)
		}
	case proto.TExit:
		r.mu.Lock()
		s := r.subs[m.Sub]
		delete(r.subs, m.Sub)
		r.mu.Unlock()
		if s != nil {
			s.sink.Exit()
		}
	}
}

func (r *Remote) send(m *proto.Msg) error {
	r.mu.Lock()
	conn := r.conn
	r.mu.Unlock()
	if conn == nil {
		return fmt.Errorf("host %s is offline", r.name)
	}
	return proto.WriteJSON(conn, m)
}

func (r *Remote) request(m *proto.Msg) (*proto.Msg, error) {
	ch := make(chan *proto.Msg, 1)
	r.mu.Lock()
	conn := r.conn
	if conn == nil {
		r.mu.Unlock()
		return nil, fmt.Errorf("host %s is offline", r.name)
	}
	r.rid++
	m.Rid = r.rid
	r.pending[m.Rid] = ch
	r.mu.Unlock()
	if err := proto.WriteJSON(conn, m); err != nil {
		return nil, err
	}
	select {
	case reply, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("host %s disconnected", r.name)
		}
		if reply.Error != "" {
			return nil, errors.New(reply.Error)
		}
		return reply, nil
	case <-time.After(20 * time.Second):
		r.mu.Lock()
		delete(r.pending, m.Rid)
		r.mu.Unlock()
		return nil, fmt.Errorf("host %s timed out", r.name)
	}
}

func (r *Remote) Info() proto.HostInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	return proto.HostInfo{Name: r.name, Target: r.target, Status: r.status, Error: r.errMsg, OS: r.os, Version: r.version}
}

func (r *Remote) Sessions() []proto.SessionInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]proto.SessionInfo, 0, len(r.sessions))
	for _, s := range r.sessions {
		out = append(out, s)
	}
	return out
}

func (r *Remote) Create(req proto.CreateReq) (proto.SessionInfo, error) {
	reply, err := r.request(&proto.Msg{T: proto.TCreate, Create: &req})
	if err != nil {
		return proto.SessionInfo{}, err
	}
	if reply.Session == nil {
		return proto.SessionInfo{}, errors.New("bad reply")
	}
	s := *reply.Session
	r.mu.Lock()
	r.sessions[localID(s.ID)] = s
	r.mu.Unlock()
	return s, nil
}

func (r *Remote) Attach(id string, offset int64, sink session.Sink) (func(), error) {
	r.mu.Lock()
	if _, ok := r.sessions[id]; !ok && r.conn != nil {
		r.mu.Unlock()
		return nil, errors.New("no such session: " + id)
	}
	r.subN++
	key := fmt.Sprintf("h%d", r.subN)
	r.subs[key] = &rsub{id: id, offset: offset, sink: sink}
	conn := r.conn
	r.mu.Unlock()
	if conn != nil {
		_ = proto.WriteJSON(conn, &proto.Msg{T: proto.TAttach, ID: id, Sub: key, Offset: offset})
	}
	return func() {
		r.mu.Lock()
		delete(r.subs, key)
		r.mu.Unlock()
		_ = r.send(&proto.Msg{T: proto.TDetach, Sub: key})
	}, nil
}

func (r *Remote) Input(id string, data []byte) error {
	r.mu.Lock()
	conn := r.conn
	r.mu.Unlock()
	if conn == nil {
		return fmt.Errorf("host %s is offline", r.name)
	}
	return conn.Write(true, proto.EncodeData(proto.KindInput, id, 0, data))
}

func (r *Remote) Resize(id string, cols, rows int) error {
	return r.send(&proto.Msg{T: proto.TResize, ID: id, Cols: cols, Rows: rows})
}

func (r *Remote) Kill(id string) error { return r.send(&proto.Msg{T: proto.TKill, ID: id}) }

func (r *Remote) Patch(id string, p proto.Patch) error {
	return r.send(&proto.Msg{T: proto.TPatch, ID: id, Patch: &p})
}

func (r *Remote) Capture(id string) (string, error) {
	reply, err := r.request(&proto.Msg{T: proto.TCapture, ID: id})
	if err != nil {
		return "", err
	}
	return reply.Text, nil
}

func (r *Remote) Search(q string) ([]proto.Match, error) {
	reply, err := r.request(&proto.Msg{T: proto.TSearch, Text: q})
	if err != nil {
		return nil, err
	}
	return reply.Matches, nil
}

func (r *Remote) ListDir(dir string) (string, []proto.DirEntry, string, error) {
	reply, err := r.request(&proto.Msg{T: proto.TListDir, Dir: dir})
	if err != nil {
		return dir, nil, "", err
	}
	return reply.Dir, reply.Entries, reply.Text, nil
}

// RestartInPlace asks the gecko on that machine to restart with the binary
// on its disk, keeping its sessions (protocol 3+). The link then reconnects.
func (r *Remote) RestartInPlace() error {
	_, err := r.request(&proto.Msg{T: proto.TUpgrade})
	return err
}

// Version is the protocol version the remote gecko reported.
func (r *Remote) Version() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.version
}

func (r *Remote) TmuxDo(action, target, arg, tabID string) error {
	_, err := r.request(&proto.Msg{T: proto.TTmux, Name: action, Target: target, Data: arg, ID: tabID})
	return err
}

func (r *Remote) Tmux() []proto.TmuxSession {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]proto.TmuxSession(nil), r.tmux...)
}

func (r *Remote) Close() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	close(r.stop)
	conn := r.conn
	r.mu.Unlock()
	if conn != nil {
		conn.Close()
	}
}

type closerFunc func() error

func (f closerFunc) Close() error { return f() }
