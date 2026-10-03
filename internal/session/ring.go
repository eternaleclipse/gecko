package session

// Ring keeps the most recent output of a session so clients can attach,
// reattach after a network drop, or switch devices without losing anything.
type Ring struct {
	buf []byte
	end int64 // total bytes ever written
}

// NewRing returns a ring holding size bytes.
func NewRing(size int) *Ring { return &Ring{buf: make([]byte, size)} }

// End is the stream offset just past the last byte written.
func (r *Ring) End() int64 { return r.end }

// Start is the oldest offset still available.
func (r *Ring) Start() int64 { return max(0, r.end-int64(len(r.buf))) }

func (r *Ring) Write(p []byte) {
	n := len(r.buf)
	if len(p) >= n {
		r.end += int64(len(p) - n)
		p = p[len(p)-n:]
	}
	pos := int(r.end % int64(n))
	c := copy(r.buf[pos:], p)
	copy(r.buf, p[c:])
	r.end += int64(len(p))
}

// ReadFrom returns a copy of everything from off (clamped to Start) to End,
// and the offset the returned data begins at.
func (r *Ring) ReadFrom(off int64) ([]byte, int64) {
	off = max(off, r.Start())
	if off >= r.end {
		return nil, r.end
	}
	n := int64(len(r.buf))
	out := make([]byte, r.end-off)
	pos := int(off % n)
	c := copy(out, r.buf[pos:])
	copy(out[c:], r.buf)
	return out, off
}

// Restore refills the ring with data that started at offset start, as
// saved by a previous process, so stream offsets stay the same.
func (r *Ring) Restore(start int64, data []byte) {
	r.end = start
	r.Write(data)
}
