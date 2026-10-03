// Package session owns the terminal sessions of one machine. Sessions live
// in the daemon, independent of any client, so they keep running when a
// browser tab closes or an SSH/mosh connection drops.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gecko-term/gecko/internal/gitinfo"
	"github.com/gecko-term/gecko/internal/inspect"
	"github.com/gecko-term/gecko/internal/proto"
	"github.com/gecko-term/gecko/internal/ptyx"
	"github.com/gecko-term/gecko/internal/shellint"
	"github.com/gecko-term/gecko/internal/tmux"
	"github.com/gecko-term/gecko/internal/vt"
)

// Version is reported to shells as TERM_PROGRAM_VERSION.
var Version = "dev"

// Sink receives a session's output. Implementations must not block.
type Sink interface {
	Data(offset int64, data []byte)
	// Reset tells the client its offset is gone; data restarts at offset,
	// and prefix restores terminal modes.
	Reset(offset int64, prefix string)
	Exit()
}

// Options configure a Manager.
type Options struct {
	Host            string
	Shell           string
	ScriptsDir      string // where shell integration scripts live
	ScrollbackBytes int
}

// Manager owns all local sessions.
type Manager struct {
	opts Options

	mu       sync.Mutex
	sessions map[string]*Session
	listen   []func(info *proto.SessionInfo, closed string)

	extMu    sync.Mutex
	tmuxList []proto.TmuxSession
	gitMu    sync.Mutex
	gitCache map[string]*gitEntry // by repo root
	onState  func()
}

// NewManager starts a manager and its inspection loops.
func NewManager(opts Options) *Manager {
	if opts.ScrollbackBytes <= 0 {
		opts.ScrollbackBytes = 4 << 20
	}
	if opts.Shell == "" {
		opts.Shell = shellint.DefaultShell()
	}
	if opts.ScriptsDir != "" {
		_ = shellint.Install(opts.ScriptsDir)
	}
	m := &Manager{opts: opts, sessions: map[string]*Session{}, gitCache: map[string]*gitEntry{}}
	go m.loop()
	return m
}

// Host returns the manager's host name.
func (m *Manager) Host() string { return m.opts.Host }

// Listen registers a callback for session changes. closed is a session id
// when the session went away (info is nil then).
func (m *Manager) Listen(fn func(info *proto.SessionInfo, closed string)) {
	m.mu.Lock()
	m.listen = append(m.listen, fn)
	m.mu.Unlock()
}

// OnState registers a callback for changes to the tmux session list.
func (m *Manager) OnState(fn func()) { m.onState = fn }

func (m *Manager) emit(info *proto.SessionInfo, closed string) {
	m.mu.Lock()
	ls := append([]func(*proto.SessionInfo, string){}, m.listen...)
	m.mu.Unlock()
	for _, l := range ls {
		l(info, closed)
	}
}

// Shutdown ends every session and waits briefly for them to exit.
func (m *Manager) Shutdown(timeout time.Duration) {
	m.mu.Lock()
	ss := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		ss = append(ss, s)
	}
	m.mu.Unlock()
	for _, s := range ss {
		_ = s.Kill()
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		n := len(m.sessions)
		m.mu.Unlock()
		if n == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// Get returns a session by id.
func (m *Manager) Get(id string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[id]
	if s == nil {
		return nil, errors.New("no such session: " + id)
	}
	return s, nil
}

// List returns all sessions, oldest first.
func (m *Manager) List() []proto.SessionInfo {
	m.mu.Lock()
	ss := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		ss = append(ss, s)
	}
	m.mu.Unlock()
	out := make([]proto.SessionInfo, 0, len(ss))
	for _, s := range ss {
		out = append(out, s.Info())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created < out[j].Created })
	return out
}

// Tmux returns the host's tmux sessions.
func (m *Manager) Tmux() []proto.TmuxSession {
	m.extMu.Lock()
	defer m.extMu.Unlock()
	return append([]proto.TmuxSession(nil), m.tmuxList...)
}

func newID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		h, _ := os.UserHomeDir()
		return filepath.Join(h, p[1:])
	}
	return p
}

var dropEnv = []string{"TMUX=", "TMUX_PANE=", "STY=", "GECKO_", "TERM=", "TERM_PROGRAM", "COLORTERM=",
	"ZDOTDIR=", "VSCODE_", "ITERM_", "WT_SESSION=", "KITTY_", "ALACRITTY_", "WEZTERM_"}

func baseEnv() []string {
	var env []string
outer:
	for _, e := range os.Environ() {
		for _, d := range dropEnv {
			if strings.HasPrefix(e, d) {
				continue outer
			}
		}
		env = append(env, e)
	}
	return env
}

// Create starts a new session.
func (m *Manager) Create(req proto.CreateReq) (*Session, error) {
	cols, rows := req.Cols, req.Rows
	if cols <= 0 || rows <= 0 {
		cols, rows = 80, 24
	}
	home, _ := os.UserHomeDir()
	cwd := expandHome(req.Cwd)
	if st, err := os.Stat(cwd); cwd == "" || err != nil || !st.IsDir() {
		cwd = home
	}
	id := newID()
	env := append(baseEnv(),
		"TERM=xterm-256color", "COLORTERM=truecolor", "TERM_PROGRAM=gecko",
		"TERM_PROGRAM_VERSION="+Version, "GECKO_SESSION="+id, "GECKO_HOST="+m.opts.Host)
	var argv []string
	shell := m.opts.Shell
	if len(req.Argv) > 0 {
		argv = append([]string{}, req.Argv...)
		if p, err := exec.LookPath(argv[0]); err == nil {
			argv[0] = p
		}
		shell = ""
	} else {
		var extra []string
		argv, extra = shellint.Command(shell, m.opts.ScriptsDir)
		env = append(env, extra...)
	}
	p, err := ptyx.Start(ptyx.Options{Argv: argv, Dir: cwd, Env: env, Cols: cols, Rows: rows})
	if err != nil {
		return nil, err
	}
	ws := req.Workspace
	if ws == "" {
		ws = "default"
	}
	s := &Session{
		m: m, pty: p, ring: NewRing(m.opts.ScrollbackBytes), vt: vt.New(cols, rows, 5000),
		subs: map[Sink]struct{}{},
		info: proto.SessionInfo{
			ID: id, Host: m.opts.Host, Workspace: ws, Name: req.Name, Cwd: cwd,
			TTY: p.TTY(), Created: time.Now().UnixMilli(),
			Cols: cols, Rows: rows, Kind: "shell",
		},
	}
	if shell == "" {
		s.info.Kind, s.info.Command = "command", strings.Join(req.Argv, " ")
	} else {
		s.info.Shell = filepath.Base(shell)
	}
	s.vt.OnOSC = s.osc
	s.vt.OnBell = s.bell
	m.mu.Lock()
	m.sessions[id] = s
	m.mu.Unlock()
	if req.Command != "" {
		// Type the command once the shell shows its first prompt (or after a
		// timeout for shells without integration), so it isn't echoed early.
		s.startup = req.Command + "\r"
		time.AfterFunc(3*time.Second, s.sendStartup)
	}
	go s.readLoop()
	info := s.Info()
	m.emit(&info, "")
	return s, nil
}

// Session is one PTY and everything known about it.
type Session struct {
	m   *Manager
	pty ptyx.Pty

	mu         sync.Mutex
	info       proto.SessionInfo
	ring       *Ring
	vt         *vt.Terminal
	subs       map[Sink]struct{}
	lastOutput time.Time
	lastInput  time.Time
	pendingCmd string
	startup    string
	dirty      bool
	closed     bool
}

// Info returns a snapshot of the session's metadata.
func (s *Session) Info() proto.SessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.infoLocked()
}

func (s *Session) infoLocked() proto.SessionInfo {
	i := s.info
	i.Offset = s.ring.End()
	i.Activity = s.lastOutput.UnixMilli()
	i.Input = s.lastInput.UnixMilli()
	i.AltScreen = s.vt.AltScreen()
	if i.Agent != nil {
		a := *i.Agent
		i.Agent = &a
	}
	return i
}

// readable is implemented by PTYs that can wait for data without reading
// it (Unix). The read itself then happens under the session lock, so
// Freeze can stop every session at an exact byte boundary.
type readable interface{ WaitReadable() error }

func (s *Session) readLoop() {
	buf := make([]byte, 64<<10)
	gate, _ := s.pty.(readable)
	for {
		if gate != nil {
			if err := gate.WaitReadable(); err != nil {
				break
			}
		}
		var n int
		var err error
		if gate != nil {
			s.mu.Lock()
			n, err = s.pty.Read(buf)
			if n > 0 {
				s.output(buf[:n])
			}
			s.mu.Unlock()
		} else {
			n, err = s.pty.Read(buf)
			if n > 0 {
				s.mu.Lock()
				s.output(buf[:n])
				s.mu.Unlock()
			}
		}
		if err != nil {
			break
		}
	}
	code, _ := s.pty.Wait()
	s.mu.Lock()
	s.closed = true
	s.info.Exited, s.info.ExitCode = true, code
	final := s.infoLocked()
	subs := s.subs
	s.subs = map[Sink]struct{}{}
	s.mu.Unlock()
	s.m.emit(&final, "") // clients learn the exit code before the session goes away
	for sub := range subs {
		sub.Exit()
	}
	s.m.mu.Lock()
	delete(s.m.sessions, s.info.ID)
	s.m.mu.Unlock()
	s.m.emit(nil, s.info.ID)
}

// output records and fans out PTY output; called with s.mu held.
func (s *Session) output(b []byte) {
	off := s.ring.End()
	s.ring.Write(b)
	_, _ = s.vt.Write(b)
	s.lastOutput = time.Now()
	data := append([]byte(nil), b...)
	for sub := range s.subs {
		sub.Data(off, data)
	}
}

// Attach subscribes sink to output starting at offset. A negative offset
// means "replay everything still buffered".
func (s *Session) Attach(offset int64, sink Sink) (detach func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		sink.Exit()
		return func() {}
	}
	data, start := s.ring.ReadFrom(max(offset, 0))
	if offset < 0 || start > offset || offset > s.ring.End() {
		sink.Reset(start, s.vt.ModePrefix())
	}
	if len(data) > 0 {
		sink.Data(start, data)
	}
	s.subs[sink] = struct{}{}
	return func() {
		s.mu.Lock()
		delete(s.subs, sink)
		s.mu.Unlock()
	}
}

// Write sends input to the session.
func (s *Session) Write(p []byte) error {
	s.mu.Lock()
	s.lastInput = time.Now()
	if s.info.Attention {
		s.info.Attention, s.dirty = false, true
	}
	s.mu.Unlock()
	_, err := s.pty.Write(p)
	return err
}

// Resize changes the terminal size.
func (s *Session) Resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 || cols > 1000 || rows > 500 {
		return errors.New("bad size")
	}
	s.mu.Lock()
	if s.info.Cols == cols && s.info.Rows == rows {
		s.mu.Unlock()
		return nil
	}
	s.info.Cols, s.info.Rows = cols, rows
	s.vt.Resize(cols, rows)
	s.dirty = true
	s.mu.Unlock()
	return s.pty.Resize(cols, rows)
}

func (s *Session) sendStartup() {
	s.mu.Lock()
	cmd := s.startup
	s.startup = ""
	s.mu.Unlock()
	if cmd != "" {
		_, _ = s.pty.Write([]byte(cmd))
	}
}

// Kill terminates the session.
func (s *Session) Kill() error { return s.pty.Close() }

// Patch updates user metadata.
func (s *Session) Patch(p proto.Patch) {
	s.mu.Lock()
	if p.Name != nil {
		s.info.Name = strings.TrimSpace(*p.Name)
	}
	if p.Workspace != nil && strings.TrimSpace(*p.Workspace) != "" {
		s.info.Workspace = strings.TrimSpace(*p.Workspace)
	}
	if p.Seen {
		s.info.Attention = false
	}
	s.dirty = true
	s.mu.Unlock()
	s.flush()
}

// Search returns up to max lines (most recent first) containing q, case
// insensitively, and how many lines matched in total.
func (s *Session) Search(q string, max int) (lines []string, count int) {
	s.mu.Lock()
	text := s.vt.Text()
	s.mu.Unlock()
	q = strings.ToLower(q)
	all := strings.Split(text, "\n")
	for i := len(all) - 1; i >= 0; i-- {
		if strings.Contains(strings.ToLower(all[i]), q) {
			count++
			if len(lines) < max {
				lines = append(lines, strings.TrimSpace(all[i]))
			}
		}
	}
	return lines, count
}

// Capture returns the session's text, using tmux's own scrollback when the
// session is showing tmux.
func (s *Session) Capture() string {
	s.mu.Lock()
	ts, text := s.info.TmuxSess, s.vt.Text()
	s.mu.Unlock()
	if ts != "" {
		if out, err := tmux.Capture(ts, 20000); err == nil {
			return strings.TrimRight(out, "\n")
		}
	}
	return text
}

func (s *Session) flush() {
	s.mu.Lock()
	if !s.dirty || s.closed {
		s.mu.Unlock()
		return
	}
	s.dirty = false
	info := s.infoLocked()
	s.mu.Unlock()
	s.m.emit(&info, "")
}

// osc handles OSC sequences from the program; called with s.mu held.
func (s *Session) osc(num int, payload string) {
	switch num {
	case 0, 2:
		if t := strings.TrimSpace(payload); t != s.info.Title {
			s.info.Title, s.dirty = t, true
		}
	case 7:
		if u, err := url.Parse(payload); err == nil && u.Path != "" {
			s.setCwd(u.Path)
		}
	case 1337:
		if d, ok := strings.CutPrefix(payload, "CurrentDir="); ok {
			s.setCwd(d)
		}
	case 133:
		s.info.Integrated = true
		switch {
		case payload == "A" || strings.HasPrefix(payload, "A;"):
			s.pendingCmd = ""
			if s.startup != "" {
				go s.sendStartup()
			}
		case payload == "C" || strings.HasPrefix(payload, "C;"):
			s.info.Running, s.info.Command = true, s.pendingCmd
		case payload == "D" || strings.HasPrefix(payload, "D;"):
			if s.info.Running {
				if code, err := strconv.Atoi(strings.TrimPrefix(payload, "D;")); err == nil {
					s.info.LastExit = &code
				}
			}
			s.info.Running = false
		}
		s.dirty = true
	case 633:
		if c, ok := strings.CutPrefix(payload, "E;"); ok {
			c, _, _ = strings.Cut(c, ";") // VS Code appends a nonce
			r := strings.NewReplacer(`\x5c`, `\`, `\x3b`, ";", `\x0a`, "\n")
			s.pendingCmd = r.Replace(c)
		} else if d, ok := strings.CutPrefix(payload, "P;Cwd="); ok {
			s.setCwd(d)
		}
	case 9:
		if strings.HasPrefix(payload, "4;") {
			return // ConEmu progress
		}
		s.notify(payload)
	case 777:
		if rest, ok := strings.CutPrefix(payload, "notify;"); ok {
			s.notify(strings.ReplaceAll(rest, ";", ": "))
		}
	case 99: // kitty notifications
		if _, body, ok := strings.Cut(payload, ";"); ok {
			s.notify(body)
		}
	}
}

func (s *Session) setCwd(d string) {
	if d != "" && d != s.info.Cwd {
		s.info.Cwd, s.dirty = d, true
	}
}

func (s *Session) notify(msg string) {
	s.info.Attention, s.info.Notice, s.dirty = true, strings.TrimSpace(msg), true
}

func (s *Session) bell() {
	// Ignore bells that are just feedback to typing (e.g. failed completion).
	if time.Since(s.lastInput) > 2*time.Second {
		s.info.Attention, s.dirty = true, true
	}
}

func (m *Manager) loop() {
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	for n := 0; ; n++ {
		<-tick.C
		m.mu.Lock()
		ss := make([]*Session, 0, len(m.sessions))
		for _, s := range m.sessions {
			ss = append(ss, s)
		}
		m.mu.Unlock()
		if n%4 == 0 {
			m.inspect(ss, n%8 == 0)
		}
		for _, s := range ss {
			s.flush()
		}
	}
}

// inspect refreshes process-derived state for every session.
func (m *Manager) inspect(ss []*Session, slow bool) {
	table, err := inspect.Snapshot()
	if err != nil {
		table = nil
	}
	now := time.Now()
	for _, s := range ss {
		var cls inspect.Info
		if table != nil && s.pty.Pid() > 0 && runtime.GOOS != "windows" {
			fg := table.Foreground(s.pty.Pid())
			if p := table.ByPid[s.pty.Pid()]; fg == nil && s.info.Shell == "" && p != nil {
				fg = []*inspect.Proc{p} // the session runs a command directly
			}
			cls = inspect.Classify(fg)
		}
		var tmuxSess, cwd string
		if cls.Kind == "tmux" && cls.Proc == "tmux" && s.info.TTY != "" {
			tmuxSess = tmux.ClientSession(s.info.TTY)
		}
		s.mu.Lock()
		integrated := s.info.Integrated
		s.mu.Unlock()
		if !integrated && slow && runtime.GOOS == "linux" {
			cwd = inspect.Cwd(s.pty.Pid())
		}
		// Git repo of the working directory: a few stats and one small read,
		// done outside the lock.
		var repo struct {
			g  gitinfo.Info
			ok bool
			st *proto.GitStatus
		}
		s.mu.Lock()
		dir := s.info.Cwd
		s.mu.Unlock()
		if cwd != "" {
			dir = cwd
		}
		repo.g, repo.ok = gitinfo.Lookup(dir)
		if repo.ok {
			repo.st = m.gitStatus(repo.g.Root)
		}
		s.mu.Lock()
		old := s.info
		if cls.Kind != "" {
			s.info.Kind, s.info.Proc, s.info.Remote = cls.Kind, cls.Proc, cls.Remote
			if cls.Command != "" {
				s.info.Command = cls.Command
			} else if !s.info.Running {
				s.info.Command = ""
			}
		}
		s.info.TmuxSess = tmuxSess
		s.info.TmuxWindow = m.activeTmuxWindow(tmuxSess)
		if cwd != "" {
			s.info.Cwd = cwd
		}
		// tmux on the other side of ssh/mosh isn't in our process table;
		// recognise its status bar on screen instead.
		s.info.RemoteTmux = ""
		if s.info.Kind == "ssh" || s.info.Kind == "mosh" {
			s.info.RemoteTmux = tmuxOnScreen(s.vt.ScreenLines())
		}
		if g, ok := repo.g, repo.ok; ok {
			s.info.Repo, s.info.RepoRoot, s.info.Branch = g.Name, g.Root, g.Branch
			s.info.GitStatus = repo.st
		} else {
			s.info.Repo, s.info.RepoRoot, s.info.Branch, s.info.GitStatus = "", "", "", nil
		}
		if cls.Agent != "" {
			st := agentStatus(s.vt.ScreenLines(), now.Sub(s.lastOutput), now.Sub(s.lastInput), s.info.Attention)
			if s.info.Agent == nil || s.info.Agent.Name != cls.Agent || s.info.Agent.Status != st.Status {
				st.Since = now.UnixMilli()
			} else {
				st.Since = s.info.Agent.Since
			}
			st.Name = cls.Agent
			s.info.Agent = &st
		} else {
			s.info.Agent = nil
		}
		if changed(old, s.info) {
			s.dirty = true
		}
		s.mu.Unlock()
	}
	if slow {
		m.scanTmux()
	}
}

func changed(a, b proto.SessionInfo) bool {
	if a.Kind != b.Kind || a.Proc != b.Proc || a.Remote != b.Remote || a.Command != b.Command ||
		a.TmuxSess != b.TmuxSess || a.TmuxWindow != b.TmuxWindow || a.RemoteTmux != b.RemoteTmux || a.Cwd != b.Cwd || a.Repo != b.Repo || a.Branch != b.Branch ||
		(a.GitStatus == nil) != (b.GitStatus == nil) || (a.GitStatus != nil && *a.GitStatus != *b.GitStatus) || (a.Agent == nil) != (b.Agent == nil) {
		return true
	}
	return a.Agent != nil && *a.Agent != *b.Agent
}

type gitEntry struct {
	st      *proto.GitStatus
	checked time.Time
	running bool
	used    time.Time
}

// gitStatus returns the last known status of the repo at root and refreshes
// it in the background when it is more than 3s old. Repos no tab has looked
// at for a minute are forgotten.
func (m *Manager) gitStatus(root string) *proto.GitStatus {
	m.gitMu.Lock()
	defer m.gitMu.Unlock()
	now := time.Now()
	for r, e := range m.gitCache {
		if now.Sub(e.used) > time.Minute && !e.running {
			delete(m.gitCache, r)
		}
	}
	e := m.gitCache[root]
	if e == nil {
		e = &gitEntry{}
		m.gitCache[root] = e
	}
	e.used = now
	if !e.running && now.Sub(e.checked) > 3*time.Second {
		e.running = true
		go func() {
			st, ok := gitinfo.ReadStatus(root, 2*time.Second)
			m.gitMu.Lock()
			e.running, e.checked = false, time.Now()
			if ok {
				e.st = &proto.GitStatus{Changed: st.Changed, Untracked: st.Untracked, Ahead: st.Ahead, Behind: st.Behind}
			} else {
				e.st = nil
			}
			m.gitMu.Unlock()
		}()
	}
	if e.st == nil {
		return nil
	}
	cp := *e.st
	return &cp
}

// activeTmuxWindow is the name of the active window of a tmux session, from
// the last scan ("" if unknown).
func (m *Manager) activeTmuxWindow(session string) string {
	if session == "" {
		return ""
	}
	m.extMu.Lock()
	defer m.extMu.Unlock()
	for _, t := range m.tmuxList {
		if t.Name == session {
			for _, w := range t.Wins {
				if w.Active {
					return w.Name
				}
			}
		}
	}
	return ""
}

// TmuxDo runs a tmux action and refreshes the session list right away.
// For switch-client, tabID names the Gecko tab whose tmux client to switch.
func (m *Manager) TmuxDo(action, target, arg, tabID string) error {
	client := ""
	if action == "switch-client" {
		s, err := m.Get(tabID)
		if err != nil {
			return err
		}
		client = s.Info().TTY
	}
	err := tmux.Do(action, target, arg, client)
	m.scanTmux()
	m.mu.Lock()
	ss := make([]*Session, 0, len(m.sessions))
	for _, x := range m.sessions {
		ss = append(ss, x)
	}
	m.mu.Unlock()
	m.inspect(ss, false) // tab titles follow the new window right away
	return err
}

// scanTmux lists the host's tmux sessions (with their windows) so they can
// be shown, switched to and attached.
func (m *Manager) scanTmux() {
	var list []proto.TmuxSession
	if tmux.Available() {
		wins := map[string][]proto.TmuxWindow{}
		for _, w := range tmux.Windows() {
			wins[w.Session] = append(wins[w.Session], proto.TmuxWindow{Index: w.Index, Name: w.Name, Active: w.Active, Panes: w.Panes, Command: w.Command})
		}
		for _, t := range tmux.Sessions() {
			list = append(list, proto.TmuxSession{Host: m.opts.Host, Name: t.Name, Windows: t.Windows, Attached: t.Attached, Wins: wins[t.Name]})
		}
	}
	m.extMu.Lock()
	changed := !reflect.DeepEqual(list, m.tmuxList)
	m.tmuxList = list
	m.extMu.Unlock()
	if changed && m.onState != nil {
		m.onState()
	}
}

var (
	reWorking = regexp.MustCompile(`(?i)(esc to interrupt|esc to cancel|ctrl\+c to (interrupt|cancel)|press esc to stop)`)
	reAsk     = regexp.MustCompile(`(?i)(do you want to|would you like to|allow (this|once|always)|\(y/n\)|\[y/n\]|approve|permission|❯ 1\. yes|press enter to|waiting for (your )?(input|approval)|continue\?)`)
	reJunk    = regexp.MustCompile(`^[\s─━│┃╭╮╰╯┌┐└┘├┤┬┴┼═║╔╗╚╝>❯›$#%·•*+\-_=~.:|]*$`)
	reHint    = regexp.MustCompile(`(?i)(for shortcuts|to toggle|bypass permissions|auto-accept|context left|tokens? used|\? for help)`)
)

// tmux's default status bar: "[session] 0:bash* 1:vim-" (status-left, then
// the window list), at the bottom or, with status-position top, the top.
var reTmuxStatus = regexp.MustCompile(`^\[([^\]]{1,40})\] +\d+:\S+?[*\-#!~MZ]*( +\d+:\S+?[*\-#!~MZ]*)*( |$)`)

// tmuxOnScreen returns the tmux session name if a tmux status bar is
// visible on screen.
func tmuxOnScreen(screen []string) string {
	for len(screen) > 0 && strings.TrimSpace(screen[len(screen)-1]) == "" {
		screen = screen[:len(screen)-1]
	}
	if len(screen) == 0 {
		return ""
	}
	for _, l := range []string{screen[len(screen)-1], screen[0]} {
		if m := reTmuxStatus.FindStringSubmatch(l); m != nil {
			return m[1]
		}
	}
	return ""
}

// agentStatus infers what an agent is doing from its screen.
func agentStatus(screen []string, sinceOutput, sinceInput time.Duration, attention bool) proto.AgentState {
	// Look at the bottom of the drawn area, where agents put status and prompts.
	for len(screen) > 0 && strings.TrimSpace(screen[len(screen)-1]) == "" {
		screen = screen[:len(screen)-1]
	}
	from := max(0, len(screen)-20)
	tail := screen[from:]
	st := proto.AgentState{Status: "idle"}
	working := ""
	asking := false
	for _, l := range tail {
		if reWorking.MatchString(l) {
			working = strings.TrimSpace(l)
		}
		if reAsk.MatchString(l) {
			asking = true
		}
	}
	switch {
	case working != "":
		st.Status, st.Detail = "working", working
	case asking || attention:
		st.Status = "needs-input"
	case sinceOutput < 1500*time.Millisecond && sinceInput > time.Second:
		st.Status = "working"
	}
	if st.Detail == "" {
		for i := len(tail) - 1; i >= 0; i-- {
			l := strings.TrimSpace(tail[i])
			if len(l) < 4 || reJunk.MatchString(l) || reHint.MatchString(l) {
				continue
			}
			st.Detail = l
			break
		}
	}
	st.Detail = strings.Trim(st.Detail, " │┃|")
	if r := []rune(st.Detail); len(r) > 160 {
		st.Detail = string(r[:160])
	}
	return st
}
