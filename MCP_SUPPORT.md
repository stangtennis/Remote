# AI Support MCP

The project now contains a stateless MCP endpoint for the AI support client:

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
- `knowledge_draft`: save a new unpublished support knowledge draft (admin only).
- `knowledge_publish`: publish an existing knowledge draft (admin only).

The endpoint has no arbitrary database or support-control access. The two
knowledge write tools are the only writes, require an approved `admin` or
`super_admin`, and are enforced again by `support_knowledge` RLS. Drafts remain
unpublished until an explicit publish call.

## Shared AI-support context

Both support paths use the same backend history and MCP context:

- Portable AI-support EXE/WebRTC actions are recorded in `support_action_audit`.
- Persistent SSH activity is recorded as structured `AI_SUPPORT_OPERATION`
  events in `audit_logs` under the AI-support client ID. Logs contain only the
  operation category, result, exit code, mode, and duration; raw commands are
  not retained.
- MCP exposes both client types through `list_clients` and `client_status`,
  merges bounded history through `client_history`, and provides `support_context`
  for status, history, and troubleshooting knowledge.
- Curated lessons belong in `support_knowledge`, can be saved as drafts through
  `knowledge_draft`, and are retrieved only after publication by
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

The enrollment backend automatically mints a separate Cloudflare Access
Service Token for each Windows machine. Only `CLOUDFLARE_API_TOKEN` and
`CLOUDFLARE_ACCOUNT_ID` are configured once as Supabase Edge secrets; no
Cloudflare Service Token ID or secret is copied to Windows by the user.

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
   updates host, verifies its pinned SHA-256, and runs it in the same
   PowerShell process with only the enrollment token, URLs, client name, and
   Cloudflare hostname. There are no Cloudflare credential prompts or manual
   service-token parameters. The script pins the downloaded public key to the
   configured Ubuntu fingerprint.
4. The script (same model as `setup-opencode-windows.ps1`) then:
       - exchanges the enrollment token and generated `ai-<hex>` client ID with
         the enrollment backend for a separate Cloudflare service token for
         this machine. A retry with the same still-unused enrollment token
         deletes the prior service token before issuing its replacement;
       - installs the pinned official Windows amd64 `cloudflared` 2026.9.1
        binary from GitHub, verifies SHA-256
        `2837888cc0f5d58f15b6dc478376de90b4d3ba5241c7947455d1e0a0df429712`,
        and stores it at `C:\ProgramData\AI-Support\cloudflared.exe` with
         current-user/SYSTEM/Administrators ACLs;
        - protects the newly issued service-token secret with DPAPI CurrentUser at
        `C:\ProgramData\AI-Support` and stores only the non-secret client ID
         separately with current-user/SYSTEM/Administrators ACLs;
      - starts `cloudflared access tcp --hostname ssh.hawkeye123.dk --url
        127.0.0.1:43000` on loopback. Token ID and secret are inherited only
        by that child through `TUNNEL_SERVICE_TOKEN_ID` and
        `TUNNEL_SERVICE_TOKEN_SECRET`, then cleared from the setup process;
      - installs the Windows **OpenSSH Client and Server** capabilities;
       - does not create a Windows support account; the logged-in Windows user
         is the SSH user and the Ubuntu operator key is stored in the protected
         AI-support state directory;
      - configures Windows OpenSSH Server to listen only on `127.0.0.1`;
      - falls back to a hidden SYSTEM `AI-Support-OpenSSH` scheduled task when
        the Windows `sshd` service exits unexpectedly, while keeping the same
        localhost-only OpenSSH configuration;
      - creates a forwarding-only tunnel key and installs its restricted
        `permitlisten` entry on Ubuntu through the loopback cloudflared bridge;
      - performs every bootstrap, password, key, and temporary reverse-tunnel
        SSH call as `dennis@127.0.0.1 -p 43000` through that bridge;
      - starts and verifies `ssh -N -T -R 127.0.0.1:<tunnel-port>:127.0.0.1:<WindowsSshPort>`;
       - installs `AI-Support-Persistent-Tunnel` as a hidden task at the
         current user's logon with keepalives and automatic reconnect. The task
         decrypts the CurrentUser DPAPI secret, starts its own loopback
         cloudflared bridge, waits for the listener, then runs the reverse SSH
         tunnel. Its cleanup stops only the cloudflared process that it started;
      - installs a forced PowerShell shell with local activity logging and
        uploads a structured operation event for each SSH command without
        retaining the command text;
     - POSTs `action=enroll-ai-support` only after the tunnel is reachable.
5. Ubuntu reaches the Windows SSH endpoint through the registered tunnel:
   `ssh -p <tunnel_port> <windows_ssh_user>@127.0.0.1`. No Ubuntu private-IP fallback
   is used by enrollment. The enrollment metadata records the public
   Cloudflare hostname `ssh.hawkeye123.dk` and SSH port `22`.
6. The AI-support token calls the `consume_ai_support_enrollment`
   `SECURITY DEFINER` RPC (service-role only), which locks the token, requires
   `purpose='ai_support'`, enforces single-use/expiry, validates bounded
   metadata and the `ai-<hex>` client ID format, upserts the client row, marks
   the token used, and writes a redacted audit event
   (`AI_SUPPORT_CLIENT_ENROLLED`). The RPC reserves a unique port in
    `42000..42999` for each ready client.

### Cloudflare Access configuration

The Access application for `ssh.hawkeye123.dk` must use the **Service Auth**
policy with **Include: Any Access Service Token**. Do not select one fixed
service token: the enrollment backend creates a different token for each
machine, so the token set is dynamic. This policy permits every Access service
token in the Cloudflare account to authenticate to this application. Use a
dedicated Cloudflare account/tenant for AI support, or ensure the account has
no unrelated service tokens.

The Cloudflare API token configured as `CLOUDFLARE_API_TOKEN` must have
`Access: Service Tokens Write`, which supports both creating and deleting
service tokens. `CLOUDFLARE_ACCOUNT_ID` and the API token remain Supabase Edge
secrets; no Cloudflare secret or manual token copy is part of enrollment.

### Revoking a client

- Each `ready` client row in the dashboard's **AI-support klienter** section
  shows its client-specific activity log and has a **Revokér** button behind an explicit `confirm()` dialog. It calls the
  authenticated `device-enrollment` action
  `revoke-ai-support-cloudflare-token` first. Only after the associated
  Cloudflare token is deleted does it call the `SECURITY DEFINER` RPC
  `revoke_ai_support_client(p_client_id)` to mark the client for terminal
  revocation.
- The RPC allows only the approved owner or an admin/super_admin; it sets
  `status='uninstall_pending'`, `updated_at=now()`, and inserts a redacted
  `AI_SUPPORT_CLIENT_REVOKED` audit event (public metadata only). Execution is
  granted to `authenticated` only — everything else, including `PUBLIC`, is
  revoked. The Ubuntu `ai-support-tunnel-reconciler` then removes the client
  setup and completes terminal database cleanup.

### Removing legacy local accounts

- Older Windows enrollments created a local `ai-support` account. The dedicated
  **Fjern gammel ai-support-bruger** action on the AI-support page downloads a
  separately pinned PowerShell cleanup script.
- The cleanup script requires Administrator PowerShell, verifies the exact
  legacy account description plus a second legacy marker, and refuses to run
  while the account owns a scheduled task or running process. It removes only
  the local account and leaves the current-user tunnel, Remote Desktop agent,
  OpenSSH configuration, and `C:\ProgramData\AI-Support` untouched.
- The old profile directory is deliberately retained and reported for manual
  review; the script does not recursively delete user data.
- Revocation is terminal for database enrollment: `consume_ai_support_enrollment`
  refuses to enroll a client identity that is not `ready` ("identity
  resurrection" is blocked at the database level), and no client role has
  `UPDATE` access to `ai_support_clients`. Re-adding the PC requires a new
  enrollment with a new client ID. The reconciler removes revoked tunnel keys
  and terminates active sessions; the Windows task then remains unable to
  reconnect because its Ubuntu key authorization is gone.

### Direction and security properties

- **The tunnel is always initiated outbound from Windows to Ubuntu.** Windows
  OpenSSH Server listens only on `127.0.0.1`; no general inbound Windows SSH
  port is opened. Windows reaches Ubuntu only through the Cloudflare Access
  hostname `ssh.hawkeye123.dk`; the old direct private-IP SSH path is not
  used. There is no Remote Desktop agent, WebRTC path, or controller dependency.
- The Cloudflare service-token secret is received only over HTTPS as a
  one-time issuance response and accepted only as a PowerShell `SecureString`,
   protected with DPAPI `CurrentUser`, and never appears in command-line
  arguments, the scheduled-task definition, the runner script, enrollment
  metadata, activity config, logs, or this repository. Supabase stores only
  `device_enrollment_tokens.cloudflare_service_token_id`, which is non-secret
  issuance/retry metadata; the generated service-token secret is not
  raw-stored there.
  The runner decrypts it only long enough to place it in the environment of its
  child `cloudflared` process and then clears its own environment variables.
   The tunnel and shell run with the logged-in user's Windows permissions; the
   setup does not create or grant a separate administrator account.
- The cloudflared executable is downloaded only over HTTPS, hash-checked
  against the pinned release value, and protected by SYSTEM/Administrators
  ACLs. Bridge cleanup uses the owned process handle and does not kill
  unrelated cloudflared processes.
- The `ai_support_clients` table stores only public metadata: client ID,
  owner, display name, hostname, platform, Ubuntu SSH metadata, tunnel port,
  Windows SSH user/port, public-key fingerprint, status (`ready`/`revoked`),
  and timestamps. The log stores bounded operation metadata only. **No private
  keys, no passwords, no raw commands, no raw enrollment/log tokens.**
- RLS allows owners and admins SELECT only; all writes go through the
  service-role RPC. A revoked client cannot be resurrected by re-enrollment.
- The MCP gains no SSH control. Only the bounded, admin-gated knowledge tools
  can write, and they cannot change support clients or execute commands.
- The script never prints passwords or private keys. The private tunnel key is
  stored locally under `C:\ProgramData\AI-Support` with current-user/SYSTEM/
  Administrators ACLs; it is never sent to Ubuntu or Supabase. The one-time enrollment token
  appears only in the invoking command line.
- Existing installations can refresh the forced shell without re-enrollment by
  running the pinned setup script with `-RefreshLogging`. This replaces the
  local shell and clears legacy raw local activity lines; it does not change the
  tunnel or client identity.

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
# Install the Ubuntu revocation reconciler as root. The service-role key is
# read from the shell environment and stored only in a mode-600 root file.
sudo --preserve-env=SUPABASE_URL,SUPABASE_SERVICE_ROLE_KEY \
  ./tools/install-ai-support-tunnel-reconciler.sh

# Publish the setup script and the Ubuntu operator public key to the downloads
# host (dashboard one-liners reference both files)
cp setup-ai-support-windows.ps1 ~/caddy/downloads/setup-ai-support-windows.ps1
cp ~/.ssh/id_rsa.pub ~/caddy/downloads/ai-support.pub

# Configure the Cloudflare Access application for ssh.hawkeye123.dk. Configure
# these Supabase Edge secrets once. The API token must have Access: Service
# Tokens Write on the account.
supabase secrets set \
  CLOUDFLARE_API_TOKEN='<cloudflare-api-token>' \
  CLOUDFLARE_ACCOUNT_ID='<cloudflare-account-id>' \
  CLOUDFLARE_SERVICE_TOKEN_DURATION='8760h'

# Migration + function
supabase db push
supabase functions deploy device-enrollment

# Publish the changed dashboard through the normal GitHub Pages deployment.
```

### Validation

- Dashboard JS: `node --check docs/js/devices.js`
- Confirm the generated command contains no `Read-Host` calls or Cloudflare
  service-token parameters.
- Confirm `cloudflared.exe` at `C:\ProgramData\AI-Support` has the pinned
  SHA-256 and that the bridge listens only on `127.0.0.1:43000`.
- Confirm the scheduled task runs as SYSTEM, contains no service-token secret
  in its arguments or runner file, and reconnects after the child SSH process
  exits.
- PowerShell parser check (on a Windows/PowerShell machine):
  `powershell -NoProfile -Command "$t=$null;$e=$null;[System.Management.Automation.Language.Parser]::ParseFile('setup-ai-support-windows.ps1',[ref]$t,[ref]$e)|Out-Null;if($e){$e;exit 1}else{'OK'}"`
- Edge Function: `deno check supabase/functions/device-enrollment/index.ts`
- Purpose/admin rules: `deno test supabase/functions/device-enrollment/purpose_auth.test.ts`
