# Features

[Docs](README.md)

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
- **[Many machines](machines.md).** Add a host by its SSH destination. Gecko runs
  `gecko bridge` there over SSH, and that machine's own daemon owns its
  sessions. A dropped connection just reconnects and resumes, much like mosh,
  for every tab at once.
- **tmux, natively.** Gecko talks to each machine's tmux server: sessions and
  windows appear in the sidebar and in the palette, a tmux tab is titled by
  its active window, and the status line has clickable window chips. Click to
  switch, **+** for a new window, double-click to rename. Opening a session that
  is already in a tab goes to that tab instead of opening another one. tmux
  inside a plain ssh tab is recognised from its status bar.
- **[Themes](themes.md).** A green take on Nord by default, plus Mission Control (a
  launch-webcast skin with DIN-style type), Digital Watch (the window becomes a
  resin wristwatch whose LCD is the terminal, resizing with the window, with a ticking 7-segment
  face and working LIGHT / MODE / START·STOP / ALARM pushers), photo themes that bring their own
  fonts and shapes (Sakura, Cyberpunk, Neo-Tokyo, Dirt Bike, Summit, Wood Shop,
  Ramen, Mt. Fuji, Jellyfish; CC0 photos, credits in
  `web/public/themes/CREDITS.md`) and the classics (Dracula, Nord, Tokyo Night, Catppuccin, One
  Dark, Gruvbox, Solarized, GitHub Light). The desktop app adds a see-through
  background.
- **Settings follow you.** Theme, fonts, key bindings and the rest live in
  `~/.gecko-terminal/settings.json` and sync live to every window and device.
  Every shortcut can be remapped (Ctrl+K, then **Keyboard shortcuts…**).
- **A startup chime.** Three soft notes play when Gecko opens. Turn it off
  with **Startup sound** in the palette. Plain browser tabs stay silent until
  you click, as browsers require.
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

