-- Persistent SSH-only AI-support tunnels.
-- The Windows setup creates a localhost-only sshd and reverse-forwards it to
-- an allocated port on the trusted Ubuntu host. No remote_devices row is used.

ALTER TABLE public.ai_support_clients
  ADD COLUMN IF NOT EXISTS tunnel_port INTEGER,
  ADD COLUMN IF NOT EXISTS windows_ssh_user TEXT,
  ADD COLUMN IF NOT EXISTS windows_ssh_port INTEGER,
  ADD COLUMN IF NOT EXISTS activity_log_token_hash TEXT;

ALTER TABLE public.ai_support_clients
  DROP CONSTRAINT IF EXISTS ai_support_clients_tunnel_port_check,
  DROP CONSTRAINT IF EXISTS ai_support_clients_windows_ssh_port_check,
  DROP CONSTRAINT IF EXISTS ai_support_clients_status_check;

ALTER TABLE public.ai_support_clients
  ADD CONSTRAINT ai_support_clients_tunnel_port_check
    CHECK (tunnel_port IS NULL OR tunnel_port BETWEEN 42000 AND 42999),
  ADD CONSTRAINT ai_support_clients_windows_ssh_port_check
    CHECK (windows_ssh_port IS NULL OR windows_ssh_port BETWEEN 1 AND 65535),
  ADD CONSTRAINT ai_support_clients_status_check
    CHECK (status IN ('ready', 'revoked', 'uninstall_pending'));

CREATE UNIQUE INDEX IF NOT EXISTS ai_support_clients_tunnel_port_ready_idx
  ON public.ai_support_clients(tunnel_port)
  WHERE status = 'ready' AND tunnel_port IS NOT NULL;

CREATE OR REPLACE FUNCTION public.consume_ai_support_enrollment(
  p_token_hash TEXT,
  p_client_id TEXT,
  p_hostname TEXT,
  p_platform TEXT,
  p_ssh_host TEXT,
  p_ssh_port INTEGER,
  p_ssh_user TEXT,
  p_ssh_key_fingerprint TEXT,
  p_tunnel_port INTEGER,
  p_windows_ssh_user TEXT,
  p_windows_ssh_port INTEGER,
  p_activity_log_token_hash TEXT
)
RETURNS TABLE(client_id TEXT, client_name TEXT, status TEXT, tunnel_port INTEGER)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
DECLARE
  v_token public.device_enrollment_tokens%ROWTYPE;
  v_existing public.ai_support_clients%ROWTYPE;
  v_client_name TEXT;
  v_fingerprint TEXT;
BEGIN
  IF p_client_id IS NULL OR p_client_id !~ '^ai-[a-z0-9]{8,32}$' THEN
    RAISE EXCEPTION 'Invalid client ID';
  END IF;
  IF p_ssh_host IS NULL OR p_ssh_host !~ '^[A-Za-z0-9._:-]{1,100}$' THEN
    RAISE EXCEPTION 'Invalid SSH host';
  END IF;
  IF p_ssh_user IS NULL OR p_ssh_user !~ '^[A-Za-z0-9._-]{1,32}$' THEN
    RAISE EXCEPTION 'Invalid SSH user';
  END IF;
  IF p_ssh_port IS NULL OR p_ssh_port < 1 OR p_ssh_port > 65535 THEN
    RAISE EXCEPTION 'Invalid SSH port';
  END IF;
  IF p_tunnel_port IS NULL OR p_tunnel_port < 42000 OR p_tunnel_port > 42999 THEN
    RAISE EXCEPTION 'Invalid tunnel port';
  END IF;
  IF p_windows_ssh_user IS NULL OR p_windows_ssh_user !~ '^[A-Za-z0-9._-]{1,32}$' THEN
    RAISE EXCEPTION 'Invalid Windows SSH user';
  END IF;
  IF p_windows_ssh_port IS NULL OR p_windows_ssh_port < 1 OR p_windows_ssh_port > 65535 THEN
    RAISE EXCEPTION 'Invalid Windows SSH port';
  END IF;
  IF p_activity_log_token_hash IS NULL OR p_activity_log_token_hash !~ '^[a-f0-9]{64}$' THEN
    RAISE EXCEPTION 'Invalid activity log token';
  END IF;

  v_fingerprint := NULLIF(btrim(p_ssh_key_fingerprint), '');
  IF v_fingerprint IS NOT NULL AND v_fingerprint !~ '^SHA256:[A-Za-z0-9+/=]{43}$' THEN
    RAISE EXCEPTION 'Invalid SSH key fingerprint';
  END IF;

  SELECT * INTO v_token
  FROM public.device_enrollment_tokens
  WHERE token_hash = p_token_hash
  FOR UPDATE;
  IF NOT FOUND OR v_token.expires_at <= now() THEN
    RAISE EXCEPTION 'Enrollment token is invalid or expired';
  END IF;
  IF v_token.used_at IS NOT NULL THEN
    RAISE EXCEPTION 'Enrollment token is already used';
  END IF;
  IF v_token.purpose <> 'ai_support' THEN
    RAISE EXCEPTION 'Enrollment token is not valid for AI-support enrollment';
  END IF;

  SELECT * INTO v_existing
  FROM public.ai_support_clients c
  WHERE c.client_id = p_client_id
  FOR UPDATE;
  IF FOUND THEN
    IF v_existing.owner_id <> v_token.owner_id THEN
      RAISE EXCEPTION 'Client identity is already owned';
    END IF;
    IF v_existing.status <> 'ready' THEN
      RAISE EXCEPTION 'Client is not available for enrollment';
    END IF;
  END IF;

  v_client_name := v_token.device_name;
  INSERT INTO public.ai_support_clients(
    client_id, owner_id, client_name, hostname, platform,
    ssh_host, ssh_port, ssh_user, ssh_key_fingerprint,
    tunnel_port, windows_ssh_user, windows_ssh_port,
    activity_log_token_hash,
    status, last_seen, created_at, updated_at
  ) VALUES (
    p_client_id, v_token.owner_id, v_client_name,
    NULLIF(btrim(left(p_hostname, 100)), ''),
    NULLIF(btrim(left(p_platform, 50)), ''),
    p_ssh_host, p_ssh_port, p_ssh_user, v_fingerprint,
    p_tunnel_port, p_windows_ssh_user, p_windows_ssh_port,
    p_activity_log_token_hash,
    'ready', now(), now(), now()
  )
  ON CONFLICT ON CONSTRAINT ai_support_clients_client_id_key DO UPDATE SET
    client_name = EXCLUDED.client_name,
    hostname = EXCLUDED.hostname,
    platform = EXCLUDED.platform,
    ssh_host = EXCLUDED.ssh_host,
    ssh_port = EXCLUDED.ssh_port,
    ssh_user = EXCLUDED.ssh_user,
    ssh_key_fingerprint = EXCLUDED.ssh_key_fingerprint,
    tunnel_port = EXCLUDED.tunnel_port,
    windows_ssh_user = EXCLUDED.windows_ssh_user,
    windows_ssh_port = EXCLUDED.windows_ssh_port,
    activity_log_token_hash = EXCLUDED.activity_log_token_hash,
    status = 'ready',
    last_seen = now(),
    updated_at = now();

  UPDATE public.device_enrollment_tokens
  SET used_at = now(), device_id = p_client_id
  WHERE id = v_token.id;

  INSERT INTO public.audit_logs(device_id, actor, event, details, severity)
  VALUES (p_client_id, v_token.owner_id, 'AI_SUPPORT_CLIENT_ENROLLED',
          jsonb_build_object(
            'client_name', v_client_name,
            'hostname', NULLIF(btrim(left(p_hostname, 100)), ''),
            'platform', NULLIF(btrim(left(p_platform, 50)), ''),
            'ssh_host', p_ssh_host,
            'ssh_port', p_ssh_port,
            'ssh_user', p_ssh_user,
            'ssh_key_fingerprint', v_fingerprint,
            'tunnel_port', p_tunnel_port,
            'windows_ssh_user', p_windows_ssh_user,
            'windows_ssh_port', p_windows_ssh_port
          ), 'info');

  RETURN QUERY SELECT p_client_id, v_client_name, 'ready'::text, p_tunnel_port;
END;
$$;

REVOKE ALL ON FUNCTION public.consume_ai_support_enrollment(TEXT, TEXT, TEXT, TEXT, TEXT, INTEGER, TEXT, TEXT, INTEGER, TEXT, INTEGER, TEXT) FROM PUBLIC;
REVOKE ALL ON FUNCTION public.consume_ai_support_enrollment(TEXT, TEXT, TEXT, TEXT, TEXT, INTEGER, TEXT, TEXT, INTEGER, TEXT, INTEGER, TEXT) FROM authenticated, anon;
GRANT EXECUTE ON FUNCTION public.consume_ai_support_enrollment(TEXT, TEXT, TEXT, TEXT, TEXT, INTEGER, TEXT, TEXT, INTEGER, TEXT, INTEGER, TEXT) TO service_role;

CREATE OR REPLACE FUNCTION public.append_ai_support_log(
  p_client_id TEXT,
  p_activity_log_token_hash TEXT,
  p_event TEXT,
  p_details JSONB
)
RETURNS VOID
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
DECLARE
  v_client public.ai_support_clients%ROWTYPE;
BEGIN
  SELECT * INTO v_client
  FROM public.ai_support_clients
  WHERE client_id = p_client_id
  FOR UPDATE;
  IF NOT FOUND OR v_client.status <> 'ready' OR v_client.activity_log_token_hash IS DISTINCT FROM p_activity_log_token_hash THEN
    RAISE EXCEPTION 'Invalid AI-support log credentials';
  END IF;
  IF p_event <> 'AI_SUPPORT_COMMAND' THEN
    RAISE EXCEPTION 'Invalid AI-support log event';
  END IF;
  INSERT INTO public.audit_logs(device_id, actor, event, details, severity)
  VALUES (v_client.client_id, v_client.owner_id, p_event, p_details, 'info');
  UPDATE public.ai_support_clients SET last_seen = now(), updated_at = now() WHERE id = v_client.id;
END;
$$;

REVOKE ALL ON FUNCTION public.append_ai_support_log(TEXT, TEXT, TEXT, JSONB) FROM PUBLIC, authenticated, anon;
GRANT EXECUTE ON FUNCTION public.append_ai_support_log(TEXT, TEXT, TEXT, JSONB) TO service_role;

CREATE OR REPLACE FUNCTION public.revoke_ai_support_client(p_client_id TEXT)
RETURNS BOOLEAN
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
DECLARE
  v_client public.ai_support_clients%ROWTYPE;
BEGIN
  IF auth.uid() IS NULL OR NOT public.is_user_approved(auth.uid()) THEN
    RAISE EXCEPTION 'Approved user access required';
  END IF;
  SELECT * INTO v_client FROM public.ai_support_clients WHERE client_id = p_client_id FOR UPDATE;
  IF NOT FOUND THEN RAISE EXCEPTION 'AI-support client not found'; END IF;
  IF v_client.owner_id <> auth.uid() AND NOT public.is_admin() THEN
    RAISE EXCEPTION 'Only the owner or an admin may revoke this AI-support client';
  END IF;
  IF v_client.status <> 'ready' THEN RAISE EXCEPTION 'AI-support client is already being removed'; END IF;
  UPDATE public.ai_support_clients SET status = 'uninstall_pending', updated_at = now() WHERE id = v_client.id;
  INSERT INTO public.audit_logs(device_id, actor, event, details, severity)
  VALUES (v_client.client_id, auth.uid(), 'AI_SUPPORT_CLIENT_REVOKED',
          jsonb_build_object('client_name', v_client.client_name, 'ssh_host', v_client.ssh_host,
            'ssh_port', v_client.ssh_port, 'ssh_user', v_client.ssh_user), 'warning');
  RETURN TRUE;
END;
$$;

REVOKE ALL ON FUNCTION public.revoke_ai_support_client(TEXT) FROM PUBLIC, anon;
GRANT EXECUTE ON FUNCTION public.revoke_ai_support_client(TEXT) TO authenticated;

CREATE OR REPLACE FUNCTION public.complete_ai_support_uninstall(p_client_id TEXT)
RETURNS BOOLEAN
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
DECLARE
  v_client public.ai_support_clients%ROWTYPE;
BEGIN
  SELECT * INTO v_client FROM public.ai_support_clients WHERE client_id = p_client_id FOR UPDATE;
  IF NOT FOUND THEN RETURN TRUE; END IF;
  IF v_client.status NOT IN ('revoked', 'uninstall_pending') THEN
    RAISE EXCEPTION 'AI-support client is still active';
  END IF;
  INSERT INTO public.audit_logs(device_id, actor, event, details, severity)
  VALUES (v_client.client_id, v_client.owner_id, 'AI_SUPPORT_CLIENT_UNINSTALLED',
          jsonb_build_object('client_name', v_client.client_name), 'info');
  DELETE FROM public.ai_support_clients WHERE id = v_client.id;
  RETURN TRUE;
END;
$$;

REVOKE ALL ON FUNCTION public.complete_ai_support_uninstall(TEXT) FROM PUBLIC, authenticated, anon;
GRANT EXECUTE ON FUNCTION public.complete_ai_support_uninstall(TEXT) TO service_role;

-- AI command contents are only visible to administrators. Keep the existing
-- activity-log behavior for ordinary device/session events.
DROP POLICY IF EXISTS "Authenticated users read audit logs" ON public.audit_logs;
CREATE POLICY "Authenticated users read audit logs" ON public.audit_logs
  FOR SELECT TO authenticated
  USING (
    auth.uid() IS NOT NULL
    AND (COALESCE(device_id, '') NOT LIKE 'ai-%' OR public.is_admin())
  );

COMMENT ON TABLE public.ai_support_clients IS
  'Registered AI-support Windows clients with persistent SSH tunnel metadata. No private keys, passwords, or raw tokens. Writes only via service-role enrollment and log RPCs.';
