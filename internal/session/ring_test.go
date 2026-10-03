package session

import "testing"

func TestRing(t *testing.T) {
	r := NewRing(8)
	r.Write([]byte("hello"))
	if d, off := r.ReadFrom(0); string(d) != "hello" || off != 0 {
		t.Fatalf("got %q %d", d, off)
	}
	r.Write([]byte(" world"))
	if d, off := r.ReadFrom(0); string(d) != "lo world" || off != 3 {
		t.Fatalf("got %q %d", d, off)
	}
	if d, off := r.ReadFrom(9); string(d) != "ld" || off != 9 {
		t.Fatalf("got %q %d", d, off)
	}
	r.Write([]byte("0123456789abc"))
	if d, _ := r.ReadFrom(0); string(d) != "56789abc" {
		t.Fatalf("got %q", d)
	}
	if r.End() != 24 {
		t.Fatalf("end %d", r.End())
	}
}

// A ring restored in a new process keeps the same stream offsets, so
// clients resume exactly where they were.
func TestRingRestore(t *testing.T) {
	a := NewRing(8)
	a.Write([]byte("hello world!"))
	data, start := a.ReadFrom(0)
	b := NewRing(8)
	b.Restore(start, data)
	if b.End() != a.End() || b.Start() != a.Start() {
		t.Fatalf("offsets: %d-%d vs %d-%d", b.Start(), b.End(), a.Start(), a.End())
	}
	if d, off := b.ReadFrom(6); string(d) != "world!" || off != 6 {
		t.Fatalf("got %q at %d", d, off)
	}
}
