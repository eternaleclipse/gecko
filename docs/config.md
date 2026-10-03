# Configuration

[Docs](README.md)

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

