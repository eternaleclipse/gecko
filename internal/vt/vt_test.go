package vt

import (
	"strings"
	"testing"
)

func TestBasic(t *testing.T) {
	term := New(10, 3, 100)
	var titles []string
	term.OnOSC = func(n int, p string) { titles = append(titles, p) }
	term.Write([]byte("hello\r\nworld\x1b]0;my title\x07\r\n1\r\n2\r\n"))
	if got := term.Text(); got != "hello\nworld\n1\n2" {
		t.Fatalf("text = %q", got)
	}
	if len(titles) != 1 || titles[0] != "my title" {
		t.Fatalf("titles = %v", titles)
	}
}

func TestAltScreenAndModes(t *testing.T) {
	term := New(20, 4, 100)
	term.Write([]byte("shell$ vim\r\n\x1b[?1049h\x1b[?2004h\x1b[H\x1b[2Jeditor\x1b[3;5Hmid"))
	if !term.AltScreen() {
		t.Fatal("expected alt screen")
	}
	if got := strings.Join(term.ScreenLines(), "|"); got != "editor||    mid|" {
		t.Fatalf("screen = %q", got)
	}
	if p := term.ModePrefix(); !strings.Contains(p, "?1049h") || !strings.Contains(p, "?2004h") {
		t.Fatalf("prefix = %q", p)
	}
	term.Write([]byte("\x1b[?1049l"))
	if term.ScreenLines()[0] != "shell$ vim" {
		t.Fatalf("main screen lost: %q", term.ScreenLines())
	}
}

func TestWideAndWrap(t *testing.T) {
	term := New(4, 2, 10)
	term.Write([]byte("ab漢字x"))
	if got := term.Text(); got != "ab漢\n字x" {
		t.Fatalf("text = %q", got)
	}
}
