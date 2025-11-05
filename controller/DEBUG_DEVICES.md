# Device Not Showing - Debugging Steps

## Issue
The controller shows 0 devices even though a device exists in the dashboard.

## Root Cause
The device exists in `remote_devices` table but is **not assigned** to your user in the `device_assignments` table.

## Solution: Assign Device to User

Run these SQL queries in Supabase SQL Editor:

### 1. Check if device exists
```sql
SELECT device_id, device_name, platform, owner_id, approved_at, is_online
FROM remote_devices
ORDER BY created_at DESC
LIMIT 5;
```

### 2. Check device assignments for your user
```sql
SELECT da.*, d.device_name
FROM device_assignments da
JOIN remote_devices d ON da.device_id = d.device_id
WHERE da.user_id = 'af870934-f10c-47c4-bf85-ed061827d316'
  AND da.revoked_at IS NULL;
```

### 3. Check your user approval status
```sql
SELECT user_id, approved, role
FROM user_approvals
WHERE user_id = 'af870934-f10c-47c4-bf85-ed061827d316';
```

### 4. Assign the device to your user (SOLUTION)

**Option A: If you're an admin, use the function:**
```sql
SELECT assign_device(
    '<DEVICE_ID>',  -- Replace with actual device_id from step 1
    'af870934-f10c-47c4-bf85-ed061827d316'::uuid,  -- Your user ID
    true,  -- Approve the device
    'Assigned via SQL'  -- Optional note
);
```

**Option B: Direct insert (if function doesn't work):**
```sql
-- First, make yourself admin if needed
UPDATE user_approvals
SET role = 'admin'
WHERE user_id = 'af870934-f10c-47c4-bf85-ed061827d316';

-- Then assign the device
INSERT INTO device_assignments (device_id, user_id, assigned_by)
SELECT 
    device_id,
    'af870934-f10c-47c4-bf85-ed061827d316'::uuid,
    'af870934-f10c-47c4-bf85-ed061827d316'::uuid
FROM remote_devices
WHERE device_id = '<DEVICE_ID>'  -- Replace with actual device_id
ON CONFLICT DO NOTHING;

-- Approve the device
UPDATE remote_devices
SET approved_at = NOW()
WHERE device_id = '<DEVICE_ID>';  -- Replace with actual device_id
```

**Option C: Assign ALL existing devices to yourself:**
```sql
-- Make yourself admin
UPDATE user_approvals
SET role = 'admin'
WHERE user_id = 'af870934-f10c-47c4-bf85-ed061827d316';

-- Assign all devices to yourself
INSERT INTO device_assignments (device_id, user_id, assigned_by)
SELECT 
    device_id,
    'af870934-f10c-47c4-bf85-ed061827d316'::uuid,
    'af870934-f10c-47c4-bf85-ed061827d316'::uuid
FROM remote_devices
WHERE approved_at IS NOT NULL
ON CONFLICT DO NOTHING;
```

## After Running the SQL

1. **Restart the controller** or just log out and log back in
2. **Check the logs** - you should see:
   ```
   INFO: [GetDevices] Successfully fetched 1 devices
   DEBUG: Device 1: Name=..., ID=..., Platform=..., Status=...
   ```
3. **The device should appear** in the Devices tab

## Why This Happened

The new device assignment system requires explicit assignment of devices to users. Simply having a device in the `remote_devices` table is not enough - it must also have an entry in `device_assignments` linking it to your user.
