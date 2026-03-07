# Setting Up Remote Desktop Relay Server on Ubuntu

## Overview
This guide explains how to set up the relay server on Ubuntu. The server will:
- Act as a central relay point
- Allow clients and admins to connect from any location
- Handle screen sharing and control data

## Server Setup on Ubuntu

### 1. Install System Dependencies
```bash
# Update package list
sudo apt update

# Install Python and pip if not already installed
sudo apt install -y python3 python3-pip python3-venv

# Install development tools
sudo apt install -y build-essential python3-dev
```

### 2. Create Project Directory
```bash
# Create directory
mkdir ~/remote-desktop
cd ~/remote-desktop

# Copy only the server files
mkdir server
# Copy server.py to the server directory
```

### 3. Set Up Python Environment
```bash
# Create virtual environment
python3 -m venv venv

# Activate virtual environment
source venv/bin/activate

# Install required packages
pip install fastapi uvicorn python-socketio
```

### 4. Configure Firewall
```bash
# Allow port 8000
sudo ufw allow 8000

# Enable firewall if not already enabled
sudo ufw enable

# Check status
sudo ufw status
```

### 5. Run the Server
```bash
# Start server
python3 server/server.py
```

The server will start on http://0.0.0.0:8000

### 6. Get Server IP
```bash
# Find your Ubuntu machine's IP address
ip addr show

# Look for the inet address on your main interface (usually eth0 or ens33)
# Example output: inet 192.168.1.100/24
```

## Running as a Service (Recommended)

1. Create service file:
```bash
sudo nano /etc/systemd/system/remote-desktop.service
```

2. Add configuration:
```ini
[Unit]
Description=Remote Desktop Relay Server
After=network.target

[Service]
User=<your-username>
WorkingDirectory=/home/<your-username>/remote-desktop
Environment="PATH=/home/<your-username>/remote-desktop/venv/bin"
ExecStart=/home/<your-username>/remote-desktop/venv/bin/python server/server.py
Restart=always

[Install]
WantedBy=multi-user.target
```

3. Start the service:
```bash
sudo systemctl enable remote-desktop
sudo systemctl start remote-desktop
sudo systemctl status remote-desktop
```

## Connecting Clients and Admins

Clients and admins can connect from any machine using your Ubuntu server's IP:

1. Windows Clients:
```bash
python client.py --server <ubuntu-ip>:8000
```

2. Windows Admin:
```bash
python admin.py --server <ubuntu-ip>:8000
```

## Troubleshooting

1. Connection Issues:
   - Verify server is running: `systemctl status remote-desktop`
   - Check firewall: `sudo ufw status`
   - Test port: `nc -zv localhost 8000`
   - Check server logs: `journalctl -u remote-desktop`

2. If clients can't connect:
   - Verify they're using correct IP and port
   - Try pinging server: `ping <ubuntu-ip>`
   - Check if port is reachable: `nc -zv <ubuntu-ip> 8000`

3. Performance Issues:
   - Monitor server resources: `top` or `htop`
   - Check network usage: `iftop` or `nethogs`
   - View real-time logs: `journalctl -u remote-desktop -f`
