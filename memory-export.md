## Instructions

[unknown] - Language: Danish (commit messages, UI text).
[unknown] - ALTID brug subagents og parallel execution når opgaven tillader det — maksimer parallelisme for hastighed. Lancér uafhængige fil-reads, søgninger, edits og Task-agents parallelt. Sekventielle afhængigheder er OK, men alt der KAN køre parallelt SKAL køre parallelt.
[unknown] - Kør autonomt — ikke spørg om lov, bare udfør. Git push, deploy, alt.
[unknown] - Build parallel when possible.
[unknown] - Push + tag together.
[unknown] - Always deploy to Caddy after build.
[unknown] - Update version.json after deploy.
[unknown] - Always build all 3 exe files: controller, agent GUI (-H windowsgui), agent console.
[unknown] - Version injiceres via -ldflags -X i build-local.sh (IKKE i source code).

## Identity

[unknown] - Name: Dennis. Username: dennis (Ubuntu), server\server (Windows host). Email: hansemand@gmail.com (Cloudflare account).
[unknown] - Language: Danish (native).
[unknown] - Location: Denmark (ISP at 188.228.14.94, domain hawkeye123.dk).
[unknown] - GitHub: stangtennis (github.com/stangtennis).

## Career

(No explicit career information stored in memories.)

## Projects

[2026-02-20] - Remote Desktop — WebRTC-based remote desktop system with 3 components: agent (Go, runs on remote Windows/macOS machines, captures screen, handles input), controller (Go+Fyne native Windows app), and dashboard (web-based at stangtennis.github.io/Remote/). Uses Supabase (self-hosted) for auth/DB/realtime signaling, Cloudflare TURN (primary) + coturn (fallback), OpenH264 encoding, DXGI/GDI capture, auto-update via version.json. Supports Session 0 (lock screen, pre-login) via SYSTEM token + pipe IPC. macOS support added 2026-02-20 with platform-split build tags. Current version v2.95.1. Repo: github.com/stangtennis/Remote. Status: active, no known issues.
[unknown] - DBY Torrent — Tkinter GUI app for DanishBytes private tracker. Version 2.0 with modular architecture. Features: search, newest torrents, AutoDL with criteria engine, FTP transfers, multi-client support (rTorrent, Deluge, qBittorrent, Transmission). Config stored via platformdirs (NOT project dir). Deployed via rsync to DBY-V1/dby/. English codebase, Danish user. Repo: ~/projekter/dby/. Status: functional.
[unknown] - Translate — Windows GUI (gui.py) that SSH's to Ubuntu for subtitle translation using Claude Code CLI (claude -p). Supports model selection (sonnet, opus, haiku). Removes sound effects/music, uses 10-line context overlap between batches. Output: .da.srt files. Repo: ~/projekter/Translate/. Status: functional.
[unknown] - Video Reklamer (autocut_ads.py) — Automatic ad removal from Danish TV recordings (TV3/Viaplay). Hybrid multi-signal detection: Comskip (primary), blackdetect, silencedetect, PySceneDetect. Frame-accurate cutting via MPEG-TS intermediate + re-encode at cut points. Pre/post-show trimming with credits detection. Achieves ~30-38% ad ratio on Danish TV. Repo: ~/projekter/video-reklamer/. Status: functional, tested on NCIS S22 + Prodigal Son.

## Preferences

[unknown] - Uses DanishBytes private tracker for torrents.
[unknown] - Runs self-hosted infrastructure: Supabase, Caddy, Portainer, Grafana, coturn, Filebrowser — all via Docker on Ubuntu VM.
[unknown] - Primary dev machine is Ubuntu 24.04 VM (linux1) on Hyper-V, cross-compiles to Windows with MinGW.
[unknown] - Windows host (SERVER, Win11 Pro) with NVIDIA GTX 1060 3GB, GPU-P partitioning to VMs.
[unknown] - Uses Cloudflare Tunnel for all HTTP (no port 80/443 forwarding), Zero Trust with email OTP for admin panels.
[unknown] - Router: UniFi Dream Machine SE.
[unknown] - claude-yolo bash function to toggle bypassPermissions on/off.
[unknown] - Prefers compact UI — device cards should be single-row, not large blocks ("hvis nu man har mange så skal man da scrolle en del rundt").
[unknown] - Device IDs are useless to display in UI ("client med det device nummer er lidt ubrugelidt at vise").
[unknown] - Expects auto-increment of version numbers without being asked ("jamen hvorfor spørger du om det?").
