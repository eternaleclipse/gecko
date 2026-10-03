package session

import "testing"

func TestTmuxOnScreen(t *testing.T) {
	cases := map[string][]string{
		"work": {"root@devbox:~# ls", "", "[work] 0:bash* 1:vim-                    \"devbox\" 10:31 03-Oct-26", ""},
		"0":    {"[0] 0:bash*                                          \"host\" 09:12 01-Jan-26", "$ ls"},
		"":     {"just a shell", "[x] not a tmux bar"},
		"main": {"$", "[main] 2:htop- 3:zsh*Z"},
	}
	for want, screen := range cases {
		if got := tmuxOnScreen(screen); got != want {
			t.Errorf("tmuxOnScreen(%q) = %q, want %q", screen, got, want)
		}
	}
}
