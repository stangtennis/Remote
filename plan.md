# Remote Desktop Plan – Supabase + GitHub Pages

Oprettet: 2025-09-19 22:11 CEST

Formål: Bygge en fjernskrivebords‑løsning ala MeshCentral/TeamViewer, der udelukkende bruger Supabase (DB, Realtime, Storage, Edge Functions) og GitHub Pages (dashboard). Ingen egne servere. Stabil, sikker, lav latency og enkel installation (én EXE).

---

## 1) Mål & principper
- Enkel og professionel brugeroplevelse (én EXE til agenten, et web‑dashboard).
- Globalt virkende P2P forbindelse via WebRTC med TURN fallback.
- Supabase som eneste backend: database, realtime, lagring, edge functions og auth.
- GitHub Pages til dashboard (statisk hosting, høj stabilitet).
- Sikkerhed først: kortlivede session‑tokens, PIN, RLS, audit logging, MFA.
- Observability: Realtime presence tracking, session metrics, struktureret logging (JSON).
- Versioning: API kontrakter versioneres fra dag 1.
- Scope: 1:1 forbindelser (ikke multi-viewer i MVP).
- Platform: Windows-first (Go + Pion), mulighed for cross-platform senere.

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
- Database tabeller: `remote_devices`, `remote_sessions`, `session_signaling`, `audit_logs`, `device_approvals`.
- Realtime kanaler per device og session til signalering/status + presence tracking.
- Edge Functions: `session-token` (udsteder kortlivede tokens og ICE) + `file-transfer` fallback + `device-register` (approval flow).
- Storage buckets: `agents/` (signed EXE med manifest), evt. fallback frame chunks.
- Auth/RLS: Supabase Auth for dashboard brugere (email/password + MFA), device API keys, granulære RLS policies.
- Cleanup: Auto-slet gamle signaling messages (24h TTL) og expired sessions.

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
  last_seen timestamptz default now(),
  api_key text unique, -- for device authentication
  approved_by uuid references auth.users(id),
  approved_at timestamptz,
  owner_id uuid references auth.users(id), -- device ejer
  created_at timestamptz default now()
);

-- Performance indexes
CREATE INDEX idx_remote_devices_online ON public.remote_devices(is_online, last_seen);
CREATE INDEX idx_remote_devices_owner ON public.remote_devices(owner_id) WHERE owner_id IS NOT NULL;

-- remote_sessions
create table if not exists public.remote_sessions (
  id uuid primary key default gen_random_uuid(),
  device_id text not null references public.remote_devices(device_id) on delete cascade,
  created_by uuid references auth.users(id),
  status text check (status in ('pending','active','ended','expired')) default 'pending',
  pin text,
  token text,
  created_at timestamptz default now(),
  expires_at timestamptz,
  ended_at timestamptz,
  metrics jsonb default '{}'::jsonb, -- {"bitrate": 2500, "fps": 30, "rtt": 45, "packet_loss": 0.1, "connection_type": "P2P"}
  version text default 'v1' -- API version
);

CREATE INDEX idx_sessions_status ON public.remote_sessions(status, expires_at);
CREATE INDEX idx_sessions_device ON public.remote_sessions(device_id, created_at);
CREATE INDEX idx_sessions_user ON public.remote_sessions(created_by) WHERE created_by IS NOT NULL;

-- session_signaling
create table if not exists public.session_signaling (
  id bigint generated always as identity primary key,
  session_id uuid not null references public.remote_sessions(id) on delete cascade,
  from_side text check (from_side in ('dashboard','agent')) not null,
  msg_type text check (msg_type in ('offer','answer','ice')) not null,
  payload jsonb not null,
  created_at timestamptz default now()
);

CREATE INDEX idx_signaling_session ON public.session_signaling(session_id, created_at);

-- Auto-cleanup old signaling (24h TTL)
CREATE OR REPLACE FUNCTION cleanup_old_signaling()
RETURNS void AS $$
BEGIN
  DELETE FROM public.session_signaling WHERE created_at < now() - interval '24 hours';
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- audit_logs
create table if not exists public.audit_logs (
  id bigint generated always as identity primary key,
  session_id uuid,
  device_id text,
  actor uuid references auth.users(id),
  event text not null, -- error_code taxonomy: AUTH_FAIL, SESSION_START, etc.
  details jsonb,
  severity text check (severity in ('info','warning','error')) default 'info',
  created_at timestamptz default now()
);

CREATE INDEX idx_audit_session ON public.audit_logs(session_id) WHERE session_id IS NOT NULL;
CREATE INDEX idx_audit_device ON public.audit_logs(device_id) WHERE device_id IS NOT NULL;
CREATE INDEX idx_audit_time ON public.audit_logs(created_at);

-- Auto-expire sessions trigger
CREATE OR REPLACE FUNCTION expire_sessions()
RETURNS trigger AS $$
BEGIN
  UPDATE public.remote_sessions 
  SET status = 'expired', ended_at = now()
  WHERE expires_at < now() AND status NOT IN ('ended', 'expired');
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_expire_sessions
  AFTER INSERT OR UPDATE ON public.remote_sessions
  FOR EACH STATEMENT
  EXECUTE FUNCTION expire_sessions();
```

## 6) RLS & sikkerhed

### 6.1 Authentication
- **Dashboard brugere**: Supabase Auth (email/password + MFA anbefalet)
- **Devices**: API key genereret ved approval, roteres regelmæssigt
- **Sessions**: Kortlivede JWT tokens (5-15 min) udstedt af Edge Function

### 6.2 RLS Policies (eksempler)
```sql
-- remote_devices: Users can only see their own devices
CREATE POLICY "Users can view own devices" ON public.remote_devices
  FOR SELECT USING (owner_id = auth.uid());

CREATE POLICY "Devices can update own status" ON public.remote_devices
  FOR UPDATE USING (api_key = current_setting('request.headers')::json->>'x-device-key');

-- remote_sessions: Users can only manage their own sessions
CREATE POLICY "Users can view own sessions" ON public.remote_sessions
  FOR SELECT USING (created_by = auth.uid());

CREATE POLICY "Users can create sessions" ON public.remote_sessions
  FOR INSERT WITH CHECK (created_by = auth.uid());

-- session_signaling: Only session participants
CREATE POLICY "Session participants can signal" ON public.session_signaling
  FOR ALL USING (
    EXISTS (
      SELECT 1 FROM public.remote_sessions 
      WHERE id = session_id 
      AND (created_by = auth.uid() OR token = current_setting('request.jwt.claims')::json->>'session_token')
    )
  );

-- audit_logs: Read by admins only
CREATE POLICY "Admins read audit logs" ON public.audit_logs
  FOR SELECT USING (auth.jwt()->>'role' = 'admin');
```

### 6.3 Security measures
- Edge Function rate limiting: 100 requests/min per user/device
- PIN: 6-cifret, 3 forsøg før lock (5 min)
- Token rotation: Sessions auto-expire efter 15 min inaktivitet
- ICE/TURN: Ephemeral credentials fra Edge Function
- Code signing: Authenticode for Windows EXE (MANDATORY)
- HTTPS only: GitHub Pages enforces HTTPS

## 7) Signaling‑flow (sekvens)
1. **User authentication**: Dashboard bruger logger ind via Supabase Auth (email/password + MFA)
2. **Device registration**: Agent registrerer med hardware ID → venter på approval via dashboard
3. **Start session**: Dashboard vælger enhed og starter session → kalder `session-token` Edge Function
4. **Token issuance**: Edge Function opretter række i `remote_sessions`, genererer token og PIN samt returnerer ICE/TURN liste med ephemeral credentials
5. **WebRTC offer**: Dashboard laver offer og publicerer i `session_signaling` → Realtime push til agent
6. **WebRTC answer**: Agent læser offer, sætter remote desc, laver answer og publicerer tilbage
7. **ICE exchange**: Candidates udveksles begge veje via `session_signaling` + Realtime
8. **Connection established**: WebRTC etableres → media/data P2P (eller via TURN)
9. **Reconnection**: Ved disconnection, ICE restart uden ny session (genbruger token hvis ikke expired)
10. **Session end**: User eller agent sender end signal → status opdateres, metrics gemmes, audit log

## 8) Fallbacks & Reconnection

### 8.1 Connection fallbacks
- **Primary**: P2P via STUN
- **Secondary**: TURN relay (Xirsys/Twilio)
- **Emergency**: Low FPS JPEG via Realtime/Storage (kun diagnostik)

### 8.2 Reconnection strategy
- **Network change**: ICE restart (genbruger PeerConnection)
- **Brief disconnect (<30s)**: Auto-reconnect med exponential backoff
- **Long disconnect (>30s)**: Ny session nødvendig
- **Token expired**: Dashboard henter nyt token via Edge Function

### 8.3 Filtransfer fallback
- **Primary**: Data channel (chunked, resumable)
- **Fallback**: `file-transfer` Edge Function + Storage (chunked upload/download)

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

### 11.1 Presence tracking
- **Realtime presence**: Supabase Realtime presence for automatic online/offline (no manual heartbeats)
- **Fallback**: Hvis presence ikke virker, 30s heartbeats til `last_seen`

### 11.2 Metrics collection
```json
// Gemmes i remote_sessions.metrics
{
  "connection_established_ms": 3200,
  "connection_type": "P2P",
  "avg_bitrate_kbps": 2500,
  "avg_fps": 28,
  "avg_rtt_ms": 45,
  "packet_loss_pct": 0.2,
  "total_bytes_sent": 52428800,
  "reconnections": 1,
  "ice_failures": 0
}
```

### 11.3 Structured logging
- **Format**: JSON med timestamp, level, component, message, context
- **Levels**: DEBUG, INFO, WARN, ERROR
- **Components**: dashboard, agent, edge-function, database

### 11.4 Error taxonomy
```
AUTH_FAIL: Authentication fejl
DEVICE_OFFLINE: Enhed ikke tilgængelig
WEBRTC_FAIL: WebRTC connection fejl
ICE_TIMEOUT: ICE candidates timeout
TURN_FAIL: TURN relay fejl
SESSION_EXPIRED: Session udløbet
RATE_LIMIT: Rate limit overskredet
```

### 11.5 Monitoring
- **Dashboard metrics**: Connection success rate (target >95%), latency P50/P99
- **Alerts**: Edge Function webhooks til critical errors
- **Debug mode**: UI overlay med ICE state, bitrate, RTT, packet loss, signalkøer

## 12) Milepæle og leveranceplan

### Fase 0 – Infrastruktur (1–2 dage)
- Supabase tabeller + indexes + RLS policies + triggers
- Edge Functions skeletons (`session-token`, `file-transfer`, `device-register`)
- Storage buckets/policies
- `.env.example` med alle nødvendige keys

### Fase 0.5 – Authentication & Authorization (1–2 dage) **NY**
- Supabase Auth setup (email/password + MFA)
- Device registration + approval flow
- API key generation for agents
- RLS policies implementation og test
- Rate limiting i Edge Functions

### Fase 1 – Dashboard skeleton (1–2 dage)
- User login/logout flow
- Enhedsliste + device approval UI
- Minimal signaling (mock agent)
- Struktureret logging (JSON)
- Testharness

### Fase 2 – Agent MVP (3–5 dage)
- Pion WebRTC, device registration
- Realtime presence tracking
- Skærm som JPEG over data channel
- Basis input (mus/tastatur)
- Metrics collection

### Fase 3 – TURN + reconnection (2–3 dage)
- Xirsys/Twilio TURN integration med ephemeral credentials
- ICE restart logic
- Reconnection med exponential backoff
- Connection quality monitoring

### Fase 4 – Video track (4–7 dage)
- GStreamer/FFmpeg → VP8/VP9/H.264
- FPS/bitrate kontrol i UI
- Adaptive bitrate

### Fase 5 – Filtransfer + fallback (1–2 dage)
- Data channel filtransfer (chunked + resumable)
- Edge Function/Storage fallback

### Fase 6 – Sikkerhed & produktion (2–3 dage)
- PIN implementation (6 cifre, 3 forsøg)
- Token rotation
- Authenticode code signing (MANDATORY)
- Auto-update med signature verification
- Windows service mode

### Fase 7 – Production hardening (2–3 dage) **NY**
- E2E tests med Playwright
- Load testing (target: 100 concurrent sessions)
- Security audit
- Penetration testing
- Cost analysis og optimization
- Documentation finalisering
- Incident runbook

## 13) Acceptkriterier
- Enhedsliste viser korrekt online status i realtime.
- Klik “Connect” → WebRTC link etableret < 5 sek. i normale net.
- Skærmstream 20–30 FPS (fase 4), 1080p, justerbar kvalitet.
- Input latency <150 ms (LAN), <300 ms (internet/TURN).
- Filtransfer via data channel; fallback virker.
- Ingen duplikater i devices (hardware‑baseret `device_id`).
- Sikker sessionstyring med PIN/token, udløb og audit logs.

## 14) Risici & mitigering

| Risiko | Impact | Sandsynlighed | Mitigering |
|--------|--------|---------------|------------|
| Symmetrisk NAT uden TURN | Høj | Medium | TURN integration fra fase 3, test NAT matrix |
| Codec browser compatibility | Medium | Lav | Start med VP8 (bred support), fallback til JPEG |
| UAC/privilegier for input | Medium | Høj | Service mode, clear documentation, elevation prompt |
| SmartScreen blocking | Høj | Høj | **MANDATORY** Authenticode signering, MSI installer |
| Supabase limits (free tier) | Høj | Medium | Monitor usage, plan upgrade path, rate limiting |
| TURN bandwidth costs | Medium | Medium | P2P prioritering, usage limits, cost alerts |
| Data breach/security | Kritisk | Lav | MFA, API keys, RLS, audit logs, penetration test |
| Device approval abuse | Medium | Medium | Rate limiting, email verification, abuse detection |
| Session hijacking | Høj | Lav | Kortlivede tokens (5-15 min), PIN, HTTPS only |
| Reconnection loops | Lav | Medium | Exponential backoff, max retry limit |
| GDPR compliance | Høj | Medium | No session recording, data retention policy, user deletion |

## 15) Cost estimation (monthly)

### Supabase (100 devices, 50 concurrent sessions/day)
- **Free tier**: 500MB DB, 2GB bandwidth, 2GB storage
  - Likely exceeded → Pro tier needed
- **Pro tier ($25/mo)**: 8GB DB, 250GB bandwidth, 100GB storage
  - Sufficient for MVP

### TURN bandwidth (Xirsys/Twilio)
- **Usage**: ~50 sessions/day × 10 min avg × 2.5 Mbps = ~9.4 GB/day = ~280 GB/mo
- **Xirsys**: ~$0.50/GB = ~$140/mo
- **Twilio**: ~$0.40/GB = ~$112/mo
- **Mitigering**: Prioriter P2P (gratis), kun TURN når nødvendig

### GitHub Pages
- **Free**: Unlimited for public repos

### Code signing certificate
- **DigiCert/Sectigo**: ~$200-500/year

**Total MVP cost**: ~$150-200/mo + $200-500/year (cert)

---

## 16) Status & Næste skridt

### ✅ WORKING - Confirmed 2025-10-02
- **Remote screen streaming**: Working across networks
- **External access**: Confirmed working from outside local network
- **TURN relay**: Twilio TURN servers working properly
- **WebRTC P2P**: Working on same network
- **Agent**: Go + Pion implementation stable
- **Dashboard**: GitHub Pages deployment functional
- **Authentication**: Supabase Auth with email/password
- **Device registration**: Hardware-based device_id working

### ⚠️ Known Issues
- **Multiple dashboard tabs**: Causes signaling conflicts - only use one tab
- **Session cleanup**: Now automatic via pg_cron (runs every 5 minutes)
- **Mouse/keyboard**: Re-enabled (requires CGO build with MinGW-w64)
- **Video encoding**: Currently JPEG frames (~10 FPS) - H.264/VP8 optimization guide available

### 🔧 Næste skridt (afklaringer)
1. ✅ Agent sprogvalg: **Go + Pion** (bekræftet)
2. ✅ TURN leverandør: **Twilio** (bekræftet og virker)
3. ✅ Re-enable mouse/keyboard control (robotgo + CGO) - **DONE**
4. ✅ Implement automatic session cleanup (pg_cron scheduled function) - **DONE**
5. ⏳ Multi-tenancy: Personlig brug eller multi-org SaaS?
6. ⏳ Optimize video encoding (H.264/VP8 instead of JPEG) - See OPTIMIZATION.md
7. ⏳ Køb code signing certificate (Sectigo anbefales) - Required for production
8. ⏳ Deploy migrations and Edge Functions to Supabase
9. ⏳ Test full build with CGO enabled (MinGW-w64 required)

## 17) TODO (oversigt)
- [x] **Fase 0**: Opsæt Supabase tabeller + indexes + RLS + triggers + Edge Functions + Storage
- [x] **Fase 0.5**: Supabase Auth + device approval flow + API keys + rate limiting
- [x] **Fase 1**: Dashboard auth flow + enhedsliste + device approval UI + mock signaling
- [x] **Fase 2**: Agent MVP (Go + Pion) + device registration + JPEG screen + input + metrics
- [x] **Fase 3**: TURN integration (Twilio) + ICE restart + reconnection logic ✅ **WORKING**
- [ ] **Fase 4**: Video track (VP8/H.264) + adaptive bitrate + FPS control (currently using JPEG frames)
- [ ] **Fase 5**: Filtransfer (data channel + Edge Function fallback)
- [ ] **Fase 6**: PIN + token rotation + code signing + auto-update + service mode
- [ ] **Fase 7**: E2E tests + load testing + security audit + documentation + runbook

---

## 18) Architecture Decision Records (ADRs)

### ADR-001: Go + Pion for Agent
**Beslutning**: Go med Pion WebRTC bibliotek
**Rationale**: Single binary deployment, excellent WebRTC support, cross-platform potential
**Alternativer**: Node.js (larger runtime), Rust (steeper learning curve)

### ADR-002: GitHub Pages for Dashboard
**Beslutning**: Statisk hosting via GitHub Pages
**Rationale**: Zero cost, HTTPS by default, CDN, high availability
**Alternativer**: Vercel, Netlify (overkill for statisk site)

### ADR-003: Supabase Realtime Presence
**Beslutning**: Realtime presence tracking over manual heartbeats
**Rationale**: Automatic online/offline, less DB load, simpler implementation
**Fallback**: Manual heartbeats hvis presence fejler

### ADR-004: 1:1 Sessions Only (MVP)
**Beslutning**: Single viewer per agent session
**Rationale**: Simplified WebRTC architecture, sufficient for MVP
**Future**: SFU for multi-viewer hvis nødvendig

### ADR-005: Mandatory Code Signing
**Beslutning**: Authenticode signering required (ikke optional)
**Rationale**: SmartScreen bypass critical for adoption
**Cost**: $200-500/year (acceptable)

### ADR-006: Session Cleanup Strategy
**Beslutning**: Manual cleanup of stale sessions during debugging; automatic cleanup via scheduled function for production
**Rationale**: Stale pending sessions caused signaling deadlocks. Agent kept reusing old session IDs
**Implementation**: 
- Delete sessions older than 1 minute from `session_signaling`
- Delete `pending` or `active` sessions older than 15 minutes from `remote_sessions`
- Schedule cleanup function to run every 5 minutes
**Lessons learned**: Always clean up state between WebRTC connection attempts

### ADR-007: Single Dashboard Tab Policy
**Beslutning**: Enforce single dashboard tab per user via detection and warning
**Rationale**: Multiple tabs subscribing to same session creates ICE candidate duplication and conflicts
**Implementation**: Use SharedWorker or BroadcastChannel to detect multiple tabs; show warning in UI
**Alternative**: Use tab-specific session IDs (more complex, defer to future)
