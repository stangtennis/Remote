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

## Shared AI-support context

Both support paths use the same backend history and MCP context:

- Portable AI-support EXE/WebRTC actions are recorded in `support_action_audit`.
- Persistent SSH commands are recorded as redacted `AI_SUPPORT_COMMAND` events
  in `audit_logs` under the AI-support client ID.
- MCP exposes both client types through `list_clients` and `client_status`,
  merges bounded history through `client_history`, and provides `support_context`
  for status, history, and troubleshooting knowledge.
- Curated, approved lessons belong in `support_knowledge` and are retrieved by
  `knowledge_search`. Credentials, private keys, screenshots, and unrestricted
  command output are never learning data.

The Windows EXE and SSH setup do not receive MCP credentials or call MCP
directly. They report normalized events to authenticated backend endpoints; the
AI uses MCP as the single context layer before and during support work.

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

## AI-support clients (persistent SSH-only enrollment)

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
   returns one purpose-scoped token:
   `purpose=ai_support` token creation requires `role='admin'` or
   `'super_admin'` in `user_approvals`; approved non-admins receive `403`.
   The token is only for `enroll-ai-support`; ordinary `purpose='agent'` token
   creation is unchanged.
   Only SHA-256 hashes of the one-time tokens are stored (30-minute expiry,
   single use). The raw tokens are shown only in the generated PowerShell
   command.
3. The generated one-liner downloads `setup-ai-support-windows.ps1` from the
   updates host and runs it with `-EnrollmentUrl`, `-EnrollmentToken`,
   `-SupportPublicKeyUrl`, and `-ClientName` (all safely single-quoted). The
   script pins the downloaded public key to the configured Ubuntu fingerprint.
4. The script (same model as `setup-opencode-windows.ps1`) then:
     - installs the Windows **OpenSSH Client and Server** capabilities;
      - creates the dedicated local `ai-support` account as a local Windows
        Administrator and installs the Ubuntu operator's public key;
     - configures Windows OpenSSH Server to listen only on `127.0.0.1`;
     - creates a forwarding-only tunnel key and installs its restricted
       `permitlisten` entry on Ubuntu;
     - starts and verifies `ssh -N -T -R 127.0.0.1:<tunnel-port>:127.0.0.1:22`;
      - installs `AI-Support-Persistent-Tunnel` as a SYSTEM startup task with
        keepalives, automatic reconnect, and hidden task visibility;
      - installs a forced PowerShell shell with local activity logging and
        uploads a redacted command event for each SSH command;
     - POSTs `action=enroll-ai-support` only after the tunnel is reachable.
5. Ubuntu reaches the Windows SSH endpoint through the registered tunnel:
   `ssh -p <tunnel_port> ai-support@127.0.0.1`.
6. The AI-support token calls the `consume_ai_support_enrollment`
   `SECURITY DEFINER` RPC (service-role only), which locks the token, requires
   `purpose='ai_support'`, enforces single-use/expiry, validates bounded
   metadata and the `ai-<hex>` client ID format, upserts the client row, marks
   the token used, and writes a redacted audit event
   (`AI_SUPPORT_CLIENT_ENROLLED`). The RPC reserves a unique port in
   `42000..42999` for each ready client.

### Revoking a client

- Each `ready` client row in the dashboard's **AI-support klienter** section
  shows its client-specific activity log and has a **Revokér** button behind an explicit `confirm()` dialog. It calls the
  authenticated `SECURITY DEFINER` RPC `revoke_ai_support_client(p_client_id)`.
- The RPC allows only the approved owner or an admin/super_admin; it sets
  `status='revoked'`, `updated_at=now()`, and inserts a redacted
  `AI_SUPPORT_CLIENT_REVOKED` audit event (public metadata only). Execution is
  granted to `authenticated` only — everything else, including `PUBLIC`, is
  revoked.
- Revocation is terminal for database enrollment: `consume_ai_support_enrollment`
  refuses to enroll a revoked `client_id` again ("identity resurrection" is
  blocked at the database level), and no client role has `UPDATE` access to
  `ai_support_clients`. Re-adding the PC requires a new enrollment with a new
  client ID. The Ubuntu `ai-support-tunnel-reconciler` removes revoked tunnel
  keys and terminates active sessions; the Windows task then remains unable to
  reconnect because its Ubuntu key authorization is gone.

### Direction and security properties

- **The tunnel is always initiated outbound from Windows to Ubuntu.** Windows
  OpenSSH Server listens only on `127.0.0.1`; no general inbound Windows SSH
  port is opened. There is no Remote Desktop agent, WebRTC path, or controller
  dependency.
- The `ai_support_clients` table stores only public metadata: client ID,
  owner, display name, hostname, platform, Ubuntu SSH metadata, tunnel port,
  Windows SSH user/port, public-key fingerprint, status (`ready`/`revoked`),
  and timestamps. The log stores redacted command metadata. **No private keys,
  no passwords, no raw enrollment/log tokens.**
- RLS allows owners and admins SELECT only; all writes go through the
  service-role RPC. A revoked client cannot be resurrected by re-enrollment.
- The read-only MCP is unchanged: it gains no SSH control and no write access.
- The script never prints passwords or private keys. The private tunnel key is
  stored locally under `C:\ProgramData\AI-Support` with SYSTEM/Administrators
  ACLs; it is never sent to Ubuntu or Supabase. The one-time enrollment token
  appears only in the invoking command line.

### ⚠️ Known test/deployment limitation: `accept-new` host-key trust

First-connect SSH uses `StrictHostKeyChecking accept-new`, which **trusts the
Ubuntu host key on first sight**. This is an accepted limitation for the
current test/deployment environment only — it is vulnerable to a
machine-in-the-middle on the very first connection to a new host. Before any
production/non-test rollout, pre-populate the state directory's `known_hosts`
(e.g. via `ssh-keyscan` over a trusted path or managed deployment) and change
the setup/task options to `StrictHostKeyChecking yes`. Host-key rotation on
the Ubuntu host also requires managed `known_hosts` cleanup with this setting.

### Deployment

```bash
# Migration + function
supabase db push
supabase functions deploy device-enrollment

# Install the Ubuntu revocation reconciler as root. The service-role key is
# read from the shell environment and stored only in a mode-600 root file.
sudo --preserve-env=SUPABASE_URL,SUPABASE_SERVICE_ROLE_KEY \
  ./tools/install-ai-support-tunnel-reconciler.sh

# Publish the setup script and the Ubuntu operator public key to the downloads
# host (dashboard one-liners reference both files)
cp setup-ai-support-windows.ps1 ~/caddy/downloads/setup-ai-support-windows.ps1
cp ~/.ssh/id_rsa.pub ~/caddy/downloads/ai-support.pub

# Publish the changed dashboard (GitHub Pages serves docs/ from the repo).
git add docs/ai-support.html docs/js/devices.js MCP_SUPPORT.md
git commit -m "Use persistent SSH-only AI-support tunnels"
git push origin main
```

### Validation

- Dashboard JS: `node --check docs/js/devices.js docs/js/auth.js`
- PowerShell parser check (on a Windows/PowerShell machine):
  `powershell -NoProfile -Command "$t=$null;$e=$null;[System.Management.Automation.Language.Parser]::ParseFile('setup-ai-support-windows.ps1',[ref]$t,[ref]$e)|Out-Null;if($e){$e;exit 1}else{'OK'}"`
- Edge Function: `deno check supabase/functions/device-enrollment/index.ts`
- Purpose/admin rules: `deno test supabase/functions/device-enrollment/purpose_auth.test.ts`
