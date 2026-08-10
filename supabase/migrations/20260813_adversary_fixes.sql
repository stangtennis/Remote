-- ============================================================================
-- Adversary-review fixes — 2026-08-13
-- Addresses residual holes found by the GPT-5.6 adversarial review (ADV-01,
-- ADV-02, ADV-06, ADV-09). ADV-03/ADV-05 were false positives (rejected with
-- live-DB evidence); ADV-04 is by-design (public sessions); ADV-07/ADV-08 are
-- edge-function fixes (deployed separately).
-- ============================================================================

-- ADV-01 (Critical): remote_sessions INSERT allowed ANY authenticated user to
-- create a session for ANY device (the created_by=auth.uid() policy did not
-- check device ownership). Tighten to require device access.
DROP POLICY IF EXISTS auth_insert_remote_sessions ON public.remote_sessions;
CREATE POLICY auth_insert_remote_sessions
ON public.remote_sessions FOR INSERT
TO authenticated
WITH CHECK (created_by = auth.uid() AND public.user_has_device_access(device_id));

-- ADV-02 (Critical): get_user_devices returned api_key via d.*, so an assigned
-- user could harvest the device's permanent credential and impersonate the agent
-- even after assignment revocation. Redact api_key from the return.
DROP FUNCTION IF EXISTS public.get_user_devices(uuid);
CREATE OR REPLACE FUNCTION public.get_user_devices(p_user_id UUID)
RETURNS SETOF remote_devices
SECURITY DEFINER
SET search_path = public
AS $$
BEGIN
    IF p_user_id IS DISTINCT FROM auth.uid()
       AND NOT is_admin() THEN
        RAISE EXCEPTION 'Not allowed to view another user''s devices';
    END IF;

    RETURN QUERY
    SELECT DISTINCT ON (d.device_id)
        d.id, d.device_id, d.device_name, d.platform, d.arch, d.cpu_count,
        d.ram_bytes, d.is_online, d.last_seen,
        NULL::text AS api_key,
        d.approved_by, d.approved_at, d.owner_id, d.created_at, d.status,
        d.approved, d.assigned_by, d.assigned_at, d.agent_version, d.public_ip,
        d.isp, d.pending_command, d.cpu_percent, d.memory_used_mb,
        d.memory_total_mb, d.disk_used_gb, d.disk_total_gb, d.connection_type,
        d.session_bytes_sent, d.session_bytes_received, d.api_key_revoked_at
    FROM remote_devices d
    LEFT JOIN device_assignments da ON d.device_id = da.device_id
    WHERE (
        (da.user_id = p_user_id AND da.revoked_at IS NULL)
        OR d.owner_id = p_user_id
    )
    ORDER BY d.device_id, d.last_seen DESC NULLS LAST;
END;
$$ LANGUAGE plpgsql;
COMMENT ON FUNCTION public.get_user_devices IS 'Returns devices owned by or assigned to a user. api_key is redacted. Restricted to caller or admins.';

-- ADV-06 (High): is_admin() and user_has_device_access() checked role but not
-- the approved flag, so a user with role=admin AND approved=false stayed admin.
CREATE OR REPLACE FUNCTION public.is_admin()
RETURNS boolean
LANGUAGE sql
STABLE SECURITY DEFINER
SET search_path = public
AS $$
  SELECT EXISTS (
    SELECT 1 FROM public.user_approvals
    WHERE user_id = auth.uid()
      AND role IN ('admin', 'super_admin')
      AND approved = true
  );
$$;

CREATE OR REPLACE FUNCTION public.user_has_device_access(p_device_id text)
RETURNS boolean AS $$
BEGIN
    RETURN EXISTS (
        SELECT 1 FROM remote_devices WHERE device_id = p_device_id AND owner_id = auth.uid()
    ) OR EXISTS (
        SELECT 1 FROM device_assignments
        WHERE device_id = p_device_id AND user_id = auth.uid() AND revoked_at IS NULL
    ) OR EXISTS (
        SELECT 1 FROM user_approvals
        WHERE user_id::uuid = auth.uid()
          AND role IN ('admin', 'super_admin')
          AND approved = true
    );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER STABLE SET search_path = public;

-- ADV-09 (Low): device_tags INSERT checked device access but did not force
-- created_by = auth.uid(), so a caller could spoof another user as the creator.
CREATE OR REPLACE FUNCTION public.device_tags_set_creator()
RETURNS trigger AS $$
BEGIN
    NEW.created_by := auth.uid();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS device_tags_set_creator ON public.device_tags;
CREATE TRIGGER device_tags_set_creator
    BEFORE INSERT ON public.device_tags
    FOR EACH ROW EXECUTE FUNCTION public.device_tags_set_creator();
