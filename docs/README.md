# Remote Desktop Dashboard

Modern web dashboard for managing remote desktop sessions.

## Features

- 🔐 **User Authentication** - Supabase Auth with email/password
- 📱 **Device Management** - View and manage registered devices
- 🖥️ **WebRTC Viewer** - Real-time remote desktop viewing
- 🖱️ **Remote Control** - Mouse and keyboard input
- 📊 **Connection Stats** - Real-time connection quality metrics
- 🔒 **Secure** - End-to-end encryption, PIN verification

## File Structure

```
dashboard/
├── index.html          # Login page
├── dashboard.html      # Main dashboard
├── css/
│   └── styles.css      # Modern dark theme styling
├── js/
│   ├── auth.js         # Authentication logic
│   ├── app.js          # Main application logic
│   ├── devices.js      # Device list management
│   ├── webrtc.js       # WebRTC connection handling
│   └── signaling.js    # Realtime signaling
└── README.md           # This file
```

## Development

### Local Testing
```bash
# Use Live Server in VS Code or any static server
# The dashboard will connect to your Supabase project
```

### Configuration
Supabase credentials are in `js/auth.js`:
- `SUPABASE_URL`: https://supabase.hawkeye123.dk
- `SUPABASE_ANON_KEY`: Your anon key

## Deployment to GitHub Pages

### Option 1: Via GitHub Web Interface
1. Push dashboard folder to your repo
2. Go to Settings → Pages
3. Source: Deploy from branch
4. Branch: main → /dashboard
5. Save

### Option 2: Via Command Line
```bash
cd f:\#Remote
git add .
git commit -m "Add dashboard"
git push origin main

# Enable Pages in repo settings
```

Your dashboard will be live at:
**https://stangtennis.github.io/Remote/**

## Usage

1. **Sign Up/Login** - Create an account on the login page
2. **Download Agent** - Get the Windows agent EXE
3. **Run Agent** - Agent will register and appear in device list
4. **Approve Device** - Click to approve new devices
5. **Start Session** - Click device to start remote session
6. **Enter PIN** - Enter the PIN shown on remote device
7. **Control** - Use mouse/keyboard to control remote computer

## Browser Requirements

- Modern browser with WebRTC support (Chrome, Edge, Firefox, Safari)
- HTTPS connection (GitHub Pages provides this automatically)
- Webcam/microphone permissions NOT required

## Security

- All authentication via Supabase Auth
- Row-level security on database
- WebRTC encrypted by default (DTLS-SRTP)
- PIN verification for session access
- Short-lived JWT tokens (15 min)

## Troubleshooting

### Can't connect to device
- Check device is online (green badge)
- Verify device is approved
- Check firewall settings
- Try refreshing device list

### WebRTC not connecting
- Check TURN credentials are configured
- Verify network allows WebRTC (some corporate networks block it)
- Check browser console for errors

### Login issues
- Verify email confirmation if signed up
- Check Supabase project is active
- Clear browser cache and try again

## Next Steps

After dashboard is deployed:
1. Build the Windows Agent (Go application)
2. Test full connection flow
3. Add file transfer UI
4. Implement advanced features (multi-monitor, etc.)
