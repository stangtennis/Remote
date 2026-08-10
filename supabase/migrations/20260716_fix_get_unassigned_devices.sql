-- v3.1.x: Fix broken get_unassigned_devices() — references non-existent
-- remote_devices.last_heartbeat column. The actual column is last_seen
-- (see 20250101000000_initial_schema.sql:17). The same bug was already fixed
-- for get_user_devices in 20251105_fix_get_user_devices.sql; this applies
-- the identical fix to get_unassigned_devices so the admin RPC stops raising
-- "column d.last_heartbeat does not exist".
--
-- Safe: function is currently broken (raises on every call), so this cannot
-- regress working behaviour. Body is reproduced verbatim from
-- 20260430_fix_super_admin_revoke.sql with only the column name corrected.

CREATE OR REPLACE FUNCTION public.get_unassigned_devices()
RETURNS TABLE (
  device_id      TEXT,
  device_name    TEXT,
  platform       TEXT,
  status         TEXT,
  last_seen      TIMESTAMPTZ,
  created_at     TIMESTAMPTZ,
  approved       BOOLEAN
)
SECURITY DEFINER
SET search_path = public
AS $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM user_approvals
    WHERE user_id::uuid = auth.uid()
      AND role IN ('admin', 'super_admin')
  ) THEN
    RAISE EXCEPTION 'Access denied: Admin role required';
  END IF;

  RETURN QUERY
  SELECT
    d.device_id,
    d.device_name,
    d.platform,
    d.status,
    d.last_seen,
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
