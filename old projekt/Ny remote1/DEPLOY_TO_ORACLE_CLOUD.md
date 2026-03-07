# Deploying to Oracle Cloud Free Tier

This guide will help you deploy the remote desktop server to Oracle Cloud's free tier VM.

## 1. Sign Up for Oracle Cloud

1. Go to https://www.oracle.com/cloud/free/
2. Click "Start for free"
3. Complete the registration (requires credit card for verification)
4. Choose a region close to your location

## 2. Create a VM Instance

1. From Oracle Cloud Dashboard:
   - Go to "Compute" → "Instances"
   - Click "Create Instance"
   - Choose "Create VM Instance"

2. Configure the instance:
   - Name: remote-desktop-server
   - Image: Canonical Ubuntu 22.04 (latest)
   - Shape: VM.Standard.A1.Flex (ARM) or VM.Standard.E2.1.Micro (AMD)
   - VCN: Create new VCN
   - Subnet: Create new subnet
   - Public IP: Yes
   - SSH Keys: Generate new key pair (SAVE THE PRIVATE KEY)

## 3. Configure Security

1. Go to your VCN's security list:
   - Add Ingress Rule:
     - Source: 0.0.0.0/0
     - Port: 8000
     - Description: Remote Desktop Server

## 4. Connect to Your VM

1. Using Windows:
   - Download PuTTY
   - Convert the private key using PuTTYgen
   - Connect using the public IP and ubuntu username

2. Using Linux/Mac:
   ```bash
   chmod 400 private_key.key
   ssh -i private_key.key ubuntu@YOUR_PUBLIC_IP
   ```

## 5. Deploy the Application

1. Install dependencies:
   ```bash
   sudo apt update
   sudo apt install -y python3-pip python3-venv git
   ```

2. Create project directory:
   ```bash
   mkdir remote-desktop
   cd remote-desktop
   ```

3. Set up Python virtual environment:
   ```bash
   python3 -m venv venv
   source venv/bin/activate
   ```

4. Upload your code:
   - Use SFTP or Git to upload the code
   - Or create files manually using nano/vim

5. Install requirements:
   ```bash
   pip install -r requirements.txt
   ```

6. Install and configure supervisor to keep the server running:
   ```bash
   sudo apt install -y supervisor
   ```

7. Create supervisor config:
   ```bash
   sudo nano /etc/supervisor/conf.d/remote-desktop.conf
   ```
   Add:
   ```ini
   [program:remote-desktop]
   directory=/home/ubuntu/remote-desktop
   command=/home/ubuntu/remote-desktop/venv/bin/python server.py
   user=ubuntu
   autostart=true
   autorestart=true
   stderr_logfile=/var/log/remote-desktop.err.log
   stdout_logfile=/var/log/remote-desktop.out.log
   ```

8. Start the service:
   ```bash
   sudo supervisorctl reread
   sudo supervisorctl update
   sudo supervisorctl start remote-desktop
   ```

## 6. Connect Clients

1. Connect client:
   ```bash
   python client.py --server http://YOUR_PUBLIC_IP:8000
   ```

2. Connect admin panel:
   ```bash
   python admin.py --server http://YOUR_PUBLIC_IP:8000
   ```

## Security Recommendations

1. Set up SSL/HTTPS:
   - Get free SSL certificate from Let's Encrypt
   - Configure Nginx as reverse proxy

2. Configure firewall (UFW):
   ```bash
   sudo ufw allow 22/tcp  # SSH
   sudo ufw allow 8000/tcp  # Remote Desktop Server
   sudo ufw enable
   ```

3. Keep system updated:
   ```bash
   sudo apt update
   sudo apt upgrade
   ```

## Troubleshooting

1. Check server logs:
   ```bash
   sudo supervisorctl tail remote-desktop
   ```

2. Check server status:
   ```bash
   sudo supervisorctl status remote-desktop
   ```

3. Restart server:
   ```bash
   sudo supervisorctl restart remote-desktop
   ```

4. Check firewall status:
   ```bash
   sudo ufw status
   ```

5. Check system resources:
   ```bash
   htop
   ```
