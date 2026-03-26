-- Update get_user_devices to return full device rows (SETOF remote_devices)
-- and include both assigned AND owned devices.
-- This replaces the old TABLE-returning version that missed columns like
-- is_online, agent_version, public_ip, isp etc.

DROP FUNCTION IF EXISTS get_user_devices(UUID);

CREATE OR REPLACE FUNCTION get_user_devices(p_user_id UUID)
RETURNS SETOF remote_devices
SECURITY DEFINER
SET search_path = public
AS $$
BEGIN
    RETURN QUERY
    SELECT DISTINCT ON (d.device_id) d.*
    FROM remote_devices d
    LEFT JOIN device_assignments da ON d.device_id = da.device_id
    WHERE (
        -- Assigned to user (not revoked)
        (da.user_id = p_user_id AND da.revoked_at IS NULL)
        -- OR owned by user
        OR d.owner_id = p_user_id
    )
    ORDER BY d.device_id, d.last_seen DESC NULLS LAST;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION get_user_devices IS 'Returns all devices owned by or assigned to a user (full row)';
