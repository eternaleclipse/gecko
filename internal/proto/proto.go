// Package proto defines Gecko's wire protocol.
//
// A connection carries two kinds of messages:
//   - text messages: JSON objects with a "t" (type) field, used for control;
//   - binary messages: terminal data frames (see EncodeData / DecodeData).
//
// The same protocol runs over WebSockets (browser clients), over the daemon's
// local socket (CLI, bridges) and over SSH stdio (remote hosts), so a remote
// daemon looks exactly like a local one.
package proto

import (
	"encoding/binary"
	"errors"
)

// Version is the protocol version, bumped whenever messages are added or
// changed, so a client can tell it is talking to an older service.
//
//	1: initial
//	2: search, probehost, installhost, hostlog; git status
//	3: upgrade (restart in place, keeping sessions)
//	4: listdir
//	5: updatehost, settings sync
//	6: tmux windows and actions
const Version = 6

// Data frame kinds.
const (
	KindOutput byte = 1 // server -> client: PTY output
	KindInput  byte = 2 // client -> server: keyboard input
)

// EncodeData builds a binary frame: kind | len(id) | id | offset(u64 BE) | data.
func EncodeData(kind byte, id string, offset int64, data []byte) []byte {
	b := make([]byte, 0, 10+len(id)+len(data))
	b = append(b, kind, byte(len(id)))
	b = append(b, id...)
	b = binary.BigEndian.AppendUint64(b, uint64(offset))
	return append(b, data...)
}

// DecodeData parses a frame built by EncodeData. data aliases b.
func DecodeData(b []byte) (kind byte, id string, offset int64, data []byte, err error) {
	if len(b) < 2 {
		return 0, "", 0, nil, errors.New("short frame")
	}
	n := int(b[1])
	if len(b) < 2+n+8 {
		return 0, "", 0, nil, errors.New("short frame")
	}
	kind, id = b[0], string(b[2:2+n])
	offset = int64(binary.BigEndian.Uint64(b[2+n:]))
	return kind, id, offset, b[2+n+8:], nil
}

// Msg is the envelope for every JSON control message. Only the fields
// relevant to a given type are set.
type Msg struct {
	T   string `json:"t"`
	Rid int    `json:"rid,omitempty"` // request id; replies echo it

	// hello
	Scope   string `json:"scope,omitempty"` // "all" (default) or "local" (no federation)
	Client  string `json:"client,omitempty"`
	Host    string `json:"host,omitempty"`
	Version int    `json:"version,omitempty"`
	OS      string `json:"os,omitempty"`

	// session addressing
	ID     string `json:"id,omitempty"`
	Sub    string `json:"sub,omitempty"` // attachment alias used in data frames
	Offset int64  `json:"offset,omitempty"`
	Cols   int    `json:"cols,omitempty"`
	Rows   int    `json:"rows,omitempty"`

	Create *CreateReq `json:"create,omitempty"`
	Patch  *Patch     `json:"patch,omitempty"`
	Name   string     `json:"name,omitempty"`
	Target string     `json:"target,omitempty"`
	Data   string     `json:"data,omitempty"`

	// replies / pushes
	OK        bool          `json:"ok,omitempty"`
	Missing   bool          `json:"missing,omitempty"` // probehost: reachable but gecko not installed
	Error     string        `json:"error,omitempty"`
	Session   *SessionInfo  `json:"session,omitempty"`
	Sessions  []SessionInfo `json:"sessions,omitempty"`
	Hosts     []HostInfo    `json:"hosts,omitempty"`
	Tmux      []TmuxSession `json:"tmux,omitempty"`
	Templates []Workspace   `json:"templates,omitempty"`
	Matches   []Match       `json:"matches,omitempty"`
	Dir       string        `json:"dir,omitempty"`
	Entries   []DirEntry    `json:"entries,omitempty"`
	Text      string        `json:"text,omitempty"`
	Prefix    string        `json:"prefix,omitempty"` // terminal modes to replay before data on reset
}

// Message types.
const (
	THello       = "hello"
	TReply       = "reply"
	TList        = "list"        // -> reply{sessions}
	TCreate      = "create"      // create -> reply{session}
	TAttach      = "attach"      // id, sub, offset -> data frames (+ "reset" if offset too old)
	TDetach      = "detach"      // sub
	TResize      = "resize"      // id, cols, rows
	TKill        = "kill"        // id
	TPatch       = "patch"       // id, patch
	TCapture     = "capture"     // id -> reply{text}: full scrollback as plain text (tmux-aware)
	TSearch      = "search"      // text -> reply{matches}: lines in any session containing text
	TInput       = "input"       // id, data (text) - JSON alternative to binary input frames
	TOpenWS      = "openws"      // name -> creates the tabs of a workspace template
	TAddHost     = "addhost"     // name, target, data (gecko path on the host)
	TProbeHost   = "probehost"   // target, data, sub -> "hostlog" pushes, reply{os, text (version), missing}
	TInstall     = "installhost" // target, os ("linux/amd64"), sub -> "hostlog" pushes, reply
	THostLog     = "hostlog"     // push: sub, text (one line of ssh output)
	TUpgrade     = "upgrade"     // restart the service with the binary on disk, keeping sessions
	TWindow      = "window"      // text: JSON window bounds after fitting, so `gecko open` can start there
	TListDir     = "listdir"     // id (which host), dir ("" = the tab's cwd, ~ ok) -> reply{dir, entries, text: home}
	TUpdateHost  = "updatehost"  // name, sub -> "hostlog" pushes; installs this gecko there and restarts it in place
	TSettings    = "settings"    // push: text = settings.json (interface settings, key bindings)
	TSetSettings = "setsettings" // text = the whole settings object to save (and broadcast)
	TTmux        = "tmux"        // host, name (action), target, data (arg), id (tab, for switch-client) -> reply
	TRmHost      = "rmhost"      // name
	TReset       = "reset"       // push: sub, offset, prefix
	TExit        = "exit"        // push: sub - session ended, no more data
	TSession     = "session"     // push: session changed
	TClosed      = "closed"      // push: id removed
	TState       = "state"       // push: hosts, tmux, templates
	TSessions    = "sessions"    // push: full session list (on connect/reconnect of a host)
)

// GitStatus is a repository's working-tree state (what a powerline prompt shows).
type GitStatus struct {
	Changed   int `json:"changed,omitempty"`
	Untracked int `json:"untracked,omitempty"`
	Ahead     int `json:"ahead,omitempty"`
	Behind    int `json:"behind,omitempty"`
}

// DirEntry is a folder inside a listed folder.
type DirEntry struct {
	Name   string `json:"name"`
	Git    bool   `json:"git,omitempty"`    // contains a git repository
	Hidden bool   `json:"hidden,omitempty"` // dot folder
}

// Match is a line of terminal text that matched a search.
type Match struct {
	ID    string `json:"id"`
	Line  string `json:"line"`
	Count int    `json:"count"` // matching lines in that session
}

// CreateReq asks for a new session.
type CreateReq struct {
	Host      string   `json:"host,omitempty"`
	Workspace string   `json:"workspace,omitempty"`
	Name      string   `json:"name,omitempty"`
	Cwd       string   `json:"cwd,omitempty"`
	Argv      []string `json:"argv,omitempty"`    // exec directly
	Command   string   `json:"command,omitempty"` // run inside the user's shell, shell stays afterwards
	Cols      int      `json:"cols,omitempty"`
	Rows      int      `json:"rows,omitempty"`
}

// Patch updates mutable session metadata.
type Patch struct {
	Name      *string `json:"name,omitempty"`
	Workspace *string `json:"workspace,omitempty"`
	Seen      bool    `json:"seen,omitempty"` // clears the attention flag
}

// SessionInfo is everything Gecko knows about a terminal session.
type SessionInfo struct {
	ID        string     `json:"id"`
	Host      string     `json:"host"`
	Workspace string     `json:"workspace"`
	Name      string     `json:"name,omitempty"`  // user-chosen
	Title     string     `json:"title,omitempty"` // OSC 0/2 title
	Shell     string     `json:"shell,omitempty"`
	Cwd       string     `json:"cwd,omitempty"`
	Repo      string     `json:"repo,omitempty"`     // git repository name
	RepoRoot  string     `json:"repoRoot,omitempty"` // its work tree root
	Branch    string     `json:"branch,omitempty"`
	GitStatus *GitStatus `json:"git,omitempty"` // nil until known
	TTY       string     `json:"tty,omitempty"`
	Created   int64      `json:"created"`
	Cols      int        `json:"cols"`
	Rows      int        `json:"rows"`
	Offset    int64      `json:"offset"`   // total output bytes
	Activity  int64      `json:"activity"` // unix ms of last output
	Input     int64      `json:"input"`    // unix ms of last input

	// Shell awareness.
	Integrated bool   `json:"integrated,omitempty"` // shell integration is reporting
	Running    bool   `json:"running,omitempty"`    // a command is executing
	Command    string `json:"command,omitempty"`    // foreground command line
	Proc       string `json:"proc,omitempty"`       // foreground process name
	Kind       string `json:"kind"`                 // shell|ssh|mosh|tmux|agent|editor|command
	Remote     string `json:"remote,omitempty"`     // ssh/mosh destination
	TmuxSess   string `json:"tmuxSession,omitempty"`
	RemoteTmux string `json:"remoteTmux,omitempty"` // tmux session seen on screen inside ssh/mosh
	TmuxWindow string `json:"tmuxWindow,omitempty"` // name of the active window of TmuxSess
	LastExit   *int   `json:"lastExit,omitempty"`
	AltScreen  bool   `json:"altScreen,omitempty"`
	Attention  bool   `json:"attention,omitempty"` // bell / notification not yet seen
	Notice     string `json:"notice,omitempty"`    // last notification text

	Agent *AgentState `json:"agent,omitempty"`

	Exited   bool `json:"exited,omitempty"`
	ExitCode int  `json:"exitCode,omitempty"`
}

// AgentState describes a coding agent running in a session or tmux pane.
type AgentState struct {
	Name   string `json:"name"`             // claude, codex, aider, ...
	Status string `json:"status"`           // working | needs-input | idle
	Detail string `json:"detail,omitempty"` // most recent meaningful screen line
	Since  int64  `json:"since"`            // unix ms of last status change
}

// TmuxSession is a tmux session on some host.
type TmuxSession struct {
	Host     string       `json:"host"`
	Name     string       `json:"name"`
	Windows  int          `json:"windows"`
	Attached int          `json:"attached"`
	Wins     []TmuxWindow `json:"wins,omitempty"`
}

// TmuxWindow is a window of a tmux session.
type TmuxWindow struct {
	Index   int    `json:"index"`
	Name    string `json:"name"`
	Active  bool   `json:"active,omitempty"`
	Panes   int    `json:"panes,omitempty"`
	Command string `json:"command,omitempty"`
}

// HostInfo describes a machine Gecko can run sessions on.
type HostInfo struct {
	Name   string `json:"name"`
	Target string `json:"target,omitempty"` // ssh destination
	Local  bool   `json:"local,omitempty"`
	Status string `json:"status"` // connected | connecting | error
	// Version is the protocol version of the gecko on that machine (0 if
	// unknown); lower than ours means it should be updated.
	Version int    `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
	OS      string `json:"os,omitempty"`
}

// Workspace is a saved project layout: a set of tabs on a host.
type Workspace struct {
	Name string    `json:"name"`
	Host string    `json:"host,omitempty"`
	Cwd  string    `json:"cwd,omitempty"`
	Tabs []TabSpec `json:"tabs,omitempty"`
}

// TabSpec is one tab of a workspace template.
type TabSpec struct {
	Name    string `json:"name,omitempty"`
	Command string `json:"command,omitempty"`
	Cwd     string `json:"cwd,omitempty"`
	Host    string `json:"host,omitempty"`
}
