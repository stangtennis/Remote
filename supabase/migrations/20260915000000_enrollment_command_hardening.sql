-- Harden the enrollment and command ledger without weakening the existing
-- bearer-token enrollment flow. This migration is safe after the initial
-- 20260914000000_client_enrollment_commands migration has been applied.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Uninstall confirmation is enforced by the database, not only by the UI.
CREATE OR REPLACE FUNCTION public.enqueue_device_command(
  p_device_id TEXT,
  p_command_type TEXT,
  p_payload JSONB DEFAULT '{}'::jsonb
)
RETURNS UUID
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
DECLARE
  v_id UUID;
BEGIN
  IF auth.uid() IS NULL OR NOT public.user_has_device_access(p_device_id) THEN
    RAISE EXCEPTION 'Device is not accessible';
  END IF;
  IF p_command_type NOT IN ('force_update', 'enable_relay', 'disable_relay', 'restart', 'lock', 'shutdown', 'uninstall') THEN
    RAISE EXCEPTION 'Unsupported device command';
  END IF;
  IF p_command_type = 'uninstall' AND COALESCE(p_payload ->> 'confirmation', '') <> 'REMOVE' THEN
    RAISE EXCEPTION 'Remote uninstall requires explicit REMOVE confirmation';
  END IF;

  IF p_command_type = 'uninstall' THEN
    UPDATE public.remote_devices
    SET lifecycle_status = 'uninstall_pending', uninstall_requested_at = now()
    WHERE device_id = p_device_id AND lifecycle_status = 'active';
    IF NOT FOUND THEN
      RAISE EXCEPTION 'Device is already uninstalling or uninstalled';
    END IF;
  END IF;

  -- Do not retain arbitrary caller payloads. The agent dispatches only the
  -- command_type allowlist above and ignores server payload content.
  INSERT INTO public.device_commands(device_id, command_type, payload, requested_by)
  VALUES (
    p_device_id,
    p_command_type,
    CASE WHEN p_command_type = 'uninstall' THEN '{"confirmation":"REMOVE"}'::jsonb ELSE '{}'::jsonb END,
    auth.uid()
  )
  RETURNING id INTO v_id;

  INSERT INTO public.audit_logs(device_id, actor, event, details, severity)
  VALUES (p_device_id, auth.uid(), 'REMOTE_COMMAND_REQUESTED',
          jsonb_build_object('command_id', v_id, 'command_type', p_command_type), 'warning');
  RETURN v_id;
END;
$$;

-- A device may only move delivered -> started -> terminal. This prevents a
-- device key from skipping the started state or completing an unclaimed row.
CREATE OR REPLACE FUNCTION public.complete_device_command(
  p_command_id UUID,
  p_status TEXT,
  p_result JSONB DEFAULT '{}'::jsonb
)
RETURNS BOOLEAN
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
DECLARE
  v_device_id TEXT;
  v_command_type TEXT;
BEGIN
  v_device_id := public.device_id_for_active_api_key();
  IF v_device_id IS NULL THEN
    RAISE EXCEPTION 'Device authentication failed';
  END IF;
  IF p_status NOT IN ('started', 'succeeded', 'failed', 'cancelled') THEN
    RAISE EXCEPTION 'Unsupported command status';
  END IF;

  SELECT command_type INTO v_command_type
  FROM public.device_commands
  WHERE id = p_command_id AND device_id = v_device_id;
  IF NOT FOUND THEN RETURN FALSE; END IF;

  UPDATE public.device_commands
  SET status = p_status,
      result = COALESCE(p_result, '{}'::jsonb),
      started_at = CASE WHEN p_status = 'started' THEN now() ELSE started_at END,
      completed_at = CASE WHEN p_status IN ('succeeded', 'failed', 'cancelled') THEN now() ELSE completed_at END
  WHERE id = p_command_id
    AND device_id = v_device_id
    AND (
      (p_status = 'started' AND status = 'delivered')
      OR (p_status IN ('succeeded', 'failed', 'cancelled') AND status = 'started')
    );
  IF NOT FOUND THEN RETURN FALSE; END IF;

  INSERT INTO public.audit_logs(device_id, event, details, severity)
  VALUES (
    v_device_id,
    CASE WHEN p_status = 'started' THEN 'REMOTE_COMMAND_STARTED'
         WHEN p_status = 'succeeded' THEN 'REMOTE_COMMAND_SUCCEEDED'
         WHEN p_status = 'cancelled' THEN 'REMOTE_COMMAND_CANCELLED'
         ELSE 'REMOTE_COMMAND_FAILED' END,
    jsonb_build_object('command_id', p_command_id, 'command_type', v_command_type, 'status', p_status),
    CASE WHEN p_status = 'failed' THEN 'error' ELSE 'info' END
  );

  IF v_command_type = 'uninstall' AND p_status IN ('succeeded', 'failed') THEN
    UPDATE public.remote_devices
    SET lifecycle_status = CASE WHEN p_status = 'succeeded' THEN 'uninstalled' ELSE 'active' END,
        uninstalled_at = CASE WHEN p_status = 'succeeded' THEN now() ELSE NULL END,
        is_online = CASE WHEN p_status = 'succeeded' THEN false ELSE is_online END,
        api_key_revoked_at = CASE WHEN p_status = 'succeeded' THEN now() ELSE api_key_revoked_at END
    WHERE device_id = v_device_id;
  END IF;
  RETURN TRUE;
END;
$$;

-- Recover a command if the agent dies after delivery but before it can report
-- started. The short lease avoids permanently stuck delivered rows while the
-- agent's normal heartbeat claims promptly.
CREATE OR REPLACE FUNCTION public.claim_device_command(p_device_id TEXT)
RETURNS TABLE(command_id UUID, command_type TEXT, payload JSONB)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
DECLARE
  v_device_id TEXT;
BEGIN
  v_device_id := public.device_id_for_active_api_key();
  IF v_device_id IS NULL OR v_device_id <> p_device_id THEN
    RAISE EXCEPTION 'Device authentication failed';
  END IF;

  RETURN QUERY
  WITH next_command AS (
    SELECT c.id
    FROM public.device_commands c
    WHERE c.device_id = v_device_id
      AND c.expires_at > now()
      AND (
        c.status = 'queued'
        OR (c.status = 'delivered' AND c.delivered_at < now() - interval '2 minutes')
      )
    ORDER BY c.created_at ASC
    FOR UPDATE SKIP LOCKED
    LIMIT 1
  )
  UPDATE public.device_commands c
  SET status = 'delivered', delivered_at = now()
  FROM next_command n
  WHERE c.id = n.id
  RETURNING c.id, c.command_type, c.payload;
END;
$$;

-- Retained uninstalled device rows are immutable from the user API.
DROP POLICY IF EXISTS "Users can delete own devices" ON public.remote_devices;
CREATE POLICY "Users can delete own devices"
ON public.remote_devices
FOR DELETE TO authenticated
USING (
  auth.uid() = owner_id
  AND public.is_user_approved(auth.uid())
  AND lifecycle_status <> 'uninstalled'
);

CREATE OR REPLACE FUNCTION public.prevent_retired_device_reactivation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  IF OLD.lifecycle_status = 'uninstalled' AND NEW.lifecycle_status <> 'uninstalled' THEN
    RAISE EXCEPTION 'Uninstalled device rows cannot be reactivated';
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS prevent_retired_device_reactivation ON public.remote_devices;
CREATE TRIGGER prevent_retired_device_reactivation
BEFORE UPDATE ON public.remote_devices
FOR EACH ROW EXECUTE FUNCTION public.prevent_retired_device_reactivation();

CREATE OR REPLACE FUNCTION public.prevent_retired_device_delete()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  IF OLD.lifecycle_status = 'uninstalled' THEN
    RAISE EXCEPTION 'Uninstalled device rows cannot be deleted';
  END IF;
  RETURN OLD;
END;
$$;

DROP TRIGGER IF EXISTS prevent_retired_device_delete ON public.remote_devices;
CREATE TRIGGER prevent_retired_device_delete
BEFORE DELETE ON public.remote_devices
FOR EACH ROW EXECUTE FUNCTION public.prevent_retired_device_delete();

-- Command history is durable audit data; no client role may delete it.
CREATE OR REPLACE FUNCTION public.prevent_device_command_delete()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'Device command history cannot be deleted';
END;
$$;

DROP TRIGGER IF EXISTS prevent_device_command_delete ON public.device_commands;
CREATE TRIGGER prevent_device_command_delete
BEFORE DELETE ON public.device_commands
FOR EACH ROW EXECUTE FUNCTION public.prevent_device_command_delete();

REVOKE DELETE ON public.device_commands FROM PUBLIC, anon, authenticated;

-- Keep the redacted SETOF remote_devices RPC compatible after the three
-- lifecycle columns were appended to remote_devices.
DROP FUNCTION IF EXISTS public.get_user_devices(UUID);
CREATE OR REPLACE FUNCTION public.get_user_devices(p_user_id UUID)
RETURNS SETOF remote_devices
SECURITY DEFINER
SET search_path = public
AS $$
BEGIN
  IF NOT public.is_user_approved(auth.uid()) THEN
    RAISE EXCEPTION 'Approved user access required';
  END IF;
  IF p_user_id IS DISTINCT FROM auth.uid() AND NOT is_admin() THEN
    RAISE EXCEPTION 'Not allowed to view another user''s devices';
  END IF;

  RETURN QUERY
  SELECT DISTINCT ON (d.device_id)
    d.id, d.device_id, d.device_name, d.platform, d.arch, d.cpu_count,
    d.ram_bytes, d.is_online, d.last_seen, NULL::text AS api_key,
    d.approved_by, d.approved_at, d.owner_id, d.created_at, d.status,
    d.approved, d.assigned_by, d.assigned_at, d.agent_version, d.public_ip,
    d.isp, d.pending_command, d.cpu_percent, d.memory_used_mb,
    d.memory_total_mb, d.disk_used_gb, d.disk_total_gb, d.connection_type,
    d.session_bytes_sent, d.session_bytes_received, d.api_key_revoked_at,
    d.lifecycle_status, d.uninstall_requested_at, d.uninstalled_at
  FROM remote_devices d
  LEFT JOIN device_assignments da ON d.device_id = da.device_id
  WHERE (
    (da.user_id = p_user_id AND da.revoked_at IS NULL)
    OR d.owner_id = p_user_id
  )
  ORDER BY d.device_id, d.last_seen DESC NULLS LAST;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION public.get_user_devices IS
  'Returns devices owned by or assigned to a user with api_key redacted.';
REVOKE ALL ON FUNCTION public.get_user_devices(UUID) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.get_user_devices(UUID) TO authenticated;

-- A consumed token is never replayable. If local credential persistence or
-- service startup fails, the dashboard can issue a new token for the same
-- active device identity; the old bearer token must remain dead.
CREATE OR REPLACE FUNCTION public.consume_device_enrollment(
  p_token_hash TEXT,
  p_device_id TEXT,
  p_platform TEXT,
  p_arch TEXT,
  p_cpu_count INTEGER DEFAULT NULL,
  p_ram_bytes BIGINT DEFAULT NULL
)
RETURNS TABLE(device_id TEXT, device_name TEXT, api_key TEXT, owner_id UUID)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
DECLARE
  v_token public.device_enrollment_tokens%ROWTYPE;
  v_existing public.remote_devices%ROWTYPE;
  v_key TEXT;
BEGIN
  SELECT * INTO v_token
  FROM public.device_enrollment_tokens
  WHERE token_hash = p_token_hash
  FOR UPDATE;
  IF NOT FOUND OR v_token.expires_at <= now() THEN
    RAISE EXCEPTION 'Enrollment token is invalid or expired';
  END IF;
  IF p_device_id IS NULL OR p_device_id !~ '^device_[a-f0-9]{32}$' THEN
    RAISE EXCEPTION 'Invalid device ID';
  END IF;

  IF v_token.used_at IS NOT NULL THEN
    RAISE EXCEPTION 'Enrollment token is already used';
  END IF;

  SELECT * INTO v_existing
  FROM public.remote_devices AS d
  WHERE d.device_id = p_device_id;
  IF FOUND THEN
    IF v_existing.owner_id <> v_token.owner_id OR v_existing.lifecycle_status = 'uninstalled' THEN
      RAISE EXCEPTION 'Device identity is already owned or retired';
    END IF;
  END IF;

  v_key := encode(extensions.gen_random_bytes(32), 'hex');
  INSERT INTO public.remote_devices(
    device_id, device_name, platform, arch, cpu_count, ram_bytes,
    api_key, owner_id, approved, approved_at, is_online, last_seen
  ) VALUES (
    p_device_id, v_token.device_name, left(p_platform, 50), left(p_arch, 50),
    p_cpu_count, p_ram_bytes, v_key, v_token.owner_id, true, now(), true, now()
  )
  ON CONFLICT ON CONSTRAINT remote_devices_device_id_key DO UPDATE SET
    device_name = EXCLUDED.device_name,
    owner_id = EXCLUDED.owner_id,
    approved = true,
    approved_at = now(),
    api_key = EXCLUDED.api_key,
    api_key_revoked_at = NULL,
    is_online = true,
    last_seen = now();

  UPDATE public.device_enrollment_tokens
  SET used_at = now(), device_id = p_device_id
  WHERE id = v_token.id;

  INSERT INTO public.audit_logs(device_id, actor, event, details, severity)
  VALUES (p_device_id, v_token.owner_id, 'DEVICE_ENROLLED',
          jsonb_build_object('device_name', v_token.device_name, 'platform', left(p_platform, 50)), 'info');

  RETURN QUERY SELECT p_device_id, v_token.device_name, v_key, v_token.owner_id;
END;
$$;

REVOKE ALL ON FUNCTION public.enqueue_device_command(TEXT, TEXT, JSONB) FROM PUBLIC;
REVOKE ALL ON FUNCTION public.complete_device_command(UUID, TEXT, JSONB) FROM PUBLIC;
REVOKE ALL ON FUNCTION public.consume_device_enrollment(TEXT, TEXT, TEXT, TEXT, INTEGER, BIGINT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.enqueue_device_command(TEXT, TEXT, JSONB) TO authenticated;
GRANT EXECUTE ON FUNCTION public.claim_device_command(TEXT) TO anon, authenticated;
GRANT EXECUTE ON FUNCTION public.complete_device_command(UUID, TEXT, JSONB) TO anon, authenticated;
GRANT EXECUTE ON FUNCTION public.consume_device_enrollment(TEXT, TEXT, TEXT, TEXT, INTEGER, BIGINT) TO service_role;
