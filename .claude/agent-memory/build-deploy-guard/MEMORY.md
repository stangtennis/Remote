# Build & Deploy Guard - Memory

## Current State
- **Version:** v2.72.0 (deployed 2026-02-17)
- **Changes:** Authenticated JWT tokens for agent, RLS tightened

## Key Facts
- Version injection via `-ldflags -X` in `build-local.sh` (NOT in source code)
- Build script: `./build-local.sh vX.XX.X` builds all 3 exe files
- Info repo local path: `/home/dennis/projekter/info`
- Info repo docs: `Remote-Desktop/` directory (README, ARCHITECTURE, BUILD, SETUP, CREDENTIALS, etc.)
- Always update info repo after any release

## Version History (recent)
- v2.72.0 - Agent switched to authenticated JWT, RLS owner-scoped policies
- v2.71.0 - Path traversal protection, input validation
- v2.70.0 - Race condition fixes in WebRTC Manager
- v2.69.0 - Quick Support, auto-reconnect, multi-monitor

## Info Repo Files Updated per Release
- `README.md` - Version numbers, recent updates list, feature checklist
- `ARCHITECTURE.md` - Technical details, security model, schema
- `BUILD.md` - Version history, build instructions, version.json examples
