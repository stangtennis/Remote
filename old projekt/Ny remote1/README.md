# Remote Desktop Application System

This is a remote desktop application system consisting of three components: server, client, and admin panel.

## Features

- Real-time screen sharing
- Remote control capabilities
- Admin panel for monitoring and controlling clients
- Secure communication between components
- Cross-platform compatibility

## Requirements

- Python 3.8 or higher
- Dependencies listed in requirements.txt

## Installation

1. Install the required dependencies:
```bash
pip install -r requirements.txt
```

## Usage

### 1. Start the Server
```bash
cd server
python server.py
```
The server will start on http://localhost:8000

### 2. Start Client Application
```bash
cd client
python client.py
```
The client will automatically connect to the server and start sharing the screen.

### 3. Start Admin Panel
```bash
cd admin
python admin.py
```
Use the admin panel to:
- View connected clients
- View client screens
- Take control of client machines
- Send messages to clients

## WAN Connection Setup

To connect clients and admin panel over the internet:

1. Server Setup:
   - Deploy the server to a cloud provider (e.g., AWS, DigitalOcean, Heroku)
   - Ensure the server has a public IP address or domain name
   - Configure your firewall to allow incoming connections on port 8000
   - For production, set up SSL/TLS for secure connections

2. Client Connection:
```bash
python client.py --server http://your-server-address:8000
```

3. Admin Panel Connection:
```bash
python admin.py --server http://your-server-address:8000
```

### Security Recommendations for WAN Setup

1. Use HTTPS instead of HTTP for all connections
2. Implement strong authentication for both clients and admins
3. Use a proper SSL certificate
4. Set up a firewall to only allow necessary ports
5. Consider using a VPN for additional security
6. Regularly update all components and dependencies

## Security Note

- Change the default admin key in the server.py and admin.py files
- Implement proper authentication and encryption for production use
- Use firewalls and VPNs for additional security

## Architecture

- Server: FastAPI + Socket.IO for real-time communication
- Client: PyQt6 for GUI + OpenCV for screen capture
- Admin: PyQt6 for GUI + Socket.IO for server communication
