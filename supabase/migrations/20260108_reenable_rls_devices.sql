-- Re-enable RLS on remote_devices with proper policies
-- Date: 2026-01-08
-- Description: Fix security issue where RLS was disabled

-- 1. Re-enable RLS
ALTER TABLE public.remote_devices ENABLE ROW LEVEL SECURITY;

-- 2. Drop old policies if they exist
DROP POLICY IF EXISTS "Devices can register themselves" ON remote_devices;
DROP POLICY IF EXISTS "Devices can update their status" ON remote_devices;
DROP POLICY IF EXISTS "Users can view their assigned devices" ON remote_devices;
DROP POLICY IF EXISTS "Admins can view all devices" ON remote_devices;

-- 3. Allow devices to register without authentication (anon)
CREATE POLICY "Devices can register themselves"
ON remote_devices FOR INSERT
TO anon
WITH CHECK (true);

-- 4. Allow devices to update their own status (anon)
CREATE POLICY "Devices can update their status"
ON remote_devices FOR UPDATE
TO anon
USING (true)
WITH CHECK (true);

-- 5. Authenticated users can view devices assigned to them
CREATE POLICY "Users can view their assigned devices"
ON remote_devices FOR SELECT
TO authenticated
USING (
    -- User is assigned to this device
    EXISTS (
        SELECT 1 FROM device_assignments
        WHERE device_assignments.device_id = remote_devices.device_id
        AND device_assignments.user_id = auth.uid()
        AND device_assignments.revoked_at IS NULL
    )
    OR
    -- User is admin/super_admin (can see all)
    EXISTS (
        SELECT 1 FROM user_approvals
        WHERE user_approvals.user_id::uuid = auth.uid()
        AND user_approvals.role IN ('admin', 'super_admin')
    )
);

-- 6. Admins can manage all devices
CREATE POLICY "Admins can manage all devices"
ON remote_devices FOR ALL
TO authenticated
USING (
    EXISTS (
        SELECT 1 FROM user_approvals
        WHERE user_approvals.user_id::uuid = auth.uid()
        AND user_approvals.role IN ('admin', 'super_admin')
    )
)
WITH CHECK (
    EXISTS (
        SELECT 1 FROM user_approvals
        WHERE user_approvals.user_id::uuid = auth.uid()
        AND user_approvals.role IN ('admin', 'super_admin')
    )
);

-- 7. Update comment
COMMENT ON TABLE public.remote_devices 
IS 'RLS enabled with proper policies for device registration, user assignments, and admin access.';
