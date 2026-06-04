# AI Handover - Remote Desktop

Date: 2026-06-04

## Repository

- Local path: `/home/dennis/projekter/Remote Desktop`
- GitHub repo: `https://github.com/stangtennis/Remote`
- Git remote: `origin https://github.com/stangtennis/Remote.git`
- Info/docs repo: `https://github.com/stangtennis/info/tree/main/Remote-Desktop`
- Local info/docs path: `/home/dennis/projekter/info/Remote-Desktop`
- Main branch: `main`
- Current git state at handover: clean, `main...origin/main`
- Latest GitHub release: `v3.1.91`
- Latest release URL: `https://github.com/stangtennis/Remote/releases/tag/v3.1.91`
- GitHub releases currently show `v3.1.91` as Latest.

## Update And Download Endpoints

- Update server: `https://updates.hawkeye123.dk/version.json`
- Downloads/update host: `https://updates.hawkeye123.dk/`
- Dashboard: `https://dashboard.hawkeye123.dk`

Current update-server state:

```json
{
  "agent_version": "v3.1.91",
  "controller_version": "v3.1.91",
  "agent_url": "https://updates.hawkeye123.dk/remote-agent-v3.1.91.exe",
  "agent_url_macos": "https://updates.hawkeye123.dk/remote-agent-macos-v3.1.91",
  "controller_url": "https://updates.hawkeye123.dk/controller-v3.1.91.exe",
  "controller_url_macos": "https://updates.hawkeye123.dk/controller-macos-v3.1.91",
  "agent_sha256": "a6da7baafb78473b9637f6cc888b310f177ef0de6caac72aafcaeb3d7f0b0f01",
  "agent_sha256_macos": "12763694b43471d4edc0f9d595f4f4d5522c1b2c9ee4d42352749f1045ce1b6c",
  "controller_sha256": "89b4154444cb395defa1f95e57241b084ca8ab1e3f45e3f7fe2fa11d93d5590a",
  "controller_sha256_macos": "da0b5c795e1e51c9cfdfb67108fc47ffb647b0b23dd0e6adf2ba9e6b0d0aece8"
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

### v3.1.90

- Increased controller-requested H.264 bitrate to `10000` kbps.
- Changed NVENC H.264 to high-profile CBR.
- Rebuilt and republished full release artifacts after version metadata alignment.

### v3.1.91

- Fixed dashboard H.264 bottom-frame corruption when large UI changes occurred, for example opening the Windows Start menu.
- Dashboard now sends explicit `bitrate: 10000` in `set_mode` for `h264` and `hybrid`.
- Agent NVENC uses browser-safe `baseline` profile again instead of `high` profile.
- Dashboard `webrtc.js` cache bumped to `v=34`.

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
./build-local.sh v3.1.91
./scripts/publish-to-caddy.sh v3.1.91 /home/dennis/caddy/downloads
```

GitHub release:

```bash
git tag -a v3.1.91 -m "Release v3.1.91"
git push origin main
git push origin v3.1.91
gh release create v3.1.91 --title "Remote Desktop v3.1.91" --notes-file builds/RELEASE_NOTES-v3.1.91.md builds/*v3.1.91*
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

First verify the controller/dashboard assets are actually `v3.1.91`.

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
