-- AI-support client enrollment: dedicated Windows->Ubuntu SSH clients.
--
-- This is intentionally separate from the normal Remote Desktop agent
-- enrollment:
--   purpose='agent'      -> remote_devices via consume_device_enrollment
--   purpose='ai_support' -> ai_support_clients via consume_ai_support_enrollment
--
-- SSH direction is Windows client -> trusted Ubuntu AI-support host. No
-- inbound Windows SSH port is involved, and this table never stores private
-- keys, passwords, or raw tokens -- only public connection metadata
-- (SSH host, port, user, and the public key fingerprint).
--
-- Compatible with 20260914000000_client_enrollment_commands and
-- 20260915000000_enrollment_command_hardening: existing agent tokens keep
-- their behavior through the 'agent' default.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ===========================================
-- 1. Enrollment token purpose
-- ===========================================

ALTER TABLE public.device_enrollment_tokens
  ADD COLUMN IF NOT EXISTS purpose TEXT NOT NULL DEFAULT 'agent';

ALTER TABLE public.device_enrollment_tokens
  DROP CONSTRAINT IF EXISTS device_enrollment_tokens_purpose_check;
ALTER TABLE public.device_enrollment_tokens
  ADD CONSTRAINT device_enrollment_tokens_purpose_check
  CHECK (purpose IN ('agent', 'ai_support'));

-- ===========================================
-- 2. AI-support client registry (public metadata only)
-- ===========================================

CREATE TABLE IF NOT EXISTS public.ai_support_clients (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  client_id TEXT NOT NULL UNIQUE CHECK (client_id ~ '^ai-[a-z0-9]{8,32}$'),
  owner_id UUID NOT NULL REFERENCES auth.users(id),
  client_name TEXT NOT NULL CHECK (char_length(client_name) BETWEEN 1 AND 64),
  hostname TEXT CHECK (hostname IS NULL OR char_length(hostname) BETWEEN 1 AND 100),
  platform TEXT CHECK (platform IS NULL OR char_length(platform) BETWEEN 1 AND 50),
  ssh_host TEXT NOT NULL CHECK (char_length(ssh_host) BETWEEN 1 AND 100),
  ssh_port INTEGER NOT NULL DEFAULT 22 CHECK (ssh_port BETWEEN 1 AND 65535),
  ssh_user TEXT NOT NULL CHECK (char_length(ssh_user) BETWEEN 1 AND 32),
  -- SHA256 fingerprint of the PUBLIC key, e.g. 'SHA256:<43 base64 chars>'.
  ssh_key_fingerprint TEXT CHECK (
    ssh_key_fingerprint IS NULL
    OR ssh_key_fingerprint ~ '^SHA256:[A-Za-z0-9+/=]{43}$'
  ),
  status TEXT NOT NULL DEFAULT 'ready' CHECK (status IN ('ready', 'revoked')),
  last_seen TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ai_support_clients_owner_idx
  ON public.ai_support_clients(owner_id, created_at DESC);

-- ===========================================
-- 3. RLS: read-only for owners/admins
-- ===========================================
-- All writes go through consume_ai_support_enrollment (service role only),
-- so no client-role INSERT/UPDATE/DELETE policies exist for this table.

ALTER TABLE public.ai_support_clients ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS "Owners can view AI-support clients" ON public.ai_support_clients;
CREATE POLICY "Owners can view AI-support clients"
  ON public.ai_support_clients FOR SELECT TO authenticated
  USING (
    public.is_user_approved(auth.uid())
    AND (owner_id = auth.uid() OR public.is_admin())
  );

-- Explicit table privileges (do not rely on default privileges):
-- authenticated may only read through the policy above; service_role does
-- all writes via the RPC below.
GRANT SELECT ON public.ai_support_clients TO authenticated;
REVOKE INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES ON public.ai_support_clients FROM anon, authenticated;
GRANT SELECT, INSERT, UPDATE, DELETE ON public.ai_support_clients TO service_role;

-- ===========================================
-- 4. Service-role-only enrollment consumption
-- ===========================================

CREATE OR REPLACE FUNCTION public.consume_ai_support_enrollment(
  p_token_hash TEXT,
  p_client_id TEXT,
  p_hostname TEXT,
  p_platform TEXT,
  p_ssh_host TEXT,
  p_ssh_port INTEGER,
  p_ssh_user TEXT,
  p_ssh_key_fingerprint TEXT
)
RETURNS TABLE(client_id TEXT, client_name TEXT, status TEXT)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
DECLARE
  v_token public.device_enrollment_tokens%ROWTYPE;
  v_existing public.ai_support_clients%ROWTYPE;
  v_client_name TEXT;
  v_ssh_port INTEGER;
  v_fingerprint TEXT;
BEGIN
  -- Bounded, strictly validated inputs. These values are rendered in the
  -- dashboard and stored as connection metadata; never trust caller lengths.
  IF p_client_id IS NULL OR p_client_id !~ '^ai-[a-z0-9]{8,32}$' THEN
    RAISE EXCEPTION 'Invalid client ID';
  END IF;
  IF p_ssh_host IS NULL OR p_ssh_host !~ '^[A-Za-z0-9._:-]{1,100}$' THEN
    RAISE EXCEPTION 'Invalid SSH host';
  END IF;
  IF p_ssh_user IS NULL OR p_ssh_user !~ '^[A-Za-z0-9._-]{1,32}$' THEN
    RAISE EXCEPTION 'Invalid SSH user';
  END IF;
  v_ssh_port := COALESCE(p_ssh_port, 22);
  IF v_ssh_port < 1 OR v_ssh_port > 65535 THEN
    RAISE EXCEPTION 'Invalid SSH port';
  END IF;
  v_fingerprint := NULLIF(btrim(p_ssh_key_fingerprint), '');
  IF v_fingerprint IS NOT NULL AND v_fingerprint !~ '^SHA256:[A-Za-z0-9+/=]{43}$' THEN
    RAISE EXCEPTION 'Invalid SSH key fingerprint';
  END IF;

  -- Lock the token row; single-use, unexpired, ai_support purpose only.
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

  -- Upsert guard: the client identity must stay with its owner and must not
  -- resurrect a revoked registration. (Alias-qualified: the OUT parameter
  -- client_id would otherwise be ambiguous against the table column.)
  SELECT * INTO v_existing
  FROM public.ai_support_clients c
  WHERE c.client_id = p_client_id
  FOR UPDATE;
  IF FOUND THEN
    IF v_existing.owner_id <> v_token.owner_id THEN
      RAISE EXCEPTION 'Client identity is already owned';
    END IF;
    IF v_existing.status = 'revoked' THEN
      RAISE EXCEPTION 'Client is revoked';
    END IF;
  END IF;

  v_client_name := v_token.device_name;

  INSERT INTO public.ai_support_clients(
    client_id, owner_id, client_name, hostname, platform,
    ssh_host, ssh_port, ssh_user, ssh_key_fingerprint,
    status, last_seen, created_at, updated_at
  ) VALUES (
    p_client_id, v_token.owner_id, v_client_name,
    NULLIF(btrim(left(p_hostname, 100)), ''),
    NULLIF(btrim(left(p_platform, 50)), ''),
    p_ssh_host, v_ssh_port, p_ssh_user, v_fingerprint,
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
    status = 'ready',
    last_seen = now(),
    updated_at = now();

  UPDATE public.device_enrollment_tokens
  SET used_at = now(), device_id = p_client_id
  WHERE id = v_token.id;

  -- Redacted audit event: only public metadata, never the raw token.
  INSERT INTO public.audit_logs(device_id, actor, event, details, severity)
  VALUES (p_client_id, v_token.owner_id, 'AI_SUPPORT_CLIENT_ENROLLED',
          jsonb_build_object(
            'client_name', v_client_name,
            'hostname', NULLIF(btrim(left(p_hostname, 100)), ''),
            'platform', NULLIF(btrim(left(p_platform, 50)), ''),
            'ssh_host', p_ssh_host,
            'ssh_port', v_ssh_port,
            'ssh_user', p_ssh_user,
            'ssh_key_fingerprint', v_fingerprint
          ), 'info');

  RETURN QUERY SELECT p_client_id, v_client_name, 'ready'::text;
END;
$$;

REVOKE ALL ON FUNCTION public.consume_ai_support_enrollment(TEXT, TEXT, TEXT, TEXT, TEXT, INTEGER, TEXT, TEXT) FROM PUBLIC;
REVOKE ALL ON FUNCTION public.consume_ai_support_enrollment(TEXT, TEXT, TEXT, TEXT, TEXT, INTEGER, TEXT, TEXT) FROM authenticated, anon;
GRANT EXECUTE ON FUNCTION public.consume_ai_support_enrollment(TEXT, TEXT, TEXT, TEXT, TEXT, INTEGER, TEXT, TEXT) TO service_role;

COMMENT ON TABLE public.ai_support_clients IS
  'Registered AI-support Windows clients (SSH metadata only; no private keys, passwords, or tokens). Writes only via consume_ai_support_enrollment.';
COMMENT ON FUNCTION public.consume_ai_support_enrollment IS
  'Single-use AI-support enrollment: locks the token, validates bounded metadata, upserts the client row, and writes a redacted audit event. Service role only.';
