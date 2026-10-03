package proto

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"sync"
)

// Conn is a message-oriented, bidirectional connection.
type Conn interface {
	// Read returns the next message; binary reports whether it is a data frame.
	Read() (binary bool, data []byte, err error)
	Write(binary bool, data []byte) error
	Close() error
}

// WriteJSON marshals m and writes it as a text message.
func WriteJSON(c Conn, m *Msg) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return c.Write(false, b)
}

// StreamConn frames messages over a byte stream (unix socket, SSH stdio):
// u32 BE length | u8 type (0 text, 1 binary) | payload.
type StreamConn struct {
	r  *bufio.Reader
	w  io.Writer
	c  io.Closer
	mu sync.Mutex
}

// NewStreamConn wraps a reader/writer pair.
func NewStreamConn(r io.Reader, w io.Writer, c io.Closer) *StreamConn {
	return &StreamConn{r: bufio.NewReaderSize(r, 64<<10), w: w, c: c}
}

const maxFrame = 16 << 20

func (s *StreamConn) Read() (bool, []byte, error) {
	var hdr [5]byte
	if _, err := io.ReadFull(s.r, hdr[:]); err != nil {
		return false, nil, err
	}
	n := binary.BigEndian.Uint32(hdr[:4])
	if n > maxFrame {
		return false, nil, errors.New("frame too large")
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(s.r, b); err != nil {
		return false, nil, err
	}
	return hdr[4] == 1, b, nil
}

func (s *StreamConn) Write(bin bool, data []byte) error {
	buf := make([]byte, 5+len(data))
	binary.BigEndian.PutUint32(buf, uint32(len(data)))
	if bin {
		buf[4] = 1
	}
	copy(buf[5:], data)
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.w.Write(buf)
	return err
}

func (s *StreamConn) Close() error {
	if s.c != nil {
		return s.c.Close()
	}
	return nil
}
