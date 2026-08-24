# Configuration Guide

## Environment Variables

Both the controller and agent support configuration via `.env` files for better security and flexibility.

### Controller Configuration

Create a `.env` file in the `controller/` directory:

```env
# Supabase Configuration
SUPABASE_URL=https://your-project.supabase.co
SUPABASE_ANON_KEY=your_anon_key_here
```

**Location**: `controller/.env`  
**Example**: See `controller/.env.example`

### Agent Configuration

Create a `.env` file in the `agent/` directory:

```env
# Supabase Configuration
SUPABASE_URL=https://your-project.supabase.co
SUPABASE_ANON_KEY=your_anon_key_here

# Device Configuration (optional)
DEVICE_NAME=My Remote Device
```

**Location**: `agent/.env`  
**Example**: See `agent/.env.example`

### Controller AI SSH Bridge

The controller AI terminal can open a key-based SSH session directly on the
Ubuntu AI host. Configure it from the **AI Terminal** tab after logging in as
an approved administrator. The controller stores only the SSH settings in:

```text
%APPDATA%\RemoteDesktopController\ssh.json       (Windows)
~/.config/RemoteDesktopController/ssh.json       (Linux)
```

The private key itself is never copied into the controller or frontend. The
configured SSH command starts in the remote AI workdir, loads
`ai-controller.env`, and then opens an interactive shell. This makes commands
such as `remote-desktop-cli ai-connect` and `remote-desktop-cli support-watch`
run on Ubuntu regardless of where the Windows controller is running.

Required SSH setup:

1. Create or use an SSH key on the controller computer.
2. Add the public key to the Ubuntu user's `~/.ssh/authorized_keys`.
3. Configure the Ubuntu hostname or reachable VPN/DNS endpoint, SSH port,
   username, private key path, and `/home/dennis/projekter/aisupport` as the
   remote workdir in the controller UI.
4. Keep `ai-controller.env` on Ubuntu with `RD_AI_CONTROLLER_KEY`.

OpenSSH must be available as `ssh.exe` on Windows or `ssh` on Linux. Host-key
verification remains enabled; the controller does not disable SSH host-key
checking.

Optional environment defaults for a new controller installation:

```env
RD_AI_SSH_HOST=ubuntu.example.com
RD_AI_SSH_USER=dennis
RD_AI_SSH_KEY=~/.ssh/id_ed25519
RD_AI_SSH_WORKDIR=/home/dennis/projekter/aisupport
RD_AI_SSH_CLOUDFLARE=true
```

The default route is `dennis@ssh.hawkeye123.dk` through Cloudflare Access. The
controller invokes `cloudflared access ssh --hostname %h` as OpenSSH's proxy
command, so `cloudflared` must be installed and authenticated on the computer
running the controller. Disable the Cloudflare checkbox only when using a
direct/VPN-reachable SSH endpoint.

The controller does not change the Ubuntu host or `SERVER`; the SSH endpoint
and any VPN/firewall access must already be available.

## How It Works

### Controller
- Uses custom `.env` parser in `controller/internal/config/config.go`
- Falls back to hardcoded defaults if `.env` file doesn't exist
- Reads `SUPABASE_URL` and `SUPABASE_ANON_KEY`

### Agent
- Uses `os.Getenv()` with fallback defaults in `agent/internal/config/config.go`
- Falls back to hardcoded defaults if environment variables not set
- Reads `SUPABASE_URL`, `SUPABASE_ANON_KEY`, and `DEVICE_NAME`

## Security Best Practices

1. **Never commit `.env` files** - Already in `.gitignore`
2. **Use `.env.example` as template** - Copy and fill in your values
3. **Rotate keys regularly** - Update Supabase keys periodically
4. **Use different keys per environment** - Dev, staging, production

## Setup Instructions

### First Time Setup

1. **Copy example files**:
   ```bash
   # Controller
   cd controller
   copy .env.example .env
   
   # Agent
   cd agent
   copy .env.example .env
   ```

2. **Edit `.env` files** with your Supabase credentials

3. **Build and run**:
   ```bash
   # Controller
   cd controller
   go build -o controller.exe .
   
   # Agent
   cd agent
   go build -o remote-agent.exe .\cmd\remote-agent
   ```

### Default Configuration (Local Supabase)

**As of v2.2.1, the application defaults to local Supabase for better testing and development:**

```env
SUPABASE_URL=http://192.168.1.92:8888
SUPABASE_ANON_KEY=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJyb2xlIjoiYW5vbiIsImlzcyI6InN1cGFiYXNlIiwiaWF0IjoxNzg2NDk3MDQ5LCJleHAiOjQ5NDAwOTcwNDl9.Inmhq9QxPXQKburj2RzRS-bZROeYfYT_k8A9ti-faVo
```

**Benefits of local Supabase:**
- ✅ Faster development (no internet latency)
- ✅ No internet dependency
- ✅ Better control over test data
- ✅ Easier debugging and troubleshooting
- ✅ Free and unlimited usage

### Using Cloud Supabase

If you prefer to use cloud Supabase, create a `.env` file with your cloud credentials:

```env
SUPABASE_URL=https://your-project.supabase.co
SUPABASE_ANON_KEY=your_cloud_anon_key_here
```

## Troubleshooting

### Configuration Not Loading

1. **Check file location**: `.env` must be in the same directory as the executable
2. **Check file format**: Use `KEY=VALUE` format, no spaces around `=`
3. **Check file encoding**: Use UTF-8 encoding
4. **Check permissions**: Ensure file is readable

### Still Using Hardcoded Values

If the application still uses hardcoded values:
- Verify `.env` file exists in the correct location
- Check logs for configuration loading errors
- Ensure environment variables are set correctly

## Migration from Hardcoded Credentials

The code already supports both methods:
- ✅ **With `.env`**: Reads from file/environment
- ✅ **Without `.env`**: Falls back to hardcoded defaults

No code changes required - just create `.env` files to override defaults.
