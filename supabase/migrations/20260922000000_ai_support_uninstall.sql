CREATE OR REPLACE FUNCTION public.request_ai_support_uninstall(
  p_client_id TEXT,
  p_activity_log_token_hash TEXT
)
RETURNS TEXT
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
  IF NOT FOUND OR v_client.activity_log_token_hash IS DISTINCT FROM p_activity_log_token_hash THEN
    RAISE EXCEPTION 'Invalid AI-support uninstall credentials';
  END IF;
  IF v_client.status IN ('revoked', 'uninstall_pending') THEN
    RETURN 'uninstall_pending';
  END IF;
  IF v_client.status <> 'ready' THEN
    RETURN 'already_uninstalled';
  END IF;
  UPDATE public.ai_support_clients
  SET status = 'uninstall_pending', updated_at = now()
  WHERE id = v_client.id;
  INSERT INTO public.audit_logs(device_id, actor, event, details, severity)
  VALUES (v_client.client_id, v_client.owner_id, 'AI_SUPPORT_CLIENT_UNINSTALL_REQUESTED',
          jsonb_build_object('client_name', v_client.client_name), 'warning');
  RETURN 'uninstall_pending';
END;
$$;

REVOKE ALL ON FUNCTION public.request_ai_support_uninstall(TEXT, TEXT) FROM PUBLIC, authenticated, anon;
GRANT EXECUTE ON FUNCTION public.request_ai_support_uninstall(TEXT, TEXT) TO service_role;

ALTER TABLE public.ai_support_clients
  ADD COLUMN IF NOT EXISTS local_uninstalled_at TIMESTAMPTZ;

ALTER TABLE public.device_enrollment_tokens
  ADD COLUMN IF NOT EXISTS cloudflare_issuance_started_at TIMESTAMPTZ;

CREATE OR REPLACE FUNCTION public.mark_ai_support_local_uninstalled(
  p_client_id TEXT,
  p_activity_log_token_hash TEXT
)
RETURNS BOOLEAN
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
  IF NOT FOUND OR v_client.activity_log_token_hash IS DISTINCT FROM p_activity_log_token_hash THEN
    RAISE EXCEPTION 'Invalid AI-support uninstall credentials';
  END IF;
  IF v_client.status NOT IN ('revoked', 'uninstall_pending') THEN
    RAISE EXCEPTION 'AI-support uninstall has not been requested';
  END IF;
  UPDATE public.ai_support_clients
  SET local_uninstalled_at = COALESCE(local_uninstalled_at, now()), updated_at = now()
  WHERE id = v_client.id;
  INSERT INTO public.audit_logs(device_id, actor, event, details, severity)
  VALUES (v_client.client_id, v_client.owner_id, 'AI_SUPPORT_CLIENT_LOCAL_UNINSTALLED',
          jsonb_build_object('client_name', v_client.client_name), 'info');
  RETURN TRUE;
END;
$$;

REVOKE ALL ON FUNCTION public.mark_ai_support_local_uninstalled(TEXT, TEXT) FROM PUBLIC, authenticated, anon;
GRANT EXECUTE ON FUNCTION public.mark_ai_support_local_uninstalled(TEXT, TEXT) TO service_role;

CREATE OR REPLACE FUNCTION public.rollback_ai_support_local_uninstall(
  p_client_id TEXT,
  p_activity_log_token_hash TEXT
)
RETURNS BOOLEAN
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
  IF NOT FOUND OR v_client.activity_log_token_hash IS DISTINCT FROM p_activity_log_token_hash THEN
    RAISE EXCEPTION 'Invalid AI-support uninstall credentials';
  END IF;
  UPDATE public.ai_support_clients
  SET local_uninstalled_at = NULL, updated_at = now()
  WHERE id = v_client.id;
  RETURN TRUE;
END;
$$;

REVOKE ALL ON FUNCTION public.rollback_ai_support_local_uninstall(TEXT, TEXT) FROM PUBLIC, authenticated, anon;
GRANT EXECUTE ON FUNCTION public.rollback_ai_support_local_uninstall(TEXT, TEXT) TO service_role;
