# Build & Deploy Guard - Memory

## Current State
- **Version:** v2.99.30 (deployed 2026-03-23)
- **Changes:** In-session chat, dashboard forbedringer (badge farver, inline stats, online/offline gruppering), H.264 auto-enable
- **Agent SHA256:** 32a154066002b5adec83cd873367472a504ee4941225ae6136da9e908b5bc605

## Key Facts
- Version injection via `-ldflags -X` in `build-local.sh` (NOT in source code)
- Build script: `./build-local.sh vX.XX.X` builds all 3 exe files + beregner SHA256 hash
- Info repo local path: `/home/dennis/projekter/info`
- Info repo docs: `Remote-Desktop/` directory (README, ARCHITECTURE, BUILD, SETUP, CREDENTIALS, etc.)
- Always update info repo after any release

## Version History (recent)
- v2.91.0 - Fix sort skaerm paa macOS (WiFi): reliable channel til chunked frames, JPEG/0xFF kollisions-fix
- v2.90.0 - Moderniseret Controller GUI: dark theme, cyan accenter, card layouts, SVG ikoner, ny internal/ui package
- v2.89.0 - Hybrid AltGr, UIPI bypass, Dashboard AltGr, Cloudflare TURN, Session 0 fixes
- v2.88.0 - Connection Health Watchdog: token refresh, health-aware heartbeat, eksponentiel backoff, stale session cleanup
- v2.87.8 - Session 0 stabilisering: MOUSEEVENTF_ABSOLUTE, dashboard reconnect, auto-update startup
- v2.86.1 - SHA256-verifikation, oprydning af downloads, AV-retry, korrupt state-fil fix
- v2.86.0 - Auto-update i Windows Service mode via SCM restart
- v2.74.3 - Auto-update cooldown 1t -> 5min
- v2.74.2 - Controller admins ser alle devices
- v2.74.1 - Fix auto-update JSON-format (version felt)
- v2.74.0 - Auto-TURN credentials fra Edge Function
- v2.73.5 - Session 0 pipe capturer, SYSTEM token fallback
- v2.73.0 - Fix service mode crashes, panic recovery
- v2.72.0 - Agent switched to authenticated JWT, RLS owner-scoped policies

## Important: version.json format changed AGAIN in v2.86.1
- v2.74.0 format: `agent_version`, `controller_version`, `agent_url`
- v2.74.1 format: `version`, `download_url`, `controller_url`
- v2.86.1 format: `agent_version`, `controller_version`, `agent_url`, `controller_url`, `agent_sha256`

## Info Repo Files Updated per Release
- `README.md` - Version numbers, recent updates list, feature checklist
- `ARCHITECTURE.md` - Technical details, security model, schema
- `BUILD.md` - Version history, build instructions, version.json examples
