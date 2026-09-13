-- Named always-online client enrollment and durable remote command ledger.
-- Enrollment tokens are one-time bearer credentials. Never log or persist the
-- raw token; only its SHA-256 hash is stored.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

ALTER TABLE public.remote_devices
  ADD COLUMN IF NOT EXISTS lifecycle_status TEXT NOT NULL DEFAULT 'active',
  ADD COLUMN IF NOT EXISTS uninstall_requested_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS uninstalled_at TIMESTAMPTZ;

ALTER TABLE public.remote_devices
  DROP CONSTRAINT IF EXISTS remote_devices_lifecycle_status_check;
ALTER TABLE public.remote_devices
  ADD CONSTRAINT remote_devices_lifecycle_status_check
  CHECK (lifecycle_status IN ('active', 'uninstall_pending', 'uninstalled'));

CREATE TABLE IF NOT EXISTS public.device_enrollment_tokens (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  token_hash TEXT NOT NULL UNIQUE,
  owner_id UUID NOT NULL REFERENCES auth.users(id),
  device_name TEXT NOT NULL CHECK (char_length(device_name) BETWEEN 1 AND 64),
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  device_id TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS device_enrollment_tokens_owner_idx
  ON public.device_enrollment_tokens(owner_id, created_at DESC);

CREATE TABLE IF NOT EXISTS public.device_commands (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  device_id TEXT NOT NULL,
  command_type TEXT NOT NULL CHECK (command_type IN (
    'force_update', 'enable_relay', 'disable_relay', 'restart',
    'lock', 'shutdown', 'uninstall'
  )),
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  requested_by UUID NOT NULL REFERENCES auth.users(id),
  status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN (
    'queued', 'delivered', 'started', 'succeeded', 'failed',
    'expired', 'cancelled'
  )),
  result JSONB NOT NULL DEFAULT '{}'::jsonb,
  expires_at TIMESTAMPTZ NOT NULL DEFAULT (now() + interval '15 minutes'),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  delivered_at TIMESTAMPTZ,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS device_commands_poll_idx
  ON public.device_commands(device_id, status, created_at ASC);
CREATE INDEX IF NOT EXISTS device_commands_requester_idx
  ON public.device_commands(requested_by, created_at DESC);

ALTER TABLE public.device_enrollment_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.device_commands ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS "Owners can view device commands" ON public.device_commands;
CREATE POLICY "Owners can view device commands"
  ON public.device_commands FOR SELECT TO authenticated
  USING (public.user_has_device_access(device_id));

-- All command writes go through these functions. This prevents clients from
-- changing requester, status, expiry, or result fields directly.
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

  IF p_command_type = 'uninstall' THEN
    UPDATE public.remote_devices
    SET lifecycle_status = 'uninstall_pending', uninstall_requested_at = now()
    WHERE device_id = p_device_id AND lifecycle_status = 'active';
    IF NOT FOUND THEN
      RAISE EXCEPTION 'Device is already uninstalling or uninstalled';
    END IF;
  END IF;

  INSERT INTO public.device_commands(device_id, command_type, payload, requested_by)
  VALUES (p_device_id, p_command_type, COALESCE(p_payload, '{}'::jsonb), auth.uid())
  RETURNING id INTO v_id;

  INSERT INTO public.audit_logs(device_id, actor, event, details, severity)
  VALUES (p_device_id, auth.uid(), 'REMOTE_COMMAND_REQUESTED',
          jsonb_build_object('command_id', v_id, 'command_type', p_command_type), 'warning');
  RETURN v_id;
END;
$$;

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
      AND c.status = 'queued'
      AND c.expires_at > now()
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

  UPDATE public.device_commands
  SET status = p_status,
      result = COALESCE(p_result, '{}'::jsonb),
      started_at = CASE WHEN p_status = 'started' AND started_at IS NULL THEN now() ELSE started_at END,
      completed_at = CASE WHEN p_status IN ('succeeded', 'failed', 'cancelled') THEN now() ELSE completed_at END
  WHERE id = p_command_id
    AND device_id = v_device_id
    AND status IN ('delivered', 'started');
  IF NOT FOUND THEN RETURN FALSE; END IF;

  SELECT command_type INTO v_command_type
  FROM public.device_commands WHERE id = p_command_id;

  INSERT INTO public.audit_logs(device_id, event, details, severity)
  VALUES (
    v_device_id,
    CASE WHEN p_status = 'started' THEN 'REMOTE_COMMAND_STARTED'
         WHEN p_status = 'succeeded' THEN 'REMOTE_COMMAND_SUCCEEDED'
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
  v_key TEXT;
BEGIN
  SELECT * INTO v_token
  FROM public.device_enrollment_tokens
  WHERE token_hash = p_token_hash
  FOR UPDATE;
  IF NOT FOUND OR v_token.used_at IS NOT NULL OR v_token.expires_at <= now() THEN
    RAISE EXCEPTION 'Enrollment token is invalid or expired';
  END IF;
  IF p_device_id IS NULL OR p_device_id !~ '^device_[a-f0-9]{32}$' THEN
    RAISE EXCEPTION 'Invalid device ID';
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
REVOKE ALL ON FUNCTION public.claim_device_command(TEXT) FROM PUBLIC;
REVOKE ALL ON FUNCTION public.complete_device_command(UUID, TEXT, JSONB) FROM PUBLIC;
REVOKE ALL ON FUNCTION public.consume_device_enrollment(TEXT, TEXT, TEXT, TEXT, INTEGER, BIGINT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.enqueue_device_command(TEXT, TEXT, JSONB) TO authenticated;
GRANT EXECUTE ON FUNCTION public.claim_device_command(TEXT) TO anon, authenticated;
GRANT EXECUTE ON FUNCTION public.complete_device_command(UUID, TEXT, JSONB) TO anon, authenticated;
GRANT EXECUTE ON FUNCTION public.consume_device_enrollment(TEXT, TEXT, TEXT, TEXT, INTEGER, BIGINT) TO service_role;

COMMENT ON TABLE public.device_commands IS
  'Durable authenticated device command ledger. Do not delete rows after completion; they are part of the audit history.';
