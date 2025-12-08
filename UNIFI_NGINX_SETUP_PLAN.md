# 🎯 Complete Setup Plan: Nginx + Wildcard DNS + UniFi Dream Machine SE

**Your Network:**
- **Public IP:** 188.228.14.94
- **Router:** UniFi Dream Machine SE (192.168.1.1)
- **Fiber Box:** Nokia (WAN connection)
- **Ubuntu Server:** 192.168.1.92
- **Domain:** *.hawkeye123.dk (wildcard)

---

## 📊 How It Works

```
Internet Request                         Nginx Routes by Hostname
─────────────────                        ────────────────────────

*.hawkeye123.dk → 188.228.14.94
                        │
                        ▼
              UniFi Port Forward (80, 443)
                        │
                        ▼
              Ubuntu Server (192.168.1.92)
                        │
                        ▼
              ┌─────────────────────────────────────┐
              │         Nginx Reverse Proxy         │
              │   (reads Host header, routes to)    │
              └─────────────────────────────────────┘
                        │
        ┌───────────────┼───────────────┬───────────────┐
        ▼               ▼               ▼               ▼
   supabase.*      archon.*       portainer.*      remote.*
   localhost:8888  localhost:3737 localhost:9000   localhost:8080
```

**Key Concept:** One wildcard DNS record + Nginx decides where each subdomain goes.

---

## ✅ Complete Step-by-Step Plan

### Phase 1: Wildcard DNS Configuration (5 minutes)

**Goal:** Point ALL subdomains to your public IP with one record

#### Step 1.1: Configure Wildcard DNS

Go to your DNS provider (where you manage hawkeye123.dk) and add:

```
Type    Name    Value           TTL
A       @       188.228.14.94   300    (root domain)
A       *       188.228.14.94   300    (all subdomains)
```

**This single wildcard covers:**
- supabase.hawkeye123.dk
- archon.hawkeye123.dk
- portainer.hawkeye123.dk
- remote.hawkeye123.dk
- anything.hawkeye123.dk
- future-service.hawkeye123.dk

#### Step 1.2: Verify DNS Propagation

```bash
# From Windows (PowerShell)
nslookup supabase.hawkeye123.dk
nslookup anything-random.hawkeye123.dk

# Both should return: 188.228.14.94
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
# From external network (use phone hotspot)
curl -I http://188.228.14.94

# Or use online tool: https://www.yougetsignal.com/tools/open-ports/
```

---

### Phase 3: Ubuntu Firewall Configuration (5 minutes)

```bash
ssh ubuntu

# Allow HTTP and HTTPS
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
sudo ufw status
```

---

### Phase 4: Install Native Nginx (10 minutes)

**Goal:** Install Nginx directly (not Docker) for full control

#### Step 4.1: Install Nginx and Certbot

```bash
ssh ubuntu

# Update packages
sudo apt update

# Install Nginx
sudo apt install -y nginx

# Install Certbot for SSL
sudo apt install -y certbot python3-certbot-nginx

# Verify Nginx is running
sudo systemctl status nginx
```

#### Step 4.2: Test Nginx

```bash
# From Windows
curl http://192.168.1.92

# Should show "Welcome to nginx!"
```

---

### Phase 5: Get Wildcard SSL Certificate (10 minutes)

**Goal:** One certificate for ALL subdomains

#### Step 5.1: Request Wildcard Certificate

```bash
ssh ubuntu

# Request wildcard certificate (requires DNS challenge)
sudo certbot certonly --manual --preferred-challenges dns \
    -d "hawkeye123.dk" -d "*.hawkeye123.dk"
```

#### Step 5.2: Add DNS TXT Record

Certbot will show something like:
```
Please deploy a DNS TXT record under the name:
_acme-challenge.hawkeye123.dk
with the following value:
xXxXxXxXxXxXxXxXxXxXxXxXxXxXxXxXxXx
```

**Go to your DNS provider and add:**
```
Type    Name              Value                                    TTL
TXT     _acme-challenge   xXxXxXxXxXxXxXxXxXxXxXxXxXxXxXxXxXx      300
```

**Wait 1-2 minutes, then press Enter in the terminal.**

#### Step 5.3: Verify Certificate

```bash
# Check certificate files exist
sudo ls -la /etc/letsencrypt/live/hawkeye123.dk/

# Should show:
# fullchain.pem
# privkey.pem
```

---

### Phase 6: Configure Nginx Virtual Hosts (15 minutes)

**Goal:** Create server blocks for each subdomain

#### Step 6.1: Create Supabase Config

```bash
sudo nano /etc/nginx/sites-available/supabase.hawkeye123.dk
```

**Paste this content:**
```nginx
server {
    listen 80;
    server_name supabase.hawkeye123.dk;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name supabase.hawkeye123.dk;

    # Wildcard SSL certificate
    ssl_certificate /etc/letsencrypt/live/hawkeye123.dk/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/hawkeye123.dk/privkey.pem;

    # SSL settings
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256;
    ssl_prefer_server_ciphers off;

    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header Strict-Transport-Security "max-age=31536000" always;

    location / {
        proxy_pass http://localhost:8888;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket support (critical for Supabase Realtime)
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 86400;
    }
}
```

**Save and exit (Ctrl+X, Y, Enter)**

#### Step 6.2: Create Archon Config

```bash
sudo nano /etc/nginx/sites-available/archon.hawkeye123.dk
```

```nginx
server {
    listen 80;
    server_name archon.hawkeye123.dk;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name archon.hawkeye123.dk;

    ssl_certificate /etc/letsencrypt/live/hawkeye123.dk/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/hawkeye123.dk/privkey.pem;

    ssl_protocols TLSv1.2 TLSv1.3;

    location / {
        proxy_pass http://localhost:3737;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

#### Step 6.3: Create Portainer Config

```bash
sudo nano /etc/nginx/sites-available/portainer.hawkeye123.dk
```

```nginx
server {
    listen 80;
    server_name portainer.hawkeye123.dk;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name portainer.hawkeye123.dk;

    ssl_certificate /etc/letsencrypt/live/hawkeye123.dk/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/hawkeye123.dk/privkey.pem;

    ssl_protocols TLSv1.2 TLSv1.3;

    location / {
        proxy_pass http://localhost:9000;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

#### Step 6.4: Enable All Sites

```bash
# Enable sites
sudo ln -s /etc/nginx/sites-available/supabase.hawkeye123.dk /etc/nginx/sites-enabled/
sudo ln -s /etc/nginx/sites-available/archon.hawkeye123.dk /etc/nginx/sites-enabled/
sudo ln -s /etc/nginx/sites-available/portainer.hawkeye123.dk /etc/nginx/sites-enabled/

# Test configuration
sudo nginx -t

# Reload Nginx
sudo systemctl reload nginx
```

---

### Phase 7: Adding New Subdomains (Future)

**This is the power of this setup!** To add a new subdomain:

```bash
# 1. Create config file
sudo nano /etc/nginx/sites-available/newservice.hawkeye123.dk

# 2. Paste template (change server_name and proxy_pass port)
server {
    listen 80;
    server_name newservice.hawkeye123.dk;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name newservice.hawkeye123.dk;

    ssl_certificate /etc/letsencrypt/live/hawkeye123.dk/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/hawkeye123.dk/privkey.pem;

    location / {
        proxy_pass http://localhost:YOUR_PORT;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}

# 3. Enable and reload
sudo ln -s /etc/nginx/sites-available/newservice.hawkeye123.dk /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

**No DNS changes needed!** The wildcard already covers it.

---

### Phase 8: Update Application Configuration (10 minutes)

**Goal:** Configure apps to use new HTTPS URLs

#### Step 8.1: Update Controller .env

```bash
# Location: f:\#Remote\controller\.env

# OLD (local only)
# SUPABASE_URL=http://192.168.1.92:8888

# NEW (works from anywhere!)
SUPABASE_URL=https://supabase.hawkeye123.dk
SUPABASE_ANON_KEY=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyAgCiAgICAicm9sZSI6ICJhbm9uIiwKICAgICJpc3MiOiAic3VwYWJhc2UtZGVtbyIsCiAgICAiaWF0IjogMTY0MTc2OTIwMCwKICAgICJleHAiOiAxNzk5NTM1NjAwCn0.dc_X5iR_VP_qT0zsiyj_I_OZ2T9FtRU2BBNWN8Bu4GE
```

#### Step 8.2: Update Agent .env

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

#### Step 8.3: Update .env.example Files

```bash
# Update controller/.env.example
SUPABASE_URL=https://supabase.hawkeye123.dk
SUPABASE_ANON_KEY=your_anon_key_here

# Update agent/.env.example
SUPABASE_URL=https://supabase.hawkeye123.dk
SUPABASE_ANON_KEY=your_anon_key_here
DEVICE_NAME=MyDevice
```

#### Step 8.4: Update CONFIGURATION.md

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

### Phase 9: Testing (15 minutes)

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
- [ ] Wildcard A record: *.hawkeye123.dk → 188.228.14.94
- [ ] Root A record: hawkeye123.dk → 188.228.14.94
- [ ] DNS propagation verified (nslookup any-subdomain.hawkeye123.dk)

### UniFi Dream Machine SE
- [ ] Port forwarding: 80 → 192.168.1.92:80
- [ ] Port forwarding: 443 → 192.168.1.92:443
- [ ] Port forwarding rules enabled
- [ ] External port test successful

### Ubuntu Server
- [ ] UFW firewall allows port 80
- [ ] UFW firewall allows port 443
- [ ] Nginx installed (apt install nginx)
- [ ] Certbot installed (apt install certbot python3-certbot-nginx)

### SSL Certificate
- [ ] Wildcard certificate requested (certbot --manual --preferred-challenges dns)
- [ ] DNS TXT record added for _acme-challenge
- [ ] Certificate files exist in /etc/letsencrypt/live/hawkeye123.dk/

### Nginx Virtual Hosts
- [ ] supabase.hawkeye123.dk config created
- [ ] archon.hawkeye123.dk config created
- [ ] portainer.hawkeye123.dk config created
- [ ] All sites enabled (ln -s to sites-enabled)
- [ ] nginx -t passes
- [ ] nginx reloaded

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
