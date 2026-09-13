-- AI-support enrollment hardening (ordered after 20260916000000).
--
-- 1. Purpose isolation for the ordinary agent enrollment RPC:
--    consume_device_enrollment now explicitly refuses tokens whose purpose is
--    not 'agent', so an ai_support token can never register a remote_devices
--    row (and, conversely, consume_ai_support_enrollment already refuses
--    agent tokens). Signature, behavior, and grants are otherwise identical
--    to the 20260915000000 implementation.
-- 2. Owner/admin revoke RPC for AI-support clients with a redacted audit
--    event. Revocation is terminal: consume_ai_support_enrollment refuses to
--    resurrect a revoked client_id, and no client role has UPDATE access to
--    ai_support_clients (writes are service-role only).

-- ===========================================
-- 1. consume_device_enrollment: agent-purpose tokens only
-- ===========================================
-- Full replacement of the current (20260915000000) implementation with one
-- addition: the locked token row must have purpose='agent'.

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

  -- Purpose isolation: only 'agent' tokens may register remote_devices.
  -- ai_support tokens are consumed exclusively by consume_ai_support_enrollment.
  IF v_token.purpose <> 'agent' THEN
    RAISE EXCEPTION 'Enrollment token is not valid for device enrollment';
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

-- Same grants as 20260915000000 (service role only). anon/authenticated are
-- revoked explicitly as well: older databases may still carry direct EXECUTE
-- grants from before the 20260915000000 tightening, and REVOKE FROM PUBLIC
-- alone would not remove them.
REVOKE ALL ON FUNCTION public.consume_device_enrollment(TEXT, TEXT, TEXT, TEXT, INTEGER, BIGINT) FROM PUBLIC;
REVOKE ALL ON FUNCTION public.consume_device_enrollment(TEXT, TEXT, TEXT, TEXT, INTEGER, BIGINT) FROM anon, authenticated;
GRANT EXECUTE ON FUNCTION public.consume_device_enrollment(TEXT, TEXT, TEXT, TEXT, INTEGER, BIGINT) TO service_role;

-- ===========================================
-- 2. revoke_ai_support_client: owner/admin terminal revoke
-- ===========================================

CREATE OR REPLACE FUNCTION public.revoke_ai_support_client(p_client_id TEXT)
RETURNS BOOLEAN
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
DECLARE
  v_client public.ai_support_clients%ROWTYPE;
BEGIN
  IF auth.uid() IS NULL THEN
    RAISE EXCEPTION 'Authentication required';
  END IF;
  IF NOT public.is_user_approved(auth.uid()) THEN
    RAISE EXCEPTION 'Approved user access required';
  END IF;
  IF p_client_id IS NULL OR p_client_id !~ '^ai-[a-z0-9]{8,32}$' THEN
    RAISE EXCEPTION 'Invalid client ID';
  END IF;

  SELECT * INTO v_client
  FROM public.ai_support_clients c
  WHERE c.client_id = p_client_id
  FOR UPDATE;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'AI-support client not found';
  END IF;
  IF v_client.owner_id <> auth.uid() AND NOT public.is_admin() THEN
    RAISE EXCEPTION 'Only the owner or an admin may revoke this AI-support client';
  END IF;
  IF v_client.status = 'revoked' THEN
    RAISE EXCEPTION 'AI-support client is already revoked';
  END IF;

  UPDATE public.ai_support_clients
  SET status = 'revoked',
      updated_at = now()
  WHERE id = v_client.id;

  -- Redacted audit event: public metadata only. The table never holds
  -- private keys, passwords, or raw tokens, and none are added here.
  INSERT INTO public.audit_logs(device_id, actor, event, details, severity)
  VALUES (v_client.client_id, auth.uid(), 'AI_SUPPORT_CLIENT_REVOKED',
          jsonb_build_object(
            'client_name', v_client.client_name,
            'ssh_host', v_client.ssh_host,
            'ssh_port', v_client.ssh_port,
            'ssh_user', v_client.ssh_user
          ), 'warning');

  RETURN TRUE;
END;
$$;

-- Authenticated callers only; nothing from public/anon.
REVOKE ALL ON FUNCTION public.revoke_ai_support_client(TEXT) FROM PUBLIC;
REVOKE ALL ON FUNCTION public.revoke_ai_support_client(TEXT) FROM anon;
GRANT EXECUTE ON FUNCTION public.revoke_ai_support_client(TEXT) TO authenticated;

COMMENT ON FUNCTION public.revoke_ai_support_client(TEXT) IS
  'Terminal revoke of an AI-support client. Approved owner or admin only; sets status=revoked and writes a redacted AI_SUPPORT_CLIENT_REVOKED audit event. A revoked client_id cannot be re-enrolled (consume_ai_support_enrollment refuses resurrected identities).';
