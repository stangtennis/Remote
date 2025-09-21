# Remote Desktop Plan – Supabase + GitHub Pages

Oprettet: 2025-09-19 22:11 CEST

Formål: Bygge en fjernskrivebords‑løsning ala MeshCentral/TeamViewer, der udelukkende bruger Supabase (DB, Realtime, Storage, Edge Functions) og GitHub Pages (dashboard). Ingen egne servere. Stabil, sikker, lav latency og enkel installation (én EXE).

---

## 1) Mål & principper
- Enkel og professionel brugeroplevelse (én EXE til agenten, et web‑dashboard).
- Globalt virkende P2P forbindelse via WebRTC med TURN fallback.
- Supabase som eneste backend: database, realtime, lagring, edge functions og auth.
- GitHub Pages til dashboard (statisk hosting, høj stabilitet).
- Sikkerhed først: kortlivede session‑tokens, PIN, RLS, audit logging.
- Observability: heartbeats hver 30s, session metrics, tydelig logging.

## 2) Arkitektur – høj‑niveau
```mermaid
flowchart LR
  subgraph Browser Dashboard (GitHub Pages)
    A[dashboard.html/js]
  end

  subgraph Supabase Cloud
    B[(Postgres)]
    C[Supabase Realtime]
    D[Edge Functions]
    E[Storage]
    F[Auth/RLS]
  end

  subgraph Agent (Windows EXE)
    G[WebRTC Peer]
    H[Screen Capture]
    I[Input Control]
    J[File I/O]
  end

  A <---> C
  G <---> C
  A -.Auth/Keys.-> F
  A --> D
  G --> D
  D <--> B
  A -.download/upload.-> E
  A <-- WebRTC Media/Data --> G
```

## 3) Komponenter
### 3.1 Dashboard (GitHub Pages)
- Enhedsliste med online/offline (fra `remote_devices`).
- Start/stop session, PIN/token, WebRTC viewer, mus/tastatur input, filtransfer.
- Kalder Edge Function for at få session‑token og ICE/TURN servere.
- Aggressiv debug‑logging og UI “debug mode” for at undgå stille fejl.

### 3.2 Supabase
- Database tabeller: `remote_devices`, `remote_sessions`, `session_signaling`, `audit_logs`.
- Realtime kanaler per device og session til signalering/status.
- Edge Functions: `session-token` (udsteder kortlivede tokens og ICE) + `file-transfer` fallback.
- Storage buckets: `agents/` (EXE), evt. fallback frame chunks.
- Auth/RLS: offentligt læs for non‑sensitive felter; skriv via tokens/Edge Functions.

### 3.3 Agent (Windows EXE)
- Anbefalet sprog: Go + Pion WebRTC (single statisk binær, nem deployment).
- Funktioner: skærmoptagelse, input (mus/tastatur), filtransfer over data channel.
- Heartbeat til `remote_devices` hver 30s; hardware‑baseret `device_id`.
- Auto‑opdatering (manifest i Storage) og mulighed for signeret EXE.

## 4) Data‑ og API‑kontrakter
### 4.1 Realtime kanalnavne
- Device kanal: `device:{device_id}`
- Session kanal: `session:{session_id}`

### 4.2 Signaling payloads (gemmes i `session_signaling` og pushes via Realtime)
```json
// Client -> session_signaling (offer)
{
  "session_id": "uuid",
  "from": "dashboard",
  "type": "offer",
  "sdp": "<SDP>",
  "ts": "ISO-8601"
}

// Agent -> session_signaling (answer)
{
  "session_id": "uuid",
  "from": "agent",
  "type": "answer",
  "sdp": "<SDP>",
  "ts": "ISO-8601"
}

// ICE candidates (begge veje)
{
  "session_id": "uuid",
  "from": "dashboard|agent",
  "type": "ice",
  "candidate": { /* standard ICE candidate */ },
  "ts": "ISO-8601"
}
```

### 4.3 Input events (WebRTC data channel)
```json
{ "t": "mouse_move",  "x": 123, "y": 456 }
{ "t": "mouse_click", "button": "left", "down": true }
{ "t": "key", "code": "KeyA", "down": true }
{ "t": "clipboard_set", "text": "..." }
```

### 4.4 Midlertidig frame payload (fase 1 – billeder over data channel)
```json
{
  "t": "frame",
  "id": 1024,
  "w": 1920,
  "h": 1080,
  "fmt": "jpeg",
  "q": 80,
  "data": "<binary/base64>"
}
```

## 5) Database skema (SQL – udkast)
```sql
-- remote_devices
create table if not exists public.remote_devices (
  id bigint generated always as identity primary key,
  device_id text not null unique,
  device_name text,
  platform text,
  arch text,
  cpu_count int,
  ram_bytes bigint,
  is_online boolean default false,
  last_seen timestamptz default now()
);

-- remote_sessions
create table if not exists public.remote_sessions (
  id uuid primary key default gen_random_uuid(),
  device_id text not null,
  created_by text,
  status text check (status in ('pending','active','ended','expired')) default 'pending',
  pin text,
  token text,
  created_at timestamptz default now(),
  expires_at timestamptz
);

-- session_signaling
create table if not exists public.session_signaling (
  id bigint generated always as identity primary key,
  session_id uuid not null references public.remote_sessions(id) on delete cascade,
  from_side text check (from_side in ('dashboard','agent')) not null,
  msg_type text check (msg_type in ('offer','answer','ice')) not null,
  payload jsonb not null,
  created_at timestamptz default now()
);

-- audit_logs
create table if not exists public.audit_logs (
  id bigint generated always as identity primary key,
  session_id uuid,
  device_id text,
  actor text,
  event text,
  details jsonb,
  created_at timestamptz default now()
);
```

## 6) RLS & sikkerhed
- `remote_devices`: SELECT offentligt (begrænset felter). UPDATE `is_online`/`last_seen` via Edge Function eller signeret agent token.
- `remote_sessions`: RLS efter `created_by` og/eller match på `token`/`pin`.
- `session_signaling`: Kun deltagere med gyldig session‑token kan skrive/læse.
- `audit_logs`: Skriv via Edge Functions; læs af admin.
- Tokens: Edge Function udsteder kortlivede JWT’er (5–15 min). PIN ved behov.
- ICE/TURN: Edge Function returnerer servere og ephemeral credentials.

## 7) Signaling‑flow (sekvens)
1. Dashboard vælger enhed og starter session → kalder `session-token` Edge Function.
2. Edge Function opretter række i `remote_sessions`, genererer token og PIN (valgfrit) samt returnerer ICE/TURN liste.
3. Dashboard laver WebRTC offer og publicerer i `session_signaling` → Realtime push.
4. Agent læser offer, sætter remote desc, laver answer og publicerer tilbage.
5. ICE candidates udveksles begge veje via `session_signaling` + Realtime.
6. WebRTC etableres → media/data P2P (eller via TURN).

## 8) Fallbacks
- Uden TURN: forsøg STUN‑only; hvis fejler, midlertidig fallback (lav FPS, JPEG via Realtime/Storage) kun til nødhjælp.
- Filtransfer fallback: eksisterende `file-transfer` Edge Function (chunked) + Storage.

## 9) Agent (Windows) – detaljer
- Sprog: Go (Pion WebRTC) for stabil single‑binary.
- Moduler:
  - WebRTC: `github.com/pion/webrtc/v3`
  - Skærm: `github.com/kbinani/screenshot` (fase 1), DXGI duplication i fase 2.
  - Input: `github.com/go-vgo/robotgo` eller Windows `SendInput` wrapper.
  - Encoder: fase 1 – JPEG/WebP; fase 2 – VP8/VP9/H.264 via GStreamer/FFmpeg.
  - Service: user‑mode eller Windows service (`golang.org/x/sys/windows/svc`).
- Auto‑opdatering via manifest i Storage; EXE‑signering anbefales (Authenticode).
- Heartbeats hver 30s; hardware‑baseret `device_id` for unik identitet.

## 10) Dashboard – detaljer
- Struktur:
  - `public/dashboard.html`
  - `public/js/app.js` (Supabase client, UI state, logs)
  - `public/js/webrtc.js` (RTC, tracks, data channels)
  - `public/js/signaling.js` (Realtime + Edge Functions)
  - `public/css/styles.css`
- Funktioner:
  - Enhedsliste med filtrering.
  - Start/Stop session, PIN input.
  - Viewer med skalering, kvalitet/FPS kontrol.
  - Input capture & mapping til agentens opløsning.
  - Filtransfer UI (data channel/fallback).
  - Debug overlay for ICE state/bitrate/RTT og fejl.

## 11) Observability & drift
- Heartbeats: opdater `is_online` + `last_seen` hver 30s.
- Audit logs: session tilstande og væsentlige hændelser.
- Alerts (valgfrit): Edge Function kan trigge webhooks.
- “Debug Mode” i dashboard: viser signalkøer, ICE state, bitrate, RTT.

## 12) Milepæle og leveranceplan
- Fase 0 – Infrastruktur (1–2 dage)
  - Supabase tabeller + RLS
  - Edge Functions skeletons (`session-token`, evt. `file-transfer`)
  - Storage buckets/policies
- Fase 1 – Dashboard skeleton (1–2 dage)
  - Enhedsliste + minimal signaling (mock agent)
  - Testharness og logging
- Fase 2 – Agent MVP (3–5 dage)
  - Pion WebRTC, heartbeat, registrering, answer/ICE
  - Skærm som JPEG over data channel (enkelt)
  - Basis input (mus/tastatur)
- Fase 3 – TURN + stabilisering (1–2 dage)
  - Xirsys/Twilio TURN integration
  - Robust ICE state‑handling og reconnect
- Fase 4 – Video track (4–7 dage)
  - GStreamer/FFmpeg → VP8/VP9/H.264
  - FPS/bitrate kontrol i UI
- Fase 5 – Filtransfer + fallback (1–2 dage)
  - Data channel filtransfer + resume
  - Edge Function/Storage fallback
- Fase 6 – Sikkerhed & produktion (2–3 dage)
  - PIN, tokens, RBAC/RLS
  - Code signing, auto‑update, installer/service
  - Testmatrix

## 13) Acceptkriterier
- Enhedsliste viser korrekt online status i realtime.
- Klik “Connect” → WebRTC link etableret < 5 sek. i normale net.
- Skærmstream 20–30 FPS (fase 4), 1080p, justerbar kvalitet.
- Input latency <150 ms (LAN), <300 ms (internet/TURN).
- Filtransfer via data channel; fallback virker.
- Ingen duplikater i devices (hardware‑baseret `device_id`).
- Sikker sessionstyring med PIN/token, udløb og audit logs.

## 14) Risici & mitigering
- Symmetrisk NAT uden TURN → start med TURN integration.
- Codec/browsere → begynd med data channel billeder; senere VP8 (bred support).
- UAC/privilegier for input → dokumentér og tilbyd service‑mode.
- SmartScreen → Authenticode signering og evt. MSI‑installer.

## 15) Næste skridt (afklaringer)
1) Agent sprogvalg: Go + Pion (anbefalet) eller Node.js?
2) TURN leverandør: Xirsys eller Twilio?
3) Godkend denne plan og opret opgaver til Fase 0.
4) Bekræft Supabase projekt/org til schema migrering.

## 16) TODO (oversigt)
- [ ] Opsæt Supabase: tabeller, RLS, Storage buckets
- [ ] Signalering via Realtime + Edge Functions (SDP/ICE)
- [ ] TURN integration (Xirsys/Twilio)
- [ ] Windows‑agent (Go + Pion): skærm, input, filer
- [ ] Pakning, auto‑opdatering, signering, upload til Storage
- [ ] Dashboard (GitHub Pages): WebRTC UI + sessioner
- [ ] Filtransfer (data channel) + Edge Function fallback
- [ ] Sikkerhed: Auth/RBAC, PIN, ephemeral tokens, audit
- [ ] Monitorering: heartbeats 30s, alarmer, logs
- [ ] Testplan: NAT matrix, browsere, performance/latency
- [ ] Udrulning & dokumentation
