package tmux

import (
	"os"
	"os/exec"
	"testing"
)

// Without a UTF-8 locale tmux prints control characters (like tab) as "_";
// a daemon started over a non-interactive ssh often has no locale at all.
func TestListingWithoutUTF8Locale(t *testing.T) {
	if !Available() {
		t.Skip("no tmux")
	}
	dir, err := os.MkdirTemp("/tmp", "gxt") // tmux socket paths must be short
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	t.Setenv("TMUX_TMPDIR", dir)
	t.Setenv("TMUX", "")
	for _, k := range []string{"LANG", "LC_ALL", "LC_CTYPE"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	t.Setenv("LANG", "C")
	if out, err := exec.Command("tmux", "-u", "new-session", "-d", "-s", "café", "-n", "wörk").CombinedOutput(); err != nil {
		t.Fatalf("tmux: %v %s", err, out)
	}
	defer exec.Command("tmux", "kill-server").Run()

	ss := Sessions()
	if len(ss) != 1 || ss[0].Name != "café" || ss[0].Windows != 1 {
		t.Fatalf("Sessions() = %+v", ss)
	}
	ws := Windows()
	if len(ws) != 1 || ws[0].Session != "café" || ws[0].Name != "wörk" || !ws[0].Active {
		t.Fatalf("Windows() = %+v", ws)
	}
}
