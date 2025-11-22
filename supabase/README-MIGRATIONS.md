# Database Migrations Guide

## Overview

This folder contains all SQL migrations for the Remote Desktop Application database schema. These migrations must be applied to your local Supabase instance for the application to work.

## Migration Files (21 total)

1. `20250101000000_initial_schema.sql` - Core tables (devices, sessions, signaling, audit_logs)
2. `20250101000001_storage_setup.sql` - Storage configuration
3. `20250102000000_fix_device_visibility.sql` - Device visibility fixes
4. `20250102000001_fix_device_approval.sql` - Device approval system
5. `20250102000002_allow_device_self_check.sql` - Device self-check permissions
6. `20250102000003_fix_heartbeat_policy.sql` - Heartbeat policy fixes
7. `20250102000004_simplify_rls_policies.sql` - RLS policy simplification
8. `20250102000005_disable_rls_devices.sql` - Disable RLS for devices
9. `20250102000006_disable_all_rls.sql` - Disable all RLS
10. `20250102000007_fix_recursion_trigger.sql` - Fix trigger recursion
11. `20250102000008_add_turn_config_column.sql` - Add TURN config column
12. `20250102000009_enable_realtime.sql` - Enable realtime subscriptions
13. `20250102000010_enable_realtime_devices.sql` - Enable realtime for devices
14. `20250102000011_allow_device_delete.sql` - Device deletion permissions
15. `20250102000012_session_cleanup_cron.sql` - Session cleanup cron job
16. `20250108000000_enable_security.sql` - Enable security features
17. `20250108000001_fix_agent_access.sql` - Fix agent access permissions
18. `20250109000000_user_approval_system.sql` - User approval system
19. `20250112000000_fix_web_agent_policies.sql` - Web agent policy fixes
20. `20251104_device_assignments.sql` - Device assignment system
21. `20251105_fix_get_user_devices.sql` - Fix get user devices function

## How to Apply Migrations

### Option 1: Via SSH (Recommended)

```powershell
# Copy migrations to Ubuntu server
scp -r .\migrations\*.sql ubuntu:/tmp/remote-migrations/

# SSH into Ubuntu and apply migrations
ssh ubuntu

# Apply each migration
cd /tmp/remote-migrations
for file in *.sql; do
  echo "Applying $file..."
  docker exec -i supabase-db psql -U postgres -d postgres < "$file"
done
```

### Option 2: Via Supabase Studio

1. Open Supabase Studio: http://192.168.1.92:8888
2. Go to SQL Editor
3. Copy and paste each migration file content
4. Execute in order (by filename)

### Option 3: Direct Database Connection

If you have `psql` installed on Windows:

```powershell
# Run the provided script
.\apply-migrations.ps1
```

Or manually:

```powershell
$env:PGPASSWORD = "postgres"
Get-ChildItem .\migrations\*.sql | Sort-Object Name | ForEach-Object {
    Write-Host "Applying $($_.Name)..."
    psql -h 192.168.1.92 -p 5432 -U postgres -d postgres -f $_.FullName
}
```

## Verification

After applying migrations, verify the schema:

```sql
-- Check tables exist
SELECT table_name FROM information_schema.tables 
WHERE table_schema = 'public' 
ORDER BY table_name;

-- Expected tables:
-- - audit_logs
-- - remote_devices
-- - remote_sessions
-- - session_signaling
-- - user_approvals
```

## Database Schema

### Core Tables

**remote_devices**
- Stores registered remote desktop agents
- Tracks device status, platform, owner
- Includes API key for authentication

**remote_sessions**
- Active and historical remote desktop sessions
- Tracks session status, metrics, WebRTC info

**session_signaling**
- WebRTC signaling messages (SDP/ICE)
- Temporary storage for connection establishment

**audit_logs**
- Audit trail for security and debugging
- Tracks all important events

**user_approvals**
- User approval system
- Controls which users can access the system

## Troubleshooting

### Migrations Already Applied

If you see "already exists" errors, the migrations may have been partially applied. You can:

1. Check what exists: `\dt` in psql
2. Skip already-applied migrations
3. Or drop and recreate: `DROP TABLE IF EXISTS table_name CASCADE;`

### Permission Errors

Ensure you're using the `postgres` superuser:
- User: `postgres`
- Password: `postgres`
- Database: `postgres`

### Connection Refused

Ensure local Supabase is running:

```bash
ssh ubuntu "cd ~/supabase-local && docker compose ps"
```

If not running:

```bash
ssh ubuntu "cd ~/supabase-local && docker compose up -d"
```

## Important Notes

⚠️ **These migrations are required for the application to work!**

Without the database schema:
- ❌ Controller cannot authenticate users
- ❌ Agent cannot register devices
- ❌ WebRTC signaling will fail
- ❌ Sessions cannot be created

✅ **After applying migrations, the application will work with local Supabase.**
