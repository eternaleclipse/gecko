# Architecture

[Docs](README.md)

```
 browser / PWA (xterm.js, WebGL)        gecko attach (any terminal)
        │  WebSocket                             │ unix socket
        └───────────────┬────────────────────────┘
                 gecko daemon  (this machine)
        ┌───────────────┼──────────────────────────────┐
   local PTYs     hub ── ssh host gecko bridge ──► gecko daemon (devbox)
   + headless VT        └ ssh … gecko bridge ────► gecko daemon (gpu-box)
   + ring buffer
   + process / tmux / agent inspection (of its own sessions)
```

- **One protocol everywhere.** JSON control messages and binary data frames
  (`kind | id | offset | bytes`) run over WebSockets, the local socket, and SSH
  stdio. A remote daemon looks exactly like the local one.
- **Every session has a 4 MiB ring buffer** with absolute byte offsets.
  Attaching replays it, and reattaching sends only the missing bytes. If a
  client fell too far behind, it gets a reset plus the terminal modes it needs
  (alt screen, mouse, bracketed paste).
- **A headless VT emulator** (`internal/vt`) tracks what is on screen. That
  drives agent status, the text view, and mode restoration. It also handles
  OSC 0/2/7/9/52/133/633/777/1337 and bells.
- **Slow clients never stall a session.** Each one gets a bounded queue. A
  client that can't keep up is disconnected and resumes from its offset.

Code map: `cmd/gecko` (CLI), `internal/session` (PTYs, inspection, agent
status), `internal/hub` (federation, remote reconnect), `internal/server`
(protocol, HTTP/WebSocket), `internal/vt`, `internal/inspect`,
`internal/shellint`, `internal/tmux`, `web/` (client).

