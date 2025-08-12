# Remote Desktop Application

A modern, serverless remote desktop application built with Supabase Realtime and JavaScript. This application allows users to securely connect to and control remote computers through a web browser from anywhere in the world, with no need for local servers or port forwarding.

## Features

### 🔐 Authentication & Security
- User registration and login system
- JWT-based authentication
- Row-level security (RLS) with Supabase
- Secure session management

### 💻 Device Management
- Add and manage remote devices
- Real-time device status monitoring
- Support for multiple device types (Desktop, Laptop, Server, Workstation)
- Cross-platform compatibility (Windows, macOS, Linux)

### 🖥️ Remote Desktop Features
- Web-based remote desktop viewer
- Real-time mouse and keyboard control
- Full-screen remote desktop experience
- Connection quality monitoring
- Session recording and history

### 📊 Dashboard & Analytics
- Comprehensive dashboard with statistics
- Session history and logs
- Device status overview
- Recent activity tracking

### 🔄 Real-time Communication
- Supabase Realtime-based global communication
- Serverless architecture with no local WebSocket dependencies
- Low-latency remote control across the internet
- Automatic reconnection handling
- Connection quality adaptation

## 📦 Agent Deployment

### Current Agent Status
- **Version**: CompleteRemoteDesktopAgent.exe (44MB)
- **Status**: ✅ COMPLETE SUCCESS - ALL FEATURES IMPLEMENTED
- **Features**: ✅ Real screen capture, ✅ Real mouse/keyboard control, ✅ WebSocket/WSS servers
- **Download**: [RemoteDesktopAgent.exe](https://ptrtibzwokjcjjxvjpin.supabase.co/storage/v1/object/public/agents/RemoteDesktopAgent.exe)
- **Authentication**: ✅ Fixed (Supabase client integration)
- **Native Modules**: ✅ screenshot-desktop, sharp, robotjs with fallbacks

### Quick Upload
```bash
# Upload new agent version
cmd /c upload-working.bat
```

📚 **[Complete Deployment Guide →](AGENT_DEPLOYMENT.md)**

## Technology Stack

- **Backend**: Supabase Edge Functions, Node.js
- **Database**: Supabase (PostgreSQL)
- **Real-time**: Supabase Realtime
- **Frontend**: Vanilla JavaScript, HTML5, CSS3
- **Authentication**: Supabase Auth
- **Storage**: Supabase Storage
- **Agent Distribution**: Automated upload via REST API
- **Styling**: Modern CSS with CSS Grid and Flexbox

## Project Structure

```
remote-desktop/
├── package.json           # Dependencies and scripts
├── database/
│   └── schema.sql         # Database schema and setup
├── supabase/
│   ├── functions/         # Supabase Edge Functions
│   │   ├── agent-builder/ # Agent builder function
│   │   └── device-manager/ # Device management function
│   └── migrations/        # Database migrations
├── public/
│   ├── index.html         # Main landing page
│   ├── dashboard.html     # Admin dashboard interface
│   ├── remote-control.html # Remote control interface
│   ├── app.js            # Frontend JavaScript
│   ├── agent-manager.js  # Agent management scripts
│   └── styles.css        # Application styles
├── agents/               # Agent source code
│   └── supabase-realtime-agent.js # Supabase Realtime agent
├── docs/                 # Documentation
│   ├── MASTER_PLAN.md    # Overall project plan
│   └── IMPLEMENTATION_GUIDE.md # Implementation details
└── README.md             # This file
```

## Setup Instructions

### Prerequisites
- Node.js (v16 or higher)
- npm or yarn
- Supabase account and project
- Supabase CLI (for development)

### 1. Clone the Repository
```bash
git clone https://github.com/stangtennis/remote-desktop.git
cd remote-desktop
```

### 2. Install Dependencies
```bash
npm install
```

### 3. Set Up Supabase Database
1. Go to your Supabase project dashboard
2. Navigate to the SQL Editor
3. Run the SQL commands from `database/schema.sql` to create the required tables
4. Enable Supabase Realtime with the required channels

### 4. Configure Environment
Update the Supabase credentials in the HTML files and agent script:
```javascript
const supabaseUrl = 'YOUR_SUPABASE_PROJECT_URL';
const supabaseKey = 'YOUR_SUPABASE_ANON_KEY';
```

### 5. Deploy Edge Functions
```bash
# Using Supabase CLI
supabase functions deploy agent-builder
supabase functions deploy device-manager
```

### 6. Access the Application
Open your browser and navigate to the deployed dashboard HTML file in Supabase Storage or serve it locally for testing.

## Database Schema

The application uses the following main tables:

- **users**: User accounts and authentication
- **remote_devices**: Registered remote computers
- **remote_sessions**: Connection sessions and history
- **connection_logs**: Detailed connection logs
- **device_permissions**: Access control for shared devices

## API Endpoints

### Authentication
- `POST /api/register` - User registration
- `POST /api/login` - User login

### Device Management
- `GET /api/devices` - Get user's devices
- `POST /api/devices` - Add new device

### Session Management
- `GET /api/sessions` - Get session history
- `POST /api/sessions/start` - Start remote session

### Health Check
- `GET /api/health` - Application health status

## Supabase Realtime Channels

### Device Channels
- `device-{deviceId}` - Device-specific communication channel
- `all-devices` - Broadcast channel for all devices

### Events
- `command` - Commands sent to devices (mouse, keyboard, screen capture)
- `response` - Responses from devices (screen frames, status updates)
- `heartbeat` - Device status heartbeats

### Payload Types
- `screen_frame` - Screen capture data
- `mouse_input` - Mouse control commands
- `keyboard_input` - Keyboard control commands
- `session_status` - Session status updates

## Security Features

- **Supabase Auth**: Secure authentication system
- **Row Level Security**: Supabase RLS policies for data protection
- **Edge Functions**: Serverless functions with secure execution
- **Session Management**: Secure session tokens and timeouts
- **Input Validation**: Client and server-side validation
- **Encrypted Communication**: Secure Supabase Realtime channels

## Usage Guide

### 1. User Registration/Login
1. Open the application in your browser
2. Register a new account or login with existing credentials
3. You'll be redirected to the dashboard

### 2. Adding Devices
1. Navigate to the "Devices" section
2. Click "Add Device"
3. Fill in device details (name, type, OS, IP address, port)
4. Save the device

### 3. Connecting to Remote Desktop
1. Go to the "Devices" section
2. Find your device (must be online)
3. Click "Connect"
4. The remote desktop viewer will open
5. Use mouse and keyboard to control the remote computer

### 4. Managing Sessions
1. View active and past sessions in the "Sessions" section
2. Monitor connection quality and duration
3. Disconnect active sessions as needed

## Development

### Running in Development Mode
```bash
# For local testing of the dashboard
npx http-server public

# For Edge Function development
supabase functions serve
```

### Code Structure
- `supabase/functions/`: Edge Functions for serverless backend
- `public/*.html`: Frontend interfaces (dashboard, remote control)
- `public/*.js`: Frontend application logic and UI management
- `agents/supabase-realtime-agent.js`: Agent with Supabase Realtime integration
- `public/styles.css`: Modern CSS styling with CSS Grid and Flexbox
- `database/schema.sql`: Complete database schema with RLS policies

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly
5. Submit a pull request

## License

ISC License - see LICENSE file for details.

## Support

For issues and questions:
1. Check the GitHub Issues page
2. Review the documentation
3. Contact the development team

## Roadmap

- [x] Serverless architecture with Supabase Realtime
- [x] Global connectivity without local servers
- [x] Standalone executable agent
- [x] **COMPLETE SUCCESS**: Real screen capture (screenshot-desktop + sharp)
- [x] **COMPLETE SUCCESS**: Real mouse/keyboard control (robotjs)
- [x] **COMPLETE SUCCESS**: WebSocket/WSS servers with SSL certificates
- [x] **COMPLETE SUCCESS**: Dashboard compatibility and deployment
- [x] File transfer capabilities
- [ ] Multi-monitor support
- [ ] Mobile app support
- [ ] Advanced security features
- [ ] Performance optimizations
- [ ] Agent auto-update mechanism
