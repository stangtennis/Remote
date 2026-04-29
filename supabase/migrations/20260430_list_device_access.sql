-- v3.1.4: list_device_access RPC + tighten device_assignments writes.
--
-- The dashboard's "Tildel adgang" modal needs to show all current
-- assignments (with emails) plus the device owner. auth.users isn't
-- exposed to the client, so we provide a SECURITY DEFINER RPC that
-- returns the access list, gated on the caller being admin/super_admin
-- OR the owner of the device.

CREATE OR REPLACE FUNCTION public.list_device_access(p_device_id text)
RETURNS TABLE (
  access_kind     text,
  assignment_id   uuid,
  email           text,
  member_user_id  uuid,
  assigned_at     timestamptz
)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public, auth
AS $$
DECLARE
  v_role     text;
  v_owner_id uuid;
BEGIN
  IF auth.uid() IS NULL THEN
    RETURN;
  END IF;

  SELECT ua.role INTO v_role FROM public.user_approvals ua WHERE ua.user_id = auth.uid();
  SELECT rd.owner_id INTO v_owner_id FROM public.remote_devices rd WHERE rd.device_id = p_device_id;

  IF NOT (
    v_role IN ('admin', 'super_admin')
    OR v_owner_id = auth.uid()
  ) THEN
    RETURN;
  END IF;

  -- Owner row first
  RETURN QUERY
  SELECT 'owner'::text, NULL::uuid, u.email::text, u.id, NULL::timestamptz
  FROM public.remote_devices d
  JOIN auth.users u ON u.id = d.owner_id
  WHERE d.device_id = p_device_id;

  -- Active assignments
  RETURN QUERY
  SELECT 'assignment'::text, da.id, u.email::text, u.id, da.assigned_at
  FROM public.device_assignments da
  JOIN auth.users u ON u.id = da.user_id
  WHERE da.device_id = p_device_id
    AND da.revoked_at IS NULL
  ORDER BY da.assigned_at;
END;
$$;

REVOKE ALL ON FUNCTION public.list_device_access(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.list_device_access(text) TO authenticated;

COMMENT ON FUNCTION public.list_device_access(text) IS
  'Return owner + active assignments for a device. Restricted to admin/super_admin or the device owner.';
