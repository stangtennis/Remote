# Build & Deploy Guard - Memory

## Current State
- **Version:** v2.74.3 (deployed 2026-02-20)
- **Changes:** Auto-TURN, fix auto-update JSON, admin device view, update cooldown 5min

## Key Facts
- Version injection via `-ldflags -X` in `build-local.sh` (NOT in source code)
- Build script: `./build-local.sh vX.XX.X` builds all 3 exe files
- Info repo local path: `/home/dennis/projekter/info`
- Info repo docs: `Remote-Desktop/` directory (README, ARCHITECTURE, BUILD, SETUP, CREDENTIALS, etc.)
- Always update info repo after any release

## Version History (recent)
- v2.74.3 - Auto-update cooldown 1t -> 5min
- v2.74.2 - Controller admins ser alle devices
- v2.74.1 - Fix auto-update JSON-format (version felt)
- v2.74.0 - Auto-TURN credentials fra Edge Function
- v2.73.5 - Session 0 pipe capturer, SYSTEM token fallback
- v2.73.0 - Fix service mode crashes, panic recovery
- v2.72.0 - Agent switched to authenticated JWT, RLS owner-scoped policies

## Important: version.json format changed in v2.74.1
- Old format: `agent_version`, `controller_version`, `agent_url`
- New format: `version`, `download_url`, `controller_url`

## Info Repo Files Updated per Release
- `README.md` - Version numbers, recent updates list, feature checklist
- `ARCHITECTURE.md` - Technical details, security model, schema
- `BUILD.md` - Version history, build instructions, version.json examples
