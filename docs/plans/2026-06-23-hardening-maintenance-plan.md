# Remote Desktop hardening og vedligeholdelsesplan

Dato: 2026-06-23

Formål: samle de observerede problemer i en konkret afkrydsningsplan, så arbejdet kan tages i den rigtige rækkefølge uden at blande sikkerhed, H.264-optimering, repo-oprydning og arkitektur sammen.

## Statusnøgle

- [ ] Ikke startet
- [~] I gang
- [x] Færdig
- [!] Blokeret eller kræver beslutning

## P0 - Sikkerhed

### Plaintext password-lagring

- [x] Kortlæg nuværende brug af `controller/internal/credentials/credentials.go`.
- [x] Bekræft hvilke passwords der gemmes i klartekst:
  - [x] Controller-login.
  - [x] Windows/RDP-login pr. device.
- [x] Beslut målplatforme for sikker lagring:
  - [x] Windows DPAPI.
  - [x] macOS Keychain.
  - [x] Linux fallback: ingen plaintext fallback; "husk password" fejler tydeligt hvis controlleren køres på Linux.
- [x] Lav migration fra eksisterende JSON-fil:
  - [x] Læs gammel klartekst én gang.
  - [x] Skriv til OS keyring.
  - [x] Fjern eller nulstil passwordfelter i JSON.
  - [x] Bevar ikke-sensitive metadata lokalt.
- [x] Sørg for at nye gemte Windows-login aldrig skrives i klartekst.
- [x] Tilføj fejlbesked i UI hvis keyring ikke er tilgængelig.
- [x] Cross-compile test credentials-pakken til Windows og macOS.
- [x] Cross-compile hele controller-modulet som Windows-target.
- [ ] Test på Windows-controller.
- [ ] Test macOS, hvis macOS-controller stadig skal understøttes.

### Secrets-audit

- [x] Tjek om `.env` er tracked.
- [x] Tjek om `LOCAL_SECRETS.env` er tracked.
- [x] Tjek om `ULTIMATE_GUIDE.md` er tracked.
- [x] Scan tracked filer for oplagte secrets:
  - [x] Supabase service keys.
  - [x] TURN static secrets.
  - [x] Passwords.
  - [x] Private tokens.
- [x] Fjern hardcoded TURN static secret fra aktiv agent/dashboard-kode.
- [x] Fjern hardcodede service-role/demo JWT fra test/docs hvor de ikke behøver ligge.
- [x] Fjern tracked `.playwright-mcp` snapshots med gamle env/secrets-uddrag og gitignore mappen.
- [x] Bekræft at konkret scan for gammel TURN-secret og service-role JWT i aktive tracked filer er ren.
- [!] Hvis `HawkeyeTurnSecret2026x` har været brugt i produktion, skal den TURN secret roteres, fordi den ligger i git-historikken.
- [!] Vurder senere om git-historik skal renses for gamle snapshots/secrets.

## P1 - H.264/video pipeline

### Afklar controller-arkitektur

- [x] Beslut om den native Fyne-controller stadig skal vedligeholdes.
- [x] Beslut om Wails/web-controlleren er den primære controller fremover.
- [x] Dokumenter beslutningen i `README.md` eller en controller-arkitektur-note.

Hvis Fyne-controlleren ikke længere er primær:

- [x] Marker Fyne H.264-path som legacy.
- [x] Undgå større H.264-refactor i Fyne-koden.
- [x] Fjern kun åbenlyst død kode og farlige bugs.

Hvis Fyne-controlleren fortsat skal vedligeholdes:

- [!] Erstat H.264 -> MJPEG -> Fyne canvas roundtrip med direkte decode til RGBA/NV12. Udsat fordi Fyne-vieweren er legacy.
- [x] Fjern `nv12ToJPEG()` hvis den ikke længere bruges.
- [x] Fjern `parseResolution()` hvis den kun bruges af den døde NV12-vej.

### NVENC/FFmpeg encoder

- [x] Gennemgå `agent/internal/video/encoder/encoder.go`.
- [x] Gennemgå `agent/internal/video/encoder/nvenc.go`.
- [x] Undgå at `SetBitrate()` genstarter hele FFmpeg-processen ved almindelige bitrate-ændringer.
- [x] Beslut en af disse strategier:
  - [x] Drop dynamisk bitrate under session og brug fast bitrate.
  - [!] Implementer robust dynamisk bitrate uden FFmpeg-restart. Fravalgt i denne runde.
- [x] Dokumenter valgt strategi.
- [x] Undgå keyframe-burst og stall ved bitrate-skift.

### Encode-loop og frame-grænser

- [x] Fjern eller reducer blokerende encode under mutex.
- [~] Undgå at capture-loopet venter på tidsbaseret FFmpeg drain under load.
- [x] Erstat quiet-period frame-detektion med NAL/start-code parsing for NVENC.
- [x] Brug access-unit grænser/AUD hvor muligt for NVENC.
- [x] Ensret `Encode()`-kontrakt:
  - [x] Dokumenter hvad "ingen frame klar" betyder.
  - [x] Brug eksplicit sentinel error for "ingen frame klar".
- [ ] Test under CPU-pres.
- [ ] Test med hurtige skærmændringer.
- [ ] Test H.264 fallback til JPEG.

## P1 - Display/oplosning

- [x] Vis remote oplosning i dashboard toolbar.
- [x] Vis remote oplosning i controller toolbar.
- [x] Dokumenter at `640x480` betyder lav Windows console-display-oplosning.
- [x] Lav guide til dummy HDMI eller virtuel display-driver.
- [x] Overvej agent health warning når capture-oplosning er `640x480`.
- [x] Overvej UI-warning i controller/dashboard: "Server sender 640x480".

## P2 - Dashboard/controller UX

- [x] Fjern preview-overlay fullscreen-knap fra dashboard.
- [x] Gør dashboard toolbar-knapper mere synlige.
- [x] Vis nyeste agent/controller-version på dashboardet.
- [x] Gør H.264/JPEG mode tydeligere med tekst eller aktiv badge, ikke kun ikon.
- [x] Gør kvalitetspreset tydeligere med aktiv label.
- [x] Tjek mobil-layout efter toolbar-kontrastændringer.
- [x] Tjek fullscreen-layout efter fjernelse af overlay-knappen.

## P2 - Repo-oprydning

### Binærer og artifacts

- [x] Find tracked `.exe`, `.msi`, `.dll`, `.zip`, `.tar.gz` og debug-builds.
- [x] Lav liste over hvilke binærer der faktisk er tracked: ingen tracked binære release/archive-artifacts fundet.
- [x] Fjern release/debug artifacts fra git-index: `agent/test_screenshot.png` fjernet; app-icons beholdt.
- [x] Bevar nødvendige tredjeparts-tools kun hvis de aktivt bruges og licens er ok: ingen tracked tredjeparts-binærer fundet.
- [x] Flyt release-artifacts til `builds/` eller download-server, ikke git: `builds/` er ignored og ingen release-artifacts er tracked.
- [x] Opdater `.gitignore`:
  - [x] `*.exe`
  - [x] `*.msi`
  - [x] `*.dll` hvis ikke source dependency.
  - [x] `builds/`
  - [x] screenshots i roden.
  - [x] lokale scripts/eksperimenter.
- [!] Vurder om git-historik skal slankes senere med `git filter-repo`.

### Root-rod

- [x] Gennemgå screenshots i repo-roden.
- [x] Gennemgå `slet.txt`.
- [x] Gennemgå `dele filer/`.
- [x] Gennemgå `old projekt/`.
- [x] Flyt relevante noter til `docs/`: ingen tracked noter krævede flytning i denne runde.
- [x] Slet eller gitignore lokale engangsfiler.

## P3 - Kodeopdeling og vedligeholdelse

### Go backend/agent

- [ ] Split `agent/internal/webrtc/peer.go` i mindre filer:
  - [ ] peer setup.
  - [ ] data channels.
  - [ ] codec/mode control.
  - [ ] lifecycle/reconnect.
- [ ] Split `agent/internal/webrtc/signaling.go`:
  - [ ] Supabase HTTP helpers.
  - [ ] session polling.
  - [ ] ICE/signaling messages.
  - [ ] heartbeat/health.
- [ ] Tilføj målrettede unit tests omkring små helper-funktioner efter split.

### Dashboard JavaScript

- [ ] Split `docs/js/webrtc.js`:
  - [ ] Peer/session setup.
  - [ ] input capture.
  - [ ] frame rendering.
  - [ ] toolbar/streaming controls.
  - [ ] stats/diagnostics.
  - [ ] clipboard.
- [ ] Undgå at ændre adfærd i første split; kun flyt kode.
- [ ] Tilføj browser smoke-test efter split.

## P3 - Dokumentation

- [x] Opdater `README.md` current version til seneste publicerede version.
- [ ] Dokumenter update-server og downloads flow.
- [ ] Dokumenter forskellen på:
  - [ ] Dashboard/web-controller.
  - [ ] Wails-controller.
  - [ ] Fyne/native-controller, hvis den stadig eksisterer.
- [ ] Dokumenter H.264 kendte begrænsninger.
- [ ] Dokumenter Session0/GDI/H.264 fallback.

## Release-builds

- [x] Synk agent/controller source-version til `v3.1.97` med builddato `2026-06-27`.
- [x] Opdater Windows resource metadata for agent/controller til `3.1.97.0`.
- [x] Byg Windows controller: `builds/controller-v3.1.97.exe`.
- [x] Byg Windows agent GUI: `builds/remote-agent-v3.1.97.exe`.
- [x] Byg Windows agent console: `builds/remote-agent-console-v3.1.97.exe`.
- [x] Generer SHA256 checksums for alle tre exe-filer.
- [x] Byg installers for `v3.1.97`.
- [x] Deploy `v3.1.97` til download/update-server.
- [x] Byg Windows controller hotfix: `builds/controller-v3.1.98.exe`.
- [x] Byg Windows controller installer hotfix: `builds/RemoteDesktopController-v3.1.98-Setup.exe`.
- [x] Deploy controller `v3.1.98` til download/update-server; agent bliver på `v3.1.97`.
- [!] H.264 test fra controller til `WIN-TEST` gav "ingen stabil video"; controller hotfix starter video-track eksplicit og skal retestes.
- [x] Byg og deploy controller `v3.1.99` med H.264 getStats-diagnostik og konservativ 4 Mbps bitrate.
- [x] H.264 fra controller til `WIN-TEST` retestet på `v3.1.99`; fejlede stadig med 0 RTP-pakker og ingen decoded frames.
- [x] Byg og deploy controller `v3.1.100` med parsing af binære JSON datachannel-beskeder, så `codec_status` fra agenten vises.
- [x] H.264 fra controller til `WIN-TEST` retestet på `v3.1.100`; agent afviste med `h264_unavailable_for_session0_gdi`.
- [x] Byg og deploy agent `v3.1.101`, så Session0 pipe-helper kan tillade H.264 når den følger en aktiv brugerdesktop.
- [x] H.264 fra controller `v3.1.100` til agent `v3.1.101` på `WIN-TEST` retestet; H.264 virker og decoder 1920x1080, men input stopper.
- [x] Byg og deploy agent `v3.1.102`, så Session0 pipe-helper input får prioritet over kontinuerlig H.264 capture.
- [x] H.264 input fra controller `v3.1.100` til agent `v3.1.102` på `WIN-TEST` retestet; input stadig ikke synligt.
- [x] Byg og deploy controller `v3.1.103` med input-send tællere i session-log og keyboard focus fallback.
- [x] H.264 input fra controller `v3.1.103` til agent `v3.1.102` retestet; controller sender input (`sent>0`) uden fejl.
- [x] Byg og deploy agent/controller `v3.1.104` med agent-side `input_status` tilbage til session-log.
- [!] H.264 input på `WIN-TEST` skal retestes på `v3.1.104`; session-log skal vise både `Input stats` og `Agent input`.
- [x] Byg og deploy agent/controller `v3.1.105` med 10 Mbps H.264 og kortere mousemove input-prioritet for mindre video-lag.
- [x] H.264 kvalitet/input-lag testet på `WIN-TEST` med `v3.1.105`; input virker, men H.264-billedet fryser under mousemove.
- [x] Byg og deploy agent `v3.1.106`; mousemove fornyer ikke længere capture-skip-prioritet.
- [x] H.264 live-opdatering under mousemove retestet på `WIN-TEST` med agent `v3.1.106`; H.264-billedet opdaterer stadig ikke stabilt.
- [x] Byg og deploy agent `v3.1.107`; force keyframe og capture-trigger kort efter forwarded input.
- [!] H.264 live-opdatering efter klik/input skal retestes på `WIN-TEST` med agent `v3.1.107`.

## Foreslået rækkefølge

1. P0: Password-lagring i OS keyring.
2. P0: Secrets-audit.
3. P1: Afklar om Fyne-controller stadig skal vedligeholdes.
4. P1: H.264 SetBitrate/encode-loop stabilisering.
5. P2: Repo-oprydning af tracked artifacts.
6. P3: Kodeopdeling efter de kritiske fixes.

## Acceptkriterier

- [x] Ingen nye passwords gemmes i klartekst.
- [x] Eksisterende password-data migreres væk fra klartekst JSON ved næste load.
- [x] Dashboard/controller viser tydelig remote oplosning.
- [ ] H.264 bitrate-skift staller ikke capture-loopet.
- [ ] H.264 frame boundaries er robuste under load.
- [x] Repoet indeholder ikke release/debug binaries som tracked source.
- [x] Arkitekturen for controller-varianter er dokumenteret.
