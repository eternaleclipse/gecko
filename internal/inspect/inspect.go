// Package inspect looks at the process table to tell what is running in a
// terminal: plain shell, ssh/mosh to another machine, tmux, an editor, or a
// coding agent.
package inspect

import (
	"path/filepath"
	"strings"
)

// Proc is one process.
type Proc struct {
	Pid, PPid, Pgid, Tpgid int
	TTY                    string // device path, e.g. /dev/pts/3 ("" if none)
	Args                   []string
	Cwd                    string // filled lazily by Cwd()
}

// Name returns the base name of the executable.
func (p *Proc) Name() string {
	if len(p.Args) == 0 {
		return ""
	}
	return strings.TrimPrefix(filepath.Base(p.Args[0]), "-")
}

// Table is a snapshot of the process table.
type Table struct {
	ByPid  map[int]*Proc
	ByPgid map[int][]*Proc
}

func newTable(ps []*Proc) *Table {
	t := &Table{ByPid: map[int]*Proc{}, ByPgid: map[int][]*Proc{}}
	for _, p := range ps {
		t.ByPid[p.Pid] = p
		t.ByPgid[p.Pgid] = append(t.ByPgid[p.Pgid], p)
	}
	return t
}

// Foreground returns the processes in the foreground process group of the
// terminal that shellPid is attached to, leader first. It returns nil when
// the shell itself is in the foreground (i.e. sitting at a prompt).
func (t *Table) Foreground(shellPid int) []*Proc {
	sh := t.ByPid[shellPid]
	if sh == nil || sh.Tpgid <= 0 || sh.Tpgid == sh.Pgid {
		return nil
	}
	group := t.ByPgid[sh.Tpgid]
	out := make([]*Proc, 0, len(group))
	if l := t.ByPid[sh.Tpgid]; l != nil {
		out = append(out, l)
	}
	for _, p := range group {
		if p.Pid != sh.Tpgid {
			out = append(out, p)
		}
	}
	return out
}

// Classification of what runs in a terminal.
type Info struct {
	Kind    string // shell | ssh | mosh | tmux | agent | editor | command
	Proc    string // primary process name
	Command string // its command line
	Remote  string // ssh/mosh destination
	Agent   string // agent name
	Pid     int
}

// Known coding agents, matched against executable and script names.
var agentNames = map[string]string{
	"claude": "claude", "claude-code": "claude", "codex": "codex", "aider": "aider",
	"gemini": "gemini", "opencode": "opencode", "goose": "goose", "amp": "amp",
	"cursor-agent": "cursor", "copilot": "copilot", "crush": "crush", "qwen": "qwen",
	"cline": "cline", "kiro": "kiro", "droid": "droid", "auggie": "auggie", "kimi": "kimi",
}

// AddAgent registers an extra agent executable name.
func AddAgent(exe, name string) { agentNames[exe] = name }

var editors = map[string]bool{"vim": true, "nvim": true, "vi": true, "emacs": true, "nano": true,
	"hx": true, "helix": true, "micro": true, "kak": true, "less": true, "man": true, "htop": true, "top": true, "btop": true}

func baseNoExt(s string) string {
	b := filepath.Base(s)
	for _, ext := range []string{".exe", ".js", ".mjs", ".cjs", ".py", ".ts"} {
		b = strings.TrimSuffix(b, ext)
	}
	return b
}

// AgentOf reports whether p looks like a coding agent.
func AgentOf(p *Proc) string {
	for i, a := range p.Args {
		if i > 3 || (i > 0 && strings.HasPrefix(a, "-")) {
			break
		}
		if n, ok := agentNames[baseNoExt(a)]; ok {
			return n
		}
		if strings.Contains(a, "@anthropic-ai/claude-code") {
			return "claude"
		}
		if strings.Contains(a, "@openai/codex") {
			return "codex"
		}
		if strings.Contains(a, "@google/gemini-cli") {
			return "gemini"
		}
	}
	return ""
}

// Classify inspects the foreground processes of a terminal.
func Classify(fg []*Proc) Info {
	if len(fg) == 0 {
		return Info{Kind: "shell"}
	}
	lead := fg[0]
	info := Info{Kind: "command", Proc: lead.Name(), Command: strings.Join(lead.Args, " "), Pid: lead.Pid}
	for _, p := range fg {
		if a := AgentOf(p); a != "" {
			return Info{Kind: "agent", Proc: p.Name(), Command: strings.Join(p.Args, " "), Agent: a, Pid: p.Pid}
		}
	}
	for _, p := range fg {
		name := baseNoExt(p.Name())
		switch {
		case name == "ssh" || name == "autossh" || name == "et":
			info.Kind, info.Remote = "ssh", SSHTarget(p.Args)
			return info
		case name == "mosh-client" || name == "mosh":
			info.Kind, info.Remote = "mosh", MoshTarget(p.Args)
			return info
		case (name == "perl" || name == "python3") && len(p.Args) > 1 && baseNoExt(p.Args[1]) == "mosh":
			info.Kind, info.Remote = "mosh", MoshTarget(p.Args[1:])
			return info
		case name == "tmux" || name == "zellij" || name == "screen":
			info.Kind, info.Proc = "tmux", name
			return info
		case editors[name]:
			info.Kind = "editor"
		}
	}
	return info
}

// ssh options that take an argument.
const sshArgOpts = "BbcDEeFIiJLlmOopQRSWw"

// SSHTarget extracts the destination from ssh's argv, as user@host or
// user@host:port when a port was given with -p.
func SSHTarget(args []string) string {
	user, port := "", ""
	dest := func(h string) string {
		h = withUser(user, h)
		if port != "" && port != "22" && !strings.Contains(strings.TrimPrefix(h, "ssh://"), ":") {
			h += ":" + port
		}
		return h
	}
	for i := 1; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			if i+1 < len(args) {
				return dest(args[i+1])
			}
			return ""
		}
		if strings.HasPrefix(a, "-") && len(a) > 1 {
			// Flags may be clustered (-vAt); the last one may take an argument.
			for j := 1; j < len(a); j++ {
				if strings.IndexByte(sshArgOpts, a[j]) >= 0 {
					val := a[j+1:]
					if val == "" && i+1 < len(args) {
						i++
						val = args[i]
					}
					switch a[j] {
					case 'l':
						user = val
					case 'p':
						port = val
					case 'o':
						if v, ok := strings.CutPrefix(strings.ToLower(val), "port="); ok {
							port = v
						}
					}
					break
				}
			}
			continue
		}
		return dest(a)
	}
	return ""
}

func withUser(user, host string) string {
	host = strings.TrimPrefix(host, "ssh://")
	if user != "" && !strings.Contains(host, "@") {
		return user + "@" + host
	}
	return host
}

// MoshTarget extracts the destination from mosh / mosh-client argv.
func MoshTarget(args []string) string {
	for i := 1; i < len(args); i++ {
		a := args[i]
		if a == "-#" && i+1 < len(args) {
			// mosh-client -# 'original args' | ip port
			for _, x := range strings.Fields(args[i+1]) {
				if !strings.HasPrefix(x, "-") && !strings.Contains(x, "=") {
					return x
				}
			}
			return ""
		}
		if a == "--" {
			if i+1 < len(args) {
				return args[i+1]
			}
			return ""
		}
		if strings.HasPrefix(a, "-") {
			if (a == "-p" || a == "--port" || a == "--ssh" || a == "--server" || a == "--predict" || a == "--family") && !strings.Contains(a, "=") {
				i++
			}
			continue
		}
		return a
	}
	return ""
}
