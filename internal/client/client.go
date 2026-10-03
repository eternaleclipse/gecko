// Package client is a small synchronous client for the daemon socket, used
// by the CLI.
package client

import (
	"encoding/json"
	"errors"

	"github.com/gecko-term/gecko/internal/daemon"
	"github.com/gecko-term/gecko/internal/proto"
)

// Client talks to the local daemon.
type Client struct {
	Conn    proto.Conn
	Host    string
	Version int    // protocol version of the daemon
	Start   string // changes when the daemon restarts in place
	rid     int
}

// Connect dials the daemon (starting it if start is set) and says hello.
func Connect(start bool) (*Client, error) {
	conn, err := daemon.Dial(start)
	if err != nil {
		return nil, err
	}
	c := &Client{Conn: conn}
	r, err := c.Call(&proto.Msg{T: proto.THello, Client: "cli", Version: proto.Version})
	if err != nil {
		conn.Close()
		return nil, err
	}
	c.Host, c.Version, c.Start = r.Host, r.Version, r.Text
	return c, nil
}

// Call sends a request and waits for its reply, discarding pushes.
func (c *Client) Call(m *proto.Msg) (*proto.Msg, error) {
	c.rid++
	m.Rid = c.rid
	if err := proto.WriteJSON(c.Conn, m); err != nil {
		return nil, err
	}
	for {
		bin, b, err := c.Conn.Read()
		if err != nil {
			return nil, err
		}
		if bin {
			continue
		}
		var r proto.Msg
		if err := json.Unmarshal(b, &r); err != nil {
			continue
		}
		if r.Rid != m.Rid || (r.T != proto.TReply && r.T != m.T) {
			continue
		}
		if r.Error != "" {
			return nil, errors.New(r.Error)
		}
		return &r, nil
	}
}

// Sessions lists all sessions.
func (c *Client) Sessions() ([]proto.SessionInfo, error) {
	r, err := c.Call(&proto.Msg{T: proto.TList})
	if err != nil {
		return nil, err
	}
	return r.Sessions, nil
}

// Close closes the connection.
func (c *Client) Close() error { return c.Conn.Close() }
