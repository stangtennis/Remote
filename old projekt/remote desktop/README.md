# Simple Remote Desktop Control

A basic remote desktop control application similar to TeamViewer, built with Python.

## Features

- Screen sharing
- Remote mouse control
- Remote keyboard input
- Real-time screen updates

## Requirements

- Python 3.7+
- Required packages listed in requirements.txt

## Installation

1. Install the required packages:
```bash
pip install -r requirements.txt
```

## Usage

1. On the computer you want to control (host):
```bash
python server.py
```

2. On the controlling computer (client):
```bash
python client.py
```

By default, the application connects to localhost. To connect to a remote computer, modify the host parameter in the client.py script.

## Security Note

This is a basic implementation and should not be used in production without proper security measures such as:
- Authentication
- Encryption
- Firewall configuration
- Access control

## How it Works

- The server captures the screen and listens for incoming connections
- The client connects to the server and displays the remote screen
- Mouse movements and keyboard inputs are sent from client to server
- The server executes these commands on the host machine
