# Gecko

A terminal for people who run shells, tmux, SSH and coding agents on several
machines at once, from a desktop or a phone.

- **Sessions outlive everything.** A small daemon owns every PTY. You can close
  the window, lose Wi-Fi, have SSH or mosh drop, or switch from laptop to phone,
  and your shells and agents keep running. Clients resume each stream from the
  last byte they saw.
- **Workspaces and tabs.** Group tabs by project. A workspace can span machines.
  Switch from the keyboard, the sidebar, or a fuzzy palette that searches
  everything.
- **Aware of what is running.** Shell integration (bash, zsh, fish) reports the
  cwd, the command, and exit codes. Tabs are named after the git repo they're
  in, with the branch, however deep in it you are. The process table shows when a tab is in
  ssh/mosh (and to where), tmux (and which session), an editor, or a coding
  agent.
- **Agent-aware.** Gecko recognises Claude Code, Codex, Gemini, Aider,
  OpenCode, Goose, Amp, Cursor Agent, Copilot and others running in its tabs,
  on any machine. The agent dock shows each one as *working*, *needs you* or
  *idle*, with the line it is showing, and one-tap replies (`1`, `2`, `y`,
  `esc`, `⏎`). You get a notification when an agent stops for you.
- **Many machines.** Add a host by its SSH destination. Gecko runs
  `gecko bridge` there over SSH, and that machine's own daemon owns its
  sessions. A dropped connection just reconnects and resumes, much like mosh,
  for every tab at once.
- **tmux, natively.** Gecko talks to each machine's tmux server: sessions and
  windows appear in the sidebar and in the palette, a tmux tab is titled by
  its active window, and the status line has clickable window chips. Click to
  switch, **+** for a new window, double-click to rename. Opening a session that
  is already in a tab goes to that tab instead of opening another one. tmux
  inside a plain ssh tab is recognised from its status bar.
- **Themes.** A green take on Nord by default, plus Mission Control (a
  launch-webcast skin with DIN-style type), Digital Watch (a total overhaul in
  the spirit of a classic resin digital watch: LCD terminal, ticking 7-segment
  face, LIGHT / MODE / ALARM pushers), photo themes that bring their own
  fonts and shapes (Sakura, Cyberpunk, Neo-Tokyo, Dirt Bike, Summit, Wood Shop,
  Ramen, Mt. Fuji, Jellyfish; CC0 photos, credits in
  `web/public/themes/CREDITS.md`) and the classics (Dracula, Nord, Tokyo Night, Catppuccin, One
  Dark, Gruvbox, Solarized, GitHub Light). The desktop app adds a see-through
  background.
- **Settings follow you.** Theme, fonts, key bindings and the rest live in
  `~/.gecko-terminal/settings.json` and sync live to every window and device.
  Every shortcut can be remapped (Ctrl+K, then **Keyboard shortcuts…**).
- **Updates keep your tabs.** `gecko upgrade` (and `make install`) restart the
  service in place: shells, agents and their output carry over, and windows
  reconnect by themselves. Remote machines update the same way from the
  sidebar.
- **Copy and paste everywhere.** Selection works even when tmux or vim grab the
  mouse (Shift-drag, or ⌥-drag on macOS). Other copy features:
  - Copy-on-select, and "Ctrl+C copies when something is selected".
  - OSC 52, so `tmux set-clipboard`, nvim and remote programs copy to your real
    clipboard, even through SSH.
  - A **Select text** view with the full scrollback as plain text. Inside tmux it
    uses tmux's own history. On phones, long-press the terminal to open it.

## Architecture

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

## Build and run

Requires Go 1.22+ and Node 18+.

```sh
make            # builds web/dist, then bin/gecko with the client embedded
make test
make install    # ~/.local/bin/gecko, plus an app-menu launcher on Linux
make dist       # cross-compiles linux/darwin/windows/freebsd × amd64/arm64
```

For client development, run `make dev` (rebuilds on change) and start the
daemon with `GECKO_WEB_DIR=$PWD/web/dist gecko daemon`. It then serves the
client from disk, so a page reload picks up changes without restarting the
daemon (and its sessions).

```sh
gecko                 # start the daemon if needed and open the app window
gecko new -w api -n claude -- claude    # new tab in workspace "api", typing `claude`
gecko ls              # all sessions on all machines
gecko agents          # what every coding agent is doing
gecko attach api      # use a session from any terminal (Ctrl-\ detaches)
gecko send claude "run the tests"       # type into a session (scriptable)
gecko capture claude  # its text, tmux-aware
gecko service install # start at login (systemd user unit / launchd / Windows Startup)
```

`gecko open` opens Gecko in its own window. It starts at exactly 80×24,
centered, and remembers the right size after the first time.

- **Desktop app (`make desktop`):** a small native Electron window with a
  see-through background. **Background opacity…** in the palette sets how
  see-through it is. On macOS you get vibrancy blur, on Windows 11 acrylic,
  and on Linux transparency (needs a compositor). Text, menus and dialogs
  stay solid. On Ubuntu 23.10+ Chromium's sandbox needs an AppArmor profile;
  run `make desktop-sandbox` once (it uses sudo). Until then `gecko open`
  falls back to a browser window. `GECKO_DESKTOP_NO_SANDBOX=1` runs without
  the sandbox instead.
- **Otherwise:** a chromeless browser app window (Chrome, Edge, Brave or
  Chromium). You can also install the page as an app from the browser menu.

### More machines

```sh
gecko host install devbox      # copies the right binary to devbox:~/.local/bin
gecko host add devbox me@devbox
```

You can also add a machine from the sidebar (**Machines → +**). The dialog
checks the SSH connection and shows its output live. If something is wrong,
it explains the error, and you can edit the destination and retry. If Gecko
isn't installed on the machine yet, it offers to copy itself over. A
destination can include a port (`me@host:2222`). Hover a machine and click ×
to remove it; its sessions keep running there.

Gecko uses your normal SSH config, keys and agent. It runs ssh with
`BatchMode` (no password prompts) and `StrictHostKeyChecking=accept-new`, so
it trusts a new host on first use but refuses a changed host key. The
remote daemon starts on demand, detached from the SSH session, and keeps
running. On Linux with lingering enabled it runs as a transient systemd user
unit, so logind won't clean it up at logout.

Any command that runs `gecko bridge` on the other side works as a transport,
for example a container:

```json
{ "name": "box", "command": ["docker", "exec", "-i", "box", "gecko", "bridge"] }
```

### Phone and tablet

By default the daemon listens on `127.0.0.1:7681` only. To use it from your
phone:

- **Tailscale (recommended):** `tailscale serve 7681`. You get HTTPS, so
  clipboard and notifications work fully.
- **LAN:** `gecko daemon --listen 0.0.0.0:7681`, or set `"listen"` in the
  config.

Then pick **Open on another device** in the palette and scan the QR code. The
link carries your access token. It is kept as a cookie, so treat the link like
a password.

The mobile layout has:

- A drawer with workspaces, agents and machines.
- A key bar: esc, tab, sticky ctrl/alt, arrows, `^C`, pgup/pgdn, select,
  paste.
- A **compose** box for writing prompts with your keyboard's autocorrect.
  Multi-line text goes to agents as a single bracketed paste.

## Keyboard

| | macOS | Linux / Windows |
|---|---|---|
| Palette: tabs, agents, machines, commands | ⌘K | Ctrl+K |
| New tab (same machine and folder) | ⌘T | Ctrl+T |
| Close tab | ⌘W | Ctrl+W |
| Go to tab 1-9 (numbered across workspaces) | ⌘1…9 | Alt+1…9 |
| Next / previous tab | ⌘⇧] / ⌘⇧[ | Ctrl+PgDn / Ctrl+PgUp, Alt+] / Alt+[ |
| Next / previous workspace | ⌘⇧↓ / ⌘⇧↑ | Alt+Shift+↓ / ↑ |
| Agent that needs you | ⌘J | Ctrl+Shift+J |
| Copy / paste | ⌘C / ⌘V | Ctrl+Shift+C / Ctrl+Shift+V (Ctrl+C copies a selection) |
| Select text (full scrollback) | ⌘⇧S | Ctrl+Shift+S |
| Find | ⌘F | Ctrl+Shift+F |
| Rename tab / workspace | F2 or double-click | F2 or double-click |
| Text size | ⌘= / ⌘- / ⌘0 | Ctrl+= / Ctrl+- / Ctrl+0 |
| Show / hide workspaces sidebar | ⌘B | Ctrl+Shift+B |
| Fullscreen (hold Esc to leave) | ⌃⌘F | F11 |

Type `?` in the palette to list every shortcut. **Change Theme** in the
palette previews themes live: Gecko (a green take on Nord, following the system's
light or dark mode), Gecko Dark and Light, Mission Control, Digital Watch, the photo themes
(Sakura, Cyberpunk, Neo-Tokyo, Dirt Bike, Summit, Wood Shop, Ramen, Mt. Fuji,
Jellyfish), Dracula, Nord, Tokyo Night, Catppuccin, One Dark,
Gruvbox, Solarized and GitHub Light. Like all settings, the theme is saved in
`~/.gecko-terminal/settings.json` and shared by every window. Browsers keep a few combos for
themselves in a normal tab. The app window (`gecko open`, or an installed PWA)
receives more of them, and every action is also in the palette.

## Config

Everything lives in `~/.gecko-terminal/` (on every OS; `GECKO_HOME` overrides
it):

| File | Holds |
|---|---|
| `config.json` | machines, workspace templates, listen address |
| `settings.json` | theme, fonts, copy behaviour, key bindings (edited from the app, or by hand; changes apply live) |
| `token` | web access token |
| `state/` | socket, logs, window size, shell integration scripts |

Older installs kept these in `~/.config/gecko` and `~/.local/state/gecko`;
they are copied over on first run.

`config.json`:

```json
{
  "name": "laptop",
  "listen": "127.0.0.1:7681",
  "hosts": [
    { "name": "devbox", "ssh": "me@devbox" },
    { "name": "gpu", "ssh": "me@gpu.internal", "gecko": "/opt/gecko/bin/gecko" }
  ],
  "workspaces": [
    {
      "name": "api", "host": "devbox", "cwd": "~/src/api",
      "tabs": [
        { "name": "claude", "command": "claude" },
        { "name": "server", "command": "make dev" },
        { "name": "shell" }
      ]
    }
  ],
  "agents": { "my-agent": "my-agent" },
  "notify": { "webhook": "https://ntfy.sh/your-topic" }
}
```

- **Workspace templates** open all their tabs at once (`gecko ws open api`, or
  from the sidebar).
- **`notify.webhook`** is POSTed when an agent starts waiting for you, so you
  hear about it with no Gecko window open. With ntfy.sh, that means a phone push.

## Security

- The web UI needs a random token (`~/.gecko-terminal/token`). Pass it once via
  `?token=`, after which it is kept as an HttpOnly, SameSite=Strict cookie.
- WebSocket upgrades check the Origin. The daemon socket is `0600` in a `0700`
  directory.
- Programs can *write* the clipboard with OSC 52, but can never read it.
- Exposing the port beyond loopback is a choice you make. Prefer Tailscale
  over opening it to the internet.

## Status and limits

Working and tested on Linux: local and remote sessions, resume after network
drops, shell integration, ssh/mosh/tmux/agent detection, CLI attach, and the web UI on desktop and mobile layouts. macOS
and Windows builds compile and use the same code paths. These are not
exercised yet:

- **macOS:** process inspection uses `ps`, and the cwd falls back to `lsof`
  without shell integration.
- **Windows:** sessions use ConPTY. There is no foreground-process inspection
  yet, so tabs show titles, bells and OSC notifications but no ssh/tmux/agent
  detection, and there is no PowerShell integration script yet.

Agent status is a heuristic. It looks for screen text such as "esc to
interrupt" and approval prompts, plus output activity, bells and OSC 9/777
notifications. It works well for Claude Code and Codex-style TUIs. Other agents
may need patterns added in `internal/session/session.go`.

Sessions survive client, network and SSH loss, and upgrades.
`gecko upgrade` (which `make install` runs for you, and which is also in the
palette) restarts the daemon in place with the new binary. It hands every
PTY, its buffered output and both listening sockets to the new process via
`exec`, so shells and agents never notice, and clients reconnect and resume
where they were. This works on Linux and macOS. On Windows, `gecko stop`
still ends sessions, and so does a reboot. Ideas for later:

- Split panes.
- A native GPU renderer shell (Tauri or Wails) around the same client.
- tmux control mode (`-CC`), so tmux windows become Gecko tabs.
- iOS and Android wrappers with push notifications.
