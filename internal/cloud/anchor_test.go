package cloud_test

import (
	"encoding/json"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gecko-term/gecko/internal/config"
	"github.com/gecko-term/gecko/internal/hub"
	"github.com/gecko-term/gecko/internal/proto"
	"github.com/gecko-term/gecko/internal/server"
	"github.com/gecko-term/gecko/internal/session"
)

// pageOn connects a web client (with the token: an owner) to h, saying
// hello with anchor, and returns its hello reply and a way to call.
func pageOn(t *testing.T, h *hub.Hub, anchor ...string) (proto.Msg, func(proto.Msg) proto.Msg, func(func(proto.Msg) bool) proto.Msg) {
	t.Helper()
	a, b := net.Pipe()
	go server.Serve(h, proto.NewStreamConn(a, a, a))
	c := proto.NewStreamConn(b, b, b)
	t.Cleanup(func() { c.Close() })
	msgs := make(chan proto.Msg, 1000)
	go func() {
		for {
			bin, raw, err := c.Read()
			if err != nil {
				close(msgs)
				return
			}
			var m proto.Msg
			if !bin && json.Unmarshal(raw, &m) == nil {
				msgs <- m
			}
		}
	}()
	next := func(ok func(proto.Msg) bool) proto.Msg {
		t.Helper()
		timeout := time.After(20 * time.Second)
		for {
			select {
			case m, open := <-msgs:
				if !open {
					t.Fatal("connection closed")
				}
				if ok(m) {
					return m
				}
			case <-timeout:
				t.Fatal("timed out")
			}
		}
	}
	rid := 0
	call := func(m proto.Msg) proto.Msg {
		t.Helper()
		rid++
		m.Rid = rid
		_ = proto.WriteJSON(c, &m)
		return next(func(r proto.Msg) bool { return r.T == proto.TReply && r.Rid == m.Rid })
	}
	_ = proto.WriteJSON(c, &proto.Msg{T: proto.THello, Client: "web", Version: proto.Version, Anchor: anchor})
	return next(func(m proto.Msg) bool { return m.T == proto.THello }), call, next
}

// Make anchor on a machine reached only through the account: the relay
// gives the laptop's page devbox as its anchor with owner rights (on by
// default: both are the owner's approved machines), and refuses once
// devbox's owner turned that off (gecko anchor account off).
func TestMakeAnchorThroughRelay(t *testing.T) {
	f := newFleet(t)
	laptop, devbox := f.machine("laptop"), f.machine("devbox")
	f.create(devbox)
	f.join(laptop, devbox)
	box := hub.New(&config.Config{Name: "box"}, session.NewManager(session.Options{Host: "box", Shell: "/bin/sh", ScriptsDir: t.TempDir()}))
	t.Cleanup(box.Close)
	const tok = "box-token-0123456789abcdef"
	srv := httptest.NewServer((&server.HTTP{Hub: box, Token: tok}).Handler())
	t.Cleanup(srv.Close)
	if err := devbox.hub.AddLinkHost("box", srv.URL+"/?token="+tok, true); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "devbox connected over the relay", func() bool { return connected(laptop, "devbox") })
	// And devbox to its own machine: the create below goes there, and a slow
	// runner could get to it first ("host box is offline").
	waitFor(t, "devbox connected to box", func() bool {
		b := devbox.hub.Backend("box")
		return b != nil && b.Info().Status == "connected"
	})

	hello, call, next := pageOn(t, laptop.hub, "devbox")
	if hello.Host != "devbox" || hello.Door != "laptop" || hello.Scope != hub.ScopeAll || hello.Error != "" {
		t.Fatalf("relay anchor while on: %+v", hello)
	}
	st := next(func(m proto.Msg) bool { return m.T == proto.TState })
	var names []string
	for _, h := range st.Hosts {
		names = append(names, h.Name)
	}
	// Both are in the account, so devbox lists the laptop too: Make anchor
	// on it is the way back.
	if strings.Join(names, ",") != "devbox,laptop,box" {
		t.Fatalf("hosts through the new anchor: %v", names)
	}
	r := call(proto.Msg{T: proto.TCreate, Create: &proto.CreateReq{Host: "box", Name: "via-relay", Argv: []string{"/bin/sh"}, Cols: 80, Rows: 24}})
	if r.Error != "" || r.Session.Host != "box" {
		t.Fatalf("create on devbox's machine through the relay: %+v", r)
	}
	// devbox's owner turns it off: the page stays on the laptop and hears
	// what to do.
	if err := devbox.hub.SetAnchorFromAccount(false); err != nil {
		t.Fatal(err)
	}
	hello, _, _ = pageOn(t, laptop.hub, "devbox")
	if hello.Host != "laptop" || hello.Kind != hub.KindAnchor || !strings.Contains(hello.Error, "anchor account on") {
		t.Fatalf("relay anchor while off: %+v", hello)
	}
	if devbox.hub.Backend("box") == nil {
		t.Fatal("box went away")
	}

	// Back on the laptop: its own list, devbox one of its machines again.
	hello, _, next = pageOn(t, laptop.hub)
	if hello.Host != "laptop" || hello.Door != "" {
		t.Fatalf("back: %+v", hello)
	}
	st = next(func(m proto.Msg) bool { return m.T == proto.TState })
	if len(st.Hosts) != 2 || st.Hosts[0].Name != "laptop" || st.Hosts[1].Name != "devbox" {
		t.Fatalf("hosts back on the laptop: %+v", st.Hosts)
	}
}
