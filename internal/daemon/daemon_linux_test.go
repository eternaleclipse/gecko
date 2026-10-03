package daemon

import (
	"io"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gecko-term/gecko/internal/ptyx"
)

// Children of the daemon must not inherit an ignored SIGHUP, or programs in
// a tab survive closing it.
func TestChildrenGetDefaultSIGHUP(t *testing.T) {
	ignoreHangup()
	p, err := ptyx.Start(ptyx.Options{Argv: []string{"/bin/sh", "-c", "grep SigIgn /proc/self/status"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(p)
	p.Wait()
	f := strings.Fields(string(out))
	if len(f) < 2 {
		t.Fatalf("unexpected output %q", out)
	}
	mask, _ := strconv.ParseUint(f[1], 16, 64)
	if mask&(1<<(syscall.SIGHUP-1)) != 0 {
		t.Fatalf("child ignores SIGHUP (SigIgn=%s)", f[1])
	}
}

// Closing a session ends its foreground job too, even one that ignores HUP.
func TestCloseKillsStubbornJob(t *testing.T) {
	p, err := ptyx.Start(ptyx.Options{Argv: []string{"/bin/sh", "-c", "trap '' HUP; sleep 30"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	go io.Copy(io.Discard, p)
	done := make(chan struct{})
	go func() { p.Wait(); close(done) }()
	time.Sleep(200 * time.Millisecond)
	p.Close()
	select {
	case <-done:
	case <-time.After(6 * time.Second):
		t.Fatal("session survived Close")
	}
}
