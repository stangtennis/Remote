# Agents Guide

Working notes for contributors to the Remote Desktop project (Go + Supabase + WebRTC) across the agent, controller, web dashboard, and supporting tooling.

## Repo layout
- `agent/` — Windows native agent (Go); service/startup scripts and core capture/input/WebRTC logic.
- `controller/` — Windows controller app (Go + Fyne UI); login, device management, admin panel.
- `docs/` — GitHub Pages dashboard (`index.html`), admin, and web agent assets (HTML/CSS/JS).
- `extension/` — Browser extension for remote control; pairs with `native-host/`.
- `native-host/` — Native messaging helper (Go) for the extension.
- `electron-agent/` — Cross-platform agent prototype.
- `supabase/` — Migrations and edge functions for the backend schema and signaling.
- Root docs: `README.md`, `CONFIGURATION.md`, `SUMMARY.md`, `ULTIMATE_GUIDE.md`, and setup notes for Nginx/Unifi.

## Prerequisites
- Windows development machine (for Fyne UI and Windows-specific agent features).
- Go 1.21+ installed and on PATH; MinGW/CGO toolchain required for some input/capture deps.
- Supabase project (URL + anon/service keys) and TURN credentials.
- Node/npm only if you need to preview the `docs/` static site locally.

## Environment & config
- Copy `.env.example` to `.env` at the repo root; fill `SUPABASE_URL`, `SUPABASE_ANON_KEY`, TURN credentials, and any service keys. Do not commit `.env`.
- Agent-specific env: `SUPABASE_URL`, `SUPABASE_ANON_KEY`, optional `DEVICE_NAME`.
- Controller uses the same Supabase values; see `CONFIGURATION.md` for details.
- Supabase migrations live in `supabase/migrations`; edge functions in `supabase/functions`.

## Build, run, test (per component)
- Formatting: run `gofmt -w` on changed Go files.
- Static checks: `go vet ./...` (from each module).
- Tests: `go test ./...` (from `agent` or `controller`; add tests near new code).
- Agent (dev): `cd agent && go run ./cmd/remote-agent`.
- Agent (build): `cd agent && go build -o remote-agent.exe ./cmd/remote-agent` or use `build.bat`.
- Controller (dev): `cd controller && go run main.go`.
- Controller (build): `cd controller && .\build.bat` (or `go build -ldflags "-s -w -H windowsgui" -o controller.exe`).
- Dashboard/web agent: static site in `docs/`; to preview locally you can serve the folder (e.g., `npx serve docs`) or open `index.html` directly. Production is via GitHub Pages.
- Services/startup: `agent/install-service.bat`, `agent/setup-startup.bat`, and related scripts; see `SERVICE_GUIDE.md`.
- Backend: run Supabase migrations/functions only when needed; coordinate before changing schema.

## CRITICAL: Build Command Rule (AI Agents MUST follow)
**For ANY long-running command (cross-compile builds take 60-120 seconds):**
1. Start command with `Blocking: false` and `WaitMsBeforeAsync: 1000`
2. Get the Background command ID from response
3. IMMEDIATELY call `command_status` with `WaitDurationSeconds: 60`
4. If still running, call `command_status` again with another 60s wait
5. Repeat until done or error

**Version files to update BEFORE building:**
- `agent/internal/tray/tray.go` - Version and BuildDate
- `controller/main.go` - Version and BuildDate

**Cross-compile commands (from Ubuntu to Windows):**
```bash
# Agent GUI
cd agent && GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ go build -ldflags '-s -w -H windowsgui' -o ../builds/remote-agent-vX.XX.X.exe ./cmd/remote-agent

# Agent Console
cd agent && GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ go build -ldflags '-s -w' -o ../builds/remote-agent-console-vX.XX.X.exe ./cmd/remote-agent

# Controller
cd controller && GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc go build -ldflags '-s -w -H windowsgui' -o ../builds/controller-vX.XX.X.exe .
```

## Current version
- **Agent:** v2.68.6 (`agent/internal/tray/tray.go`)
- **Controller:** v2.68.6 (`controller/main.go`)
- **Update server:** `https://updates.hawkeye123.dk/version.json`
- **Downloads:** `https://downloads.hawkeye123.dk/`

## Recent changes (v2.68.x)
- **v2.68.6:** Fix H264 mode dropping to idle tiles (2 FPS freeze on button clicks).
- **v2.68.5:** Start Menu + Desktop shortcuts for controller and agent install.
- **v2.68.4:** Fix taskkill killing own process during install/uninstall.
- **v2.68.3:** Add "Install as Program" to controller (Program Files, autostart, shortcuts).
- **v2.68.2:** Auto-stop tray before install/uninstall + dashboard black screen fix (0xFE chunk format).

## Deployment workflow
After building, deploy to the Ubuntu server:
```bash
# Copy to Caddy downloads
cp builds/remote-agent-vX.XX.X.exe ~/caddy/downloads/remote-agent.exe
cp builds/controller-vX.XX.X.exe ~/caddy/downloads/controller.exe

# Update version.json
cat > ~/caddy/downloads/version.json << 'EOF'
{
  "agent_version": "vX.XX.X",
  "controller_version": "vX.XX.X",
  "agent_url": "https://updates.hawkeye123.dk/remote-agent.exe",
  "controller_url": "https://updates.hawkeye123.dk/controller.exe"
}
EOF
```

## Key architecture notes
- **Agent streaming modes:** `ModeIdleTiles` (2 FPS), `ModeActiveTiles` (20 FPS JPEG), `ModeActiveH264` (25 FPS H.264 via RTP video track). Mode switching in `determineMode()` in `agent/internal/webrtc/peer.go`.
- **H.264 pipeline (agent):** OpenH264 encoder → RTP video track → WebRTC.
- **H.264 pipeline (controller):** RTP track → SampleBuilder → FFmpeg subprocess (DXVA2 hw accel) → NV12 → JPEG → Fyne canvas.
- **Chunked JPEG (dashboard):** Agent sends `0xFE` magic (5-byte header with frame ID) or `0xFF` magic (3-byte header). Dashboard `webrtc.js` handles both.
- **Controller install:** Copies to `C:\Program Files\RemoteDesktopController`, sets autostart via registry, creates Start Menu shortcut, optional Desktop shortcut.
- **Agent install:** Copies to `C:\Program Files\RemoteDesktopAgent`, sets autostart via registry, creates Start Menu shortcut.
- **Process management:** `stopRunningAgent()`/`stopRunningController()` use `tasklist` + `taskkill /PID` to kill other instances while preserving the current GUI process.

## Branches and releases
- Branches: `main` (stable), `agent` (agent work), `dashboard` (web UI), `controller` (controller app).
- Releases: GitHub Actions build artifacts on tag/branch (controller and agent). Tag and push per release notes; download binaries from Actions or Releases.

## Contribution guidelines
- Keep changes scoped; align with component directories.
- Run `gofmt`, `go vet`, and relevant `go test` before committing.
- Avoid committing secrets, `.env`, or generated binaries; keep release artifacts out of Git history.
- Update docs (README/CHANGELOG/feature-specific guides) when behavior or setup changes.
- Use clear, imperative commit messages; reference issues or plans when applicable.

## Debugging tips
- Agent: use `run-agent-once.bat` or service logs to inspect startup, registration, and WebRTC connection flow.
- Controller: run from source to see console output; check Supabase auth settings if login fails.
- Dashboard: check browser devtools console; Supabase realtime/signaling errors often surface there.
- H.264: Check agent logs for "H.264 encode fejl" or "Video track write fejl". Controller logs show "RTP packets received" and "H.264 decoded frame" counts.

## Security
- Treat Supabase keys and TURN credentials as secrets; use `.env` locally and GitHub Actions secrets in CI.
- Verify RLS/policies before deploying schema changes; avoid weakening access control without review.
