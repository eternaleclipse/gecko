package session

import (
	"testing"

	"github.com/gecko-term/gecko/internal/inspect"
	"github.com/gecko-term/gecko/internal/proto"
	"github.com/gecko-term/gecko/internal/vt"
)

// Title sequences recorded from Claude Code 2.1.302 in a pty: at start, while
// it works on a prompt, back at its prompt, and on /exit.
const (
	claudeStart   = "\x1b]7501;?\x07\x1b]0;✳ Claude Code\x07\x1b]7501;?\x07"
	claudeWorking = "\x1b]0;◐ Claude Code\x07\x1b]0;◐ Terminal poem and gecko echo\x07\x1b]0;◑ Terminal poem and gecko echo\x07"
	claudeIdle    = "\x1b]0;✳ Terminal poem and gecko echo\x07"
	claudeExit    = "\x1b]0;\x07"
)

func titleSession() *Session {
	s := &Session{vt: vt.New(80, 24, 100)}
	s.vt.OnOSC = s.osc
	return s
}

// pi 1.3 as recorded in a pty: it asks whether the terminal listens
// (OSC 7501 "?", which carries no state) and sets its title once. Its
// title is never spun, so it says nothing about the status.
const (
	piStart = "\x1b]7501;?\x07\x1b]0;π - gecko-agent-demo\x07\x1b]7501;?\x07"
	piExit  = "\x1b]0;\x07"
)

func TestTitleAgentOverSSH(t *testing.T) {
	s := titleSession()
	ssh := inspect.Info{Kind: "ssh", Proc: "ssh", Pid: 42}
	check := func(step, wantName, wantHint string) {
		t.Helper()
		if n, h := s.agentOf(ssh, "", ""); n != wantName || h != wantHint {
			t.Fatalf("%s: agentOf = %q %q, want %q %q", step, n, h, wantName, wantHint)
		}
	}
	check("before", "", "")
	s.vt.Write([]byte("user@box:~$ claude\r\n" + claudeStart))
	check("start", "claude", "idle")
	s.vt.Write([]byte(claudeWorking))
	check("working", "claude", "working")
	s.vt.Write([]byte(claudeIdle))
	check("idle", "claude", "idle")
	s.vt.Write([]byte(claudeExit))
	check("exit", "", "")

	// pi, over ssh: recognised by its title alone, and its OSC 7501 query
	// leaves the status to the general screen rules.
	s.vt.Write([]byte("user@box:~$ pi\r\n" + piStart))
	check("pi start", "pi", "")
	s.vt.Write([]byte(piExit))
	check("pi exit", "", "")

	// Gone when the process it ran under is.
	s.vt.Write([]byte(claudeStart))
	check("again", "claude", "idle")
	if n, _ := s.agentOf(inspect.Info{Kind: "ssh", Pid: 43}, "", ""); n != "" {
		t.Fatalf("another ssh: %q", n)
	}
	check("stays gone", "", "")

	// ... when the local shell is back at its prompt ...
	s.vt.Write([]byte(claudeStart))
	if n, _ := s.agentOf(inspect.Info{Kind: "shell"}, "", ""); n != "" {
		t.Fatalf("shell: %q", n)
	}
	// ... or when a shell prompt (OSC 133) shows.
	s.vt.Write([]byte(claudeStart))
	check("again", "claude", "idle")
	s.vt.Write([]byte("\x1b]133;A\x07"))
	check("prompt", "", "")
}

func TestTitleAgentFalsePositives(t *testing.T) {
	s := titleSession()
	ssh := inspect.Info{Kind: "ssh", Pid: 7}
	for _, title := range []string{"user@box: ~", "vim main.go", "◐ building", "⠋ npm install", "✦ thinking (x)", "Claude Codex", "π", "π -"} {
		s.vt.Write([]byte("\x1b]0;" + title + "\x07"))
		if n, _ := s.agentOf(ssh, "", ""); n != "" {
			t.Errorf("title %q: agent %q", title, n)
		}
	}
	// A spinner title only keeps an agent already seen.
	s.vt.Write([]byte(claudeStart + "\x1b]2;◐ Fixing the tests\x07"))
	if n, h := s.agentOf(ssh, "", ""); n != "claude" || h != "working" {
		t.Errorf("spinner after start: %q %q", n, h)
	}
	s.vt.Write([]byte("\x1b]2;user@box: ~\x07"))
	if n, _ := s.agentOf(ssh, "", ""); n != "" {
		t.Errorf("after the shell's title: %q", n)
	}
}

func TestAgentOfPrecedence(t *testing.T) {
	s := titleSession()
	s.vt.Write([]byte(claudeWorking[:len("\x1b]0;◐ Claude Code\x07")]))
	// A local claude process wins and takes its status hint from the title.
	if n, h := s.agentOf(inspect.Info{Kind: "agent", Agent: "claude", Pid: 5}, "", ""); n != "claude" || h != "working" {
		t.Errorf("process: %q %q", n, h)
	}
	// A tmux client: only the pane counts, not the outer title.
	if n, _ := s.agentOf(inspect.Info{Kind: "tmux", Proc: "tmux", Pid: 6}, "", ""); n != "" {
		t.Errorf("tmux, no pane agent: %q", n)
	}
	if n, h := s.agentOf(inspect.Info{Kind: "tmux", Proc: "tmux", Pid: 6}, "codex", ""); n != "codex" || h != "" {
		t.Errorf("tmux pane: %q %q", n, h)
	}
}

func TestWithHint(t *testing.T) {
	for _, c := range [][3]string{{"idle", "working", "working"}, {"needs-input", "working", "needs-input"}, {"idle", "needs-input", "needs-input"}, {"working", "idle", "working"}, {"idle", "", "idle"}} {
		if got := withHint(c[0], c[1]); got != c[2] {
			t.Errorf("withHint(%q, %q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}

func TestWithoutTmuxStatus(t *testing.T) {
	got := withoutTmuxStatus([]string{"❯ hi", "", "[main] 0:claude* 1:bash-", ""})
	if len(got) != 2 || got[0] != "❯ hi" {
		t.Fatalf("%q", got)
	}
}

// What Codex sends about itself, recorded from codex-cli 0.163 in a pty:
// its state (OSC 7501) and its title, at its prompt, while it works and
// while it waits for an approval (the title blinking).
const (
	codexIdle    = "\x1b]0;gecko-agent-demo\x07\x1b]7501;state=idle:app=codex\x07"
	codexWorking = "\x1b]7501;state=working:app=codex\x07\x1b]0;⠋ gecko-agent-demo\x07\x1b]0;⠙ gecko-agent-demo\x07"
	codexBlocked = "\x1b]0;[ ! ] Action Required | gecko-agent-demo\x07\x1b]7501;state=blocked:app=codex\x07\x1b]0;[ . ] Action Required | gecko-agent-demo\x07"
)

func TestAgentStateOSC(t *testing.T) {
	// A local codex: its state is the hint, the title's too when there is
	// no state.
	s := titleSession()
	s.info.Agent = &proto.AgentState{Name: "codex"}
	local := inspect.Info{Kind: "agent", Agent: "codex", Pid: 9}
	for _, c := range []struct{ step, out, hint string }{
		{"idle", codexIdle, "idle"},
		{"working", codexWorking, "working"},
		{"blocked", codexBlocked, "needs-input"},
		{"idle again", codexIdle, "idle"},
	} {
		s.vt.Write([]byte(c.out))
		if n, h := s.agentOf(local, "", ""); n != "codex" || h != c.hint {
			t.Errorf("%s: agentOf = %q %q, want codex %q", c.step, n, h, c.hint)
		}
	}
	s = titleSession()
	s.info.Agent = &proto.AgentState{Name: "codex"}
	s.vt.Write([]byte("\x1b]0;⠋ gecko-agent-demo\x07"))
	if _, h := s.agentOf(local, "", ""); h != "working" {
		t.Errorf("spinner title alone: %q", h)
	}

	// Over ssh: the state says which agent runs, until the process it
	// came under goes or a prompt shows.
	s = titleSession()
	ssh := inspect.Info{Kind: "ssh", Proc: "ssh", Pid: 42}
	s.vt.Write([]byte("user@box:~$ codex\r\n" + codexIdle))
	if n, h := s.agentOf(ssh, "", ""); n != "codex" || h != "idle" {
		t.Fatalf("ssh idle: %q %q", n, h)
	}
	s.vt.Write([]byte(codexBlocked))
	if n, h := s.agentOf(ssh, "", ""); n != "codex" || h != "needs-input" {
		t.Fatalf("ssh blocked: %q %q", n, h)
	}
	if n, _ := s.agentOf(inspect.Info{Kind: "ssh", Pid: 43}, "", ""); n != "" {
		t.Fatalf("another ssh: %q", n)
	}
	s.vt.Write([]byte(codexWorking + "\x1b]133;A\x07"))
	if n, _ := s.agentOf(ssh, "", ""); n != "" {
		t.Fatalf("after a prompt: %q", n)
	}
	// Queries, unknown apps and states say nothing.
	for _, p := range []string{"?", "state=working:app=vim", "state=working", "app=codex"} {
		s.vt.Write([]byte("\x1b]7501;" + p + "\x07"))
		if n, h := s.agentOf(ssh, "", ""); h != "" || (n != "" && n != "codex") {
			t.Errorf("%q: %q %q", p, n, h)
		}
	}
}

// OpenCode asks on start whether the terminal shows kitty notifications
// (recorded from opencode 1.19): a question, not a notification, so it
// isn't waiting for the user.
func TestKittyNotificationProbe(t *testing.T) {
	s := titleSession()
	s.vt.Write([]byte("\x1b]99;i=opentui-notifications:p=?;\x07"))
	if s.info.Attention {
		t.Fatal("the probe raised attention")
	}
	s.vt.Write([]byte("\x1b]99;i=1:d=1;Build finished\x07"))
	if !s.info.Attention || s.info.Notice != "Build finished" {
		t.Fatalf("a notification: %v %q", s.info.Attention, s.info.Notice)
	}
}
