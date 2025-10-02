# Remote Desktop Application

A serverless remote desktop solution built with Supabase, WebRTC, and GitHub Pages - inspired by MeshCentral/TeamViewer.

## ✅ Status: **WORKING** (Confirmed 2025-10-02)

- Remote screen streaming functional across networks
- External access working (confirmed from outside local network)
- TURN relay working via Twilio
- WebRTC P2P connections stable
- Agent and Dashboard deployed and operational

## Architecture

- **Backend**: Supabase (Database, Realtime, Storage, Edge Functions, Auth)
- **Dashboard**: GitHub Pages (Static hosting)
- **Agent**: Go + Pion WebRTC (Single Windows EXE)
- **Connectivity**: WebRTC P2P with TURN fallback

## Project Structure

```
f:\#Remote\
├── supabase/              # Supabase backend
│   ├── migrations/        # Database schema migrations
│   └── functions/         # Edge Functions
│       ├── session-token/
│       ├── device-register/
│       └── file-transfer/
├── dashboard/             # GitHub Pages dashboard
│   ├── index.html
│   ├── css/
│   ├── js/
│   └── assets/
├── agent/                 # Go agent application
│   ├── cmd/
│   ├── internal/
│   └── go.mod
├── plan.md                # Comprehensive project plan
└── .env.example           # Environment variables template
```

## Setup Instructions

### Prerequisites

- [Supabase CLI](https://supabase.com/docs/guides/cli)
- [Go 1.21+](https://golang.org/dl/)
- [Git](https://git-scm.com/)
- Supabase account
- Code signing certificate (for production)

### 1. Supabase Setup

```bash
# Login to Supabase
supabase login

# Link to your project
supabase link --project-ref your-project-ref

# Copy environment variables
cp .env.example .env
# Edit .env with your Supabase keys

# Run migrations
cd supabase
supabase db push

# Deploy Edge Functions
supabase functions deploy session-token
supabase functions deploy device-register
supabase functions deploy file-transfer
```

### 2. Dashboard Setup

The dashboard is a static site hosted on GitHub Pages.

```bash
cd dashboard
# Open index.html in browser for local testing
# Or use Live Server in VS Code
```

To deploy to GitHub Pages:
1. Push to GitHub
2. Enable GitHub Pages in repo settings
3. Set source to main branch, /dashboard folder

### 3. Agent Setup

```bash
cd agent
go mod init github.com/yourusername/remote-agent
go mod tidy

# Development build
go build -o remote-agent.exe ./cmd/remote-agent

# Production build (with code signing)
go build -ldflags="-s -w" -o remote-agent.exe ./cmd/remote-agent
signtool sign /f cert.pfx /p password /t http://timestamp.digicert.com remote-agent.exe
```

## Development Phases

- [x] **Fase 0**: Infrastructure (Supabase tables, Edge Functions, Storage)
- [x] **Fase 0.5**: Authentication & Authorization
- [x] **Fase 1**: Dashboard skeleton
- [x] **Fase 2**: Agent MVP (JPEG screen + input)
- [x] **Fase 3**: TURN + reconnection ✅ **WORKING**
- [ ] **Fase 4**: Video track (VP8/H.264) - Currently using JPEG frames
- [ ] **Fase 5**: File transfer
- [ ] **Fase 6**: Security & production - Partially complete (auth working)
- [ ] **Fase 7**: Production hardening

See `plan.md` for detailed milestones.

## Known Issues

- **Multiple dashboard tabs**: Only use one browser tab to avoid signaling conflicts
- **Session cleanup**: Occasionally needs manual cleanup of stale sessions
- **Mouse/keyboard control**: Currently disabled pending robotgo + CGO setup

## Security

- **MFA**: Enabled for dashboard users
- **API Keys**: Generated per device with rotation
- **RLS**: Row-level security on all tables
- **Tokens**: Short-lived JWT (5-15 min)
- **Code Signing**: Mandatory for Windows EXE
- **Rate Limiting**: 100 req/min per user/device

## Cost Estimation (Monthly)

- Supabase Pro: $25/mo
- TURN (Twilio): ~$112/mo (280GB @ $0.40/GB)
- GitHub Pages: Free
- Code Signing Cert: ~$200-500/year

**Total**: ~$150-200/mo + cert

## Documentation

- `plan.md` - Comprehensive project plan with architecture, security, and phases
- Each component has its own README in respective folders

## License

[Your License Here]

## Support

[Your Support Information]
