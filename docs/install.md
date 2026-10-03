# Install and run

[Docs](README.md)

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

