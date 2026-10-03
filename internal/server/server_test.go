package server

import (
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/gecko-term/gecko/internal/config"
	"github.com/gecko-term/gecko/internal/hub"
	"github.com/gecko-term/gecko/internal/proto"
	"github.com/gecko-term/gecko/internal/session"
)

// TestEndToEnd drives a real PTY session through hub and protocol:
// create, attach, type, receive output, resume from an offset.
func TestEndToEnd(t *testing.T) {
	m := session.NewManager(session.Options{Host: "test", Shell: "/bin/sh", ScriptsDir: t.TempDir()})
	h := hub.New(&config.Config{Name: "test"}, m)
	a, b := net.Pipe()
	go Serve(h, proto.NewStreamConn(a, a, a))
	c := proto.NewStreamConn(b, b, b)
	defer c.Close()

	msgs := make(chan proto.Msg, 64)
	data := make(chan []byte, 64)
	go func() {
		for {
			bin, raw, err := c.Read()
			if err != nil {
				close(msgs)
				return
			}
			if bin {
				_, _, _, d, _ := proto.DecodeData(raw)
				data <- append([]byte(nil), d...)
				continue
			}
			var m proto.Msg
			_ = json.Unmarshal(raw, &m)
			msgs <- m
		}
	}()
	waitMsg := func(pred func(proto.Msg) bool) proto.Msg {
		t.Helper()
		timeout := time.After(5 * time.Second)
		for {
			select {
			case m := <-msgs:
				if pred(m) {
					return m
				}
			case <-timeout:
				t.Fatal("timed out waiting for message")
			}
		}
	}
	waitData := func(want string) {
		t.Helper()
		var got strings.Builder
		timeout := time.After(5 * time.Second)
		for !strings.Contains(got.String(), want) {
			select {
			case d := <-data:
				got.Write(d)
			case <-timeout:
				t.Fatalf("never saw %q in %q", want, got.String())
			}
		}
	}

	_ = proto.WriteJSON(c, &proto.Msg{T: proto.THello, Client: "test"})
	waitMsg(func(m proto.Msg) bool { return m.T == proto.THello })

	_ = proto.WriteJSON(c, &proto.Msg{T: proto.TCreate, Rid: 1, Create: &proto.CreateReq{Argv: []string{"/bin/sh"}, Workspace: "w"}})
	r := waitMsg(func(m proto.Msg) bool { return m.T == proto.TReply && m.Rid == 1 })
	if r.Error != "" || r.Session == nil || !strings.HasPrefix(r.Session.ID, "test/") {
		t.Fatalf("create: %+v", r)
	}
	id := r.Session.ID

	_ = proto.WriteJSON(c, &proto.Msg{T: proto.TAttach, ID: id, Offset: -1})
	_ = c.Write(true, proto.EncodeData(proto.KindInput, id, 0, []byte("echo gecko-$((40+2))\n")))
	waitData("gecko-42")

	_ = proto.WriteJSON(c, &proto.Msg{T: proto.TCapture, Rid: 2, ID: id})
	r = waitMsg(func(m proto.Msg) bool { return m.T == proto.TReply && m.Rid == 2 })
	if !strings.Contains(r.Text, "gecko-42") {
		t.Fatalf("capture: %q", r.Text)
	}

	_ = proto.WriteJSON(c, &proto.Msg{T: proto.TSearch, Rid: 4, Text: "GECKO-4"})
	r = waitMsg(func(m proto.Msg) bool { return m.T == proto.TReply && m.Rid == 4 })
	if len(r.Matches) == 0 || r.Matches[0].ID != id || !strings.Contains(r.Matches[0].Line, "gecko-42") {
		t.Fatalf("search: %+v", r.Matches)
	}

	_ = proto.WriteJSON(c, &proto.Msg{T: proto.TKill, Rid: 3, ID: id})
	waitMsg(func(m proto.Msg) bool { return m.T == proto.TClosed && m.ID == id })
}

// A hub (another Gecko) connecting to this one gets the current state right
// after the hello, without waiting for something to change.
func TestHubGetsStateOnConnect(t *testing.T) {
	m := session.NewManager(session.Options{Host: "remote", Shell: "/bin/sh", ScriptsDir: t.TempDir()})
	h := hub.New(&config.Config{Name: "remote"}, m)
	a, b := net.Pipe()
	go Serve(h, proto.NewStreamConn(a, a, a))
	c := proto.NewStreamConn(b, b, b)
	defer c.Close()
	_ = proto.WriteJSON(c, &proto.Msg{T: proto.THello, Client: "hub", Scope: hub.ScopeLocal})
	got := map[string]bool{}
	deadline := time.Now().Add(3 * time.Second)
	for len(got) < 2 && time.Now().Before(deadline) {
		_, raw, err := c.Read()
		if err != nil {
			t.Fatal(err)
		}
		var msg proto.Msg
		_ = json.Unmarshal(raw, &msg)
		got[msg.T] = true
	}
	if !got[proto.THello] || !got[proto.TState] {
		t.Fatalf("got %v, want hello and state", got)
	}
}
