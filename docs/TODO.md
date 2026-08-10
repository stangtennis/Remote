# Remote Desktop — Tilbage-liste

Oversigt over tilbageværende arbejde efter gap-analysen (2026-08-07).
**Status:** ✅ = færdig · ⬜ = tilbage

---

## ✅ Færdig (16 fixes — denne session)

- ✅ XSS — ny `jsEsc()` til JS-strengkontekst (`controller/frontend/js/app.js`)
- ✅ Reconnection-leak — `Stop()`→`Cancel()` (`controller/internal/viewer/viewer.go`)
- ✅ IDOR `get_user_devices` — auth-guard (kun caller/admin)
- ✅ Anon support-signaling respekterer `is_public` (private sessions lukket)
- ✅ Storage SELECT brudt → rettet til `sessionId/`-konvention (download virker)
- ✅ native-host crash — comma-ok type guards (`native-host/main.go`)
- ✅ native-host install — manifest placeholder + generisk path-rewrite i scripts
- ✅ libjpeg-turbo panic → fallback til pure-Go encoder (`jpeg_std_fallback.go`)
- ✅ self-PID protect — `process kill` nægter agentens egen PID
- ✅ device_tags cross-tenant — scoping via `user_has_device_access`
- ✅ CSP på 5 sider (dashboard/admin/agent/support/history)
- ✅ webrtc_sessions cleanup koblet på pg_cron + `remote_sessions.token` UNIQUE
- ✅ `handleFileRequest` læser ikke længere hele filen i hukommelse (memory-DoS fjernet)
- ✅ `LICENSE` (MIT) tilføjet
- ✅ Falske H.265/VP9 codec-options fjernet + `electron-agent`-ref slettet
- ✅ `search_path` hærdning på `is_admin`/`is_super_admin`

Migration: `supabase/migrations/20260807_security_hardening.sql`

---

## 🔴 Tilbage — Høj prioritet

- ✅ **WebRTC input bounds** — muse-koord clamp (relativ + absolut til resolution) + scroll-cap ±1000 + pipe-path clamp (`input_handler.go`). Key-whitelist/terminal-cap er Medium-refinement.
- ⚠️ **force_update/shutdown/restart/lock** — **AFVIST med evidence:** en connected klient har allerede SYSTEM-shell via shell-channel, så disse kommandoer er ikke en eskalering; selve connection er RLS-gated (device-access). Agent-side rolle-tjek er ubrugeligt (agent kender ikke controller-rolle).
- ✅ **Support-PIN rate-limiting** — IP-baseret lockout: max 10 forsøg / 10 min → HTTP 429; ny `support_pin_attempts`-tabel (RLS, ingen klient-policies) + hourly cleanup-cron. Migration `20260808_support_pin_lockout.sql` + `support-signal/index.ts`
- ✅ **Logout** — invaliderer nu Supabase-session server-side (`POST /auth/v1/logout`) + stopper `autoRefreshToken`-goroutine via context (`controller/app.go`, `supabase/client.go`)
- 🟡 **Tests** — `filetransfer.sanitizePath` tilføjet ✅ (kører grønt på Linux); resten (`webrtc`, `auth`, `shell`, `terminal`, `credentials`, `supabase`) mangler stadig
- ✅ **CI test-gate** — `go vet`+`go test` fandtes allerede (`ci.yml`+`test.yml`); `govulncheck` (advisory) tilføjet

## ✅ Runde 2 (2026-08-07) — build-verificeret

- supabase `Client.SignOut()` + cancellable `autoRefreshToken` → **bygger + vetter rent på Linux** ✅
- `filetransfer.sanitizePath` tests → **bestået** (`go test ./internal/filetransfer/`) ✅
- `input_handler.go` clamping → gofmt-rent; kompileres i Windows-CI (Linux-byg blokeret af desktop/input platform-deps)

## ✅ Runde 5 — adversarial-review fund (GPT-5.6 via codex-ask.sh) rettet 2026-08-13

Uafhængig GPT-5.6 review (8-min kald via baggrund-wrapper) fandt 6 ægte residualhuller + 2 false positives. Alle ægte rettet + deployet + verificeret mod live DB:

- ✅ **ADV-01 (Critical)** `remote_sessions` INSERT kræver nu `user_has_device_access` (var: enhver bruger kunne oprette session på enhver device)
- ✅ **ADV-02 (Critical)** `get_user_devices` redakterer `api_key` (var: tildelte brugere kunne høste permanent device-credential)
- ✅ **ADV-06 (High)** `is_admin()` + `user_has_device_access()` tjekker nu `approved=true` (var: admin med approved=false forblev admin)
- ✅ **ADV-07 (High)** `device-register` validerer JWT via getUser (var: fake auth-header accepteret) → smoke-test: 401
- ✅ **ADV-08 (Medium)** `session-cleanup` kræver admin (var: enhver bruger kunne trigge global cleanup)
- ✅ **ADV-09 (Low)** `device_tags` trigger sætter `created_by=auth.uid()` (var: creator-spoof)
- ⚪ **ADV-03, ADV-05** — AFVIST: GPT brugte forældet kilde; live-DB har allerede guard/email-binding (verificeret)
- 🟡 **ADV-04** — DELVIS/by-design: private sessions beskyttes af `is_public=false`; public er åben pr. design
- Migration `20260813_adversary_fixes.sql` + edge functions `device-register`/`session-cleanup` deployet

## 🟡 Tilbage — Medium

- ✅ **Fyne-viewer dead code** — legacy `controller/internal/viewer/` slettet (aktiv UI er `frontend/js/viewer.js`); Fyne-dep. beholdes (`ui/`, `filebrowser/`, `filetransfer/browser.go` bruger det stadig)
- ✅ **OpenH264 checksum** — v2.1.1 SHA256 (win64/darwin/linux) pinnet i `openh264_download.go`; downloader afviser mismatch før biblioteket installeres
- ✅ **`webrtc_sessions.user_id`** — `text`→`uuid` (migration `20260810`; NULLIF-guard, transient tabel)
- ✅ **Linux secret store** — fil-baseret fallback (0600, sha256-keyed) så "remember me" virker på Linux (build+vet OK)
- ✅ **Audio-checkbox** — disabled + "ikke tilgængelig" (element bevares, undgår JS-break)
- ✅ **`send-welcome-email`** — HTML-escaper nu `email`/`tempPassword` (`escapeHtml`)
- ⬜ **`_ = err` discards** — `DataChannel.Send`, codec/interceptor-reg, OpenH264 options (bred, lav ROI)
- ✅ **innerHTML/console-log** — TURN/SDP/ICE dumps fjernet + devices.js emails/fejl escaped (turn-test candidate-strings er lavrisiko)
- ✅ **`session-cleanup-beacon`** — `verify_jwt=false` erklæret i `config.toml`
- ✅ **`accept_invitation` race** — `FOR UPDATE` + `accepted_at IS NULL` re-check (migration `20260809`)
- ✅ **`claim_device_connection`** — afleder `controller_id`/`kicked_by` fra `auth.uid()` (migration `20260809`)
- 🟡 **Web-agent reconnect** — grace-period (15s) på `disconnected` tilføjet; fuld re-negotiation udsat (dashboard-drevet)
- ✅ **Manglende indexes** — `device_transfers` (4), `user_invitations` (2), `support_sessions.pin` (migration `20260809`)

## 🟢 Tilbage — Low/docs

- ✅ **`config.toml`** — `verify_jwt` for alle funktioner + fjernet placeholder redirect-URL
- ✅ **README-MIGRATIONS** — fjernet "21 total"-påstand; henviser til mappen som source of truth (54 nu)
- ✅ **README "rate limiting"** — erstattet falsk "100 req/min" med sandt "Support PIN lockout"
- ⬜ **Hardcoded version** — `docs/js/auth.js:267` (v3.1.97 bagt ind)

---

## 🔧 Verifikationsgæld (før release)

- ✅ **Bygget v3.1.117** — Windows (controller+agent+console, turbo) + macOS (universal) + NSIS installers + SHA256 (`builds/`)
- ✅ **SQL-migrationer anvendt på live DB** — `20260807`→`20260812` (backup: `/home/dennis/backups/supabase/supabase_pre_deploy_20260807_211955.sql.gz`)
- ✅ **Edge functions deployet** — `support-signal` (PIN-lockout) + `send-welcome-email` (HTML-escape); edge-runtime genstartet; smoke-test OK
- ✅ **DB-verificeret:** get_user_devices IDOR-guard, session_signaling is_public, storage SELECT, device_tags scoped (legacy permissive fjernet), webrtc_sessions.user_id uuid, remote_sessions.token unique, support_pin_attempts table + lockout tæller
- ⬜ **Manuel test på Windows/macOS:** XSS, storage download, native-host install, libjpeg-fallback, logout, web-agent grace-period

---

## Note: OpenUltraCode completion-status

- gofmt: rent ✅
- Build v3.1.117: Windows + macOS ✅ (build fangede og rettede en float64-typefejl undervejs)
- SQL: **anvendt + verificeret mod live DB** ✅
- Edge functions: **deployet + smoke-testet** ✅
- Adversarial review: GPT-5.6 (codex) timed out → kørt som GLM-fallback
