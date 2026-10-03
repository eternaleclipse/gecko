// Package ptyx is a tiny cross-platform pseudo-terminal abstraction:
// creack/pty on Unix, ConPTY on Windows.
package ptyx

import "io"

// Pty is a running child process attached to a pseudo terminal.
type Pty interface {
	io.ReadWriteCloser
	Resize(cols, rows int) error
	Pid() int
	// TTY returns the slave device path (e.g. /dev/pts/4), or "" if unknown.
	TTY() string
	// Wait blocks until the child exits and returns its exit code.
	Wait() (int, error)
}

// Options describe the process to start.
type Options struct {
	Argv []string
	Dir  string
	Env  []string
	Cols int
	Rows int
}
