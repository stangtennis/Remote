-- v3.1.5: Allow super_admin to revoke / list unassigned devices.
--
-- The original functions in 20251104_device_assignments.sql gated on
-- `role = 'admin'` only. Super_admins (which is a strictly higher
-- privilege than admin) hit "Access denied: Admin role required" when
-- using the legacy admin.html UI to revoke assignments. assign_device
-- already accepts both roles; this brings revoke + list_unassigned
-- in line.

CREATE OR REPLACE FUNCTION public.revoke_device_assignment(
  p_device_id TEXT,
  p_user_id UUID
)
RETURNS JSONB
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
  v_admin_id UUID;
  v_result   JSONB;
BEGIN
  SELECT user_id::uuid INTO v_admin_id
  FROM user_approvals
  WHERE user_id::uuid = auth.uid()
    AND role IN ('admin', 'super_admin');

  IF v_admin_id IS NULL THEN
    RAISE EXCEPTION 'Access denied: Admin role required';
  END IF;

  UPDATE device_assignments
  SET revoked_at = NOW()
  WHERE device_id = p_device_id
    AND user_id = p_user_id
    AND revoked_at IS NULL;

  IF NOT EXISTS (
    SELECT 1 FROM device_assignments
    WHERE device_id = p_device_id
      AND revoked_at IS NULL
      AND user_id != p_user_id
  ) THEN
    UPDATE remote_devices
    SET owner_id = NULL
    WHERE device_id = p_device_id;
  END IF;

  v_result := jsonb_build_object(
    'success', true,
    'device_id', p_device_id,
    'user_id', p_user_id,
    'revoked_at', NOW()
  );
  RETURN v_result;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION public.get_unassigned_devices()
RETURNS TABLE (
  device_id      TEXT,
  device_name    TEXT,
  platform       TEXT,
  status         TEXT,
  last_heartbeat TIMESTAMPTZ,
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
