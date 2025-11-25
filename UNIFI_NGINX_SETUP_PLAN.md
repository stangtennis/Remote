# 🎯 Complete Setup Plan: Nginx + UniFi Dream Machine SE + hawkeye123.dk

**Your Network:**
- **Public IP:** 188.228.14.94
- **Router:** UniFi Dream Machine SE (192.168.1.1)
- **Fiber Box:** Nokia (WAN connection)
- **Ubuntu Server:** 192.168.1.92
- **Domain:** *.hawkeye123.dk (wildcard)

---

## 📊 Network Topology

```
Internet (188.228.14.94)
    ↓
Nokia Fiber Box
    ↓
UniFi Dream Machine SE (192.168.1.1) [WAN: 188.228.14.94, LAN: 192.168.1.0/24]
    ↓
Ubuntu Server (192.168.1.92)
    ↓
├─ Nginx Proxy Manager (ports 80, 443, 81)
├─ Supabase (port 8888)
├─ Archon (port 3737)
└─ Portainer (port 9000)
```

---

## ✅ Complete Step-by-Step Plan

### Phase 1: DNS Configuration (5 minutes)

**Goal:** Point your domain to your public IP

#### Step 1.1: Configure DNS Records

Go to your DNS provider (where you manage hawkeye123.dk) and add these A records:

```
Type    Name        Value           TTL
A       supabase    188.228.14.94   300
A       archon      188.228.14.94   300
A       portainer   188.228.14.94   300
A       remote      188.228.14.94   300
```

**Or use wildcard (if supported):**
```
Type    Name    Value           TTL
A       *       188.228.14.94   300
```

#### Step 1.2: Verify DNS Propagation

```bash
# From Windows (PowerShell)
nslookup supabase.hawkeye123.dk

# Should return: 188.228.14.94
```

**Wait 5-10 minutes for DNS to propagate.**

---

### Phase 2: UniFi Dream Machine SE Configuration (10 minutes)

**Goal:** Forward HTTP/HTTPS traffic to Ubuntu server

#### Step 2.1: Access UniFi Controller

1. Open browser: **https://192.168.1.1**
2. Login with your UniFi credentials
3. Go to **Settings** → **Routing & Firewall** → **Port Forwarding**

#### Step 2.2: Create Port Forwarding Rules

**Rule 1: HTTP (Port 80)**
- **Name:** `Nginx-HTTP`
- **Enabled:** ✅ Yes
- **From:** `Any` or `Internet (WAN)`
- **Port:** `80`
- **Forward IP:** `192.168.1.92`
- **Forward Port:** `80`
- **Protocol:** `TCP`

**Rule 2: HTTPS (Port 443)**
- **Name:** `Nginx-HTTPS`
- **Enabled:** ✅ Yes
- **From:** `Any` or `Internet (WAN)`
- **Port:** `443`
- **Forward IP:** `192.168.1.92`
- **Forward Port:** `443`
- **Protocol:** `TCP`

#### Step 2.3: Verify Port Forwarding

```bash
# From external network (use phone hotspot or ask friend)
curl -I http://188.228.14.94

# Or use online tool: https://www.yougetsignal.com/tools/open-ports/
# Test ports 80 and 443
```

---

### Phase 3: Ubuntu Server Firewall Configuration (5 minutes)

**Goal:** Allow incoming traffic on ports 80 and 443

#### Step 3.1: Configure UFW Firewall

```bash
ssh ubuntu

# Check current firewall status
sudo ufw status

# Allow HTTP and HTTPS
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp

# Enable firewall if not already enabled
sudo ufw enable

# Verify rules
sudo ufw status numbered
```

**Expected output:**
```
Status: active

To                         Action      From
--                         ------      ----
80/tcp                     ALLOW       Anywhere
443/tcp                    ALLOW       Anywhere
```

---

### Phase 4: Install Nginx Proxy Manager (10 minutes)

**Goal:** Set up reverse proxy with automatic SSL

#### Step 4.1: Create Directory and Docker Compose

```bash
ssh ubuntu

# Create directory
mkdir -p ~/nginx-proxy-manager
cd ~/nginx-proxy-manager

# Create docker-compose.yml
cat > docker-compose.yml << 'EOF'
version: '3.8'

services:
  nginx-proxy-manager:
    image: 'jc21/nginx-proxy-manager:latest'
    container_name: nginx-proxy-manager
    restart: unless-stopped
    ports:
      - '80:80'      # HTTP
      - '443:443'    # HTTPS
      - '81:81'      # Admin UI
    volumes:
      - ./data:/data
      - ./letsencrypt:/etc/letsencrypt
    environment:
      # Optional: Uncomment to change default admin email
      # DEFAULT_EMAIL: admin@hawkeye123.dk
      DB_SQLITE_FILE: "/data/database.sqlite"
    networks:
      - npm-network

networks:
  npm-network:
    driver: bridge
EOF
```

#### Step 4.2: Start Nginx Proxy Manager

```bash
# Start the container
docker compose up -d

# Check it's running
docker compose ps

# View logs
docker compose logs -f
```

**Expected output:**
```
NAME                    STATUS          PORTS
nginx-proxy-manager     Up 10 seconds   0.0.0.0:80-81->80-81/tcp, 0.0.0.0:443->443/tcp
```

#### Step 4.3: Verify Local Access

```bash
# From Windows
curl http://192.168.1.92:81

# Or open browser: http://192.168.1.92:81
```

---

### Phase 5: Configure Nginx Proxy Manager (15 minutes)

**Goal:** Set up proxy hosts with SSL certificates

#### Step 5.1: Access Admin UI

1. Open browser: **http://192.168.1.92:81**
2. **Default credentials:**
   - Email: `admin@example.com`
   - Password: `changeme`
3. **IMPORTANT:** You'll be forced to change these on first login
   - New Email: `your@email.com`
   - New Password: `[strong password]`

#### Step 5.2: Add Supabase Proxy Host

1. Click **"Proxy Hosts"** tab
2. Click **"Add Proxy Host"** button

**Details Tab:**
- **Domain Names:** `supabase.hawkeye123.dk`
- **Scheme:** `http`
- **Forward Hostname / IP:** `192.168.1.92`
- **Forward Port:** `8888`
- **Cache Assets:** ❌ Off
- **Block Common Exploits:** ✅ On
- **Websockets Support:** ✅ On ⚠️ **CRITICAL for Supabase Realtime!**
- **Access List:** `Publicly Accessible`

**Custom Locations (Optional):**
- Leave empty for now

**SSL Tab:**
- **SSL Certificate:** `Request a new SSL Certificate`
- **Force SSL:** ✅ On
- **HTTP/2 Support:** ✅ On
- **HSTS Enabled:** ✅ On
- **HSTS Subdomains:** ❌ Off
- **Email Address for Let's Encrypt:** `your@email.com`
- **I Agree to the Let's Encrypt Terms of Service:** ✅ On

3. Click **"Save"**

**Wait 30-60 seconds for SSL certificate to be issued.**

#### Step 5.3: Verify Supabase Access

```bash
# Test from anywhere (Windows PowerShell)
curl https://supabase.hawkeye123.dk

# Should return Supabase response (not error)
```

#### Step 5.4: Add Archon Proxy Host (Optional)

Repeat Step 5.2 with:
- **Domain:** `archon.hawkeye123.dk`
- **Forward Port:** `3737`
- **Websockets:** ✅ On
- **SSL:** ✅ Request new certificate

#### Step 5.5: Add Portainer Proxy Host (Optional)

Repeat Step 5.2 with:
- **Domain:** `portainer.hawkeye123.dk`
- **Forward Port:** `9000`
- **Websockets:** ✅ On
- **SSL:** ✅ Request new certificate

---

### Phase 6: Update Application Configuration (10 minutes)

**Goal:** Configure apps to use new HTTPS URLs

#### Step 6.1: Update Controller .env

```bash
# Location: f:\#Remote\controller\.env

# OLD (local only)
# SUPABASE_URL=http://192.168.1.92:8888

# NEW (works from anywhere!)
SUPABASE_URL=https://supabase.hawkeye123.dk
SUPABASE_ANON_KEY=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyAgCiAgICAicm9sZSI6ICJhbm9uIiwKICAgICJpc3MiOiAic3VwYWJhc2UtZGVtbyIsCiAgICAiaWF0IjogMTY0MTc2OTIwMCwKICAgICJleHAiOiAxNzk5NTM1NjAwCn0.dc_X5iR_VP_qT0zsiyj_I_OZ2T9FtRU2BBNWN8Bu4GE
```

#### Step 6.2: Update Agent .env

```bash
# Location: f:\#Remote\agent\.env

# NEW (works from anywhere!)
SUPABASE_URL=https://supabase.hawkeye123.dk
SUPABASE_ANON_KEY=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyAgCiAgICAicm9sZSI6ICJhbm9uIiwKICAgICJpc3MiOiAic3VwYWJhc2UtZGVtbyIsCiAgICAiaWF0IjogMTY0MTc2OTIwMCwKICAgICJleHAiOiAxNzk5NTM1NjAwCn0.dc_X5iR_VP_qT0zsiyj_I_OZ2T9FtRU2BBNWN8Bu4GE
DEVICE_NAME=MyDevice
DEVICE_ID=auto-generated
API_KEY=optional-api-key
HEARTBEAT_INTERVAL=30
```

#### Step 6.3: Update .env.example Files

```bash
# Update controller/.env.example
SUPABASE_URL=https://supabase.hawkeye123.dk
SUPABASE_ANON_KEY=your_anon_key_here

# Update agent/.env.example
SUPABASE_URL=https://supabase.hawkeye123.dk
SUPABASE_ANON_KEY=your_anon_key_here
DEVICE_NAME=MyDevice
```

#### Step 6.4: Update CONFIGURATION.md

Add section about production configuration:

```markdown
## Production Configuration (Remote Access)

For remote access from anywhere, use the public HTTPS URLs:

### Controller
SUPABASE_URL=https://supabase.hawkeye123.dk
SUPABASE_ANON_KEY=your_anon_key

### Agent
SUPABASE_URL=https://supabase.hawkeye123.dk
SUPABASE_ANON_KEY=your_anon_key
DEVICE_NAME=MyDevice
```

---

### Phase 7: Testing (15 minutes)

**Goal:** Verify everything works end-to-end

#### Test 7.1: DNS Resolution

```powershell
# From Windows
nslookup supabase.hawkeye123.dk
# Should return: 188.228.14.94

nslookup archon.hawkeye123.dk
# Should return: 188.228.14.94
```

#### Test 7.2: HTTPS Access

```powershell
# Test Supabase
curl https://supabase.hawkeye123.dk

# Test Archon (if configured)
curl https://archon.hawkeye123.dk

# Test in browser - should show green lock icon 🔒
```

#### Test 7.3: Controller Connection (Local Network)

```bash
# Build and run controller
cd f:\#Remote\controller
go build -o remote-controller.exe .
.\remote-controller.exe

# Should connect to https://supabase.hawkeye123.dk
# Check logs for successful connection
```

#### Test 7.4: Agent Connection (Local Network)

```bash
# Build and run agent
cd f:\#Remote\agent
.\build.bat
.\remote-agent.exe

# Should register with https://supabase.hawkeye123.dk
# Check logs for successful registration
```

#### Test 7.5: Remote Access Test

**From external network (phone hotspot or different location):**

```bash
# Test HTTPS access
curl https://supabase.hawkeye123.dk

# Run agent from remote location
# Should connect successfully
```

---

## 📋 Complete Checklist

### DNS Configuration
- [ ] A record for supabase.hawkeye123.dk → 188.228.14.94
- [ ] A record for archon.hawkeye123.dk → 188.228.14.94
- [ ] A record for portainer.hawkeye123.dk → 188.228.14.94
- [ ] DNS propagation verified (nslookup)

### UniFi Dream Machine SE
- [ ] Port forwarding: 80 → 192.168.1.92:80
- [ ] Port forwarding: 443 → 192.168.1.92:443
- [ ] Port forwarding rules enabled
- [ ] External port test successful

### Ubuntu Server
- [ ] UFW firewall allows port 80
- [ ] UFW firewall allows port 443
- [ ] Nginx Proxy Manager installed
- [ ] Nginx Proxy Manager running (docker compose ps)
- [ ] Admin UI accessible (http://192.168.1.92:81)

### Nginx Proxy Manager
- [ ] Default password changed
- [ ] Supabase proxy host created
- [ ] SSL certificate issued for supabase.hawkeye123.dk
- [ ] Websockets support enabled
- [ ] Force SSL enabled
- [ ] HTTPS access verified
- [ ] (Optional) Archon proxy host created
- [ ] (Optional) Portainer proxy host created

### Application Configuration
- [ ] controller/.env updated with HTTPS URL
- [ ] agent/.env updated with HTTPS URL
- [ ] controller/.env.example updated
- [ ] agent/.env.example updated
- [ ] CONFIGURATION.md updated
- [ ] Controller connects successfully
- [ ] Agent registers successfully

### Testing
- [ ] DNS resolution works
- [ ] HTTPS access works (green lock)
- [ ] Controller connects from LAN
- [ ] Agent connects from LAN
- [ ] Controller connects from WAN (external network)
- [ ] Agent connects from WAN (external network)
- [ ] WebSocket connections work (Realtime)

---

## 🔒 Security Checklist

- [ ] Nginx Proxy Manager admin password changed
- [ ] "Block Common Exploits" enabled on all proxy hosts
- [ ] "Force SSL" enabled on all proxy hosts
- [ ] HSTS enabled
- [ ] UFW firewall configured (only 80, 443 open to internet)
- [ ] UniFi firewall rules reviewed
- [ ] SSL certificates auto-renewing (Let's Encrypt)
- [ ] Regular security updates scheduled

---

## 🐛 Troubleshooting Guide

### Issue: DNS not resolving

**Symptoms:** `nslookup supabase.hawkeye123.dk` returns no results

**Solutions:**
```bash
# Check DNS provider settings
# Verify A record exists
# Wait 5-10 minutes for propagation
# Clear DNS cache: ipconfig /flushdns
# Try different DNS server: nslookup supabase.hawkeye123.dk 8.8.8.8
```

### Issue: Port forwarding not working

**Symptoms:** External port test fails, can't access from internet

**Solutions:**
1. Verify UniFi port forwarding rules are enabled
2. Check if Nokia fiber box has additional firewall
3. Test from external network (phone hotspot)
4. Verify public IP hasn't changed: `curl ifconfig.me`
5. Check UniFi logs for blocked connections

### Issue: SSL certificate fails

**Symptoms:** "Certificate request failed" in Nginx Proxy Manager

**Solutions:**
```bash
# Verify DNS points to correct IP
nslookup supabase.hawkeye123.dk

# Verify port 80 is accessible from internet
# (Let's Encrypt needs port 80 for validation)

# Check Nginx Proxy Manager logs
docker compose logs -f

# Try manual certificate request
# Delete failed certificate and try again
```

### Issue: 502 Bad Gateway

**Symptoms:** HTTPS works but returns 502 error

**Solutions:**
```bash
# Check Supabase is running
ssh ubuntu "cd ~/supabase-local && docker compose ps"

# Verify Supabase accessible locally
curl http://192.168.1.92:8888

# Check Nginx Proxy Manager logs
docker compose logs nginx-proxy-manager

# Verify forward IP/port in proxy host settings
```

### Issue: WebSocket connections fail

**Symptoms:** Supabase Realtime doesn't work

**Solutions:**
1. Verify "Websockets Support" is enabled in proxy host
2. Check browser DevTools → Network → WS filter
3. Test WebSocket connection: `wscat -c wss://supabase.hawkeye123.dk/realtime/v1/websocket`
4. Check Supabase Realtime logs

### Issue: Controller/Agent can't connect

**Symptoms:** Apps show connection errors

**Solutions:**
```bash
# Verify .env file has correct URL
cat controller/.env
cat agent/.env

# Test HTTPS access manually
curl https://supabase.hawkeye123.dk

# Check app logs for specific error
# Rebuild apps after .env changes
```

---

## 📊 Network Ports Summary

| Port | Protocol | Source | Destination | Purpose |
|------|----------|--------|-------------|---------|
| 80 | TCP | Internet | 192.168.1.92:80 | HTTP (redirects to HTTPS) |
| 443 | TCP | Internet | 192.168.1.92:443 | HTTPS (SSL/TLS) |
| 81 | TCP | LAN only | 192.168.1.92:81 | Nginx Proxy Manager Admin UI |
| 8888 | TCP | LAN only | 192.168.1.92:8888 | Supabase (proxied via Nginx) |
| 3737 | TCP | LAN only | 192.168.1.92:3737 | Archon UI (proxied via Nginx) |
| 9000 | TCP | LAN only | 192.168.1.92:9000 | Portainer (proxied via Nginx) |

**Note:** Only ports 80 and 443 need to be open to the internet. All other services are proxied through Nginx.

---

## 🚀 Quick Commands Reference

### UniFi Dream Machine SE

```bash
# Access controller
https://192.168.1.1

# Check port forwarding
Settings → Routing & Firewall → Port Forwarding
```

### Ubuntu Server

```bash
# SSH to server
ssh ubuntu

# Check firewall
sudo ufw status

# Check Nginx Proxy Manager
cd ~/nginx-proxy-manager
docker compose ps
docker compose logs -f

# Restart Nginx Proxy Manager
docker compose restart

# Update Nginx Proxy Manager
docker compose pull
docker compose up -d
```

### DNS Testing

```powershell
# Windows PowerShell
nslookup supabase.hawkeye123.dk
nslookup archon.hawkeye123.dk

# Test from external DNS
nslookup supabase.hawkeye123.dk 8.8.8.8
```

### HTTPS Testing

```powershell
# Test HTTPS
curl https://supabase.hawkeye123.dk
curl https://archon.hawkeye123.dk

# Test with headers
curl -I https://supabase.hawkeye123.dk
```

---

## 📅 Maintenance Schedule

### Daily
- Monitor Nginx Proxy Manager logs for errors
- Check SSL certificate expiry warnings

### Weekly
- Review UniFi security logs
- Check for Docker image updates

### Monthly
- Verify SSL certificates are auto-renewing
- Review firewall rules
- Update Docker images
- Backup Nginx Proxy Manager configuration

### Quarterly
- Security audit
- Review and update documentation
- Test disaster recovery procedures

---

## 🎯 Expected Timeline

| Phase | Duration | Total Time |
|-------|----------|------------|
| DNS Configuration | 5 min | 5 min |
| UniFi Configuration | 10 min | 15 min |
| Ubuntu Firewall | 5 min | 20 min |
| Install Nginx Proxy Manager | 10 min | 30 min |
| Configure Proxy Hosts | 15 min | 45 min |
| Update Applications | 10 min | 55 min |
| Testing | 15 min | **70 min** |

**Total estimated time: ~1 hour 10 minutes**

---

## ✅ Success Criteria

You'll know everything is working when:

1. ✅ `https://supabase.hawkeye123.dk` shows green lock icon in browser
2. ✅ Controller connects from anywhere (not just LAN)
3. ✅ Agent registers from anywhere (not just LAN)
4. ✅ Remote desktop connection works from external network
5. ✅ WebSocket connections work (Supabase Realtime)
6. ✅ SSL certificates auto-renew (check after 60 days)

---

**Your remote desktop application will now work from anywhere in the world with secure HTTPS!** 🌐🔒🎉
