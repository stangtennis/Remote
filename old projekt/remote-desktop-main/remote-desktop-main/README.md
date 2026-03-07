# Remote Desktop Application

A lightweight and secure remote desktop application that allows screen sharing and remote control over LAN.

## Features

- Real-time screen sharing with high-quality video (30 FPS)
- Full remote control functionality:
  - Mouse movements and clicks (left, right, middle)
  - Keyboard input with modifier keys (Ctrl, Alt, Shift)
  - Special keys support (F1-F12, arrows, etc)
  - Mouse wheel scrolling
- Session management and security
- Works over LAN with proper browser setup
- Clean and intuitive user interface

## Requirements

- Python 3.8+
- Chrome or Edge browser
- Windows operating system (for remote control features)
- Local network connection

## Installation

1. Clone the repository
2. Install dependencies:
   ```bash
   pip install -r requirements.txt
   ```

## Usage

### Starting the Server
```bash
python web_server.py
```
The server will start on port 7080.

### Sharing Your Screen
1. Open Chrome and go to `http://localhost:7080`
2. Click "Get Support"
3. Allow screen sharing when prompted
4. Share the session ID with the controller

### Controlling a Shared Screen
1. Open Chrome and go to `http://LOCAL_IP:7080/controller`
2. Click on the available session
3. You can now:
   - See the shared screen in real-time
   - Control mouse and keyboard
   - Use special keys and modifiers
   - Scroll and navigate

### Browser Setup for LAN Access
1. Open Chrome
2. Go to `chrome://flags`
3. Search for "Insecure origins treated as secure"
4. Add `http://LOCAL_IP:7080`
5. Enable the flag
6. Restart Chrome

## Security Notes

- Only use over trusted local networks
- Screen sharing requires user permission
- Sessions are unique and temporary
- All connections are monitored and logged

## Troubleshooting

- If screen updates are slow, check your network connection
- For keyboard issues, ensure no other apps are capturing keys
- If mouse control doesn't work, try reconnecting
- For LAN access issues, verify browser settings

## Version History

See [Changelog](docs/changelog.md) for full version history.

## License

MIT License - See LICENSE file for details
