# More machines

[Docs](README.md)

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

