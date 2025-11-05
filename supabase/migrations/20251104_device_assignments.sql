-- Migration: Device Assignment System
-- Date: 2025-11-04
-- Description: Enable admin-managed device assignments (TeamViewer-style)

-- 1. Update user_approvals table (add role if not exists)
ALTER TABLE user_approvals
ADD COLUMN IF NOT EXISTS role TEXT DEFAULT 'user';

-- Update existing admins (if any exist with is_admin flag)
UPDATE user_approvals
SET role = 'admin'
WHERE approved = TRUE
AND user_id IN (
    SELECT user_id FROM user_approvals 
    WHERE approved = TRUE 
    LIMIT 1
);

-- 2. Update remote_devices table
ALTER TABLE remote_devices
ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'offline',
ADD COLUMN IF NOT EXISTS approved BOOLEAN DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS assigned_by UUID REFERENCES auth.users(id),
ADD COLUMN IF NOT EXISTS assigned_at TIMESTAMPTZ;

-- Make owner_id nullable (devices can exist without owner)
ALTER TABLE remote_devices
ALTER COLUMN owner_id DROP NOT NULL;

-- 3. Create device_assignments table
CREATE TABLE IF NOT EXISTS device_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id TEXT NOT NULL REFERENCES remote_devices(device_id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    assigned_by UUID NOT NULL REFERENCES auth.users(id),
    assigned_at TIMESTAMPTZ DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,
    notes TEXT,
    UNIQUE(device_id, user_id, revoked_at)
);

-- 4. Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_device_assignments_device 
ON device_assignments(device_id) 
WHERE revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_device_assignments_user 
ON device_assignments(user_id) 
WHERE revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_unassigned_devices 
ON remote_devices(approved, created_at) 
WHERE owner_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_approved_devices 
ON remote_devices(approved, status);

-- 5. Create function to get user's assigned devices
DROP FUNCTION IF EXISTS get_user_devices(UUID);

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

-- 6. Create function to get unassigned devices (admin only)
CREATE OR REPLACE FUNCTION get_unassigned_devices()
RETURNS TABLE (
    device_id TEXT,
    device_name TEXT,
    platform TEXT,
    status TEXT,
    last_heartbeat TIMESTAMPTZ,
    created_at TIMESTAMPTZ,
    approved BOOLEAN
) 
SECURITY DEFINER
SET search_path = public
AS $$
BEGIN
    -- Check if user is admin
    IF NOT EXISTS (
        SELECT 1 FROM user_approvals 
        WHERE user_id::uuid = auth.uid()
        AND role = 'admin'
    ) THEN
        RAISE EXCEPTION 'Access denied: Admin role required';
    END IF;

    RETURN QUERY
    SELECT 
        d.device_id,
        d.device_name,
        d.platform,
        d.status,
        d.last_heartbeat,
        d.created_at,
        d.approved
    FROM remote_devices d
    WHERE NOT EXISTS (
        SELECT 1 FROM device_assignments da
        WHERE da.device_id = d.device_id
        AND da.revoked_at IS NULL
    )
    ORDER BY d.created_at DESC;
END;
$$ LANGUAGE plpgsql;

-- 7. Create function to assign device to user
CREATE OR REPLACE FUNCTION assign_device(
    p_device_id TEXT,
    p_user_id UUID,
    p_approve_device BOOLEAN DEFAULT TRUE,
    p_notes TEXT DEFAULT NULL
)
RETURNS JSONB
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
    v_admin_id UUID;
    v_result JSONB;
BEGIN
    -- Check if caller is admin
    SELECT user_id::uuid INTO v_admin_id
    FROM user_approvals 
    WHERE user_id::uuid = auth.uid()
    AND role = 'admin';
    
    IF v_admin_id IS NULL THEN
        RAISE EXCEPTION 'Access denied: Admin role required';
    END IF;

    -- Check if device exists
    IF NOT EXISTS (SELECT 1 FROM remote_devices WHERE device_id = p_device_id) THEN
        RAISE EXCEPTION 'Device not found: %', p_device_id;
    END IF;

    -- Check if user exists and is approved
    IF NOT EXISTS (
        SELECT 1 FROM user_approvals 
        WHERE user_id::uuid = p_user_id 
        AND approved = TRUE
    ) THEN
        RAISE EXCEPTION 'User not found or not approved: %', p_user_id;
    END IF;

    -- Insert assignment (or update if exists)
    INSERT INTO device_assignments (
        device_id,
        user_id,
        assigned_by,
        notes
    ) VALUES (
        p_device_id,
        p_user_id,
        v_admin_id,
        p_notes
    )
    ON CONFLICT (device_id, user_id, revoked_at) 
    DO UPDATE SET
        assigned_at = NOW(),
        assigned_by = v_admin_id,
        notes = p_notes;

    -- Approve device if requested
    IF p_approve_device THEN
        UPDATE remote_devices
        SET approved = TRUE,
            assigned_by = v_admin_id,
            assigned_at = NOW()
        WHERE device_id = p_device_id;
    END IF;

    -- Return result
    SELECT jsonb_build_object(
        'success', TRUE,
        'device_id', p_device_id,
        'user_id', p_user_id,
        'assigned_by', v_admin_id,
        'approved', p_approve_device
    ) INTO v_result;

    RETURN v_result;
END;
$$ LANGUAGE plpgsql;

-- 8. Create function to revoke device assignment
CREATE OR REPLACE FUNCTION revoke_device_assignment(
    p_device_id TEXT,
    p_user_id UUID
)
RETURNS JSONB
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
    v_admin_id UUID;
    v_result JSONB;
BEGIN
    -- Check if caller is admin
    SELECT user_id::uuid INTO v_admin_id
    FROM user_approvals 
    WHERE user_id::uuid = auth.uid()
    AND role = 'admin';
    
    IF v_admin_id IS NULL THEN
        RAISE EXCEPTION 'Access denied: Admin role required';
    END IF;

    -- Revoke assignment
    UPDATE device_assignments
    SET revoked_at = NOW()
    WHERE device_id = p_device_id
        AND user_id = p_user_id
        AND revoked_at IS NULL;

    -- Return result
    SELECT jsonb_build_object(
        'success', TRUE,
        'device_id', p_device_id,
        'user_id', p_user_id,
        'revoked_by', v_admin_id
    ) INTO v_result;

    RETURN v_result;
END;
$$ LANGUAGE plpgsql;

-- 9. Update RLS policies for device_assignments
ALTER TABLE device_assignments ENABLE ROW LEVEL SECURITY;

-- Drop existing policies if they exist
DROP POLICY IF EXISTS "Users can view their device assignments" ON device_assignments;
DROP POLICY IF EXISTS "Admins can manage device assignments" ON device_assignments;
DROP POLICY IF EXISTS "Devices can register themselves" ON remote_devices;
DROP POLICY IF EXISTS "Devices can update their status" ON remote_devices;

-- Users can see their own assignments
CREATE POLICY "Users can view their device assignments"
ON device_assignments FOR SELECT
TO authenticated
USING (user_id = auth.uid());

-- Admins can manage all assignments
CREATE POLICY "Admins can manage device assignments"
ON device_assignments FOR ALL
TO authenticated
USING (
    EXISTS (
        SELECT 1 FROM user_approvals
        WHERE user_id::uuid = auth.uid()
        AND role = 'admin'
    )
);

-- 10. Update RLS policies for remote_devices
-- Allow devices to register without authentication
CREATE POLICY "Devices can register themselves"
ON remote_devices FOR INSERT
TO anon
WITH CHECK (true);

-- Allow devices to update their status
CREATE POLICY "Devices can update their status"
ON remote_devices FOR UPDATE
TO anon
USING (true)
WITH CHECK (true);

-- 11. Migrate existing devices
-- Assign existing devices to their current owners
INSERT INTO device_assignments (device_id, user_id, assigned_by)
SELECT 
    device_id,
    owner_id::uuid,
    owner_id::uuid  -- Self-assigned
FROM remote_devices
WHERE owner_id IS NOT NULL
ON CONFLICT DO NOTHING;

-- Approve all existing devices
UPDATE remote_devices
SET approved = TRUE
WHERE owner_id IS NOT NULL;

-- 12. Add comments for documentation
COMMENT ON TABLE device_assignments IS 'Tracks which users are assigned to which devices';
COMMENT ON FUNCTION get_user_devices IS 'Returns all devices assigned to a specific user';
COMMENT ON FUNCTION get_unassigned_devices IS 'Returns all unassigned devices (admin only)';
COMMENT ON FUNCTION assign_device IS 'Assigns a device to a user (admin only)';
COMMENT ON FUNCTION revoke_device_assignment IS 'Revokes a device assignment (admin only)';
