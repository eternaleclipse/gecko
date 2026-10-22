package inspect

import "testing"

func TestSSHTarget(t *testing.T) {
	cases := map[string][]string{
		"devbox":         {"ssh", "devbox"},
		"me@devbox:2222": {"ssh", "-p", "2222", "-l", "me", "devbox", "uptime"},
		"root@devbox":    {"ssh", "root@devbox"},
		"box:2200":       {"ssh", "-oPort=2200", "box"},
		"me@devbox":      {"ssh", "-p", "22", "me@devbox"},
		"root@10.0.0.1":  {"ssh", "-At", "-i", "key", "root@10.0.0.1"},
		"host":           {"ssh", "-oStrictHostKeyChecking=no", "host"},
		"jump-target":    {"ssh", "-J", "bastion", "jump-target"},
	}
	for want, args := range cases {
		if got := SSHTarget(args); got != want {
			t.Errorf("SSHTarget(%v) = %q, want %q", args, got, want)
		}
	}
}

func TestMoshTarget(t *testing.T) {
	if got := MoshTarget([]string{"mosh", "--ssh=ssh -p 22", "me@box"}); got != "me@box" {
		t.Errorf("got %q", got)
	}
	if got := MoshTarget([]string{"mosh-client", "-#", "me@box -- tmux a", "|", "1.2.3.4", "60001"}); got != "me@box" {
		t.Errorf("got %q", got)
	}
}

func TestClassify(t *testing.T) {
	fg := []*Proc{{Pid: 10, Args: []string{"node", "/usr/lib/node_modules/@anthropic-ai/claude-code/cli.js"}}}
	if i := Classify(fg); i.Kind != "agent" || i.Agent != "claude" {
		t.Errorf("got %+v", i)
	}
	if i := Classify([]*Proc{{Args: []string{"/usr/bin/tmux", "a"}}}); i.Kind != "tmux" {
		t.Errorf("got %+v", i)
	}
	if i := Classify([]*Proc{{Args: []string{"codex"}}}); i.Agent != "codex" {
		t.Errorf("got %+v", i)
	}
	// Codex from npm (its node script, then the binary it starts) and
	// OpenCode from snap, as ps shows them.
	for _, args := range [][]string{
		{"node", "/home/u/.local/bin/codex", "-c", "model=x"},
		{"/home/u/.local/lib/node_modules/@openai/codex/node_modules/@openai/codex-linux-x64/vendor/x86_64-unknown-linux-musl/bin/codex", "-c", "model=x"},
		{"node", "/usr/lib/node_modules/@openai/codex/bin/codex.js"},
	} {
		if i := Classify([]*Proc{{Pid: 11, Args: args}}); i.Agent != "codex" {
			t.Errorf("%q: got %+v", args, i)
		}
	}
	for _, args := range [][]string{{"/snap/opencode/277/bin/opencode"}, {"/snap/bin/opencode", "run", "hi"}, {"opencode"}} {
		if i := Classify([]*Proc{{Pid: 12, Args: args}}); i.Agent != "opencode" {
			t.Errorf("%q: got %+v", args, i)
		}
	}
	// pi from npm, by its launcher name and by the path of its bundle.
	for _, args := range [][]string{
		{"pi"},
		{"/home/u/.pi/agent/bin/pi"},
		{"node", "/home/u/.pi/agent/install/releases/1.3.0/node_modules/@earendil-works/pi-coding-agent/dist/bundle/cli.js"},
	} {
		if i := Classify([]*Proc{{Pid: 13, Args: args}}); i.Agent != "pi" {
			t.Errorf("%q: got %+v", args, i)
		}
	}
	if i := Classify(nil); i.Kind != "shell" {
		t.Errorf("got %+v", i)
	}
}

func TestTitleAgent(t *testing.T) {
	cases := []struct{ title, current, agent, status string }{
		{"✳ Claude Code", "", "claude", "idle"},
		{"◐ Claude Code", "", "claude", "working"},
		{"◑ Terminal poem and gecko echo", "claude", "claude", "working"},
		{"◑ Terminal poem and gecko echo", "", "", ""},
		{"✳ Terminal poem and gecko echo", "", "claude", "idle"},
		{"◇  Ready (gecko)", "", "gemini", "idle"},
		{"✦  Reading files (gecko)", "gemini", "gemini", "working"},
		{"✋  Action Required (gecko)", "", "gemini", "needs-input"},
		{"", "claude", "", ""},
		{"user@box: ~/src", "claude", "", ""},
		// Recorded from codex-cli 0.163 and opencode 1.19.
		{"⠹ gecko-agent-demo", "codex", "codex", "working"},
		{"⠹ ⠹ | gecko-agent-demo", "codex", "codex", "working"},
		{"⠹ gecko-agent-demo", "", "", ""},
		{"[ ! ] Action Required | gecko-agent-demo", "codex", "codex", "needs-input"},
		{"[ . ] Action Required | ⠇ | gecko-agent-demo", "codex", "codex", "needs-input"},
		{"[ ! ] Action Required | gecko-agent-demo", "", "", ""},
		{"gecko-agent-demo", "codex", "", ""},
		{"OpenCode", "", "opencode", ""},
		{"OC | Greeting", "", "opencode", ""},
		// Recorded from pi 1.3 in a pty: "π - <dir>", set once and never spun.
		{"π - gecko", "", "pi", ""},
		{"π - /home/u/src/api", "", "pi", ""},
		// A shell whose directory is named "π" is not pi.
		{"π", "", "", ""},
		{"π -", "", "", ""},
	}
	for _, c := range cases {
		a, st, ok := TitleAgent(c.title, c.current)
		if a != c.agent || st != c.status || ok != (c.agent != "") {
			t.Errorf("TitleAgent(%q, %q) = %q %q %v", c.title, c.current, a, st, ok)
		}
	}
}

// A pane as tmux reports it: pane_pid is the pane's shell; what runs in the
// foreground of its tty decides.
func TestPaneAgent(t *testing.T) {
	tty := "/dev/pts/9"
	shell := &Proc{Pid: 100, PPid: 1, Pgid: 100, TTY: tty, Args: []string{"-bash"}}
	table := func(fg ...*Proc) *Table {
		tp := 100
		if len(fg) > 0 {
			tp = fg[0].Pgid
		}
		shell.Tpgid = tp
		ps := []*Proc{shell}
		for _, p := range fg {
			p.Tpgid = tp
			ps = append(ps, p)
		}
		return newTable(ps)
	}
	claude := &Proc{Pid: 200, PPid: 100, Pgid: 200, TTY: tty, Args: []string{"claude"}}
	if a, st := table(claude).PaneAgent(100, "✳ Fix tests"); a != "claude" || st != "idle" {
		t.Errorf("claude in pane: %q %q", a, st)
	}
	if a, st := table(claude).PaneAgent(100, "ubunty"); a != "claude" || st != "" {
		t.Errorf("claude, default title: %q %q", a, st)
	}
	ssh := &Proc{Pid: 300, PPid: 100, Pgid: 300, TTY: tty, Args: []string{"ssh", "box"}}
	if a, st := table(ssh).PaneAgent(100, "◐ Claude Code"); a != "claude" || st != "working" {
		t.Errorf("claude over ssh in pane: %q %q", a, st)
	}
	if a, _ := table(ssh).PaneAgent(100, "box"); a != "" {
		t.Errorf("plain ssh: %q", a)
	}
	// The shell back at its prompt: a title left behind doesn't count.
	if a, _ := table().PaneAgent(100, "✳ Fix tests"); a != "" {
		t.Errorf("stale title: %q", a)
	}
	// A pane started with the agent itself (tmux new 'claude').
	direct := &Proc{Pid: 400, PPid: 1, Pgid: 400, Tpgid: 400, TTY: tty, Args: []string{"node", "/opt/@anthropic-ai/claude-code/cli.js"}}
	if a, _ := newTable([]*Proc{direct}).PaneAgent(400, ""); a != "claude" {
		t.Errorf("direct: %q", a)
	}
	if a, _ := newTable(nil).PaneAgent(1, "✳ x"); a != "" {
		t.Errorf("gone: %q", a)
	}
}
