//go:build windows

package ptyx

import (
	"context"
	"errors"
	"strings"

	"github.com/UserExistsError/conpty"
)

type winPty struct{ c *conpty.ConPty }

// Start launches opts.Argv attached to a new ConPTY.
func Start(opts Options) (Pty, error) {
	if len(opts.Argv) == 0 {
		return nil, errors.New("empty argv")
	}
	if !conpty.IsConPtyAvailable() {
		return nil, errors.New("ConPTY requires Windows 10 1809 or newer")
	}
	quoted := make([]string, len(opts.Argv))
	for i, a := range opts.Argv {
		if strings.ContainsAny(a, " \t\"") {
			a = `"` + strings.ReplaceAll(a, `"`, `\"`) + `"`
		}
		quoted[i] = a
	}
	c, err := conpty.Start(strings.Join(quoted, " "),
		conpty.ConPtyDimensions(opts.Cols, opts.Rows),
		conpty.ConPtyWorkDir(opts.Dir),
		conpty.ConPtyEnv(opts.Env))
	if err != nil {
		return nil, err
	}
	return &winPty{c: c}, nil
}

func (p *winPty) Read(b []byte) (int, error)  { return p.c.Read(b) }
func (p *winPty) Write(b []byte) (int, error) { return p.c.Write(b) }
func (p *winPty) Close() error                { return p.c.Close() }
func (p *winPty) Pid() int                    { return p.c.Pid() }
func (p *winPty) TTY() string                 { return "" }
func (p *winPty) Resize(cols, rows int) error { return p.c.Resize(cols, rows) }

func (p *winPty) Wait() (int, error) {
	code, err := p.c.Wait(context.Background())
	return int(code), err
}

// Adopt is not supported on Windows: ConPTY handles can't be handed to a
// new process image the way Unix pty fds survive exec.
func Adopt(fd uintptr, pid int, tty string) (Pty, error) {
	return nil, errors.New("not supported on Windows")
}
