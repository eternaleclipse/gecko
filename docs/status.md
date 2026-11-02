# Status and limits

[Docs](README.md)

Working and tested on Linux: local and remote sessions, resume after network
drops, shell integration, ssh/mosh/tmux/agent detection, CLI attach, and the web UI on desktop and mobile layouts. macOS
and Windows builds compile and use the same code paths. These are not
exercised yet:

- **macOS:** process inspection uses `ps`, and the cwd falls back to `lsof`
  without shell integration.
- **Windows:** sessions use ConPTY. There is no foreground-process inspection
  yet, so tabs show titles, bells and OSC notifications but no ssh/tmux/agent
  detection, and there is no PowerShell integration script yet.

Apps and packages, each built and smoke-tested in CI:

- **iOS:** the [iOS app](ios.md) runs on iPhone and iPad, built for the iOS
  Simulator for now (no App Store yet).
- **Desktop:** [desktop apps](desktop-packages.md) for macOS (dmg), Windows
  (setup or zip) and Linux (AppImage), unsigned.
- **Linux:** [packages](linux-packages.md) for Debian and Ubuntu, Fedora,
  Arch and snap, and static binaries that run on any Linux.
- **Termux:** Gecko runs in [Termux](android.md#termux) on Android, and the
  Android app opens it like any other machine.

New in 0.3.3:

- **The desktop window doesn't freeze:** when its page is killed (the
  out-of-memory killer picks it first, or a crash), the window reloads it
  instead of sitting on its last frame with tabs that never change. The
  sessions were never lost; they live in the daemon.
- **`gecko url` right from the start:** a daemon that `gecko url` or
  `gecko open` had just started could report the default port 7681 instead
  of the one it listens on, so the window opened on nothing.

New in 0.3.2:

- **Codex and OpenCode, first class:** each has its own status reader,
  built from their real screens, so a paused, stopped or waiting agent no
  longer shows as working, approvals show their choices in the agent dock,
  and a new OpenCode tab is Idle, not Needs you. Codex is also recognised
  over ssh.
- **Long-press with the keyboard up:** on Android, a long-press on a machine
  opens its menu (and Make anchor) on the first try; the keyboard going no
  longer closes it.
- **Share off cuts everyone off:** `gecko share off` also closes a browser
  that was still connecting when sharing stopped.

New in 0.3.1:

- **Make anchor (protocol 11):** any machine in the sidebar can become the
  anchor, and the icon always marks it (see
  [Make anchor](anchor.md#make-anchor-protocol-11)). The choice is remembered
  across launches, with a **Back to** list, anchoring through the relay is on
  by default, and the window keeps its own settings after a switch. Upgrade
  the anchor or door and the machines: a v10 machine says "Update Gecko on X
  to make it the anchor".
- **One gecko per machine:** `make install` and Update Gecko restart the
  service with the binary they just installed, wherever the old one ran
  from, and other copies become links to it (see
  [Where gecko lives](install.md#where-gecko-lives)). A restart that can't
  work (a binary still being copied) now fails up front with the reason and
  leaves the machines connected, instead of "write |1: file already closed".
- **Closed tabs stay closed:** a closed tab could stay listed and come back
  on every reconnect, its shell left a zombie. Closing now always ends it,
  and an upgrade is no longer refused while a closed tab is still ending.
- **Phones keep their size:** a session open on the phone and on a window
  left open elsewhere no longer turns jumbled on the phone until a pinch.
  Only the device in use (touched, typed in or come back to in the last 2
  minutes) sets the size after a reconnect, and coming back to the app or
  touching the terminal takes the size back.

New in 0.3.0:

- **Transparency:** background opacity shows the desktop on macOS and
  Windows too, not a gray material, checked in CI on all three systems.
- **The anchor (protocol 10):** the Gecko you connect to manages every other
  machine (see [The anchor](anchor.md)). The server now enforces scope: relay
  peers only get that machine's own tabs, whatever they ask for. v9 machines
  still work with a v10 anchor.
- **Web app on phones:** installable, with Settings on the tab bar, pinch for
  text size, and tabs that switch on every tap. `make test-pwa` runs it in
  WebKit and Chromium, and CI also runs Safari in the iOS Simulator.

Agent status is read from the screen. Codex and OpenCode each have a reader
of their own, made from their real screens (recorded in
`internal/session/testdata/agents`, with the versions there): their working
markers, their approval dialogs and questions with the answers on offer, and
where Esc leaves them. Codex also reports its state with OSC 7501 and in its
title, which count too. For these two, output alone never means working:
they redraw while they wait. Other agents get general rules, made from
Claude Code's screens: text such as "esc to interrupt" and approval prompts,
output activity, bells and OSC 9/777 notifications. An agent they don't fit
may need a reader in `internal/session/agents.go`.

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
- Push notifications for the iOS app (APNs), so it notifies in real time
  while closed (see [iOS app](ios.md)), and a signed build of it.
