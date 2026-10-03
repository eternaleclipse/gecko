package inspect

import "testing"

func TestSSHTarget(t *testing.T) {
	cases := map[string][]string{
		"devbox":         {"ssh", "devbox"},
		"me@devbox:2222": {"ssh", "-p", "2222", "-l", "me", "devbox", "uptime"},
		"root@devbox": {"ssh", "root@devbox"},
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
	if i := Classify(nil); i.Kind != "shell" {
		t.Errorf("got %+v", i)
	}
}
