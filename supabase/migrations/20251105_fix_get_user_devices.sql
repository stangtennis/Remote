-- Fix get_user_devices function to use correct column names
-- The database schema uses last_seen, not last_heartbeat

-- Drop the existing function first (required when changing return type)
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
