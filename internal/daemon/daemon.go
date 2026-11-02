// Package daemon runs the long-lived Gecko process and lets clients find
// (or start) it.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/gecko-term/gecko/internal/cloud"
	"github.com/gecko-term/gecko/internal/config"
	"github.com/gecko-term/gecko/internal/execx"
	"github.com/gecko-term/gecko/internal/hub"
	"github.com/gecko-term/gecko/internal/inspect"
	"github.com/gecko-term/gecko/internal/proto"
	"github.com/gecko-term/gecko/internal/server"
	"github.com/gecko-term/gecko/internal/session"
)

// Run runs the daemon in the foreground until SIGINT/SIGTERM.
func Run(cfg *config.Config, listen string) error {
	config.Migrate()
	if listen != "" {
		cfg.Listen = listen
	}
	if err := os.MkdirAll(config.StateDir(), 0o700); err != nil {
		return err
	}
	sock := config.Socket()
	if err := os.MkdirAll(filepath.Dir(sock), 0o700); err != nil {
		return err
	}
	// Windows has no POSIX mode bits (Go reports 0777 for every directory);
	// the profile directory's ACL is what keeps other users out there.
	if runtime.GOOS != "windows" {
		if st, err := os.Stat(filepath.Dir(sock)); err != nil || st.Mode().Perm()&0o077 != 0 {
			return errors.New("refusing to use socket dir with loose permissions: " + filepath.Dir(sock))
		}
	}
	// Started by `gecko upgrade`? Then the previous process handed us its
	// sockets and sessions.
	hand, err := readHandoff()
	if err != nil {
		log.Printf("upgrade handoff: %v", err)
	}
	var ln net.Listener
	if hand != nil && hand.UnixFd != 0 {
		ln, err = net.FileListener(os.NewFile(hand.UnixFd, "gecko.sock"))
		if err != nil {
			return err
		}
	} else {
		if c, err := net.DialTimeout("unix", sock, time.Second); err == nil {
			c.Close()
			return errors.New("gecko daemon already running (" + sock + ")")
		}
		_ = os.Remove(sock)
		if ln, err = net.Listen("unix", sock); err != nil {
			return err
		}
		_ = os.Chmod(sock, 0o600)
	}
	defer os.Remove(sock)
	// After an upgrade the inherited socket may be at the old location
	// (before ~/.gecko-terminal); listen at the current one too.
	var extra net.Listener
	if hand != nil {
		if _, err := os.Stat(sock); err != nil {
			_ = os.MkdirAll(filepath.Dir(sock), 0o700)
			if l, err := net.Listen("unix", sock); err == nil {
				_ = os.Chmod(sock, 0o600)
				extra = l
			}
		}
	}

	token, err := config.Token()
	if err != nil {
		return err
	}
	for exe, name := range cfg.Agents {
		inspect.AddAgent(exe, name)
	}
	m := session.NewManager(session.Options{
		Host: cfg.Name, Shell: cfg.Shell, ScrollbackBytes: cfg.ScrollbackBytes,
		ScriptsDir: filepath.Join(config.StateDir(), "shell"),
	})
	if hand != nil {
		for _, err := range m.Adopt(hand.Sessions) {
			log.Printf("upgrade: lost a session: %v", err)
		}
		reap(hand.Reap)
		log.Printf("upgraded in place; kept %d sessions", len(hand.Sessions))
	}
	h := hub.New(cfg, m)
	defer h.Close()
	// Account features: nothing happens here unless this machine is signed
	// in (gecko login). Machines of the account connect through the relay
	// and get the same protocol a bridge gets, limited to this machine (a peer
	// can't reach the machines this one federates, see server.ServePeer).
	acct := cloud.NewAgent(h, cfg.CloudEnabled(), func(c proto.Conn) { server.ServePeer(h, c) })
	h.SetCloud(acct)
	acct.Start()

	accept := func(l net.Listener) {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go server.ServeLocal(h, proto.NewStreamConn(c, c, c))
		}
	}
	if extra != nil {
		defer extra.Close()
	}

	web := &server.HTTP{Hub: h, Token: token, Listen: cfg.Listen}
	srv := &http.Server{Addr: cfg.Listen, Handler: web.Handler(), ReadHeaderTimeout: 10 * time.Second}
	webErr := make(chan error, 1)
	var hl net.Listener
	startWT := func() {}
	if cfg.Listen != "off" {
		if hand != nil && hand.TCPFd != 0 {
			hl, err = net.FileListener(os.NewFile(hand.TCPFd, "web"))
		} else {
			hl, err = net.Listen("tcp", cfg.Listen)
		}
		if err != nil {
			log.Printf("web UI disabled: %v", err)
		} else {
			startWT = func() {
				// WebTransport (QUIC) on the same address and port number,
				// UDP. GECKO_WEBTRANSPORT=off leaves browsers on the WebSocket.
				if os.Getenv("GECKO_WEBTRANSPORT") == "off" {
					log.Printf("webtransport off (GECKO_WEBTRANSPORT)")
				} else if pc, err := net.ListenPacket("udp", hl.Addr().String()); err != nil {
					log.Printf("webtransport disabled: %v", err)
				} else if err := web.StartWT(pc); err != nil {
					pc.Close()
					log.Printf("webtransport disabled: %v", err)
				}
			}
			startWT()
			defer web.CloseWT()
			_ = os.WriteFile(filepath.Join(config.StateDir(), "web"), []byte(hl.Addr().String()), 0o600)
			log.Printf("gecko %s on %s: web UI at http://%s/", session.Version, cfg.Name, hl.Addr())
			go func() { webErr <- srv.Serve(hl) }()
		}
	}
	_ = os.WriteFile(filepath.Join(config.StateDir(), "pid"), []byte(strconv.Itoa(os.Getpid())), 0o600)

	// Answer on the socket only now: a client that started this daemon
	// (gecko url, gecko open) reads the web address from state/web as soon
	// as it connects, and used to get the default port when it got in first.
	// Until then connections wait in the socket's backlog.
	go accept(ln)
	if extra != nil {
		go accept(extra)
	}

	// `gecko upgrade`: replace this process with the (new) binary on disk,
	// keeping every session and both listening sockets.
	share := &sharer{hub: h}
	defer share.stop()
	h.Share = share.set

	h.Upgrade = func(want string) (func() error, error) {
		exe, err := upgradeTarget(want)
		if err != nil {
			return nil, err
		}
		return func() error {
			// Close QUIC sessions properly first: unlike TCP, nothing tells a
			// browser that a UDP socket went away with the old process.
			web.CloseWT()
			var err error
			if extra != nil {
				err = upgrade(exe, m, h, extra, hl) // the one at the current path
			} else {
				err = upgrade(exe, m, h, ln, hl)
			}
			startWT() // still here, so the upgrade failed
			return err
		}, nil
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	ignoreHangup()
	select {
	case s := <-sig:
		log.Printf("received %v, shutting down", s)
	case err := <-webErr:
		log.Printf("web server: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	ln.Close()
	m.Shutdown(4 * time.Second) // don't leave orphaned shells and agents behind
	return nil
}

// WebAddr returns the address the running daemon's web UI listens on.
func WebAddr() string {
	b, err := os.ReadFile(filepath.Join(config.StateDir(), "web"))
	if err != nil {
		return "127.0.0.1:7681"
	}
	return string(b)
}

// Dial connects to the local daemon, starting it if needed.
func Dial(start bool) (proto.Conn, error) {
	config.Migrate()
	c, err := net.DialTimeout("unix", config.Socket(), 2*time.Second)
	if err != nil && config.LegacySocket() != "" {
		// A daemon started before the move to ~/.gecko-terminal.
		if lc, lerr := net.DialTimeout("unix", config.LegacySocket(), 2*time.Second); lerr == nil {
			c, err = lc, nil
		}
	}
	if err == nil {
		return proto.NewStreamConn(c, c, c), nil
	}
	if !start {
		return nil, errors.New("gecko daemon is not running (start it with `gecko daemon` or just `gecko`)")
	}
	if err := Spawn(); err != nil {
		return nil, fmt.Errorf("starting daemon: %w", err)
	}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		if c, err := net.DialTimeout("unix", config.Socket(), time.Second); err == nil {
			return proto.NewStreamConn(c, c, c), nil
		}
	}
	return nil, errors.New("daemon did not start; see " + LogPath())
}

// LogPath is where a spawned daemon logs.
func LogPath() string { return filepath.Join(config.StateDir(), "daemon.log") }

// Spawn starts the daemon in the background, detached from this terminal
// and session so it outlives SSH / mosh disconnects.
func Spawn() error {
	exe, err := config.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(config.StateDir(), 0o700); err != nil {
		return err
	}
	if spawnService(exe) {
		return nil
	}
	logf, err := os.OpenFile(LogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer logf.Close()
	cmd := exec.Command(exe, "daemon")
	cmd.Stdout, cmd.Stderr = logf, logf
	// Somewhere stable, and USERPROFILE on Windows where HOME is unset.
	if home, err := os.UserHomeDir(); err == nil {
		cmd.Dir = home
	}
	detach(cmd)
	execx.ThroughLinker(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
