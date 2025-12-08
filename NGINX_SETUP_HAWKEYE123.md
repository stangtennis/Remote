# 🔒 Nginx Proxy Setup for hawkeye123.dk

**Your Domain:** `*.hawkeye123.dk` (wildcard - can use any subdomain!)

---

## 🎯 Recommended Subdomains

- **`supabase.hawkeye123.dk`** → Local Supabase (192.168.1.92:8888)
- **`archon.hawkeye123.dk`** → Archon UI (192.168.1.92:3737)
- **`portainer.hawkeye123.dk`** → Portainer (192.168.1.92:9000)
- **`remote.hawkeye123.dk`** → Web Dashboard (if deploying)

---

## 🚀 Quick Setup with Nginx Proxy Manager

### Step 1: Install Nginx Proxy Manager

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
    ports:
      - '80:80'      # HTTP
      - '443:443'    # HTTPS
      - '81:81'      # Admin UI
    volumes:
      - ./data:/data
      - ./letsencrypt:/etc/letsencrypt
    restart: unless-stopped
    networks:
      - proxy-network

networks:
  proxy-network:
    driver: bridge
EOF

# Start it
docker compose up -d

# Check it's running
docker compose ps
docker compose logs -f
```

### Step 2: Configure Router Port Forwarding

**On your router, forward these ports to 192.168.1.92:**
- Port **80** → 192.168.1.92:80 (HTTP)
- Port **443** → 192.168.1.92:443 (HTTPS)

### Step 3: Configure DNS (If Not Already Done)

**Add A records for your subdomains:**
- `supabase.hawkeye123.dk` → Your public IP
- `archon.hawkeye123.dk` → Your public IP
- `portainer.hawkeye123.dk` → Your public IP

Or use the wildcard if already configured:
- `*.hawkeye123.dk` → Your public IP

### Step 4: Access Admin UI

1. Open browser: **http://192.168.1.92:81**
2. **Default Login:**
   - Email: `admin@example.com`
   - Password: `changeme`
3. **IMPORTANT:** Change password immediately!

### Step 5: Add Supabase Proxy Host

**In Nginx Proxy Manager UI:**

1. Click **"Proxy Hosts"** → **"Add Proxy Host"**

2. **Details Tab:**
   - **Domain Names:** `supabase.hawkeye123.dk`
   - **Scheme:** `http`
   - **Forward Hostname / IP:** `192.168.1.92`
   - **Forward Port:** `8888`
   - **Cache Assets:** ❌ Off
   - **Block Common Exploits:** ✅ On
   - **Websockets Support:** ✅ On (Important for Supabase Realtime!)

3. **SSL Tab:**
   - **SSL Certificate:** Request a new SSL Certificate
   - **Force SSL:** ✅ On
   - **HTTP/2 Support:** ✅ On
   - **HSTS Enabled:** ✅ On
   - **Email Address:** your@email.com
   - **I Agree to the Let's Encrypt Terms of Service:** ✅ On

4. Click **"Save"**

**Wait 30-60 seconds for SSL certificate to be issued.**

### Step 6: Test Supabase Access

```bash
# Test from anywhere
curl https://supabase.hawkeye123.dk

# Should return Supabase response
```

---

## 🎨 Add More Services (Optional)

### Archon UI

**Add Proxy Host:**
- **Domain:** `archon.hawkeye123.dk`
- **Forward to:** `192.168.1.92:3737`
- **Enable SSL:** ✅ Yes

### Portainer

**Add Proxy Host:**
- **Domain:** `portainer.hawkeye123.dk`
- **Forward to:** `192.168.1.92:9000`
- **Enable SSL:** ✅ Yes

---

## 🔧 Update Application Configuration

### Controller .env

```bash
# OLD (local only)
# SUPABASE_URL=http://192.168.1.92:8888

# NEW (works from anywhere!)
SUPABASE_URL=https://supabase.hawkeye123.dk
SUPABASE_ANON_KEY=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyAgCiAgICAicm9sZSI6ICJhbm9uIiwKICAgICJpc3MiOiAic3VwYWJhc2UtZGVtbyIsCiAgICAiaWF0IjogMTY0MTc2OTIwMCwKICAgICJleHAiOiAxNzk5NTM1NjAwCn0.dc_X5iR_VP_qT0zsiyj_I_OZ2T9FtRU2BBNWN8Bu4GE
```

### Agent .env

```bash
# NEW (works from anywhere!)
SUPABASE_URL=https://supabase.hawkeye123.dk
SUPABASE_ANON_KEY=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyAgCiAgICAicm9sZSI6ICJhbm9uIiwKICAgICJpc3MiOiAic3VwYWJhc2UtZGVtbyIsCiAgICAiaWF0IjogMTY0MTc2OTIwMCwKICAgICJleHAiOiAxNzk5NTM1NjAwCn0.dc_X5iR_VP_qT0zsiyj_I_OZ2T9FtRU2BBNWN8Bu4GE
DEVICE_NAME=MyDevice
```

### Update CONFIGURATION.md

Update the configuration guide with the new URLs:

```markdown
## Production Configuration (Remote Access)

SUPABASE_URL=https://supabase.hawkeye123.dk
SUPABASE_ANON_KEY=your_anon_key
```

---

## ✅ Testing Checklist

- [ ] Nginx Proxy Manager running: `docker compose ps`
- [ ] Port forwarding configured (80, 443)
- [ ] DNS records pointing to public IP
- [ ] Admin UI accessible: http://192.168.1.92:81
- [ ] Supabase proxy host added
- [ ] SSL certificate issued (green lock icon)
- [ ] HTTPS works: https://supabase.hawkeye123.dk
- [ ] Controller .env updated
- [ ] Agent .env updated
- [ ] Controller can connect remotely
- [ ] Agent can connect remotely

---

## 🔒 Security Checklist

- [ ] Changed Nginx Proxy Manager default password
- [ ] Enabled "Block Common Exploits"
- [ ] Enabled "Force SSL"
- [ ] Enabled HSTS
- [ ] Firewall configured (only 80, 443 open)
- [ ] SSL certificates auto-renewing

---

## 🐛 Troubleshooting

### Can't access Admin UI (http://192.168.1.92:81)

```bash
# Check if container is running
docker compose ps

# Check logs
docker compose logs -f

# Restart if needed
docker compose restart
```

### SSL Certificate Fails

```bash
# Check DNS
nslookup supabase.hawkeye123.dk

# Should return your public IP

# Check port forwarding
# From external network, test:
curl -I http://supabase.hawkeye123.dk

# Should get response (not timeout)
```

### 502 Bad Gateway

```bash
# Check Supabase is running
ssh ubuntu "cd ~/supabase-local && docker compose ps"

# Check Supabase logs
ssh ubuntu "cd ~/supabase-local && docker compose logs"

# Verify Supabase accessible locally
curl http://192.168.1.92:8888
```

### WebSocket Connection Issues

- ✅ Ensure "Websockets Support" is enabled in proxy host
- ✅ Check Supabase Realtime is running
- ✅ Test with browser DevTools (Network tab, WS filter)

---

## 📊 Your Complete Setup

```
Internet
  ↓
Router (port forward 80, 443)
  ↓
Nginx Proxy Manager (192.168.1.92:80/443)
  ↓
├─ supabase.hawkeye123.dk → Supabase (192.168.1.92:8888)
├─ archon.hawkeye123.dk → Archon UI (192.168.1.92:3737)
└─ portainer.hawkeye123.dk → Portainer (192.168.1.92:9000)
```

**All with automatic HTTPS/SSL certificates!** 🎉

---

## 🚀 Next Steps

1. **Set up Nginx Proxy Manager** (5 minutes)
2. **Configure port forwarding** on router
3. **Add Supabase proxy host** with SSL
4. **Update .env files** in controller and agent
5. **Test remote access** from outside network
6. **Optional:** Add Archon and Portainer proxies

---

## 📝 Quick Commands Reference

```bash
# Start Nginx Proxy Manager
cd ~/nginx-proxy-manager && docker compose up -d

# Stop
docker compose down

# View logs
docker compose logs -f

# Restart
docker compose restart

# Update to latest version
docker compose pull
docker compose up -d
```

---

**Your Remote Desktop app will now work from anywhere with secure HTTPS!** 🌐🔒
