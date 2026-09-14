# AI Support MCP

The project now contains a stateless, read-only MCP endpoint for the AI support client:

```text
https://supabase.hawkeye123.dk/functions/v1/readonly-mcp
```

It authenticates with an approved Supabase user JWT and keeps Supabase RLS in
force. The endpoint does not accept arbitrary SQL and does not expose API keys,
tokens, pending commands, signaling data, or unsanitized audit details. It does
not issue an MCP session ID and does not maintain server-side MCP session state.

## Tools

- `list_clients`: list accessible clients and safe online/agent metrics.
- `client_status`: read the current status of one accessible client.
- `client_history`: read bounded, redacted audit and support-action history.
- `knowledge_search`: search published support runbooks and troubleshooting notes.

The endpoint itself performs no writes. MCP tool calls are read-only with
respect to the support data. If tool-call telemetry is required, it must be
collected by the authenticated gateway or client outside this endpoint; it must
not be implemented by granting the MCP request path write access.

## Deployment

The function requires the existing Supabase Edge Function deployment flow:

```bash
supabase functions deploy readonly-mcp
supabase db push
```

The migration creates `support_knowledge`. Only published entries are returned
to normal authenticated users; approved admins can manage knowledge entries.
Do not add passwords, access tokens, private keys, or raw command output to the
knowledge table.

## AI-support clients (Windows -> Ubuntu SSH + Remote Desktop enrollment)

This is a dedicated enrollment path that is **distinct** from both the portable
AI-support EXE (one-click support codes / Quick Support) and the read-only MCP
endpoint above. It registers a Windows PC as an AI-support *client* that an AI
helper on the trusted Ubuntu host can work with over SSH.

### Exact flow

1. An approved admin opens the **AI-support klienter** section in
    `docs/ai-support.html` (separate from "Dine enheder" and Quick Support) and
    clicks *Generér PowerShell-streng*.
2. The dashboard calls the `device-enrollment` Edge Function with
   `action=create, purpose=ai_support`. The server enforces the admin gate and
   returns two purpose-scoped tokens:
   `purpose=ai_support` token creation requires `role='admin'` or
   `'super_admin'` in `user_approvals`; approved non-admins receive `403`.
   One token is for `enroll-ai-support`; the other is for the existing native
   agent `enroll` path. Ordinary `purpose='agent'` token creation is unchanged.
   Only SHA-256 hashes of the one-time tokens are stored (30-minute expiry,
   single use). The raw tokens are shown only in the generated PowerShell
   command.
3. The generated one-liner downloads `setup-ai-support-windows.ps1` from the
   updates host and runs it with `-EnrollmentUrl`, `-EnrollmentToken`,
   `-AgentEnrollmentToken`, and `-ClientName` (all safely single-quoted).
4. The script (same model as `setup-opencode-windows.ps1`) then:
    - installs the Windows **OpenSSH Client** capability if missing (requires
      an Administrator PowerShell only when the capability is missing);
   - creates a dedicated `id_ed25519_ai_support` key if absent;
   - configures the SSH host alias `ai-support-ubuntu`
     (`IdentitiesOnly`, `RequestTTY`, `ServerAliveInterval 30`,
     `StrictHostKeyChecking accept-new`);
   - copies the **public** key to the Ubuntu AI-support host and verifies
     key-based SSH with `BatchMode` (no password fallback);
    - installs `~/bin/ai-support-opencode.cmd`, a wrapper that starts the
      existing `remote-desktop-cli support-watch` watcher on Ubuntu and then
      execs `opencode` in the remote project;
    - computes the SHA-256 fingerprint of the **public** key;
    - downloads the current `remote-agent.exe`, exchanges the separate agent
      token, verifies the pinned SHA-256 artifact, installs the persistent
      Windows service, and starts it (one UAC prompt);
    - POSTs `action=enroll-ai-support` to `device-enrollment` **only after SSH
      verification and agent startup succeed**.
5. The agent token uses the existing `consume_device_enrollment` RPC and
   creates the PC in `remote_devices`. Therefore Ubuntu can use
   `remote-desktop-cli list`, `remote-desktop-cli connect <name>`, or trusted
   `remote-desktop-cli ai-connect <name>` with `RD_AI_CONTROLLER_KEY`.
6. The AI-support token calls the `consume_ai_support_enrollment`
   `SECURITY DEFINER` RPC (service-role only), which locks the token, requires
   `purpose='ai_support'`, enforces single-use/expiry, validates bounded
   metadata and the `ai-<hex>` client ID format, upserts the client row, marks
   the token used, and writes a redacted audit event
   (`AI_SUPPORT_CLIENT_ENROLLED`).

### Revoking a client

- Each `ready` client row in the dashboard's **AI-support klienter** section
  has a **Revokér** button behind an explicit `confirm()` dialog. It calls the
  authenticated `SECURITY DEFINER` RPC `revoke_ai_support_client(p_client_id)`.
- The RPC allows only the approved owner or an admin/super_admin; it sets
  `status='revoked'`, `updated_at=now()`, and inserts a redacted
  `AI_SUPPORT_CLIENT_REVOKED` audit event (public metadata only). Execution is
  granted to `authenticated` only — everything else, including `PUBLIC`, is
  revoked.
- Revocation is terminal: `consume_ai_support_enrollment` refuses to enroll a
  revoked `client_id` again ("identity resurrection" is blocked at the
  database level), and no client role has `UPDATE` access to
  `ai_support_clients`. Re-adding the PC requires a new enrollment with a new
  client ID.

### Direction and security properties

- **SSH direction is always Windows client -> trusted Ubuntu AI-support host.**
  The script never installs a Windows SSH server or a general inbound SSH
  port. The persistent Remote Desktop agent is a separate outbound-connected
  WebRTC path and adds its existing program-scoped Windows Firewall rule.
- The `ai_support_clients` table stores only public metadata: client ID,
  owner, display name, hostname, platform, SSH host/port/user, public-key
  fingerprint, status (`ready`/`revoked`), and timestamps. **No private keys,
  no passwords, no raw tokens.**
- RLS allows owners and admins SELECT only; all writes go through the
  service-role RPC. A revoked client cannot be resurrected by re-enrollment.
- The read-only MCP is unchanged: it gains no SSH control and no write access,
  but its existing `list_clients`, `client_status`, and `client_history` tools
  can see the enrolled PC through `remote_devices`.
- The script never prints or stores passwords or private key material; the
  one-time enrollment tokens appear only in the invoking command line.
- The elevated agent binary is downloaded from an immutable, versioned URL and
  must match the SHA-256 hash embedded in the generated command. Update both
  values when publishing a new agent release.

### ⚠️ Known test/deployment limitation: `accept-new` host-key trust

First-connect SSH uses `StrictHostKeyChecking accept-new`, which **trusts the
Ubuntu host key on first sight**. This is an accepted limitation for the
current test/deployment environment only — it is vulnerable to a
machine-in-the-middle on the very first connection to a new host. Before any
production/non-test rollout, pre-populate `known_hosts` (e.g. via
`ssh-keyscan` over a trusted path or managed deployment) and switch the alias
to `StrictHostKeyChecking yes`. Host-key rotation on the Ubuntu host also
requires manual `known_hosts` cleanup with this setting.

### Deployment

```bash
# Migration + function
supabase db push
supabase functions deploy device-enrollment

# Publish the setup script and the immutable agent artifact to the downloads
# host (dashboard one-liners reference both files)
cp setup-ai-support-windows.ps1 ~/caddy/downloads/setup-ai-support-windows.ps1
cp builds/remote-agent-v3.1.131.exe ~/caddy/downloads/remote-agent-v3.1.131.exe

# Publish the changed dashboard (GitHub Pages serves docs/ from the repo).
# Required for this change set: docs/js/devices.js (revoke button + role
# helper), docs/js/auth.js (shared role lookup), and the cache-bust version
# bump for both scripts in docs/dashboard.html (?v=).
git add docs/dashboard.html docs/js/devices.js docs/js/auth.js
git commit -m "Dashboard: AI-support client revoke + admin role ordering"
git push origin main
```

### Validation

- Dashboard JS: `node --check docs/js/devices.js docs/js/auth.js`
- PowerShell parser check (on a Windows/PowerShell machine):
  `powershell -NoProfile -Command "$t=$null;$e=$null;[System.Management.Automation.Language.Parser]::ParseFile('setup-ai-support-windows.ps1',[ref]$t,[ref]$e)|Out-Null;if($e){$e;exit 1}else{'OK'}"`
- Edge Function: `deno check supabase/functions/device-enrollment/index.ts`
- Purpose/admin rules: `deno test supabase/functions/device-enrollment/purpose_auth.test.ts`
