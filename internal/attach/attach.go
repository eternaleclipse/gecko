// Package attach connects the current terminal to a Gecko session, like
// `tmux attach`. Useful over plain SSH, or from any terminal emulator.
package attach

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sync"
	"time"

	"golang.org/x/term"

	"github.com/gecko-term/gecko/internal/client"
	"github.com/gecko-term/gecko/internal/proto"
)

var terminalReply = regexp.MustCompile(`\x1b\[[?>=]?[0-9;]*[cRn]|\x1b\][0-9;]*[^\x07\x1b]*(\x07|\x1b\\)|\x1bP[^\x1b]*\x1b\\`)

// DetachKey is Ctrl-\ by default.
const DetachKey = 0x1c

const restoreModes = "\x1b[?1049l\x1b[?25h\x1b[?1000l\x1b[?1002l\x1b[?1003l\x1b[?1006l\x1b[?2004l\x1b[?1l\x1b[0m"

// Run attaches until the session exits or the user detaches.
func Run(c *client.Client, id string) error {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return fmt.Errorf("attach needs a terminal")
	}
	old, err := term.MakeRaw(fd)
	if err != nil {
		return err
	}
	var once sync.Once
	restore := func() {
		once.Do(func() {
			os.Stdout.WriteString(restoreModes)
			_ = term.Restore(fd, old)
		})
	}
	defer restore()

	sendSize := func() {
		if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
			_ = proto.WriteJSON(c.Conn, &proto.Msg{T: proto.TResize, ID: id, Cols: w, Rows: h})
		}
	}
	sendSize()
	stopWinch := onResize(sendSize)
	defer stopWinch()

	os.Stdout.WriteString("\x1b[H\x1b[2J\x1b[3J")
	if err := proto.WriteJSON(c.Conn, &proto.Msg{T: proto.TAttach, ID: id, Sub: "a", Offset: -1}); err != nil {
		return err
	}

	done := make(chan string, 2)
	start := time.Now()
	go func() {
		buf := make([]byte, 32<<10)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				done <- ""
				return
			}
			if time.Since(start) < 1500*time.Millisecond {
				// Answers from this terminal to queries in the replayed
				// history; the program asked long ago, don't type them in.
				n = copy(buf, terminalReply.ReplaceAll(buf[:n], nil))
				if n == 0 {
					continue
				}
			}
			for i := 0; i < n; i++ {
				if buf[i] == DetachKey {
					if i > 0 {
						_ = c.Conn.Write(true, proto.EncodeData(proto.KindInput, id, 0, buf[:i]))
					}
					done <- "[detached from " + id + "]"
					return
				}
			}
			if err := c.Conn.Write(true, proto.EncodeData(proto.KindInput, id, 0, buf[:n])); err != nil {
				done <- ""
				return
			}
		}
	}()
	go func() {
		for {
			bin, b, err := c.Conn.Read()
			if err != nil {
				done <- "[lost connection to gecko daemon]"
				return
			}
			if bin {
				if _, _, _, data, err := proto.DecodeData(b); err == nil {
					os.Stdout.Write(data)
				}
				continue
			}
			var m proto.Msg
			if json.Unmarshal(b, &m) != nil {
				continue
			}
			switch m.T {
			case proto.TReset:
				os.Stdout.WriteString(m.Prefix)
			case proto.TExit:
				msg := "[session " + id + " exited]"
				if m.Error != "" {
					msg = "[" + m.Error + "]"
				}
				done <- msg
				return
			}
		}
	}()
	msg := <-done
	_ = proto.WriteJSON(c.Conn, &proto.Msg{T: proto.TDetach, Sub: "a"})
	restore()
	if msg != "" {
		fmt.Fprintln(os.Stderr, "\r\n"+msg)
	}
	return nil
}
