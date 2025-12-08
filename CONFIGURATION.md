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
SUPABASE_ANON_KEY=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyAgCiAgICAicm9sZSI6ICJhbm9uIiwKICAgICJpc3MiOiAic3VwYWJhc2UtZGVtbyIsCiAgICAiaWF0IjogMTY0MTc2OTIwMCwKICAgICJleHAiOiAxNzk5NTM1NjAwCn0.dc_X5iR_VP_qT0zsiyj_I_OZ2T9FtRU2BBNWN8Bu4GE
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
