# 🔒 Nginx Reverse Proxy Setup for Local Supabase with SSL

**Goal:** Expose local Supabase (192.168.1.92:8888) to the internet with automatic HTTPS/SSL certificates.

---

## 📋 Prerequisites

1. ✅ **Domain name** pointing to your public IP (e.g., `supabase.yourdomain.com`)
   - Free options: DuckDNS, No-IP, FreeDNS
   - Or use a paid domain
2. ✅ **Router port forwarding:**
   - Port 80 → 192.168.1.92:80
   - Port 443 → 192.168.1.92:443
3. ✅ **Ubuntu server** at 192.168.1.92
4. ✅ **Docker** and **Docker Compose** installed

---

## 🏗️ Architecture

```
Internet → Router (port forward) → Nginx (192.168.1.92:80/443) → Supabase (192.168.1.92:8888)
                                      ↓
                                  Let's Encrypt SSL
```

---

## 🚀 Option 1: Nginx Proxy Manager (Recommended - Easiest)

**Why:** GUI-based, automatic SSL, super easy!

### Setup

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
EOF

# Start it
docker compose up -d

# Check logs
docker compose logs -f
```

### Configure via Web UI

1. **Access Admin UI:** http://192.168.1.92:81
2. **Default Login:**
   - Email: `admin@example.com`
   - Password: `changeme`
3. **Change password immediately!**
4. **Add Proxy Host:**
   - Domain: `supabase.yourdomain.com`
   - Scheme: `http`
   - Forward Hostname: `192.168.1.92`
   - Forward Port: `8888`
   - Enable: ✅ Block Common Exploits
   - Enable: ✅ Websockets Support
5. **SSL Tab:**
   - Enable: ✅ SSL
   - Select: "Request a new SSL Certificate"
   - Enable: ✅ Force SSL
   - Enable: ✅ HTTP/2 Support
   - Email: your@email.com
   - Enable: ✅ Agree to Let's Encrypt Terms
6. **Save**

**Done! Your Supabase is now accessible at `https://supabase.yourdomain.com`** 🎉

---

## 🛠️ Option 2: Manual Nginx + Certbot (Advanced)

### Step 1: Create Nginx Configuration

```bash
ssh ubuntu

# Create directory structure
mkdir -p ~/nginx-proxy/{conf.d,webroot}
cd ~/nginx-proxy
```

### Step 2: Create docker-compose.yml

```yaml
version: '3.8'

services:
  nginx:
    image: nginx:alpine
    container_name: nginx-proxy
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./conf.d:/etc/nginx/conf.d:ro
      - ./webroot:/var/www/html
      - certbot-etc:/etc/letsencrypt
      - certbot-var:/var/lib/letsencrypt
    restart: unless-stopped

  certbot:
    image: certbot/certbot
    container_name: certbot
    volumes:
      - certbot-etc:/etc/letsencrypt
      - certbot-var:/var/lib/letsencrypt
      - ./webroot:/var/www/html
    depends_on:
      - nginx

volumes:
  certbot-etc:
  certbot-var:
```

### Step 3: Create Nginx Config

```bash
cat > conf.d/supabase.conf << 'EOF'
# HTTP - Redirect to HTTPS
server {
    listen 80;
    server_name supabase.yourdomain.com;

    # Let's Encrypt challenge
    location /.well-known/acme-challenge/ {
        root /var/www/html;
    }

    # Redirect to HTTPS
    location / {
        return 301 https://$server_name$request_uri;
    }
}

# HTTPS - Proxy to Supabase
server {
    listen 443 ssl http2;
    server_name supabase.yourdomain.com;

    # SSL Configuration
    ssl_certificate /etc/letsencrypt/live/supabase.yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/supabase.yourdomain.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    # Proxy to Supabase
    location / {
        proxy_pass http://192.168.1.92:8888;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
        
        # Timeouts
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }

    # WebSocket support (Supabase Realtime)
    location /realtime/v1/ {
        proxy_pass http://192.168.1.92:8888;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
    }
}
EOF
```

### Step 4: Get SSL Certificate

```bash
# Start nginx
docker compose up -d nginx

# Get certificate
docker compose run --rm certbot certonly \
  --webroot \
  --webroot-path=/var/www/html \
  --email your@email.com \
  --agree-tos \
  --no-eff-email \
  -d supabase.yourdomain.com

# Restart nginx
docker compose restart nginx
```

### Step 5: Setup Auto-Renewal

```bash
# Add cron job
crontab -e

# Add this line (runs daily at 3 AM)
0 3 * * * cd ~/nginx-proxy && docker compose run --rm certbot renew && docker compose restart nginx
```

---

## 🔧 Update Application Configuration

### Controller .env
```bash
SUPABASE_URL=https://supabase.yourdomain.com
SUPABASE_ANON_KEY=your_anon_key
```

### Agent .env
```bash
SUPABASE_URL=https://supabase.yourdomain.com
SUPABASE_ANON_KEY=your_anon_key
DEVICE_NAME=MyDevice
```

---

## ✅ Testing

```bash
# Test HTTP redirect
curl -I http://supabase.yourdomain.com

# Test HTTPS
curl -I https://supabase.yourdomain.com

# Test from controller/agent
# Update .env and restart applications
```

---

## 🎯 Bonus: Proxy Multiple Services

You can also proxy Archon, Portainer, etc.:

### Nginx Proxy Manager
- `archon.yourdomain.com` → 192.168.1.92:3737
- `portainer.yourdomain.com` → 192.168.1.92:9000
- `supabase.yourdomain.com` → 192.168.1.92:8888

All with automatic SSL! 🎉

---

## 🔒 Security Considerations

1. **Firewall Rules:**
   ```bash
   sudo ufw allow 80/tcp
   sudo ufw allow 443/tcp
   sudo ufw enable
   ```

2. **Keep Updated:**
   ```bash
   docker compose pull
   docker compose up -d
   ```

3. **Monitor Logs:**
   ```bash
   docker compose logs -f nginx
   ```

---

## 🐛 Troubleshooting

### Certificate Fails
- ✅ Check domain DNS points to your public IP
- ✅ Verify port forwarding (80, 443)
- ✅ Ensure port 80 is accessible from internet

### 502 Bad Gateway
- ✅ Check Supabase is running: `docker ps`
- ✅ Verify proxy_pass URL is correct
- ✅ Check nginx logs: `docker compose logs nginx`

### WebSocket Issues
- ✅ Ensure Upgrade headers are configured
- ✅ Check Supabase Realtime settings
- ✅ Test with browser DevTools

---

## 📊 Comparison

| Method | Difficulty | GUI | Auto-SSL | Best For |
|--------|-----------|-----|----------|----------|
| **Nginx Proxy Manager** | ⭐ Easy | ✅ Yes | ✅ Yes | Beginners |
| Manual Nginx + Certbot | ⭐⭐ Medium | ❌ No | ⚙️ Manual | Advanced users |

---

**Recommendation: Use Nginx Proxy Manager for easiest setup!** 🚀
