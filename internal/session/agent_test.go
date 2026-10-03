package session

import (
	"testing"
	"time"

	"github.com/gecko-term/gecko/internal/vt"
)

func TestAgentStatus(t *testing.T) {
	term := vt.New(80, 40, 100)
	term.Write([]byte("\x1b[?1049h\x1b[H\x1b[2J╭──────╮\r\n│ > fix the bug │\r\n✻ Thinking… (3s · esc to interrupt)\r\n"))
	st := agentStatus(term.ScreenLines(), 5*time.Second, time.Hour, false)
	if st.Status != "working" || st.Detail != "✻ Thinking… (3s · esc to interrupt)" {
		t.Fatalf("%+v %q", st, term.ScreenLines())
	}
	term.Write([]byte("\x1b[H\x1b[2JEdit file src/main.go\r\nDo you want to make this edit?\r\n❯ 1. Yes\r\n  2. No\r\n"))
	st = agentStatus(term.ScreenLines(), 5*time.Second, time.Hour, false)
	if st.Status != "needs-input" {
		t.Fatalf("%+v %q", st, term.ScreenLines())
	}
}
