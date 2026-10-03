package attach

import (
	"os"
	"time"

	"golang.org/x/term"
)

// Windows has no SIGWINCH; poll the console size.
func onResize(fn func()) (stop func()) {
	done := make(chan struct{})
	go func() {
		w0, h0, _ := term.GetSize(int(os.Stdout.Fd()))
		t := time.NewTicker(300 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil && (w != w0 || h != h0) {
					w0, h0 = w, h
					fn()
				}
			}
		}
	}()
	return func() { close(done) }
}
