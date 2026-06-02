# AI Handover - Remote Desktop

Date: 2026-06-03

## Repository

- Local path: `/home/dennis/projekter/Remote Desktop`
- GitHub repo: `https://github.com/stangtennis/Remote`
- Git remote: `origin https://github.com/stangtennis/Remote.git`
- Main branch: `main`
- Current git state at handover: clean, `main...origin/main`
- Latest GitHub release: `v3.1.89`
- Latest release URL: `https://github.com/stangtennis/Remote/releases/tag/v3.1.89`
- GitHub releases currently show `v3.1.89` as Latest.

## Update And Download Endpoints

- Update server: `https://updates.hawkeye123.dk/version.json`
- Downloads/update host: `https://updates.hawkeye123.dk/`
- Dashboard: `https://dashboard.hawkeye123.dk`

Current update-server state:

```json
{
  "agent_version": "v3.1.89",
  "controller_version": "v3.1.89",
  "agent_url": "https://updates.hawkeye123.dk/remote-agent-v3.1.89.exe",
  "agent_url_macos": "https://updates.hawkeye123.dk/remote-agent-macos-v3.1.89",
  "controller_url": "https://updates.hawkeye123.dk/controller-v3.1.89.exe",
  "controller_url_macos": "https://updates.hawkeye123.dk/controller-macos-v3.1.89",
  "agent_sha256": "2b85e57baf5a67433b54880f0498dee1f144a19e5fe4af7ecd01ef33465a15ef",
  "agent_sha256_macos": "1e9549210e407074d2f8198663a1e4432ea6ca16a621f15222d2b6baef5d6353",
  "controller_sha256": "08e482059578d8611e6b21efcb5bcdeeee56c08539df2dbb50fe7e518a08cb19",
  "controller_sha256_macos": "f24eafe7067d29dd09d8d24f65f020987f89059035271b41ca7558d6abe5d817"
}
```

## Project Layout

- `agent/` - native agent, Windows/macOS capture, input, WebRTC, updater.
- `controller/` - native/Wails controller, device list, viewer, H.264/JPEG switching.
- `docs/` - GitHub Pages dashboard and browser viewer.
- `supabase/` - migrations and edge functions.
- `builds/` - generated release artifacts, not source of truth.
- `scripts/publish-to-caddy.sh` - publishes builds to update/download server.

## Current User-Facing Status

The active work focused on `WIN11DL`, VPN/relay connectivity, and controller H.264 freeze.

`WIN11DL`:

- Device ID: `device_66c7a17db21cd529e147e962ad56ff5e`
- Device was approved in Supabase after it was found as `approved=false`.
- Agent was updated through remote `force_update` commands during the work.
- Agent relay mode was enabled with remote pending command `enable_relay`.
- Connection eventually worked through TURN relay.

Do not require the user to set relay on the controller. User explicitly wants controller and agent to work normally by default.

## Relay Design

Relay is not default globally.

Default behavior remains normal WebRTC ICE:

- Controller default: `ICETransportPolicyAll`
- Agent default: normal all-candidate behavior
- TURN is available as fallback

Relay-only is opt-in:

- Controller: `RD_FORCE_RELAY=1`
- Agent: `RD_FORCE_RELAY=1`
- Agent can also be toggled remotely via pending commands:
  - `enable_relay`
  - `disable_relay`

For `WIN11DL`, relay was applied only on the agent side so the controller can keep working normally.

## Recent Release Summary

### v3.1.84

- Added controller opt-in relay mode via `RD_FORCE_RELAY=1`.
- Native Wails controller, native viewer, and CLI can force relay-only.
- Default remains normal `ICETransportPolicyAll`.

### v3.1.85

- Added agent `RD_FORCE_RELAY=1` support.
- Agent can advertise TURN relay candidates only when explicitly enabled.

### v3.1.86

- Added agent pending commands:
  - `enable_relay`
  - `disable_relay`
- Allows changing relay mode remotely without physical access.

### v3.1.87

- Added `turn.hawkeye123.dk` coturn fallback to agent ICE server list.
- Matched the fallback already used by dashboard/controller.

### v3.1.88

- Ensured agent always includes coturn fallback even if Edge Function TURN fetch fails.
- Fixed relay-only agent producing SDP answer but no ICE candidates when auth token is stale.

### v3.1.89

- Fixed controller H.264 freeze.
- Controller no longer treats stale video dimensions as active H.264.
- Controller H.264 toggle bitrate reduced from `32000` kbps to `4000` kbps.
- Controller automatically falls back to JPEG if H.264 does not produce moving frames after about 4.5 seconds.

## Important Files Changed Recently

- `controller/frontend/js/viewer.js`
  - Wails/native controller viewer.
  - H.264 toggle and fallback logic.
  - Current H.264 bitrate helper returns `4000` kbps.
  - Tracks real H.264 video-frame progress and falls back to JPEG if stalled.

- `agent/internal/webrtc/signaling.go`
  - Agent ICE server fetching and fallback.
  - Adds coturn fallback even if Edge Function credentials fail.

- `agent/internal/webrtc/peer.go`
  - Agent `RD_FORCE_RELAY` handling.
  - Applies `ICETransportPolicyRelay` only when enabled.

- `agent/internal/device/presence.go`
  - Remote pending commands.
  - Handles `force_update`, `enable_relay`, `disable_relay`, `restart`, `lock`, `shutdown`.

- `docs/js/webrtc.js`
  - Dashboard WebRTC viewer.
  - Dashboard got a canvas/H.264 fallback fix separately.

- `docs/dashboard.html`
  - Script cache bump for dashboard WebRTC JS.

- Version metadata:
  - `controller/main.go`
  - `agent/internal/version/version.go`
  - `agent/internal/tray/tray_windows.go`
  - `agent/internal/tray/tray_darwin.go`

## Verification Commands Used

Controller/JS checks:

```bash
node --check controller/frontend/js/viewer.js
node --check docs/js/webrtc.js
git diff --check
cd controller && go test ./internal/webrtc ./cmd/remote-desktop-cli && go test .
```

Build and publish:

```bash
./build-local.sh v3.1.89
./scripts/publish-to-caddy.sh v3.1.89 /home/dennis/caddy/downloads
```

GitHub release:

```bash
git tag -a v3.1.89 -m "Release v3.1.89"
git push origin main
git push origin v3.1.89
gh release create v3.1.89 --title "Remote Desktop v3.1.89" --notes-file builds/RELEASE_NOTES-v3.1.89.md builds/*v3.1.89*
```

Status checks:

```bash
git status --short --branch
gh release list --limit 5
curl -fsSL https://updates.hawkeye123.dk/version.json
```

## Useful Supabase Operations

Use `.env` for Supabase URL and service role key. Do not print secrets in user-facing output.

Check `WIN11DL`:

```bash
set -a; source .env; set +a
curl -fsS "$SUPABASE_URL/rest/v1/remote_devices?device_id=eq.device_66c7a17db21cd529e147e962ad56ff5e&select=device_name,agent_version,is_online,last_seen,pending_command,connection_type,session_bytes_sent,session_bytes_received,approved,public_ip,isp" \
  -H "apikey: $SUPABASE_SERVICE_ROLE_KEY" \
  -H "Authorization: Bearer $SUPABASE_SERVICE_ROLE_KEY" | jq '.[0]'
```

Enable relay on only `WIN11DL`:

```bash
set -a; source .env; set +a
curl -fsS -X PATCH "$SUPABASE_URL/rest/v1/remote_devices?device_id=eq.device_66c7a17db21cd529e147e962ad56ff5e" \
  -H "apikey: $SUPABASE_SERVICE_ROLE_KEY" \
  -H "Authorization: Bearer $SUPABASE_SERVICE_ROLE_KEY" \
  -H "Content-Type: application/json" \
  --data '{"pending_command":"enable_relay"}'
```

Force update only `WIN11DL`:

```bash
set -a; source .env; set +a
curl -fsS -X PATCH "$SUPABASE_URL/rest/v1/remote_devices?device_id=eq.device_66c7a17db21cd529e147e962ad56ff5e" \
  -H "apikey: $SUPABASE_SERVICE_ROLE_KEY" \
  -H "Authorization: Bearer $SUPABASE_SERVICE_ROLE_KEY" \
  -H "Content-Type: application/json" \
  --data '{"pending_command":"force_update"}'
```

## If H.264 Still Freezes In Controller

First verify the controller is actually `v3.1.89`.

Then ask for the controller session log. In the controller viewer UI, the log should show:

- Codec requested/active status.
- Video dimensions.
- Connection and ICE state.
- Connection type, ideally `TURN (Relay)` for `WIN11DL`.
- Whether fallback message appears: `H.264 gav ingen stabil video - skifter tilbage til JPEG`.

Likely next debugging targets:

- `controller/frontend/js/viewer.js` H.264 fallback timer and `_hasRecentH264Progress()`.
- Agent logs around:
  - `🎛️ Received set_mode request`
  - `🎬 H.264 tilstand aktiveret`
  - `H.264 encode fejl`
  - `Video track write fejl`
- `agent/internal/webrtc/streaming.go`, especially H.264 encode/write path.

## Build Notes And Caveats

- Broad Linux agent tests may fail due Windows-only build tags/CGO/DXGI. Cross-build is the relevant verification for the Windows agent.
- Follow `AGENTS.md` build rules for long-running cross-build commands.
- Do not revert user changes unless explicitly asked.
- Do not commit `.env`, secrets, credentials, or generated binaries.
- Do not put Windows login credentials into repo docs.

## Security Notes

- `.env` contains Supabase keys and must not be committed.
- Treat TURN secrets and Supabase service role keys as secrets.
- Avoid weakening Supabase RLS/policies without a clear reason.

