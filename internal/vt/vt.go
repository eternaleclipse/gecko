// Package vt is a small headless terminal emulator. It does not track colors;
// its job is to know what text is on screen (for agent awareness, copy mode
// and capture) plus the modes needed to restore a terminal on reattach, and
// to surface OSC sequences and bells to the session.
package vt

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	stGround = iota
	stEsc
	stEscSkip // ESC ( X etc: skip one byte
	stCSI
	stOSC
	stOSCEsc
	stString // DCS/APC/PM/SOS: ignored until ST
	stStringEsc
)

const wideTail = -1 // marks the second cell of a double-width rune

// Terminal is a headless screen. It is not safe for concurrent use.
type Terminal struct {
	cols, rows int
	main, alt  [][]rune
	altOn      bool
	cx, cy     int
	saved      [2]int
	top, bot   int
	wrapNext   bool
	lastRune   rune

	scrollback    [][]rune
	maxScrollback int

	modes map[int]bool

	state  int
	params []byte
	osc    []byte
	ubuf   []byte
	uneed  int

	// OnOSC is called with the OSC number and the payload after "N;".
	OnOSC func(num int, payload string)
	// OnBell is called on BEL outside of control strings.
	OnBell func()
}

// New creates a terminal of the given size keeping maxScrollback lines.
func New(cols, rows, maxScrollback int) *Terminal {
	t := &Terminal{maxScrollback: maxScrollback, modes: map[int]bool{25: true}}
	t.cols, t.rows = max(cols, 1), max(rows, 1)
	t.main = blank(t.cols, t.rows)
	t.alt = blank(t.cols, t.rows)
	t.top, t.bot = 0, t.rows-1
	return t
}

func blank(cols, rows int) [][]rune {
	s := make([][]rune, rows)
	for i := range s {
		s[i] = make([]rune, cols)
	}
	return s
}

func (t *Terminal) screen() [][]rune {
	if t.altOn {
		return t.alt
	}
	return t.main
}

// Size returns the current dimensions.
func (t *Terminal) Size() (int, int) { return t.cols, t.rows }

// AltScreen reports whether the alternate screen is active.
func (t *Terminal) AltScreen() bool { return t.altOn }

// Resize changes the screen size (without reflow).
func (t *Terminal) Resize(cols, rows int) {
	cols, rows = max(cols, 1), max(rows, 1)
	if cols == t.cols && rows == t.rows {
		return
	}
	fix := func(s [][]rune, keepBottom bool) [][]rune {
		// Drop lines from the top if the cursor would fall off.
		for len(s) > rows {
			if keepBottom && t.cy >= rows {
				if &s[0] == &t.main[0] {
					t.pushScrollback(s[0])
				}
				s = s[1:]
				t.cy--
			} else {
				s = s[:len(s)-1]
			}
		}
		for len(s) < rows {
			s = append(s, make([]rune, cols))
		}
		for i, l := range s {
			if len(l) > cols {
				s[i] = l[:cols]
			} else if len(l) < cols {
				s[i] = append(l, make([]rune, cols-len(l))...)
			}
		}
		return s
	}
	t.main = fix(t.main, !t.altOn)
	t.alt = fix(t.alt, t.altOn)
	t.cols, t.rows = cols, rows
	t.top, t.bot = 0, rows-1
	t.cx, t.cy = min(t.cx, cols-1), min(max(t.cy, 0), rows-1)
	t.wrapNext = false
}

// Write feeds output bytes through the emulator.
func (t *Terminal) Write(p []byte) (int, error) {
	for _, b := range p {
		t.feed(b)
	}
	return len(p), nil
}

func (t *Terminal) feed(b byte) {
	switch t.state {
	case stGround:
		if t.uneed > 0 {
			if b&0xC0 == 0x80 {
				t.ubuf = append(t.ubuf, b)
				if len(t.ubuf) == t.uneed {
					r, _ := utf8.DecodeRune(t.ubuf)
					t.uneed, t.ubuf = 0, t.ubuf[:0]
					t.print(r)
				}
				return
			}
			t.uneed, t.ubuf = 0, t.ubuf[:0]
			t.print(utf8.RuneError)
		}
		switch {
		case b == 0x1b:
			t.state = stEsc
		case b < 0x20 || b == 0x7f:
			t.control(b)
		case b < 0x80:
			t.print(rune(b))
		case b&0xE0 == 0xC0:
			t.ubuf, t.uneed = append(t.ubuf[:0], b), 2
		case b&0xF0 == 0xE0:
			t.ubuf, t.uneed = append(t.ubuf[:0], b), 3
		case b&0xF8 == 0xF0:
			t.ubuf, t.uneed = append(t.ubuf[:0], b), 4
		default:
			t.print(utf8.RuneError)
		}
	case stEsc:
		t.state = stGround
		switch b {
		case '[':
			t.state, t.params = stCSI, t.params[:0]
		case ']':
			t.state, t.osc = stOSC, t.osc[:0]
		case 'P', '_', '^', 'X':
			t.state = stString
		case '(', ')', '*', '+', '-', '.', '/', '#', '%', ' ':
			t.state = stEscSkip
		case '7':
			t.saved = [2]int{t.cx, t.cy}
		case '8':
			t.cx, t.cy = t.saved[0], t.saved[1]
			t.wrapNext = false
		case 'D':
			t.index()
		case 'E':
			t.cx = 0
			t.index()
		case 'M':
			t.reverseIndex()
		case 'c':
			t.reset()
		}
	case stEscSkip:
		t.state = stGround
	case stCSI:
		if b >= 0x40 && b <= 0x7e {
			t.state = stGround
			t.csi(b)
		} else if b == 0x1b {
			t.state = stEsc
		} else if len(t.params) < 256 {
			t.params = append(t.params, b)
		}
	case stOSC:
		switch b {
		case 0x07:
			t.state = stGround
			t.dispatchOSC()
		case 0x1b:
			t.state = stOSCEsc
		default:
			if len(t.osc) < 1<<20 {
				t.osc = append(t.osc, b)
			}
		}
	case stOSCEsc:
		t.state = stGround
		t.dispatchOSC()
		if b != '\\' {
			t.feed(0x1b)
			t.feed(b)
		}
	case stString:
		if b == 0x1b {
			t.state = stStringEsc
		} else if b == 0x07 {
			t.state = stGround
		}
	case stStringEsc:
		if b == '\\' {
			t.state = stGround
		} else {
			t.state = stString
		}
	}
}

func (t *Terminal) dispatchOSC() {
	if t.OnOSC == nil {
		return
	}
	s := string(t.osc)
	num, rest, _ := strings.Cut(s, ";")
	n, err := strconv.Atoi(num)
	if err != nil {
		return
	}
	t.OnOSC(n, rest)
}

func (t *Terminal) control(b byte) {
	switch b {
	case 0x07:
		if t.OnBell != nil {
			t.OnBell()
		}
	case 0x08:
		if t.cx > 0 {
			t.cx--
		}
		t.wrapNext = false
	case 0x09:
		t.cx = min((t.cx/8+1)*8, t.cols-1)
	case 0x0a, 0x0b, 0x0c:
		t.index()
	case 0x0d:
		t.cx = 0
		t.wrapNext = false
	}
}

func (t *Terminal) print(r rune) {
	w := RuneWidth(r)
	if w == 0 {
		return
	}
	t.lastRune = r
	if t.wrapNext {
		t.cx = 0
		t.index()
		t.wrapNext = false
	}
	if w == 2 && t.cx == t.cols-1 {
		t.screen()[t.cy][t.cx] = 0
		t.cx = 0
		t.index()
	}
	line := t.screen()[t.cy]
	line[t.cx] = r
	if w == 2 && t.cx+1 < t.cols {
		line[t.cx+1] = wideTail
	}
	t.cx += w
	if t.cx >= t.cols {
		t.cx = t.cols - 1
		t.wrapNext = true
	}
}

func (t *Terminal) pushScrollback(l []rune) {
	if t.maxScrollback <= 0 {
		return
	}
	t.scrollback = append(t.scrollback, l)
	if over := len(t.scrollback) - t.maxScrollback; over > t.maxScrollback/4 {
		t.scrollback = append([][]rune(nil), t.scrollback[over:]...)
	}
}

func (t *Terminal) scrollUp(n int) {
	s := t.screen()
	for ; n > 0; n-- {
		if !t.altOn && t.top == 0 {
			t.pushScrollback(s[t.top])
		}
		copy(s[t.top:t.bot], s[t.top+1:t.bot+1])
		s[t.bot] = make([]rune, t.cols)
	}
}

func (t *Terminal) scrollDown(n int) {
	s := t.screen()
	for ; n > 0; n-- {
		copy(s[t.top+1:t.bot+1], s[t.top:t.bot])
		s[t.top] = make([]rune, t.cols)
	}
}

func (t *Terminal) index() {
	if t.cy == t.bot {
		t.scrollUp(1)
	} else if t.cy < t.rows-1 {
		t.cy++
	}
}

func (t *Terminal) reverseIndex() {
	if t.cy == t.top {
		t.scrollDown(1)
	} else if t.cy > 0 {
		t.cy--
	}
}

func (t *Terminal) reset() {
	t.main, t.alt = blank(t.cols, t.rows), blank(t.cols, t.rows)
	t.altOn, t.cx, t.cy, t.top, t.bot = false, 0, 0, 0, t.rows-1
	t.modes = map[int]bool{25: true}
}

func (t *Terminal) parseParams() (private byte, ps []int) {
	p := t.params
	if len(p) > 0 && (p[0] == '?' || p[0] == '>' || p[0] == '<' || p[0] == '=') {
		private, p = p[0], p[1:]
	}
	for _, f := range strings.FieldsFunc(string(p), func(r rune) bool { return r == ';' }) {
		f, _, _ = strings.Cut(f, ":")
		n, _ := strconv.Atoi(f)
		ps = append(ps, n)
	}
	return
}

func param(ps []int, i, def int) int {
	if i < len(ps) && ps[i] > 0 {
		return ps[i]
	}
	return def
}

func (t *Terminal) clear(y, x0, x1 int) {
	l := t.screen()[y]
	for x := max(x0, 0); x < min(x1, t.cols); x++ {
		l[x] = 0
	}
}

func (t *Terminal) csi(final byte) {
	priv, ps := t.parseParams()
	if priv == '>' || priv == '<' || priv == '=' {
		return
	}
	s := t.screen()
	t.wrapNext = false
	switch final {
	case 'A':
		t.cy = max(t.cy-param(ps, 0, 1), 0)
	case 'B', 'e':
		t.cy = min(t.cy+param(ps, 0, 1), t.rows-1)
	case 'C', 'a':
		t.cx = min(t.cx+param(ps, 0, 1), t.cols-1)
	case 'D':
		t.cx = max(t.cx-param(ps, 0, 1), 0)
	case 'E':
		t.cx, t.cy = 0, min(t.cy+param(ps, 0, 1), t.rows-1)
	case 'F':
		t.cx, t.cy = 0, max(t.cy-param(ps, 0, 1), 0)
	case 'G', '`':
		t.cx = min(param(ps, 0, 1)-1, t.cols-1)
	case 'd':
		t.cy = min(param(ps, 0, 1)-1, t.rows-1)
	case 'H', 'f':
		t.cy = min(param(ps, 0, 1)-1, t.rows-1)
		t.cx = min(param(ps, 1, 1)-1, t.cols-1)
	case 'J':
		switch param(ps, 0, 0) {
		case 0:
			t.clear(t.cy, t.cx, t.cols)
			for y := t.cy + 1; y < t.rows; y++ {
				t.clear(y, 0, t.cols)
			}
		case 1:
			t.clear(t.cy, 0, t.cx+1)
			for y := 0; y < t.cy; y++ {
				t.clear(y, 0, t.cols)
			}
		case 2, 3:
			if !t.altOn && final == 'J' && param(ps, 0, 0) == 2 {
				// Keep what was on screen, like real terminals do on clear.
				for y := 0; y < t.rows; y++ {
					if strings.TrimSpace(lineString(s[y])) != "" {
						t.pushScrollback(append([]rune(nil), s[y]...))
					}
				}
			}
			for y := 0; y < t.rows; y++ {
				t.clear(y, 0, t.cols)
			}
		}
	case 'K':
		switch param(ps, 0, 0) {
		case 0:
			t.clear(t.cy, t.cx, t.cols)
		case 1:
			t.clear(t.cy, 0, t.cx+1)
		case 2:
			t.clear(t.cy, 0, t.cols)
		}
	case 'X':
		t.clear(t.cy, t.cx, t.cx+param(ps, 0, 1))
	case 'P':
		l := s[t.cy]
		n := min(param(ps, 0, 1), t.cols-t.cx)
		copy(l[t.cx:], l[t.cx+n:])
		t.clear(t.cy, t.cols-n, t.cols)
	case '@':
		l := s[t.cy]
		n := min(param(ps, 0, 1), t.cols-t.cx)
		copy(l[t.cx+n:], l[t.cx:])
		t.clear(t.cy, t.cx, t.cx+n)
	case 'L', 'M':
		if t.cy < t.top || t.cy > t.bot {
			return
		}
		top := t.top
		t.top = t.cy
		if final == 'L' {
			t.scrollDown(param(ps, 0, 1))
		} else {
			alt := t.altOn
			t.altOn = true // never push deleted lines to scrollback
			t.scrollUp(param(ps, 0, 1))
			t.altOn = alt
		}
		t.top = top
	case 'S':
		t.scrollUp(param(ps, 0, 1))
	case 'T':
		t.scrollDown(param(ps, 0, 1))
	case 'b':
		for i := param(ps, 0, 1); i > 0 && t.lastRune != 0; i-- {
			t.print(t.lastRune)
		}
	case 'r':
		top, bot := param(ps, 0, 1)-1, param(ps, 1, t.rows)-1
		if top < bot && bot < t.rows {
			t.top, t.bot = top, bot
			t.cx, t.cy = 0, 0
		}
	case 's':
		t.saved = [2]int{t.cx, t.cy}
	case 'u':
		t.cx, t.cy = t.saved[0], t.saved[1]
	case 'h', 'l':
		if priv == '?' {
			for _, m := range ps {
				t.setMode(m, final == 'h')
			}
		}
	}
	t.cx, t.cy = min(max(t.cx, 0), t.cols-1), min(max(t.cy, 0), t.rows-1)
}

func (t *Terminal) setMode(m int, on bool) {
	switch m {
	case 47, 1047, 1049:
		if on == t.altOn {
			break
		}
		if on {
			if m == 1049 {
				t.saved = [2]int{t.cx, t.cy}
			}
			t.alt = blank(t.cols, t.rows)
		} else if m == 1049 {
			t.cx, t.cy = t.saved[0], t.saved[1]
		}
		t.altOn = on
		t.top, t.bot = 0, t.rows-1
	}
	t.modes[m] = on
}

// restorable private modes, in replay order.
var restorable = []int{1049, 1, 7, 25, 1000, 1002, 1003, 1004, 1005, 1006, 1015, 2004}

// ModePrefix returns escape sequences that recreate the current
// terminal modes. It is sent to clients whose replay starts mid-stream.
func (t *Terminal) ModePrefix() string {
	var b strings.Builder
	for _, m := range restorable {
		on, ok := t.modes[m]
		if !ok {
			continue
		}
		if m == 1049 && !t.altOn {
			continue
		}
		c := byte('l')
		if on {
			c = 'h'
		}
		b.WriteString("\x1b[?" + strconv.Itoa(m) + string(c))
	}
	return b.String()
}

func lineString(l []rune) string {
	var b strings.Builder
	for _, r := range l {
		switch r {
		case wideTail:
		case 0:
			b.WriteByte(' ')
		default:
			b.WriteRune(r)
		}
	}
	return strings.TrimRight(b.String(), " ")
}

// ScreenLines returns the visible screen as text lines.
func (t *Terminal) ScreenLines() []string {
	s := t.screen()
	out := make([]string, len(s))
	for i, l := range s {
		out[i] = lineString(l)
	}
	return out
}

// Text returns scrollback plus the visible screen, trailing blank lines trimmed.
func (t *Terminal) Text() string {
	lines := make([]string, 0, len(t.scrollback)+t.rows)
	for _, l := range t.scrollback {
		lines = append(lines, lineString(l))
	}
	lines = append(lines, t.ScreenLines()...)
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// RuneWidth returns the number of cells r occupies (0, 1 or 2).
func RuneWidth(r rune) int {
	switch {
	case r < 0x20:
		return 0
	case r < 0x300:
		return 1
	case r <= 0x36F, r >= 0x200B && r <= 0x200F, r >= 0x20D0 && r <= 0x20FF,
		r >= 0xFE00 && r <= 0xFE0F, r == 0x2060, r >= 0x1F3FB && r <= 0x1F3FF:
		return 0
	case r >= 0x1100 && r <= 0x115F, r >= 0x2E80 && r <= 0x303E, r >= 0x3041 && r <= 0x33FF,
		r >= 0x3400 && r <= 0x4DBF, r >= 0x4E00 && r <= 0x9FFF, r >= 0xA000 && r <= 0xA4CF,
		r >= 0xAC00 && r <= 0xD7A3, r >= 0xF900 && r <= 0xFAFF, r >= 0xFE30 && r <= 0xFE4F,
		r >= 0xFF00 && r <= 0xFF60, r >= 0xFFE0 && r <= 0xFFE6, r >= 0x1F300 && r <= 0x1F64F,
		r >= 0x1F900 && r <= 0x1F9FF, r >= 0x1FA70 && r <= 0x1FAFF, r >= 0x20000 && r <= 0x3FFFD:
		return 2
	}
	return 1
}
