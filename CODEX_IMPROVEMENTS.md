# Codex Forbedringsforslag - Oversigt

## 🔴 P0 - Kritisk (Sikkerhed)
| Problem | Status | Handling |
|---------|--------|----------|
| Hardcodede credentials i kode (Supabase URL/key, TURN creds) | ⚠️ Kendt | Flyt til env/edge functions |
| Backup-fil `connection.go.544935917091284356` ligger i repo | ⚠️ | Slet og tilføj til .gitignore |
| SDP logging kan lække følsom info | ⚠️ | Reducer logging |

---

## 🟡 P1 - Connection/Stabilitet
| Forbedring | Kompleksitet | Gevinst |
|------------|--------------|---------|
| **TURN på controller** - bruger kun STUN nu, fejler bag NAT | Medium | Stor - flere connections virker |
| **Trickle-ICE** - hurtigere connect (i stedet for at vente på GatheringComplete) | Medium | Hurtigere connect |

---

## 🟢 P2 - Performance/Kvalitet
| Forbedring | Kompleksitet | Gevinst |
|------------|--------------|---------|
| **Unreliable datachannel til video** - undgå head-of-line blocking | Lav | Bedre latency ved pakketab |
| **Undgå dobbelt-capture** - RGBA til motion + JPEG separat | Lav | CPU besparelse |
| **Frame ID i chunking** - robusthed ved out-of-order | Lav | Færre korrupte frames |
| **H.264 færdiggøres** - RTP track i controller | Høj | Meget bedre kvalitet/båndbredde |

---

## 🔵 P3 - Features
| Feature | Status | Problem |
|---------|--------|---------|
| **Clipboard** | ✅ Virker | Evt. rate-limit og komprimér billeder |
| **File Transfer** | ❌ Broken | Controller skriver ikke til fil, encoding mismatch |

### File Transfer Problemer:
1. Controller har TODO på linje 253 - skriver ikke til fil
2. Data encoding mismatch: JSON base64 vs raw bytes
3. Binær data håndteres forkert med `string(buffer[:n])`

**Forslag:** Drop JSON for chunks, send binært med header (transfer_id, offset, len, crc32)

---

## ✅ Allerede Godt
- Adaptive JPEG pipeline (buffer/RTT/loss/CPU/motion/idle)
- DXGI reinit ved desktop-skift
- Clipboard med hash/echo-loop prevention
- Input på separat unreliable channel

---

## Anbefalet Prioritering

### Fase 1 (Quick Wins)
1. ✅ TURN på controller (NAT-problemer)
2. ✅ Slet backup-fil
3. ✅ Unreliable video datachannel

### Fase 2 (Stabilitet)
1. Frame ID i chunking
2. Trickle-ICE
3. Reducer SDP logging

### Fase 3 (Features)
1. Fix file transfer
2. H.264 path færdiggøres
