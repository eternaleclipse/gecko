//go:build !windows

package attach

import (
	"os"
	"os/signal"
	"syscall"
)

func onResize(fn func()) (stop func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			fn()
		}
	}()
	return func() { signal.Stop(ch); close(ch) }
}
