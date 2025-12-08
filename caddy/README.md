# 🚀 Caddy Reverse Proxy Setup

Simpel reverse proxy med automatisk HTTPS via Let's Encrypt.

## Fordele over Nginx Proxy Manager

| Feature | Caddy | Nginx PM |
|---------|-------|----------|
| Automatisk HTTPS | ✅ Indbygget | ⚠️ Kræver GUI setup |
| Konfiguration | 15 linjer | GUI + mange klik |
| WebSocket | ✅ Automatisk | ⚠️ Manuel toggle |
| Hot reload | ✅ `caddy reload` | ⚠️ Container restart |

## Installation

### 1. Stop Nginx Proxy Manager (hvis kørende)

```bash
ssh ubuntu
cd ~/nginx-proxy-manager
docker compose down
```

### 2. Kopier Caddy filer til server

```bash
# Fra din lokale maskine
scp -r caddy/* ubuntu:~/caddy/
```

Eller på serveren:
```bash
mkdir -p ~/caddy
cd ~/caddy
# Kopier docker-compose.yml og Caddyfile hertil
```

### 3. Start Caddy

```bash
cd ~/caddy
docker compose up -d
```

### 4. Tjek logs

```bash
docker compose logs -f
```

Du vil se Caddy automatisk hente SSL certifikater fra Let's Encrypt.

## Test

```bash
# Test HTTPS (vent 30 sek på certifikater første gang)
curl -I https://supabase.hawkeye123.dk
curl -I https://archon.hawkeye123.dk
curl -I https://portainer.hawkeye123.dk
```

## Kommandoer

```bash
# Start
docker compose up -d

# Stop
docker compose down

# Logs
docker compose logs -f

# Reload config (uden downtime)
docker compose exec caddy caddy reload --config /etc/caddy/Caddyfile

# Restart
docker compose restart
```

## Tilføj ny service

Rediger `Caddyfile` og tilføj:

```caddyfile
nyservice.hawkeye123.dk {
    reverse_proxy 192.168.1.92:PORT
}
```

Reload derefter:
```bash
docker compose exec caddy caddy reload --config /etc/caddy/Caddyfile
```

## Fejlfinding

### SSL certifikat fejler

```bash
# Tjek DNS peger på din IP
nslookup supabase.hawkeye123.dk

# Tjek port 80/443 er åbne
curl -I http://supabase.hawkeye123.dk
```

### 502 Bad Gateway

```bash
# Tjek backend service kører
curl http://192.168.1.92:8888

# Tjek Caddy logs
docker compose logs caddy
```

### Reload config

```bash
docker compose exec caddy caddy reload --config /etc/caddy/Caddyfile
```

## Arkitektur

```
Internet
  ↓
Router (port 80, 443 → 192.168.1.92)
  ↓
Caddy (automatisk HTTPS)
  ├─ supabase.hawkeye123.dk → 192.168.1.92:8888
  ├─ archon.hawkeye123.dk   → 192.168.1.92:3737
  └─ portainer.hawkeye123.dk → 192.168.1.92:9000
```
