# remote-desktop-mcp

A bounded local MCP adapter (stdio) that fronts the existing
`remote-desktop-cli` binary and proxies shared support context from the central
`readonly-mcp` Edge Function. It adds no WebRTC or remote-control logic. CLI
status lookups use the existing credential loader; context tools authenticate
to the Edge Function with the same credentials.

Built with `github.com/mark3labs/mcp-go` v0.44.0 over stdio.

## Tools (bounded allow-list)

| Tool | CLI used | Behavior |
|---|---|---|
| `list_devices` | `remote-desktop-cli list` | Parses device lines into structured JSON; falls back to bounded raw text when output is unrecognized. |
| `status` | `remote-desktop-cli status` | Bounded local daemon/connection status. Daemon-absent is reported as plain text, not an error. |
| `support_list` | `remote-desktop-cli support-list` | Parses active AI support sessions (key, status, expiry, session id) into structured JSON; bounded raw fallback. |
| `support_watch_status` | none | Reports whether a same-user `remote-desktop-cli support-watch` process is running (via a safe `/proc` scan — `pgrep` with a fixed argv on non-Linux) and the last ~30 lines of its log. Never starts, stops, or signals the watcher. |
| `support_context` | central `readonly-mcp` | Returns one client's bounded status, history, and optional published troubleshooting knowledge. |
| `client_history` | central `readonly-mcp` | Returns bounded, redacted history for a remote or persistent SSH AI-support client. |
| `knowledge_search` | central `readonly-mcp` | Searches the central published support knowledge base. |
| `knowledge_draft` | central `readonly-mcp` | Creates an unpublished knowledge draft; admin-only at the central endpoint. |
| `knowledge_publish` | central `readonly-mcp` | Publishes an existing knowledge draft; admin-only at the central endpoint. |

**Never exposed here:** `click`, `type`, `press_key`, `scroll`, `exec`,
`upload`, `download`, SSH, `connect`/`disconnect`, screenshots, process
management. The adapter's internal subcommand allow-list is exactly
`list`, `status`, `support-list`.

## Safety properties

- **No shell.** The CLI is invoked with `exec.Command` using a fixed
  two-element argv (`[cli, subcommand]`). Nothing from MCP requests reaches
  the subprocess.
- **Environment preserved.** The subprocess inherits the adapter's
  environment verbatim, so `RD_CREDENTIALS_FILE`, `RD_EMAIL`, `RD_PASSWORD`
  and `HOME` work exactly as when running the CLI by hand. The adapter never
  reads, stores, or prints credential values.
- **Fail fast.** The process exits with a clear stderr message if the CLI
  binary is missing/not executable or if credentials are absent (env vars or
  a mode-0600 credentials file defining both `RD_EMAIL` and `RD_PASSWORD`).
- **Bounded output.** Tool results are capped (~8 KiB, 64 KiB capture cap)
  and credential-looking values (`password=`, `token=`, `apikey:`,
  `Bearer ...`) are redacted before anything is returned.
- **Central context.** Read tools call the central Edge MCP endpoint. The two
  knowledge write tools only create unpublished drafts or publish an explicitly
  selected entry; the endpoint enforces approved admin/super_admin access and
  database RLS.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `REMOTE_DESKTOP_CLI` | `$HOME/.local/bin/remote-desktop-cli` | Path to the CLI binary. |
| `RD_CREDENTIALS_FILE` | `$HOME/.config/remote-desktop/credentials.env` | Credentials file consumed by the CLI itself (mode 0600). |
| `RD_WATCH_LOG` | `$XDG_STATE_HOME/remote-desktop/support-watch.log` or `$HOME/.local/state/remote-desktop/support-watch.log` | Watcher log read by `support_watch_status`. |

## Local use

Build and point any MCP client (stdio) at it, e.g. for OpenCode:

```json
{
  "local": {
    "type": "local",
    "command": "/path/to/remote-desktop-mcp",
    "environment": {
      "REMOTE_DESKTOP_CLI": "/home/you/.local/bin/remote-desktop-cli"
    }
  }
}
```

No secrets, credentials, or config files belong in this repository; keep the
credentials file mode 0600 outside the repo.

## Development

```sh
cd controller
gofmt -l ./cmd/remote-desktop-mcp
go vet ./cmd/remote-desktop-mcp
go test ./cmd/remote-desktop-mcp
```

Tests use stub CLI scripts, fake `/proc` trees, and temporary credential
files — they never touch the network, WebRTC, or real credentials.
