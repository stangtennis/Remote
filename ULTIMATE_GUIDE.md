# 🚀 ULTIMATE COMPLETE GUIDE
**Everything About Your Ubuntu + Archon + Windsurf Setup in ONE Document**

Last Updated: 2025-12-18 | Server: 192.168.1.92 | User: dennis

---

## 🌐 QUICK ACCESS - ALL SERVICES

### Public HTTPS (via Caddy)
| Service | URL | Login |
|---------|-----|-------|
| **Supabase Studio** | https://supabase.hawkeye123.dk | `supabase` / `this_password_is_insecure_and_should_be_updated` |

### Local Only (Internal Network)
| Service | URL | Login |
|---------|-----|-------|
| **Caddy** | Automatisk HTTPS via Let's Encrypt | Ingen login nødvendig |
| **Archon UI** | http://192.168.1.92:3737 | No login |
| **Supabase Studio** | http://192.168.1.92:8888 | `supabase` / `this_password_is_insecure_and_should_be_updated` |
| **Portainer** | http://192.168.1.92:9000 | Your Portainer credentials |
| **Ollama API** | http://192.168.1.92:11434 | No login |

### Network Drives
| Drive | Path |
|-------|------|
| **P:\\** | `\\192.168.1.92\projekter` (projects) |
| **O:\\** | `\\192.168.1.92\home` (full home) |

---

## 🔐 ALL CREDENTIALS (Master List)

| Service | Username | Password/Key |
|---------|----------|--------------|
| **Ubuntu SSH** | dennis | SSH key (no password) |
| **Samba/Network Drives** | dennis | `Suzuki77wW!!` |
| **Supabase Studio** | supabase | `this_password_is_insecure_and_should_be_updated` |
| **PostgreSQL** | postgres | `postgres` |
| **Portainer API Token** | - | `ptr_XxKkdO1CQy8QyF1FGx0lymIj3/sPl2iEthNBNltrMAY=` |

---

## 🌍 PUBLIC HTTPS ACCESS

### Domain & SSL
```yaml
Domain: *.hawkeye123.dk (wildcard)
Public IP: 188.228.14.94
SSL Certificate: Let's Encrypt Wildcard (auto-renewed)
Managed by: Caddy
```

### Public Endpoints
```yaml
# Only Supabase is exposed publicly (for Remote Desktop app)
Supabase: https://supabase.hawkeye123.dk → http://192.168.1.92:8888

# These are LOCAL ONLY (not exposed to internet for security)
Archon: http://192.168.1.92:3737 (local only)
Portainer: http://192.168.1.92:9000 (local only)
```

### Why Only Supabase is Public?
- Remote Desktop app needs Supabase for authentication and signaling
- Archon and Portainer are admin tools - no need for public access
- Reduces attack surface - fewer exposed services = better security

---

## 🔧 CADDY

### Access
Caddy kører som Docker container og håndterer automatisk HTTPS via Let's Encrypt.
Ingen web UI - konfiguration via Caddyfile.

### Location
```bash
Directory: ~/caddy/
Caddyfile: ~/caddy/Caddyfile
Downloads: ~/caddy/downloads/
```

### Docker Container
```bash
# Start Caddy
docker run -d --name caddy --restart unless-stopped \
  -p 80:80 -p 443:443 \
  -v ~/caddy/Caddyfile:/etc/caddy/Caddyfile:ro \
  -v ~/caddy/downloads:/downloads:ro \
  -v caddy_caddy_data:/data \
  -v caddy_caddy_config:/config \
  caddy:latest

# Restart Caddy
docker restart caddy

# View logs
docker logs caddy --tail 50

# Stop Caddy
docker stop caddy
```

### Caddyfile Configuration
```caddyfile
# Supabase - Kong API Gateway
supabase.hawkeye123.dk {
    reverse_proxy 192.168.1.92:8888
}

# Archon UI
archon.hawkeye123.dk {
    reverse_proxy 192.168.1.92:3737
}

# Portainer
portainer.hawkeye123.dk {
    reverse_proxy 192.168.1.92:9000
}

# Remote Desktop Dashboard - redirect til GitHub Pages
remote.hawkeye123.dk {
    redir https://stangtennis.github.io/Remote{uri} permanent
}

# Remote Desktop Downloads - serve binære filer
downloads.hawkeye123.dk {
    root * /downloads
    file_server browse
    
    header {
        Access-Control-Allow-Origin *
        Access-Control-Allow-Methods "GET, OPTIONS"
    }
}
```

### Subdomains
- `supabase.hawkeye123.dk` - Supabase API (Kong Gateway)
- `archon.hawkeye123.dk` - Archon UI
- `portainer.hawkeye123.dk` - Portainer
- `remote.hawkeye123.dk` - Redirect til GitHub Pages
- `downloads.hawkeye123.dk` - Binary downloads (agent, controller)

### Opdater Downloads
```bash
# Kopier nye builds til downloads folder
cp builds/remote-agent-vX.XX.X.exe ~/caddy/downloads/remote-agent.exe
cp builds/controller-vX.XX.X.exe ~/caddy/downloads/controller.exe
cp builds/remote-agent-console-vX.XX.X.exe ~/caddy/downloads/remote-agent-console.exe
```

### Remote Commands
```bash
# Start Caddy
ssh ubuntu "docker start caddy"

# Check status
ssh ubuntu "docker ps | grep caddy"

# View logs
ssh ubuntu "docker logs caddy --tail 50"
```
## ⚡ SUPABASE EDGE FUNCTIONS

### What Are Edge Functions?
Serverless TypeScript/Deno functions that run on Supabase. Better security than direct database access.

### Deployed Functions
| Function | Endpoint | Purpose |
|----------|----------|---------|
| **device-register** | `/functions/v1/device-register` | Register new devices |
| **session-token** | `/functions/v1/session-token` | Create sessions with PIN/TURN |
| **session-cleanup** | `/functions/v1/session-cleanup` | Cleanup old sessions |
| **file-transfer** | `/functions/v1/file-transfer` | Handle file transfers |

### Function Location
```bash
# On Ubuntu server
~/supabase-local/volumes/functions/

# Source code (in Remote Desktop repo)
f:\#Remote\supabase\functions\
```

### Test Edge Function
```bash
# Test device-register
curl -X POST http://192.168.1.92:8888/functions/v1/device-register \
  -H "Content-Type: application/json" \
  -H "apikey: YOUR_ANON_KEY" \
  -d '{"device_id":"test-123","platform":"windows","arch":"amd64"}'
```

### Deploy New Functions
```bash
# Copy functions to server
scp -r f:\#Remote\supabase\functions\* dennis@192.168.1.92:~/supabase-local/volumes/functions/

# Fix permissions
ssh ubuntu "chmod -R 755 ~/supabase-local/volumes/functions/*"

# Restart edge runtime
ssh ubuntu "docker restart supabase-edge-functions"
```

---

## 🤖 ARCHON MCP SERVER

### What is Archon?
Project management and knowledge base system with MCP (Model Context Protocol) integration for AI assistants like Windsurf.

### Access URLs
```yaml
Archon UI: http://192.168.1.92:3737
Archon API: http://192.168.1.92:8181
Archon MCP: http://192.168.1.92:8051/mcp
```

### MCP Tools Available
```
- find_projects / manage_project
- find_tasks / manage_task
- find_documents / manage_document
- rag_search_knowledge_base
- rag_search_code_examples
- health_check
```

### Windsurf MCP Configuration
**Location**: `C:\Users\server\.codeium\windsurf\mcp_config.json`

```json
{
  "mcpServers": {
    "archon": {
      "serverUrl": "http://192.168.1.92:8051/mcp"
    },
    "context7": {
      "command": "npx",
      "args": ["-y", "@upstash/context7-mcp"]
    },
    "memory": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-memory"]
    },
    "sequential-thinking": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-sequential-thinking"]
    },
    "mcp-playwright": {
      "command": "npx",
      "args": ["@anthropic/mcp-playwright"]
    }
  }
}
```

### Test Archon MCP
In Windsurf, type:
```
list all projects
```
or
```
find tasks with status todo
```

### Archon Commands
```bash
# Start Archon
ssh ubuntu "cd ~/projects/archon && docker compose up -d"

# Stop Archon
ssh ubuntu "cd ~/projects/archon && docker compose down"

# View logs
ssh ubuntu "docker logs archon-mcp --tail 50"

# Restart
ssh ubuntu "cd ~/projects/archon && docker compose restart"
```

---

## 📊 REMOTE DESKTOP PROJECT

### Project Info
```yaml
Repository: https://github.com/stangtennis/Remote
Agent Version: v2.64.0
Controller Version: v2.63.9
Last Updated: 2025-12-18
```

### Downloads
```yaml
Releases: https://github.com/stangtennis/Remote/releases
Agent Installer: RemoteDesktopAgent-Setup-v2.64.0.exe
Controller Installer: RemoteDesktopController-Setup-v2.63.9.exe
```

### Current Features
- ✅ WebRTC video streaming (adaptive 2-30 FPS)
- ✅ Bandwidth optimization (50-80% savings on static desktop)
- ✅ Full mouse & keyboard control
- ✅ File browser and transfer
- ✅ Clipboard sync (text and images)
- ✅ Fullscreen mode with auto-hide toolbar
- ✅ Adaptive quality based on network/CPU

### Performance
| Scenario | Bandwidth |
|----------|-----------|
| Static desktop | ~0.5-2 Mbit/s |
| Active use | ~10-25 Mbit/s |

### Build Commands (from Ubuntu)
```bash
# Agent GUI
cd ~/projekter/Remote\ Desktop/agent && \
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ \
go build -ldflags '-s -w -H windowsgui' -o ../builds/remote-agent.exe ./cmd/remote-agent

# Controller
cd ~/projekter/Remote\ Desktop/controller && \
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
CC=x86_64-w64-mingw32-gcc \
go build -ldflags '-s -w -H windowsgui' -o ../builds/controller.exe .
```

### Planned Features
- Hardware H.264 encoding (GPU-accelerated)
- Multi-monitor support
- Audio streaming

---

## 📊 MONITORING STACK (Grafana + Prometheus + Loki)

### Access URLs
| Service | URL | Login |
|---------|-----|-------|
| **Grafana** | http://192.168.1.92:3030 | `admin` / `Suzuki77wW!!` |
| **Prometheus** | http://192.168.1.92:9090 | No login |
| **cAdvisor** | http://192.168.1.92:8080 | No login |
| **Loki** | http://192.168.1.92:3100 | No login |

### Hvad Overvåges?
| Exporter | Port | Metrics |
|----------|------|---------|
| **Node Exporter** | 9100 | CPU, RAM, Disk, Network (host) |
| **cAdvisor** | 8080 | Docker containers |
| **Postgres Exporter** | 9187 | Supabase database |
| **Promtail → Loki** | 3100 | Logs fra alle containers |

### Location
```bash
Directory: ~/monitoring/
Docker Compose: ~/monitoring/docker-compose.yml
```

### Monitoring Commands
```bash
# Start monitoring
ssh dennis@192.168.1.92 "cd ~/monitoring && docker compose up -d"

# Stop monitoring
ssh dennis@192.168.1.92 "cd ~/monitoring && docker compose down"

# View logs
ssh dennis@192.168.1.92 "docker logs grafana --tail 50"
ssh dennis@192.168.1.92 "docker logs prometheus --tail 50"

# Restart
ssh dennis@192.168.1.92 "cd ~/monitoring && docker compose restart"
```

### Grafana Dashboards
Pre-konfigurerede dashboards:
- **Docker & System Overview** - CPU, RAM, Disk, Container stats
- **Logs Explorer** - Via Loki datasource

### Import Flere Dashboards
1. Gå til Grafana → Dashboards → Import
2. Brug dashboard ID fra https://grafana.com/grafana/dashboards/
3. Anbefalede:
   - **1860** - Node Exporter Full
   - **893** - Docker and system monitoring
   - **13946** - Docker Container & Host Metrics

### Prometheus Targets
Tjek om alle exporters virker:
```
http://192.168.1.92:9090/targets
```

---

# Complete Setup - All Commands & Code

**Every single command, configuration, and code snippet from your Ubuntu + Archon + Windsurf setup**

---

## 📋 Table of Contents

1. [Server Information](#server-information)
2. [SSH Setup](#ssh-setup)
3. [Supabase Setup](#supabase-setup)
4. [Archon Setup](#archon-setup)
5. [Ollama Setup](#ollama-setup)
6. [Samba/Network Drives](#samba-network-drives)
7. [Windsurf Configuration](#windsurf-configuration)
8. [Docker Commands](#docker-commands)
9. [Database Commands](#database-commands)
10. [All Configuration Files](#all-configuration-files)

---

## 🖥️ Server Information

```yaml
Server IP: 192.168.1.92
Hostname: dennis-Virtual-Machine
OS: Ubuntu 22.04 LTS
Username: dennis
SSH: Passwordless (key-based)
Samba Password: Suzuki77wW!!
```

---

## 🔐 SSH Setup

### Generate SSH Key (Windows)

```powershell
# Generate new SSH key
ssh-keygen -t rsa -b 4096
# Location: C:\Users\server\.ssh\id_rsa
```

### Copy SSH Key to Ubuntu

```powershell
# Copy public key to Ubuntu
type $env:USERPROFILE\.ssh\id_rsa.pub | ssh dennis@192.168.1.92 "mkdir -p ~/.ssh && cat >> ~/.ssh/authorized_keys"

# Set correct permissions on Ubuntu
ssh dennis@192.168.1.92 "chmod 700 ~/.ssh && chmod 600 ~/.ssh/authorized_keys"

# Test passwordless connection
ssh dennis@192.168.1.92 "whoami"
```

### SSH Config File

**Location**: `C:\Users\server\.ssh\config`

```ssh-config
# Ubuntu Development Server (Local)
Host ubuntu-server
    HostName 192.168.1.92
    User dennis
    IdentityFile C:\Users\server\.ssh\id_rsa
    ForwardAgent yes
    ServerAliveInterval 60
    ServerAliveCountMax 3
    Compression yes

# Short alias for Ubuntu
Host ubuntu
    HostName 192.168.1.92
    User dennis
    IdentityFile C:\Users\server\.ssh\id_rsa
    ForwardAgent yes
    ServerAliveInterval 60
    ServerAliveCountMax 3
```

### Test SSH Connection

```powershell
# Using full hostname
ssh dennis@192.168.1.92

# Using config alias
ssh ubuntu-server
ssh ubuntu

# Run single command
ssh ubuntu "ls -la"

# Run multiple commands
ssh ubuntu "cd ~/projekter && ls -la"
```

---

## 🗄️ Supabase Setup

### Install Supabase (Ubuntu)

```bash
# Create directory
mkdir -p ~/supabase-local
cd ~/supabase-local

# Download docker-compose.yml
wget https://raw.githubusercontent.com/supabase/supabase/master/docker/docker-compose.yml

# Create .env file
cat > .env << 'EOF'
KONG_HTTP_PORT=8888
KONG_HTTPS_PORT=8443
EOF

# Start Supabase
docker compose up -d
```

### Supabase Access Information

```yaml
Supabase Studio: http://192.168.1.92:8888
API URL: http://192.168.1.92:8888
Database Host: 192.168.1.92
Database Port: 5432
Database Name: postgres
Database User: postgres
Database Password: postgres
Studio Username: supabase
Studio Password: this_password_is_insecure_and_should_be_updated
```

### Supabase Keys

```bash
# Anon Key
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyAgCiAgICAicm9sZSI6ICJhbm9uIiwKICAgICJpc3MiOiAic3VwYWJhc2UtZGVtbyIsCiAgICAiaWF0IjogMTY0MTc2OTIwMCwKICAgICJleHAiOiAxNzk5NTM1NjAwCn0.dc_X5iR_VP_qT0zsiyj_I_OZ2T9FtRU2BBNWN8Bu4GE

# Service Role Key
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyAgCiAgICAicm9sZSI6ICJzZXJ2aWNlX3JvbGUiLAogICAgImlzcyI6ICJzdXBhYmFzZS1kZW1vIiwKICAgICJpYXQiOiAxNjQxNzY5MjAwLAogICAgImV4cCI6IDE3OTk1MzU2MDAKfQ.DaYlNEoUrrEn2Ig7tqibS-PHK5vgusbcbo7X36XVt4Q
```

### Supabase Docker Commands

```bash
# Start Supabase
cd ~/supabase-local
docker compose up -d

# Stop Supabase
docker compose down

# View logs
docker compose logs -f

# Restart specific service
docker compose restart supabase-db

# Check status
docker compose ps
```

### From Windows

```powershell
# Start Supabase
ssh ubuntu "cd ~/supabase-local && docker compose up -d"

# Stop Supabase
ssh ubuntu "cd ~/supabase-local && docker compose down"

# View logs
ssh ubuntu "cd ~/supabase-local && docker compose logs -f supabase-db"
```

---

## 🏗️ Archon Setup

### Clone Archon Repository (Ubuntu)

```bash
# Clone repository
cd ~/projects
git clone https://github.com/coleam00/Archon.git archon
cd archon
```

### Archon Environment File

**Location**: `~/projects/archon/.env`

```bash
# Create .env file
cat > ~/projects/archon/.env << 'EOF'
SUPABASE_URL=http://192.168.1.92:8888
SUPABASE_SERVICE_KEY=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyAgCiAgICAicm9sZSI6ICJzZXJ2aWNlX3JvbGUiLAogICAgImlzcyI6ICJzdXBhYmFzZS1kZW1vIiwKICAgICJpYXQiOiAxNjQxNzY5MjAwLAogICAgImV4cCI6IDE3OTk1MzU2MDAKfQ.DaYlNEoUrrEn2Ig7tqibS-PHK5vgusbcbo7X36XVt4Q
ARCHON_SERVER_PORT=8181
ARCHON_MCP_PORT=8051
ARCHON_UI_PORT=3737
OLLAMA_BASE_URL=http://192.168.1.92:11434
EOF
```

### Start Archon Services

```bash
# Navigate to Archon directory
cd ~/projects/archon

# Start all services
docker compose up -d

# Check status
docker compose ps

# View logs
docker compose logs -f archon-mcp
docker compose logs -f archon-server
docker compose logs -f archon-ui
```

### Archon Access URLs

```yaml
Archon UI: http://192.168.1.92:3737
Archon API: http://192.168.1.92:8181
Archon MCP: http://192.168.1.92:8051/mcp
```

### Archon Docker Commands

```bash
# Stop all services
cd ~/projects/archon
docker compose down

# Restart all services
docker compose restart

# Restart specific service
docker compose restart archon-mcp

# Rebuild after code changes
docker compose up --build -d

# View logs (last 50 lines)
docker logs archon-mcp --tail 50

# Follow logs in real-time
docker logs -f archon-mcp
```

### From Windows

```powershell
# Start Archon
ssh ubuntu "cd ~/projects/archon && docker compose up -d"

# Stop Archon
ssh ubuntu "cd ~/projects/archon && docker compose down"

# Restart Archon
ssh ubuntu "cd ~/projects/archon && docker compose restart"

# View logs
ssh ubuntu "docker logs archon-mcp --tail 50"

# Check health
ssh ubuntu "wget -qO- http://localhost:8181/health"
```

---
## 🤖 Ollama Setup

### Install Ollama (Ubuntu)

```bash
# Install Ollama
curl -fsSL https://ollama.com/install.sh | sh

# Start Ollama service
sudo systemctl start ollama
sudo systemctl enable ollama

# Check status
systemctl status ollama
```

### Pull Models

```bash
# Pull LLM model
ollama pull llama3.2

# Pull embedding model
ollama pull nomic-embed-text

# List installed models
ollama list
```

### Ollama Access

```yaml
Ollama API: http://192.168.1.92:11434
LLM Model: llama3.2:latest
Embedding Model: nomic-embed-text:latest
```

### Ollama Commands

```bash
# Test Ollama
curl http://localhost:11434/api/tags

# Restart Ollama
sudo systemctl restart ollama

# View logs
journalctl -u ollama -f
```

### From Windows

```powershell
# Test Ollama
ssh ubuntu "curl http://localhost:11434/api/tags"

# List models
ssh ubuntu "ollama list"

# Restart Ollama
ssh ubuntu "sudo systemctl restart ollama"
```

---

## 📁 Samba/Network Drives

### Install Samba (Ubuntu)

```bash
# Install Samba
sudo apt update
sudo apt install -y samba

# Set Samba password
sudo smbpasswd -a dennis
# Password: Suzuki77wW!!

# Enable user
sudo smbpasswd -e dennis
```

### Samba Configuration

**Location**: `/etc/samba/smb.conf`

```bash
# Add to /etc/samba/smb.conf
sudo tee -a /etc/samba/smb.conf > /dev/null << 'EOF'

[projekter]
   comment = Dennis Projects Folder
   path = /home/dennis/projekter
   browseable = yes
   read only = no
   writable = yes
   guest ok = no
   valid users = dennis
   create mask = 0755
   directory mask = 0755

[home]
   comment = Dennis Home Directory  
   path = /home/dennis
   browseable = yes
   read only = no
   writable = yes
   guest ok = no
   valid users = dennis
   create mask = 0755
   directory mask = 0755
EOF
```

### Start Samba

```bash
# Restart Samba
sudo systemctl restart smbd nmbd

# Enable on boot
sudo systemctl enable smbd nmbd

# Allow through firewall
sudo ufw allow samba

# Check status
sudo systemctl status smbd
```

### Map Network Drives (Windows)

```powershell
# Map O: drive (home directory)
net use O: \\192.168.1.92\home /user:dennis Suzuki77wW!! /persistent:yes

# Map P: drive (projekter folder)
net use P: \\192.168.1.92\projekter /user:dennis Suzuki77wW!! /persistent:yes

# List mapped drives
net use

# Disconnect drive
net use O: /delete
net use P: /delete
```

### Reconnect Script

**Location**: `C:\Users\server\reconnect-ubuntu-drives-with-credentials.bat`

```batch
@echo off
set USERNAME=dennis
set PASSWORD=Suzuki77wW!!
set SERVER=192.168.1.92

ping -n 1 -w 1000 %SERVER% >nul 2>&1
if errorlevel 1 (
    echo Ubuntu server is not reachable!
    pause
    exit /b 1
)

net use O: /delete >nul 2>&1
net use O: \\%SERVER%\home /user:%USERNAME% %PASSWORD% /persistent:yes

net use P: /delete >nul 2>&1
net use P: \\%SERVER%\projekter /user:%USERNAME% %PASSWORD% /persistent:yes

echo Drives reconnected!
pause
```

---

## 💻 Windsurf Configuration

### Windsurf MCP Config

**Location**: `C:\Users\server\.codeium\windsurf\mcp_config.json`

```json
{
  "mcpServers": {
    "archon": {
      "serverUrl": "http://192.168.1.92:8051/mcp"
    },
    "memory": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-memory"],
      "env": {}
    },
    "sequential-thinking": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-sequential-thinking"],
      "env": {}
    },
    "puppeteer": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-puppeteer"],
      "env": {}
    },
    "context7": {
      "command": "npx",
      "args": ["-y", "@upstash/context7-mcp"]
    },
    "portainer-docker": {
      "command": "node",
      "args": ["f:\\##mcpserver1\\clean-portainer-mcp.js"],
      "env": {
        "PORT": "8100",
        "PORTAINER_URL": "http://192.168.1.92:9000",
        "PORTAINER_API_KEY": "ptr_XxKkdO1CQy8QyF1FGx0lymIj3/sPl2iEthNBNltrMAY=",
        "ENDPOINT_ID": "3"
      }
    }
  }
}
```

### Windsurf Settings

**Location**: `C:\Users\server\AppData\Roaming\Windsurf\User\settings.json`

```json
{
  "mcpServers": {
    "archon": {
      "serverUrl": "http://192.168.1.92:8051/mcp"
    }
  }
}
```

### Test Archon MCP

```
# In Windsurf, run:
list projects
```

---

## 🐳 Docker Commands

### Container Management

```bash
# List running containers
docker ps

# List all containers
docker ps -a

# Stop container
docker stop container-name

# Start container
docker start container-name

# Restart container
docker restart container-name

# View logs
docker logs container-name --tail 50
docker logs -f container-name

# Execute command in container
docker exec container-name command
docker exec -it container-name /bin/bash
```

### Docker Compose

```bash
# Start services
docker compose up -d

# Stop services
docker compose down

# Restart services
docker compose restart

# Rebuild and start
docker compose up --build -d

# View logs
docker compose logs -f service-name

# Check status
docker compose ps
```

### System Cleanup

```bash
# Remove stopped containers
docker container prune -f

# Remove unused images
docker image prune -a -f

# Remove unused volumes
docker volume prune -f

# Remove everything unused
docker system prune -a -f

# Check disk usage
docker system df
```

### From Windows

```powershell
# List containers
ssh ubuntu "docker ps"

# View logs
ssh ubuntu "docker logs archon-mcp --tail 50"

# Restart container
ssh ubuntu "docker restart archon-mcp"

# Check disk usage
ssh ubuntu "docker system df"
```

---

## 🗄️ Database Commands

### PostgreSQL Direct Access

```bash
# Connect to database
docker exec -it supabase-db psql -U postgres -d postgres

# Run single query
docker exec -i supabase-db psql -U postgres -d postgres -c "SELECT * FROM archon_projects;"

# List tables
docker exec -i supabase-db psql -U postgres -d postgres -c "\dt"
```

### Common Queries

```sql
-- List all projects
SELECT id, title, created_at FROM archon_projects ORDER BY created_at DESC;

-- List all tasks
SELECT id, title, status, project_id FROM archon_tasks ORDER BY created_at DESC;

-- Count projects
SELECT COUNT(*) FROM archon_projects;

-- Count tasks by status
SELECT status, COUNT(*) FROM archon_tasks GROUP BY status;

-- Show database size
SELECT pg_size_pretty(pg_database_size(current_database()));

-- List all tables
\dt

-- Describe table
\d archon_projects

-- Show table size
SELECT pg_size_pretty(pg_total_relation_size('archon_projects'));
```

### Backup & Restore

```bash
# Backup database
docker exec supabase-db pg_dump -U postgres postgres > ~/backups/supabase_$(date +%Y%m%d).sql

# Restore database
docker exec -i supabase-db psql -U postgres -d postgres < ~/backups/supabase_20250122.sql

# Backup specific table
docker exec supabase-db pg_dump -U postgres -t archon_projects postgres > ~/backups/projects_$(date +%Y%m%d).sql
```

### From Windows

```powershell
# Run query
ssh ubuntu 'docker exec -i supabase-db psql -U postgres -d postgres -c "SELECT COUNT(*) FROM archon_projects;"'

# List tables
ssh ubuntu 'docker exec -i supabase-db psql -U postgres -d postgres -c "\dt"'

# Backup database
ssh ubuntu "docker exec supabase-db pg_dump -U postgres postgres > ~/backups/supabase_backup.sql"
```

---

## 📝 All Configuration Files

### 1. SSH Config (`C:\Users\server\.ssh\config`)

```ssh-config
Host ubuntu-server
    HostName 192.168.1.92
    User dennis
    IdentityFile C:\Users\server\.ssh\id_rsa
    ForwardAgent yes
    ServerAliveInterval 60
    ServerAliveCountMax 3
    Compression yes

Host ubuntu
    HostName 192.168.1.92
    User dennis
    IdentityFile C:\Users\server\.ssh\id_rsa
    ForwardAgent yes
    ServerAliveInterval 60
    ServerAliveCountMax 3
```

### 2. Archon .env (`~/projects/archon/.env`)

```bash
SUPABASE_URL=http://192.168.1.92:8888
SUPABASE_SERVICE_KEY=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyAgCiAgICAicm9sZSI6ICJzZXJ2aWNlX3JvbGUiLAogICAgImlzcyI6ICJzdXBhYmFzZS1kZW1vIiwKICAgICJpYXQiOiAxNjQxNzY5MjAwLAogICAgImV4cCI6IDE3OTk1MzU2MDAKfQ.DaYlNEoUrrEn2Ig7tqibS-PHK5vgusbcbo7X36XVt4Q
ARCHON_SERVER_PORT=8181
ARCHON_MCP_PORT=8051
ARCHON_UI_PORT=3737
OLLAMA_BASE_URL=http://192.168.1.92:11434
```

### 3. Supabase .env (`~/supabase-local/.env`)

```bash
KONG_HTTP_PORT=8888
KONG_HTTPS_PORT=8443
```

### 4. Windsurf MCP Config (`C:\Users\server\.codeium\windsurf\mcp_config.json`)

```json
{
  "mcpServers": {
    "archon": {
      "serverUrl": "http://192.168.1.92:8051/mcp"
    }
  }
}
```

### 5. Samba Config (`/etc/samba/smb.conf` - append)

```ini
[projekter]
   comment = Dennis Projects Folder
   path = /home/dennis/projekter
   browseable = yes
   read only = no
   writable = yes
   guest ok = no
   valid users = dennis
   create mask = 0755
   directory mask = 0755

[home]
   comment = Dennis Home Directory  
   path = /home/dennis
   browseable = yes
   read only = no
   writable = yes
   guest ok = no
   valid users = dennis
   create mask = 0755
   directory mask = 0755
```

---

## 🔗 All URLs & Endpoints

```yaml
# Archon
Archon UI: http://192.168.1.92:3737
Archon API: http://192.168.1.92:8181
Archon MCP: http://192.168.1.92:8051/mcp

# Supabase
Supabase Studio: http://192.168.1.92:8888
Supabase API: http://192.168.1.92:8888
PostgreSQL: 192.168.1.92:5432

# Other Services
Portainer: http://192.168.1.92:9000
Ollama: http://192.168.1.92:11434

# Network Drives
Projekter: \\192.168.1.92\projekter (P:)
Home: \\192.168.1.92\home (O:)
```

---

## 🔐 All Credentials

```yaml
# Ubuntu SSH
Username: dennis
Auth: SSH key (C:\Users\server\.ssh\id_rsa)

# Samba
Username: dennis
Password: Suzuki77wW!!

# Supabase Studio
Username: supabase
Password: this_password_is_insecure_and_should_be_updated

# PostgreSQL
Username: postgres
Password: postgres
Database: postgres

# Portainer
URL: http://192.168.1.92:9000
API Token: ptr_XxKkdO1CQy8QyF1FGx0lymIj3/sPl2iEthNBNltrMAY=
```

---

## 🚀 Quick Start Commands

### Start Everything

```bash
# SSH into Ubuntu
ssh ubuntu

# Start Supabase
cd ~/supabase-local && docker compose up -d

# Start Archon
cd ~/projects/archon && docker compose up -d

# Check all services
docker ps
```

### From Windows (One Command)

```powershell
ssh ubuntu "cd ~/supabase-local && docker compose up -d && cd ~/projects/archon && docker compose up -d && docker ps"
```

### Stop Everything

```bash
# Stop Archon
cd ~/projects/archon && docker compose down

# Stop Supabase
cd ~/supabase-local && docker compose down
```

### Check Status

```bash
# Check all containers
docker ps

# Check Archon health
wget -qO- http://localhost:8181/health

# Check Supabase
docker exec -i supabase-db psql -U postgres -d postgres -c "SELECT 1;"

# Check Ollama
curl http://localhost:11434/api/tags
```

---

## 🆘 Troubleshooting Commands

### SSH Issues

```powershell
# Test connection
ping 192.168.1.92

# Test SSH
ssh ubuntu "whoami"

# Check SSH key
Test-Path C:\Users\server\.ssh\id_rsa
```

### Docker Issues

```bash
# Check Docker service
sudo systemctl status docker

# Restart Docker
sudo systemctl restart docker

# Check disk space
df -h
docker system df
```

### Archon Issues

```bash
# Check logs
docker logs archon-mcp --tail 50
docker logs archon-server --tail 50
docker logs archon-ui --tail 50

# Check environment
docker exec archon-mcp env | grep -E '(SUPABASE|OLLAMA)'

# Restart services
cd ~/projects/archon && docker compose restart
```

### Supabase Issues

```bash
# Check logs
docker logs supabase-db --tail 50

# Test database
docker exec -i supabase-db psql -U postgres -d postgres -c "SELECT 1;"

# Restart Supabase
cd ~/supabase-local && docker compose restart
```

### Network Drive Issues

```powershell
# Test Samba port
Test-NetConnection -ComputerName 192.168.1.92 -Port 445

# Reconnect drives
C:\Users\server\reconnect-ubuntu-drives-with-credentials.bat

# Check Samba on Ubuntu
ssh ubuntu "sudo systemctl status smbd"
```

---

## 📦 Setup on New Machine

### 1. Copy SSH Key

```powershell
# Copy from old machine to new machine:
C:\Users\server\.ssh\id_rsa
C:\Users\server\.ssh\id_rsa.pub

# Place in:
C:\Users\[NewUsername]\.ssh\
```

### 2. Create SSH Config

Create `C:\Users\[NewUsername]\.ssh\config`:

```ssh-config
Host ubuntu-server
    HostName 192.168.1.92
    User dennis
    IdentityFile C:\Users\[NewUsername]\.ssh\id_rsa
    ForwardAgent yes
    ServerAliveInterval 60
    ServerAliveCountMax 3
```

### 3. Configure Windsurf MCP

Create `C:\Users\[NewUsername]\.codeium\windsurf\mcp_config.json`:

```json
{
  "mcpServers": {
    "archon": {
      "serverUrl": "http://192.168.1.92:8051/mcp"
    }
  }
}
```

### 4. Map Network Drives

```powershell
net use O: \\192.168.1.92\home /user:dennis Suzuki77wW!! /persistent:yes
net use P: \\192.168.1.92\projekter /user:dennis Suzuki77wW!! /persistent:yes
```

### 5. Test Everything

```powershell
# Test SSH
ssh ubuntu-server "whoami"

# Test Archon MCP (in Windsurf)
list projects

# Test network drives
Get-ChildItem P:\
```

---

## 📚 Related Documentation

- `F:\Archon\UBUNTU_MASTER_REFERENCE.md` - Complete reference
- `F:\Archon\UBUNTU_COMMANDS_REFERENCE.md` - All Ubuntu commands
- `F:\Archon\WINDSURF_REMOTE_SSH_GUIDE.md` - Remote SSH setup
- `F:\Archon\UBUNTU_DRIVES_RECONNECT.md` - Network drive guide
- `F:\Archon\LOCAL_SUPABASE_SETUP.md` - Supabase setup
- `F:\Archon\windsurfrules.md` - Archon workflow rules

---

**Last Updated**: 2025-11-22  
**Server**: Ubuntu 22.04 LTS @ 192.168.1.92  
**User**: dennis  
**Purpose**: Complete command reference for entire setup

---

## 🌐 All Frontend URLs & Web Interfaces

### Main Services

```yaml
# Archon
Archon UI (Main Interface): http://192.168.1.92:3737
  - Project management
  - Task tracking
  - Knowledge base
  - RAG search

Archon API Documentation: http://192.168.1.92:8181/docs
  - API endpoints
  - Interactive testing

# Supabase
Supabase Studio (Database UI): http://192.168.1.92:8888
  - Database tables
  - SQL editor
  - Authentication
  - Storage
  - API documentation

# Portainer
Portainer (Docker Management): http://192.168.1.92:9000
  - Container management
  - Images
  - Volumes
  - Networks
  - Logs viewer

# Ollama
Ollama API: http://192.168.1.92:11434
  - Model management
  - API testing
```

### Quick Access Links

```
Open in browser:
- Archon: http://192.168.1.92:3737
- Supabase: http://192.168.1.92:8888
- Portainer: http://192.168.1.92:9000
```

### Network Drive Access (Windows Explorer)

```
File Explorer paths:
- P:\ (Projects folder)
- O:\ (Full home directory)
- \\192.168.1.92\projekter
- \\192.168.1.92\home
```

---

## 🗄️ Supabase Management (SQL Commands)

### Access Supabase SQL Editor

**Via Web UI**: http://192.168.1.92:8888
1. Login with: supabase / this_password_is_insecure_and_should_be_updated
2. Click "SQL Editor" in left sidebar
3. Run any SQL command

**Via Command Line** (from Windows):

```powershell
# Interactive SQL session
ssh ubuntu "docker exec -it supabase-db psql -U postgres -d postgres"

# Run single SQL command
ssh ubuntu 'docker exec -i supabase-db psql -U postgres -d postgres -c "YOUR_SQL_HERE"'
```

### View Archon Tables

```sql
-- List all Archon tables
SELECT table_name 
FROM information_schema.tables 
WHERE table_schema = 'public' 
AND table_name LIKE 'archon_%'
ORDER BY table_name;

-- Show table structure
\d archon_projects
\d archon_tasks
\d archon_documents
\d archon_knowledge_sources
```

### Query Archon Data

```sql
-- List all projects
SELECT 
    id,
    title,
    description,
    created_at,
    updated_at
FROM archon_projects
ORDER BY created_at DESC;

-- List all tasks with project names
SELECT 
    t.id,
    t.title,
    t.status,
    t.assignee,
    p.title as project_name,
    t.created_at
FROM archon_tasks t
LEFT JOIN archon_projects p ON t.project_id = p.id
ORDER BY t.created_at DESC;

-- Count tasks by status
SELECT 
    status,
    COUNT(*) as count
FROM archon_tasks
GROUP BY status
ORDER BY count DESC;

-- List knowledge sources
SELECT 
    id,
    title,
    url,
    source_type,
    created_at
FROM archon_knowledge_sources
ORDER BY created_at DESC;
```

### Create New Project (SQL)

```sql
-- Create a new project
INSERT INTO archon_projects (
    id,
    title,
    description,
    created_at,
    updated_at
) VALUES (
    gen_random_uuid(),
    'My New Project',
    'Project description here',
    NOW(),
    NOW()
)
RETURNING id, title;
```

### Create New Task (SQL)

```sql
-- Create a new task
INSERT INTO archon_tasks (
    id,
    project_id,
    title,
    description,
    status,
    assignee,
    task_order,
    created_at,
    updated_at
) VALUES (
    gen_random_uuid(),
    'YOUR_PROJECT_ID_HERE',
    'Task title',
    'Task description',
    'todo',
    'User',
    10,
    NOW(),
    NOW()
)
RETURNING id, title, status;
```

### Update Task Status

```sql
-- Update task to 'doing'
UPDATE archon_tasks
SET 
    status = 'doing',
    updated_at = NOW()
WHERE id = 'YOUR_TASK_ID_HERE'
RETURNING id, title, status;

-- Mark task as done
UPDATE archon_tasks
SET 
    status = 'done',
    updated_at = NOW()
WHERE id = 'YOUR_TASK_ID_HERE'
RETURNING id, title, status;
```

### Delete Data

```sql
-- Delete a task
DELETE FROM archon_tasks
WHERE id = 'YOUR_TASK_ID_HERE'
RETURNING title;

-- Delete a project (and all its tasks)
DELETE FROM archon_tasks WHERE project_id = 'YOUR_PROJECT_ID_HERE';
DELETE FROM archon_projects WHERE id = 'YOUR_PROJECT_ID_HERE'
RETURNING title;
```

### Database Maintenance

```sql
-- Show database size
SELECT 
    pg_size_pretty(pg_database_size(current_database())) as database_size;

-- Show table sizes
SELECT 
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;

-- Show row counts
SELECT 
    schemaname,
    tablename,
    n_live_tup as row_count
FROM pg_stat_user_tables
WHERE schemaname = 'public'
ORDER BY n_live_tup DESC;

-- Vacuum database (cleanup)
VACUUM ANALYZE;
```

### Backup Specific Tables

```bash
# Backup projects table
ssh ubuntu "docker exec supabase-db pg_dump -U postgres -t archon_projects postgres > ~/backups/projects_$(date +%Y%m%d).sql"

# Backup tasks table
ssh ubuntu "docker exec supabase-db pg_dump -U postgres -t archon_tasks postgres > ~/backups/tasks_$(date +%Y%m%d).sql"

# Backup all Archon tables
ssh ubuntu "docker exec supabase-db pg_dump -U postgres -t 'archon_*' postgres > ~/backups/archon_all_$(date +%Y%m%d).sql"
```

### Restore from Backup

```bash
# Restore specific table
ssh ubuntu "docker exec -i supabase-db psql -U postgres -d postgres < ~/backups/projects_20250122.sql"

# Restore all tables
ssh ubuntu "docker exec -i supabase-db psql -U postgres -d postgres < ~/backups/archon_all_20250122.sql"
```

---

## 🔧 Supabase Management from Windows

### Run SQL from PowerShell

```powershell
# Single query
$sql = "SELECT COUNT(*) FROM archon_projects;"
ssh ubuntu "docker exec -i supabase-db psql -U postgres -d postgres -c `"$sql`""

# Multiple queries
$sql = @"
SELECT 'Projects:' as type, COUNT(*) as count FROM archon_projects
UNION ALL
SELECT 'Tasks:' as type, COUNT(*) as count FROM archon_tasks;
"@
ssh ubuntu "docker exec -i supabase-db psql -U postgres -d postgres -c `"$sql`""
```

### Create Project from PowerShell

```powershell
$projectTitle = "My New Project"
$projectDesc = "Description here"
$sql = "INSERT INTO archon_projects (id, title, description, created_at, updated_at) VALUES (gen_random_uuid(), '$projectTitle', '$projectDesc', NOW(), NOW()) RETURNING id, title;"
ssh ubuntu "docker exec -i supabase-db psql -U postgres -d postgres -c `"$sql`""
```

### Create Task from PowerShell

```powershell
$projectId = "YOUR_PROJECT_ID"
$taskTitle = "My New Task"
$sql = "INSERT INTO archon_tasks (id, project_id, title, status, assignee, created_at, updated_at) VALUES (gen_random_uuid(), '$projectId', '$taskTitle', 'todo', 'User', NOW(), NOW()) RETURNING id, title;"
ssh ubuntu "docker exec -i supabase-db psql -U postgres -d postgres -c `"$sql`""
```

### Query Data from PowerShell

```powershell
# Get all projects
ssh ubuntu 'docker exec -i supabase-db psql -U postgres -d postgres -c "SELECT id, title FROM archon_projects ORDER BY created_at DESC;"'

# Get tasks for specific project
$projectId = "YOUR_PROJECT_ID"
ssh ubuntu "docker exec -i supabase-db psql -U postgres -d postgres -c `"SELECT id, title, status FROM archon_tasks WHERE project_id = '$projectId';`""
```

---

## 📊 Useful SQL Queries

### Project Statistics

```sql
-- Project with most tasks
SELECT 
    p.title,
    COUNT(t.id) as task_count
FROM archon_projects p
LEFT JOIN archon_tasks t ON p.id = t.project_id
GROUP BY p.id, p.title
ORDER BY task_count DESC
LIMIT 10;

-- Tasks by status per project
SELECT 
    p.title as project,
    t.status,
    COUNT(*) as count
FROM archon_projects p
LEFT JOIN archon_tasks t ON p.id = t.project_id
GROUP BY p.title, t.status
ORDER BY p.title, t.status;
```

### Recent Activity

```sql
-- Recently created projects
SELECT 
    title,
    created_at,
    AGE(NOW(), created_at) as age
FROM archon_projects
ORDER BY created_at DESC
LIMIT 10;

-- Recently updated tasks
SELECT 
    t.title,
    t.status,
    p.title as project,
    t.updated_at,
    AGE(NOW(), t.updated_at) as last_update
FROM archon_tasks t
LEFT JOIN archon_projects p ON t.project_id = p.id
ORDER BY t.updated_at DESC
LIMIT 10;
```

### Search

```sql
-- Search projects by title
SELECT id, title, description
FROM archon_projects
WHERE title ILIKE '%search_term%'
   OR description ILIKE '%search_term%'
ORDER BY created_at DESC;

-- Search tasks by title
SELECT t.id, t.title, t.status, p.title as project
FROM archon_tasks t
LEFT JOIN archon_projects p ON t.project_id = p.id
WHERE t.title ILIKE '%search_term%'
   OR t.description ILIKE '%search_term%'
ORDER BY t.created_at DESC;
```

---


---


# Ubuntu Server - Complete Master Reference

**Everything you need to know about your Ubuntu server setup**

---

## 🔐 Server Information

| Item | Value |
|------|-------|
| **Server IP** | 192.168.1.92 |
| **Hostname** | dennis-Virtual-Machine |
| **OS** | Ubuntu 22.04 LTS |
| **Username** | dennis |
| **SSH Access** | Passwordless (SSH key authentication) |
| **Samba Password** | Suzuki77wW!! |

---

## 🔑 SSH Setup (Passwordless Login)

### How It Works
Your Windows machine has an SSH key that Ubuntu trusts, so you don't need to type a password.

### SSH Key Location (Windows)
```
C:\Users\server\.ssh\id_rsa (private key)
C:\Users\server\.ssh\id_rsa.pub (public key)
```

### Connect from Windows
```powershell
# Simple connection
ssh dennis@192.168.1.92

# Run single command
ssh dennis@192.168.1.92 "ls -la"

# Run multiple commands
ssh dennis@192.168.1.92 "cd ~/projekter && ls -la"
```

### Setup SSH on New Machine

If you need to set up SSH on a new Windows machine:

```powershell
# 1. Generate SSH key (if you don't have one)
ssh-keygen -t rsa -b 4096

# 2. Copy public key to Ubuntu
type $env:USERPROFILE\.ssh\id_rsa.pub | ssh dennis@192.168.1.92 "mkdir -p ~/.ssh && cat >> ~/.ssh/authorized_keys"

# 3. Set correct permissions on Ubuntu
ssh dennis@192.168.1.92 "chmod 700 ~/.ssh && chmod 600 ~/.ssh/authorized_keys"

# 4. Test passwordless connection
ssh dennis@192.168.1.92 "whoami"
```

### Copy SSH Key to New Machine

If you want to use the same SSH key on a new Windows machine:

```powershell
# Copy these files from old machine to new machine:
# C:\Users\server\.ssh\id_rsa
# C:\Users\server\.ssh\id_rsa.pub

# Place them in the same location on new machine:
# C:\Users\[YourUsername]\.ssh\
```

---

## 📁 Network Drives (Samba)

### Mounted Drives

| Drive | Path | Contents |
|-------|------|----------|
| **O:** | `\\192.168.1.92\home` | Full Ubuntu home directory |
| **P:** | `\\192.168.1.92\projekter` | Your projects folder |

### Reconnect Drives

**Desktop Shortcut**: "Reconnect Ubuntu Drives"

**Manual Command**:
```powershell
net use O: \\192.168.1.92\home /user:dennis Suzuki77wW!! /persistent:yes
net use P: \\192.168.1.92\projekter /user:dennis Suzuki77wW!! /persistent:yes
```

**Batch File**: `C:\Users\server\reconnect-ubuntu-drives-with-credentials.bat`

### Samba Configuration (Ubuntu)

**Config File**: `/etc/samba/smb.conf`

**Shares**:
```ini
[projekter]
   comment = Dennis Projects Folder
   path = /home/dennis/projekter
   browseable = yes
   read only = no
   writable = yes
   guest ok = no
   valid users = dennis
   create mask = 0755
   directory mask = 0755

[home]
   comment = Dennis Home Directory  
   path = /home/dennis
   browseable = yes
   read only = no
   writable = yes
   guest ok = no
   valid users = dennis
   create mask = 0755
   directory mask = 0755
```

**Manage Samba**:
```bash
# Restart Samba
sudo systemctl restart smbd nmbd

# Check status
sudo systemctl status smbd

# Change Samba password
sudo smbpasswd -a dennis

# View logs
sudo tail -f /var/log/samba/log.smbd
```

---

## 🗄️ Supabase (Local)

### Access Information

| Item | Value |
|------|-------|
| **Supabase Studio** | http://192.168.1.92:8888 |
| **API URL** | http://192.168.1.92:8888 |
| **Database Host** | 192.168.1.92 |
| **Database Port** | 5432 |
| **Database Name** | postgres |
| **Database User** | postgres |
| **Database Password** | postgres |
| **Studio Username** | supabase |
| **Studio Password** | this_password_is_insecure_and_should_be_updated |

### Supabase Keys

**Anon Key**:
```
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyAgCiAgICAicm9sZSI6ICJhbm9uIiwKICAgICJpc3MiOiAic3VwYWJhc2UtZGVtbyIsCiAgICAiaWF0IjogMTY0MTc2OTIwMCwKICAgICJleHAiOiAxNzk5NTM1NjAwCn0.dc_X5iR_VP_qT0zsiyj_I_OZ2T9FtRU2BBNWN8Bu4GE
```

**Service Role Key**:
```
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyAgCiAgICAicm9sZSI6ICJzZXJ2aWNlX3JvbGUiLAogICAgImlzcyI6ICJzdXBhYmFzZS1kZW1vIiwKICAgICJpYXQiOiAxNjQxNzY5MjAwLAogICAgImV4cCI6IDE3OTk1MzU2MDAKfQ.DaYlNEoUrrEn2Ig7tqibS-PHK5vgusbcbo7X36XVt4Q
```

### Supabase Location

**Directory**: `~/supabase-local/`

**Full Path**: `/home/dennis/supabase-local/`

**Windows Path**: `O:\supabase-local\`

### Supabase Docker Commands

```bash
# Navigate to Supabase directory
cd ~/supabase-local

# Start Supabase
docker compose up -d

# Stop Supabase
docker compose down

# View logs
docker compose logs -f

# Restart specific service
docker compose restart supabase-db

# Check status
docker compose ps
```

### Database Access

**From Windows (via SSH)**:
```powershell
# Connect to database
ssh dennis@192.168.1.92 "docker exec -it supabase-db psql -U postgres -d postgres"

# Run single query
ssh dennis@192.168.1.92 'docker exec -i supabase-db psql -U postgres -d postgres -c "SELECT * FROM archon_projects;"'

# List all tables
ssh dennis@192.168.1.92 'docker exec -i supabase-db psql -U postgres -d postgres -c "\dt"'
```

**Direct Connection (from any SQL client)**:
```
Host: 192.168.1.92
Port: 5432
Database: postgres
Username: postgres
Password: postgres
```

### Common Database Queries

```sql
-- List all projects
SELECT id, title, created_at FROM archon_projects;

-- List all tasks
SELECT id, title, status FROM archon_tasks;

-- Count projects
SELECT COUNT(*) FROM archon_projects;

-- Show database size
SELECT pg_size_pretty(pg_database_size(current_database()));

-- List all tables
\dt

-- Describe table
\d archon_projects
```

### Backup & Restore

```bash
# Backup database
ssh dennis@192.168.1.92 "docker exec supabase-db pg_dump -U postgres postgres > ~/backups/supabase_$(date +%Y%m%d).sql"

# Restore database
ssh dennis@192.168.1.92 "docker exec -i supabase-db psql -U postgres -d postgres < ~/backups/supabase_20250122.sql"
```

---

## 🏗️ Archon

### Access Information

| Service | URL |
|---------|-----|
| **Archon UI** | http://192.168.1.92:3737 |
| **Archon API** | http://192.168.1.92:8181 |
| **Archon MCP** | http://192.168.1.92:8051/mcp |

### Archon Location

**Directory**: `~/projects/archon/`

**Full Path**: `/home/dennis/projects/archon/`

**Windows Path**: `O:\projects\archon\`

### Archon Configuration

**Environment File**: `~/projects/archon/.env`

```bash
SUPABASE_URL=http://192.168.1.92:8888
SUPABASE_SERVICE_KEY=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyAgCiAgICAicm9sZSI6ICJzZXJ2aWNlX3JvbGUiLAogICAgImlzcyI6ICJzdXBhYmFzZS1kZW1vIiwKICAgICJpYXQiOiAxNjQxNzY5MjAwLAogICAgImV4cCI6IDE3OTk1MzU2MDAKfQ.DaYlNEoUrrEn2Ig7tqibS-PHK5vgusbcbo7X36XVt4Q
ARCHON_SERVER_PORT=8181
ARCHON_MCP_PORT=8051
ARCHON_UI_PORT=3737
OLLAMA_BASE_URL=http://192.168.1.92:11434
```

### Archon Docker Commands

```bash
# Navigate to Archon directory
cd ~/projects/archon

# Start all services
docker compose up -d

# Stop all services
docker compose down

# Restart all services
docker compose restart

# Restart specific service
docker compose restart archon-mcp

# Rebuild after code changes
docker compose up --build -d

# View logs
docker compose logs -f archon-mcp
docker compose logs -f archon-server
docker compose logs -f archon-ui

# Check status
docker compose ps
```

### Archon Services

| Container | Port | Description |
|-----------|------|-------------|
| archon-mcp | 8051 | MCP server for Windsurf |
| archon-server | 8181 | API server |
| archon-ui | 3737 | Web interface |

### Check Archon Health

```powershell
# From Windows
ssh dennis@192.168.1.92 "wget -qO- http://localhost:8181/health"

# Check MCP
ssh dennis@192.168.1.92 "docker logs archon-mcp --tail 20"

# Check API
ssh dennis@192.168.1.92 "docker logs archon-server --tail 20"
```

---

## 🤖 Ollama (LLM & Embeddings)

### Access Information

| Item | Value |
|------|-------|
| **Ollama API** | http://192.168.1.92:11434 |
| **LLM Model** | llama3.2:latest |
| **Embedding Model** | nomic-embed-text:latest |

### Ollama Commands

```bash
# Check Ollama status
ssh dennis@192.168.1.92 "systemctl status ollama"

# List installed models
ssh dennis@192.168.1.92 "ollama list"

# Pull new model
ssh dennis@192.168.1.92 "ollama pull llama3.2"

# Test Ollama
ssh dennis@192.168.1.92 "curl http://localhost:11434/api/tags"

# Restart Ollama
ssh dennis@192.168.1.92 "sudo systemctl restart ollama"
```

### Ollama in Archon

Archon uses Ollama for:
- **LLM**: Text generation and chat
- **Embeddings**: Vector search in knowledge base

Configuration is stored in Supabase `archon_settings` table.

---

## 🐳 Docker Management

### View All Containers

```bash
# List running containers
ssh dennis@192.168.1.92 "docker ps"

# List all containers (including stopped)
ssh dennis@192.168.1.92 "docker ps -a"

# Formatted view
ssh dennis@192.168.1.92 "docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'"
```

### Container Logs

```bash
# View recent logs
ssh dennis@192.168.1.92 "docker logs archon-mcp --tail 50"

# Follow logs in real-time
ssh dennis@192.168.1.92 "docker logs -f archon-mcp"

# Logs from last 30 minutes
ssh dennis@192.168.1.92 "docker logs archon-mcp --since 30m"
```

### Container Management

```bash
# Stop container
ssh dennis@192.168.1.92 "docker stop archon-mcp"

# Start container
ssh dennis@192.168.1.92 "docker start archon-mcp"

# Restart container
ssh dennis@192.168.1.92 "docker restart archon-mcp"

# Execute command in container
ssh dennis@192.168.1.92 "docker exec archon-mcp env | grep SUPABASE"

# Interactive shell
ssh dennis@192.168.1.92 "docker exec -it archon-mcp /bin/bash"
```

### Docker System

```bash
# Check disk usage
ssh dennis@192.168.1.92 "docker system df"

# Clean up
ssh dennis@192.168.1.92 "docker system prune -a -f"

# Check container resources
ssh dennis@192.168.1.92 "docker stats --no-stream"
```

---

## 🌐 Portainer

### Access Information

| Item | Value |
|------|-------|
| **Portainer UI** | http://192.168.1.92:9000 |
| **API Token** | ptr_XxKkdO1CQy8QyF1FGx0lymIj3/sPl2iEthNBNltrMAY= |
| **Endpoint ID** | 3 |

### Portainer Commands

```bash
# Check Portainer status
ssh dennis@192.168.1.92 "docker ps | grep portainer"

# Restart Portainer
ssh dennis@192.168.1.92 "docker restart portainer"

# View logs
ssh dennis@192.168.1.92 "docker logs portainer"
```

---

## 💻 Windsurf MCP Configuration

### MCP Config File Location

**Windows**: `C:\Users\server\.codeium\windsurf\mcp_config.json`

### Archon MCP Configuration

```json
{
  "mcpServers": {
    "archon": {
      "serverUrl": "http://192.168.1.92:8051/mcp"
    }
  }
}
```

### Windsurf Settings File

**Windows**: `C:\Users\server\AppData\Roaming\Windsurf\User\settings.json`

```json
{
  "mcpServers": {
    "archon": {
      "serverUrl": "http://192.168.1.92:8051/mcp"
    }
  }
}
```

### Test Archon MCP

```powershell
# In Windsurf, run:
list projects
```

---

## 📂 Directory Structure

### Ubuntu Directories

```
/home/dennis/
├── projekter/              # Your projects (P: drive)
│   └── Remote Desktop/     # Remote Desktop project
├── projects/
│   └── archon/            # Archon installation
├── supabase-local/        # Local Supabase
├── Desktop/
├── Documents/
├── Downloads/
└── ...
```

### Windows Access

```
O:\ = /home/dennis/         (Full home directory)
P:\ = /home/dennis/projekter/  (Projects only)
```

---

## 🔧 System Management

### Check System Status

```bash
# Disk space
ssh dennis@192.168.1.92 "df -h"

# Memory usage
ssh dennis@192.168.1.92 "free -h"

# CPU usage
ssh dennis@192.168.1.92 "top -bn1 | head -20"

# Running processes
ssh dennis@192.168.1.92 "ps aux | grep -E '(docker|ollama|samba)'"
```

### Network & Ports

```bash
# Check listening ports
ssh dennis@192.168.1.92 "sudo netstat -tulpn | grep LISTEN"

# Check specific port
ssh dennis@192.168.1.92 "sudo netstat -tulpn | grep 8051"
```

**From Windows**:
```powershell
# Test port connectivity
Test-NetConnection -ComputerName 192.168.1.92 -Port 8051
Test-NetConnection -ComputerName 192.168.1.92 -Port 8888
Test-NetConnection -ComputerName 192.168.1.92 -Port 3737
```

### Service Management

```bash
# Docker
ssh dennis@192.168.1.92 "sudo systemctl status docker"
ssh dennis@192.168.1.92 "sudo systemctl restart docker"

# Ollama
ssh dennis@192.168.1.92 "systemctl status ollama"
ssh dennis@192.168.1.92 "sudo systemctl restart ollama"

# Samba
ssh dennis@192.168.1.92 "sudo systemctl status smbd"
ssh dennis@192.168.1.92 "sudo systemctl restart smbd nmbd"
```

---

## 🚀 Quick Start Commands

### Start Everything

```bash
# SSH into Ubuntu
ssh dennis@192.168.1.92

# Start Supabase
cd ~/supabase-local && docker compose up -d

# Start Archon
cd ~/projects/archon && docker compose up -d

# Check all services
docker ps
```

### Stop Everything

```bash
# Stop Archon
cd ~/projects/archon && docker compose down

# Stop Supabase
cd ~/supabase-local && docker compose down
```

### Restart Everything

```bash
# Restart Archon
cd ~/projects/archon && docker compose restart

# Restart Supabase
cd ~/supabase-local && docker compose restart
```

---

## 🆘 Troubleshooting

### Can't SSH

```powershell
# Test connection
ping 192.168.1.92

# Check SSH key
Test-Path $env:USERPROFILE\.ssh\id_rsa

# Try with password
ssh -o PubkeyAuthentication=no dennis@192.168.1.92
```

### Network Drives Not Working

```powershell
# Test Samba port
Test-NetConnection -ComputerName 192.168.1.92 -Port 445

# Reconnect drives
C:\Users\server\reconnect-ubuntu-drives-with-credentials.bat

# Check Samba on Ubuntu
ssh dennis@192.168.1.92 "sudo systemctl status smbd"
```

### Archon MCP Not Working

```bash
# Check MCP container
ssh dennis@192.168.1.92 "docker ps | grep archon-mcp"

# Check logs
ssh dennis@192.168.1.92 "docker logs archon-mcp --tail 50"

# Restart MCP
ssh dennis@192.168.1.92 "cd ~/projects/archon && docker compose restart archon-mcp"

# Check environment
ssh dennis@192.168.1.92 "docker exec archon-mcp env | grep -E '(SUPABASE|API)'"
```

### Supabase Not Accessible

```bash
# Check Supabase containers
ssh dennis@192.168.1.92 "docker ps | grep supabase"

# Check database
ssh dennis@192.168.1.92 'docker exec -i supabase-db psql -U postgres -d postgres -c "SELECT 1;"'

# Restart Supabase
ssh dennis@192.168.1.92 "cd ~/supabase-local && docker compose restart"
```

### Ollama Not Responding

```bash
# Check Ollama status
ssh dennis@192.168.1.92 "systemctl status ollama"

# Test Ollama
ssh dennis@192.168.1.92 "curl http://localhost:11434/api/tags"

# Restart Ollama
ssh dennis@192.168.1.92 "sudo systemctl restart ollama"
```

---

## 📋 All URLs & Endpoints

| Service | URL | Purpose |
|---------|-----|---------|
| **Archon UI** | http://192.168.1.92:3737 | Web interface |
| **Archon API** | http://192.168.1.92:8181 | REST API |
| **Archon MCP** | http://192.168.1.92:8051/mcp | MCP for Windsurf |
| **Supabase Studio** | http://192.168.1.92:8888 | Database UI |
| **Supabase API** | http://192.168.1.92:8888 | REST API |
| **PostgreSQL** | 192.168.1.92:5432 | Direct DB access |
| **Portainer** | http://192.168.1.92:9000 | Docker UI |
| **Ollama** | http://192.168.1.92:11434 | LLM API |
| **Samba (projekter)** | \\192.168.1.92\projekter | Network drive |
| **Samba (home)** | \\192.168.1.92\home | Network drive |

---

## 🔐 All Credentials

| Service | Username | Password/Key |
|---------|----------|--------------|
| **Ubuntu SSH** | dennis | (SSH key - no password) |
| **Samba** | dennis | Suzuki77wW!! |
| **Supabase Studio** | supabase | this_password_is_insecure_and_should_be_updated |
| **PostgreSQL** | postgres | postgres |
| **Portainer** | admin | (set during first login) |

---

## 📦 For New Windsurf Instance

### 1. Copy SSH Key

```powershell
# Copy from old machine:
C:\Users\server\.ssh\id_rsa
C:\Users\server\.ssh\id_rsa.pub

# To new machine:
C:\Users\[YourUsername]\.ssh\
```

### 2. Configure Windsurf MCP

Create/edit: `C:\Users\[YourUsername]\.codeium\windsurf\mcp_config.json`

```json
{
  "mcpServers": {
    "archon": {
      "serverUrl": "http://192.168.1.92:8051/mcp"
    },
    "memory": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-memory"],
      "env": {}
    },
    "sequential-thinking": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-sequential-thinking"],
      "env": {}
    },
    "puppeteer": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-puppeteer"],
      "env": {}
    },
    "context7": {
      "command": "npx",
      "args": ["-y", "@upstash/context7-mcp"]
    }
  }
}
```

### 3. Map Network Drives

```powershell
net use O: \\192.168.1.92\home /user:dennis Suzuki77wW!! /persistent:yes
net use P: \\192.168.1.92\projekter /user:dennis Suzuki77wW!! /persistent:yes
```

### 4. Test Connection

```powershell
# Test SSH
ssh dennis@192.168.1.92 "whoami"

# Test Archon MCP (in Windsurf)
list projects

# Test network drives
Get-ChildItem P:\
```

---

## 📚 Related Documentation Files

- `F:\Archon\UBUNTU_COMMANDS_REFERENCE.md` - All Ubuntu commands
- `F:\Archon\UBUNTU_DRIVES_RECONNECT.md` - Network drive reconnection
- `F:\Archon\MOUNT_UBUNTU_FOLDER_GUIDE.md` - Samba setup guide
- `F:\Archon\LOCAL_SUPABASE_SETUP.md` - Supabase setup guide
- `F:\Archon\UBUNTU_ARCHON_COMPLETE_SETUP.md` - Archon setup guide
- `F:\Archon\windsurfrules.md` - Archon workflow rules

---

**Last Updated**: 2025-12-01  
**Server**: Ubuntu 22.04 LTS @ 192.168.1.92  
**User**: dennis  
**Created for**: Easy setup on new Windsurf instances

---

## 🐳 DOCKER SERVICES OVERVIEW

### All Running Containers
```bash
# Check all containers
ssh ubuntu "docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'"
```

### Service Groups

**Caddy** (~/caddy/)
```
caddy           Ports: 80, 443 (HTTPS auto via Let's Encrypt)
```

**Supabase** (~/supabase-local/)
```
supabase-kong               Port: 8888 (API Gateway)
supabase-studio             Port: 3000 (internal)
supabase-db                 Port: 5432 (PostgreSQL)
supabase-auth               (GoTrue auth)
supabase-rest               (PostgREST)
supabase-realtime           (Realtime subscriptions)
supabase-storage            (File storage)
supabase-edge-functions     (Edge Functions runtime)
supabase-meta               (Metadata)
supabase-analytics          Port: 4000
supabase-imgproxy           (Image processing)
supabase-vector             (Vector/logging)
supabase-pooler             Ports: 5432, 6543
```

**Archon** (~/projects/archon/)
```
archon-mcp                  Port: 8051 (MCP server)
archon-server               Port: 8181 (API)
archon-ui                   Port: 3737 (Web UI)
```

**Portainer** (standalone)
```
portainer                   Port: 9000
```

### Start All Services (One Command)
```bash
ssh ubuntu "docker start caddy && cd ~/supabase-local && docker compose up -d && cd ~/projects/archon && docker compose up -d"
```

### Stop All Services
```bash
ssh ubuntu "cd ~/projects/archon && docker compose down && cd ~/supabase-local && docker compose down && docker stop caddy"
```

---

## 📡 API ENDPOINTS REFERENCE

### Supabase API
```yaml
Base URL: https://supabase.hawkeye123.dk (public)
         http://192.168.1.92:8888 (local)

# REST API
GET/POST/PATCH/DELETE: /rest/v1/{table_name}

# Edge Functions
POST: /functions/v1/{function_name}

# Auth
POST: /auth/v1/signup
POST: /auth/v1/token?grant_type=password
POST: /auth/v1/logout

# Headers Required
apikey: {ANON_KEY}
Authorization: Bearer {JWT_TOKEN}
Content-Type: application/json
```

### Supabase Keys
```bash
# Anon Key (public, safe to expose)
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyAgCiAgICAicm9sZSI6ICJhbm9uIiwKICAgICJpc3MiOiAic3VwYWJhc2UtZGVtbyIsCiAgICAiaWF0IjogMTY0MTc2OTIwMCwKICAgICJleHAiOiAxNzk5NTM1NjAwCn0.dc_X5iR_VP_qT0zsiyj_I_OZ2T9FtRU2BBNWN8Bu4GE

# Service Role Key (SECRET - never expose!)
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyAgCiAgICAicm9sZSI6ICJzZXJ2aWNlX3JvbGUiLAogICAgImlzcyI6ICJzdXBhYmFzZS1kZW1vIiwKICAgICJpYXQiOiAxNjQxNzY5MjAwLAogICAgImV4cCI6IDE3OTk1MzU2MDAKfQ.DaYlNEoUrrEn2Ig7tqibS-PHK5vgusbcbo7X36XVt4Q
```

### Archon API
```yaml
Base URL: http://192.168.1.92:8181

# Health Check
GET: /health

# Projects
GET: /api/projects
POST: /api/projects
GET: /api/projects/{id}

# Tasks
GET: /api/tasks
POST: /api/tasks
PATCH: /api/tasks/{id}
```

### Archon MCP
```yaml
MCP Endpoint: http://192.168.1.92:8051/mcp

# Available Tools (via Windsurf)
- find_projects
- manage_project
- find_tasks
- manage_task
- find_documents
- manage_document
- rag_search_knowledge_base
- rag_search_code_examples
- health_check
```

### Portainer API
```yaml
Base URL: http://192.168.1.92:9000/api

# Headers
X-API-Key: ptr_XxKkdO1CQy8QyF1FGx0lymIj3/sPl2iEthNBNltrMAY=

# Endpoints
GET: /endpoints
GET: /endpoints/{id}/docker/containers/json
POST: /endpoints/{id}/docker/containers/{id}/start
POST: /endpoints/{id}/docker/containers/{id}/stop
```

---

## 🔨 BUILD & RELEASE (EXE Files)

### Lokal Build (Denne Maskine)

#### Prerequisites
```bash
# Kræver Go 1.21+ og MinGW (GCC) for agent
# Installer MinGW: choco install mingw
```

#### Build Agent (remote-agent.exe)
```powershell
cd f:\#Remote\agent
.\build.bat

# Eller manuelt:
set CGO_ENABLED=1
go build -ldflags "-s -w" -o remote-agent.exe ./cmd/remote-agent
```

#### Build Controller (controller.exe)
```powershell
cd f:\#Remote\controller
.\build.bat

# Eller manuelt:
go build -ldflags "-s -w -H windowsgui" -o controller.exe .
```

#### Build Begge
```powershell
# Agent
cd f:\#Remote\agent && .\build.bat

# Controller
cd f:\#Remote\controller && .\build.bat
```

---

### GitHub Actions (Automatisk Release)

#### Trigger Release
```powershell
# 1. Tag ny version
cd f:\#Remote
git tag v2.5.0

# 2. Push tag til GitHub
git push origin v2.5.0

# 3. GitHub Actions bygger automatisk og laver release
```

#### Workflow Files
```
.github/workflows/
├── release.yml           # Bygger BEGGE (agent + controller)
├── release-agent.yml     # Kun agent
├── release-controller.yml # Kun controller
└── build-controller.yml  # Test build
```

#### Hvad Sker Der?
1. Push tag `v*` → Trigger workflow
2. GitHub Actions:
   - Checkout kode
   - Setup Go 1.21
   - Install MinGW (for agent CGO)
   - Build agent + controller
   - Create GitHub Release
   - Upload .exe filer

#### Download Releases
```
https://github.com/stangtennis/Remote/releases
```

---

### Build Flags Forklaret

| Flag | Betydning |
|------|-----------|
| `-ldflags "-s -w"` | Strip debug info (mindre exe) |
| `-H windowsgui` | Ingen konsol vindue (kun controller) |
| `CGO_ENABLED=1` | Kræves for robotgo (agent) |

---

### Hurtig Reference

| Handling | Kommando |
|----------|----------|
| **Build agent lokalt** | `cd f:\#Remote\agent && .\build.bat` |
| **Build controller lokalt** | `cd f:\#Remote\controller && .\build.bat` |
| **Release via GitHub** | `git tag v2.5.0 && git push origin v2.5.0` |
| **Se releases** | https://github.com/stangtennis/Remote/releases |

---

---

## 🖥️ REMOTE DESKTOP APPLICATION

### Overview
WebRTC-based remote desktop solution with three components:
- **Agent** - Runs on remote machine (Windows), captures screen, handles input
- **Controller** - Desktop app (Windows) for controlling remote machines
- **Dashboard** - Web interface for controlling remote machines

### Current Version: v2.6.8 (2025-12-05)

### Repository
```yaml
GitHub: https://github.com/stangtennis/Remote
Dashboard: https://stangtennis.github.io/Remote/dashboard.html
Releases: https://github.com/stangtennis/Remote/releases
```

### Components Location
```yaml
Agent: f:\#Remote\agent\
Controller: f:\#Remote\controller\
Dashboard: f:\#Remote\docs\
Supabase Functions: f:\#Remote\supabase\functions\
```

### Build Commands
```powershell
# Build Agent
cd f:\#Remote\agent
go build -ldflags "-s -w" -o remote-agent.exe ./cmd/remote-agent

# Build Controller
cd f:\#Remote\controller
go build -ldflags "-s -w -H windowsgui" -o remote-controller.exe .

# Create GitHub Release
cd f:\#Remote
gh release create v2.6.8 agent/remote-agent-v2.6.8.exe --title "v2.6.8" --notes "Release notes"
```

### Supabase Configuration
```yaml
URL: https://supabase.hawkeye123.dk
Anon Key: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyAgCiAgICAicm9sZSI6ICJhbm9uIiwKICAgICJpc3MiOiAic3VwYWJhc2UtZGVtbyIsCiAgICAiaWF0IjogMTY0MTc2OTIwMCwKICAgICJleHAiOiAxNzk5NTM1NjAwCn0.dc_X5iR_VP_qT0zsiyj_I_OZ2T9FtRU2BBNWN8Bu4GE
```

### Database Tables
| Table | Purpose |
|-------|---------|
| `remote_devices` | Registered devices (agents) |
| `remote_sessions` | Active/pending sessions |
| `session_signaling` | WebRTC signaling messages |
| `webrtc_sessions` | Controller-to-agent sessions |

### Edge Functions
| Function | Purpose |
|----------|---------|
| `device-register` | Register new devices |
| `session-token` | Create sessions with PIN/TURN |
| `session-cleanup` | Cleanup old sessions |

---

### Recent Fixes (v2.6.x Series)

#### v2.6.8 - DPI Scaling Fix
- **Problem:** Mouse coordinates were wrong on high-DPI displays (125%, 150% scaling)
- **Cause:** `robotgo.Move()` has known DPI scaling issues on Windows
- **Fix:** Use Windows API `SetCursorPos` directly instead of robotgo
- **File:** `agent/internal/input/mouse.go`

#### v2.6.7 - Explicit Relative Coordinates
- **Problem:** Coordinate type detection was unreliable
- **Fix:** Dashboard sends `rel: true` flag with mouse events
- **Files:** `docs/js/webrtc.js`, `agent/internal/webrtc/peer.go`

#### v2.6.5 - Mouse Coordinate Conversion
- **Problem:** Dashboard sent relative coords (0-1), agent expected absolute
- **Fix:** Added `MoveRelative()` function for relative coordinates
- **File:** `agent/internal/input/mouse.go`

#### v2.6.3 - ICE Candidate Buffering
- **Problem:** ICE candidates sent before answer, causing connection failures
- **Fix:** Buffer ICE candidates until answer is sent
- **Files:** `agent/internal/webrtc/peer.go`, `agent/internal/webrtc/signaling.go`

#### v2.6.2 - ICE Candidate Format
- **Problem:** Browser error "sdpMid and sdpMLineIndex are both null"
- **Fix:** Ensure `sdpMid` and `sdpMLineIndex` are always included
- **File:** `agent/internal/webrtc/signaling.go`

---

### Features

#### ✅ Working Features
| Feature | Controller | Dashboard | Agent |
|---------|------------|-----------|-------|
| Screen Streaming | ✅ | ✅ | ✅ |
| Mouse Control | ✅ | ✅ | ✅ |
| Keyboard Input | ✅ | ✅ | ✅ |
| Clipboard Sync | ✅ | ✅ | ✅ |
| Cursor Hiding | - | - | ✅ |
| Device Registration | ✅ | ✅ | ✅ |
| Session PIN | ✅ | ✅ | ✅ |

#### Clipboard Sync
- **Agent → Controller/Dashboard:** Automatic (agent monitors clipboard)
- **Controller/Dashboard → Agent:** Ctrl+V sends local clipboard to agent
- **Files:**
  - `agent/internal/clipboard/monitor.go` - Monitors agent clipboard
  - `agent/internal/clipboard/receiver.go` - Receives clipboard from controller
  - `docs/js/webrtc.js` - Dashboard clipboard handling

#### Cursor Hiding
- Local cursor is hidden on agent when remote session is active
- Restored when session ends
- Uses Windows API `ShowCursor`
- **File:** `agent/internal/input/mouse.go`

---

### Architecture

```
┌─────────────────┐     WebRTC      ┌─────────────────┐
│   Controller    │◄───────────────►│     Agent       │
│   (Windows)     │                 │   (Windows)     │
└────────┬────────┘                 └────────┬────────┘
         │                                   │
         │ Supabase                          │ Supabase
         │ (Auth, Signaling)                 │ (Registration)
         │                                   │
         ▼                                   ▼
┌─────────────────────────────────────────────────────┐
│                    Supabase                          │
│  - Authentication (email/password)                   │
│  - Device registration (remote_devices)              │
│  - Session signaling (session_signaling)             │
│  - Edge Functions (device-register, session-token)   │
└─────────────────────────────────────────────────────┘
         ▲
         │ Supabase
         │ (Auth, Signaling)
         │
┌────────┴────────┐
│    Dashboard    │
│  (Web Browser)  │
└─────────────────┘
```

### WebRTC Flow
1. **Dashboard/Controller** creates session in Supabase
2. **Agent** polls for pending sessions
3. **Dashboard/Controller** sends WebRTC offer
4. **Agent** creates answer and sends back
5. **ICE candidates** exchanged for NAT traversal
6. **Data channel** opens for input events
7. **Video track** streams screen capture

---

### Troubleshooting

#### Mouse Not Working Correctly
1. Check agent version (must be v2.6.8+)
2. Check DPI scaling on agent machine
3. Verify `rel: true` flag in mouse events (dashboard)
4. Check agent logs for coordinate values

#### Connection Fails
1. Check Supabase is accessible
2. Verify agent is registered (`remote_devices` table)
3. Check browser console for signaling errors
4. Verify ICE candidates are being exchanged

#### Clipboard Not Syncing
1. Ensure "Enable Clipboard Sync" is on (Controller settings)
2. Check agent logs for clipboard messages
3. Browser must have clipboard permissions (dashboard)

#### Screen Capture Errors
- "DXGI capture failed: timeout" is normal when screen is static
- Errors increase when no screen changes occur
- Not a problem unless video freezes

---

### Version History

| Version | Date | Changes |
|---------|------|---------|
| v2.10.0 | 2025-12-06 | Session 0 desktop switch, file browser, 60 FPS streaming |
| v2.6.8 | 2025-12-05 | Fix DPI scaling with SetCursorPos API |
| v2.6.7 | 2025-12-04 | Explicit rel flag for coordinates |
| v2.6.6 | 2025-12-04 | Separate Move/MoveRelative functions |
| v2.6.5 | 2025-12-04 | Mouse coordinate conversion fix |
| v2.6.4 | 2025-12-04 | Cursor hiding during remote session |
| v2.6.3 | 2025-12-04 | ICE candidate buffering |
| v2.6.2 | 2025-12-04 | ICE candidate format fix |
| v2.6.1 | 2025-12-04 | Dashboard signaling improvements |
| v2.6.0 | 2025-12-04 | Web dashboard support |

---

## 🖥️ REMOTE DESKTOP APPLICATION - COMPLETE REFERENCE

### Current Version: v2.10.0

### Repository & Downloads
```yaml
GitHub: https://github.com/stangtennis/Remote
Releases: https://github.com/stangtennis/Remote/releases
Dashboard: https://stangtennis.github.io/Remote/
```

### Components
| Component | Description | Location |
|-----------|-------------|----------|
| **Agent** | Windows service/app on remote machine | `agent/` |
| **Controller** | Desktop app to control remote machines | `controller/` |
| **Dashboard** | Web interface for remote control | `docs/` |
| **Supabase** | Backend for auth, signaling, device management | `supabase/` |

### Build Commands
```powershell
# Build Agent (Windows)
cd F:\#Remote\agent
.\build.bat

# Build Controller (Windows)
cd F:\#Remote\controller
go build -o remote-controller.exe .

# Release to GitHub
cd F:\#Remote
Copy-Item agent\remote-agent.exe agent\remote-agent-v2.10.0.exe -Force
gh release upload v2.10.0 agent/remote-agent-v2.10.0.exe --clobber
```

### Agent Features (v2.10.0)
- **Screen Capture**: DXGI (fast) or GDI (Session 0 compatible)
- **Streaming**: 60 FPS JPEG over WebRTC data channel
- **Input**: Mouse and keyboard control via SendInput API
- **Clipboard**: Bidirectional text and image sync
- **File Transfer**: Chunked file transfer over data channel
- **Session 0 Support**: Desktop switch for login screen capture/input
- **Service Mode**: Can run as Windows service

### Controller Features (v2.10.0)
- **Fyne UI**: Native Windows desktop application
- **Multi-device**: Connect to multiple agents
- **File Browser**: Total Commander-style dual-pane file manager
- **Clipboard Sync**: Automatic clipboard sharing
- **FPS Display**: Real-time performance stats

### Dashboard Features
- **Web-based**: Works in any modern browser
- **Device Management**: View and manage registered devices
- **Admin Panel**: User approvals, device assignments, invitations
- **Remote Control**: Full mouse/keyboard control via browser

---

### Agent Installation

#### As Application (Development)
```powershell
# Run directly
cd F:\#Remote\agent
.\remote-agent.exe
```

#### As Windows Service (Production)
```powershell
# Install service
cd F:\#Remote\agent
.\install-service.bat

# Or manually:
sc create RemoteDesktopAgent binPath= "C:\Path\To\remote-agent.exe" start= auto obj= LocalSystem
sc start RemoteDesktopAgent
```

#### Startup on Login (Alternative)
```powershell
# Add to startup folder
cd F:\#Remote\agent
.\setup-startup.bat
```

---

### Supabase Database Schema

#### Tables
| Table | Purpose |
|-------|---------|
| `remote_devices` | Registered devices (agents) |
| `user_approvals` | User accounts and roles |
| `device_assignments` | Device-to-user assignments |
| `user_invitations` | Pending invitations |
| `device_transfers` | Device transfer history |
| `remote_sessions` | Active/past sessions |
| `audit_logs` | Security audit trail |

#### Key SQL Functions
```sql
-- Assign device to user (admin only)
SELECT assign_device('device_id', 'user_uuid', true, 'notes');

-- Send invitation (admin only)
SELECT send_invitation('email@example.com', 'user');

-- Transfer device (super_admin only)
SELECT transfer_device('device_id', 'to_user_uuid', 'reason');

-- Approve user
SELECT approve_user('user_uuid', 'Approved by admin');
```

#### Run SQL on Supabase
```powershell
# Create SQL file
echo "SELECT * FROM remote_devices;" > query.sql

# Copy and execute
scp query.sql dennis@192.168.1.92:/tmp/
ssh dennis@192.168.1.92 "docker cp /tmp/query.sql supabase-db:/tmp/ && docker exec supabase-db psql -U postgres -f /tmp/query.sql"
```

---

### User Roles

| Role | Permissions |
|------|-------------|
| `user` | View own devices, connect to assigned devices |
| `admin` | + Approve users, assign devices, send invitations |
| `super_admin` | + Transfer devices, manage admins |

### Admin Panel Access
1. Login to dashboard: https://stangtennis.github.io/Remote/
2. Click "🔐 Admin Panel" (only visible for admin/super_admin)
3. Manage users, devices, invitations

---

### Session 0 / Login Screen Support

#### How It Works
1. Agent detects Session 0 (no user logged in)
2. Uses GDI capture instead of DXGI
3. Calls `SwitchToInputDesktop()` before capture/input
4. Allows viewing and interacting with Windows login screen

#### Limitations
- **Ctrl+Alt+Del**: Not supported (requires SAS or DisableCAD)
- **UAC Prompts**: May not be capturable on secure desktop

#### Enable DisableCAD (Optional)
```
Registry: HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Policies\System
Value: DisableCAD = 1 (DWORD)
```

---

### Streaming Configuration

#### Agent Settings (peer.go)
```go
// Frame rate
ticker := time.NewTicker(16 * time.Millisecond) // 60 FPS

// JPEG quality (1-100)
jpeg, err := m.screenCapturer.CaptureJPEG(85)

// Buffer size before dropping frames
if m.dataChannel.BufferedAmount() > 16*1024*1024 { // 16MB
    // Drop frame
}
```

#### Adjust for Network
| Network | FPS | Quality | Buffer |
|---------|-----|---------|--------|
| LAN | 60 | 85 | 16MB |
| Fast WiFi | 30 | 70 | 8MB |
| Slow | 15 | 50 | 4MB |

---

### Troubleshooting

#### Agent Won't Start
```powershell
# Check if already running
tasklist | findstr remote-agent

# Check service status
sc query RemoteDesktopAgent

# View logs
Get-Content "$env:APPDATA\RemoteAgent\agent.log" -Tail 50
```

#### Black Screen / No Video
1. Check agent is running and registered
2. Verify device is online in dashboard
3. Check if Session 0 (login screen) - needs GDI mode
4. Restart agent

#### Mouse Not Working
1. Verify agent has admin privileges
2. Check DPI scaling settings
3. Try relative coordinates mode

#### Connection Failed
1. Check Supabase is accessible
2. Verify TURN server credentials
3. Check firewall rules

---

### Development Workflow

#### Make Changes
```powershell
# Edit code in F:\#Remote\

# Build
cd F:\#Remote\agent && .\build.bat

# Test locally
.\remote-agent.exe

# Commit and push
cd F:\#Remote
git add -A
git commit -m "Description of changes"
git push

# Release
Copy-Item agent\remote-agent.exe agent\remote-agent-v2.10.0.exe -Force
gh release upload v2.10.0 agent/remote-agent-v2.10.0.exe --clobber
```

#### Key Files
| File | Purpose |
|------|---------|
| `agent/internal/webrtc/peer.go` | WebRTC, streaming, input handling |
| `agent/internal/screen/capturer.go` | Screen capture (DXGI/GDI) |
| `agent/internal/input/mouse.go` | Mouse control |
| `agent/internal/input/keyboard.go` | Keyboard control |
| `agent/internal/desktop/desktop_windows.go` | Desktop switching |
| `controller/internal/viewer/viewer.go` | Controller UI |
| `docs/js/devices.js` | Dashboard device management |
| `docs/admin.html` | Admin panel |

---

## 🔄 CHANGELOG

### 2025-12-06
- ✅ Added Session 0 desktop switch for login screen support
- ✅ Implemented Total Commander-style file browser in controller
- ✅ Fixed admin panel to show for super_admin role
- ✅ Fixed device management (admins see all devices, claim button)
- ✅ Created send_invitation and transfer_device SQL functions
- ✅ Fixed admin panel field names (approved_at, is_online)
- ✅ Optimized streaming: 60 FPS, 85% quality, 16MB buffer
- ✅ Updated ULTIMATE_GUIDE with comprehensive Remote Desktop docs

### 2025-12-05
- ✅ Fixed mouse DPI scaling issues (v2.6.8)
- ✅ Added clipboard sync to dashboard
- ✅ Implemented relative coordinate flag
- ✅ Implemented file transfer crash fix

### 2025-12-04
- ✅ Fixed web dashboard connection issues
- ✅ Fixed ICE candidate format for browsers
- ✅ Added cursor hiding during remote session
- ✅ Fixed mouse coordinate conversion
- ✅ Multiple agent releases (v2.6.0 - v2.6.7)

### 2025-12-01
- ✅ Migrated to Caddy (from Nginx Proxy Manager)
- ✅ Implemented Supabase Edge Functions
- ✅ Updated agent to use Edge Functions for device registration
- ✅ Removed public access to Archon and Portainer (security)
- ✅ Only Supabase exposed publicly (required for Remote Desktop app)
- ✅ Updated all credentials and documentation

### 2025-11-30
- ✅ Set up Authelia 2FA (later removed - not needed)
- ✅ Configured wildcard SSL certificate
- ✅ Set up Caddy

### 2025-11-22
- ✅ Initial Ubuntu server setup
- ✅ Supabase local installation
- ✅ Archon installation and configuration
- ✅ Samba network drives
- ✅ SSH passwordless authentication
