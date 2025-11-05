# ⚠️ DATABASE MIGRATION REQUIRED

## Issue Found
The controller was failing to load clients because the database RPC function `get_user_devices` was using incorrect column names.

## What Was Fixed

### 1. Database Schema Mismatch
- **Problem**: RPC function referenced `d.last_heartbeat` which doesn't exist
- **Solution**: Updated to use `d.last_seen` (the actual column name)
- **Also Fixed**: Added missing `owner_id` field and proper status calculation

### 2. Fyne UI Threading Issues
- **Problem**: UI updates from goroutine caused threading errors
- **Solution**: Added proper `window.Canvas().Content().Refresh()` calls

### 3. Go Struct Mismatch
- **Problem**: Device struct had `LastHeartbeat` field
- **Solution**: Changed to `LastSeen` to match database

## How to Apply the Fix

### Option 1: Using Supabase CLI (Recommended)
```bash
cd f:\#Remote
supabase db push
```

### Option 2: Manual SQL Execution
1. Go to your Supabase Dashboard
2. Navigate to SQL Editor
3. Run the migration file: `supabase/migrations/20251105_fix_get_user_devices.sql`

### Option 3: Direct SQL
Copy and paste this into Supabase SQL Editor:

```sql
CREATE OR REPLACE FUNCTION get_user_devices(p_user_id UUID)
RETURNS TABLE (
    device_id TEXT,
    device_name TEXT,
    platform TEXT,
    owner_id TEXT,
    status TEXT,
    last_seen TIMESTAMPTZ,
    created_at TIMESTAMPTZ,
    assigned_at TIMESTAMPTZ
) 
SECURITY DEFINER
SET search_path = public
AS $$
BEGIN
    RETURN QUERY
    SELECT 
        d.device_id,
        d.device_name,
        d.platform,
        d.owner_id::TEXT,
        CASE 
            WHEN d.is_online THEN 'online'
            WHEN d.last_seen > NOW() - INTERVAL '5 minutes' THEN 'away'
            ELSE 'offline'
        END as status,
        d.last_seen,
        d.created_at,
        da.assigned_at
    FROM remote_devices d
    INNER JOIN device_assignments da 
        ON d.device_id = da.device_id
    WHERE da.user_id = p_user_id
        AND da.revoked_at IS NULL
        AND d.approved_at IS NOT NULL
    ORDER BY d.last_seen DESC NULLS LAST;
END;
$$ LANGUAGE plpgsql;
```

## After Migration

1. **Rebuild the controller** (already done):
   ```bash
   cd f:\#Remote\controller
   go build -o controller.exe .
   ```

2. **Test the controller**:
   - Run `controller.exe`
   - Log in with your credentials
   - Check the logs in `logs/` directory
   - Devices should now load successfully

## What the Logs Should Show

After the fix, you should see:
```
INFO: Fetching devices for user: <user-id>
DEBUG: [GetDevices] Received response with status: 200
INFO: [GetDevices] Successfully fetched X devices
DEBUG: Device 1: Name=..., ID=..., Platform=..., Status=...
INFO: ✅ Successfully loaded X assigned devices
```

No more errors about `column d.last_heartbeat does not exist`!
