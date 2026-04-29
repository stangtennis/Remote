-- v3.1.3: device assignment helper.
--
-- Dashboard admins need to look up a recipient by email to delegate /
-- transfer device ownership. auth.users is not exposed to the client,
-- so we add a SECURITY DEFINER RPC that returns the user_id only when
-- the caller is admin/super_admin.

CREATE OR REPLACE FUNCTION public.find_user_id_by_email(p_email text)
RETURNS uuid
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public, auth
AS $$
DECLARE
  v_role text;
  v_id   uuid;
BEGIN
  IF auth.uid() IS NULL THEN
    RETURN NULL;
  END IF;

  SELECT role INTO v_role FROM public.user_approvals WHERE user_id = auth.uid();
  IF v_role IS NULL OR v_role NOT IN ('admin', 'super_admin') THEN
    RETURN NULL;
  END IF;

  SELECT id INTO v_id FROM auth.users WHERE lower(email) = lower(p_email);
  RETURN v_id;
END;
$$;

REVOKE ALL ON FUNCTION public.find_user_id_by_email(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.find_user_id_by_email(text) TO authenticated;

COMMENT ON FUNCTION public.find_user_id_by_email(text) IS
  'Resolve email → user_id. Restricted to admin/super_admin via auth.uid() check; returns NULL otherwise.';
