//go:build windows

package attach

import (
	"os"

	"golang.org/x/sys/windows"
)

// enableVT turns on the console's escape-sequence processing, so a session's
// colors, cursor moves and alternate screen render instead of arriving as
// raw escapes. MakeRaw only prepares the input side.
func enableVT() (restore func()) {
	nop := func() {}
	h := windows.Handle(os.Stdout.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return nop
	}
	if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
		return nop
	}
	if err := windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
		return nop
	}
	return func() { _ = windows.SetConsoleMode(h, mode) }
}
