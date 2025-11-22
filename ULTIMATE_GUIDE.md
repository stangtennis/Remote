# 🚀 ULTIMATE COMPLETE GUIDE
**Everything About Your Ubuntu + Archon + Windsurf Setup in ONE Document**

Last Updated: 2025-11-22 | Server: 192.168.1.92 | User: dennis

## QUICK ACCESS
- Archon UI: http://192.168.1.92:3737
- Supabase: http://192.168.1.92:8888
- Portainer: http://192.168.1.92:9000
- Drives: P:\ (projects) | O:\ (home)

---



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

**Last Updated**: 2025-11-22  
**Server**: Ubuntu 22.04 LTS @ 192.168.1.92  
**User**: dennis  
**Created for**: Easy setup on new Windsurf instances
