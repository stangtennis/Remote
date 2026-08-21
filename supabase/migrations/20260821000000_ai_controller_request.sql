ALTER TABLE public.support_sessions
  ADD COLUMN IF NOT EXISTS controller_requested BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS controller_claimed_by TEXT,
  ADD COLUMN IF NOT EXISTS controller_claimed_at TIMESTAMPTZ;

COMMENT ON COLUMN public.support_sessions.controller_requested IS
  'Admin dashboard selected this AI support session for the Ubuntu controller.';
COMMENT ON COLUMN public.support_sessions.controller_claimed_at IS
  'Short lease preventing two Ubuntu controller watchers from selecting the same session.';

ALTER TABLE public.support_action_audit
  DROP CONSTRAINT IF EXISTS support_action_audit_actor_type_check;
ALTER TABLE public.support_action_audit
  ADD CONSTRAINT support_action_audit_actor_type_check
  CHECK (actor_type IN ('admin', 'client', 'operator', 'ai', 'agent', 'system'));

CREATE OR REPLACE FUNCTION public.revoke_support_session(
  p_session_id UUID,
  p_admin_id UUID,
  p_reason TEXT
)
RETURNS BOOLEAN
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
  v_now TIMESTAMPTZ := now();
BEGIN
  UPDATE public.support_sessions
  SET status = 'ended',
      ended_at = v_now,
      revoked_at = v_now,
      revoked_by = p_admin_id,
      revoke_reason = COALESCE(NULLIF(left(p_reason, 500), ''), 'Revoked by admin'),
      controller_requested = false,
      controller_claimed_by = NULL,
      controller_claimed_at = NULL
  WHERE id = p_session_id
    AND (created_by = p_admin_id OR is_public = true)
    AND status IN ('pending', 'active');

  IF NOT FOUND THEN
    RETURN FALSE;
  END IF;

  INSERT INTO public.session_signaling(session_id, from_side, msg_type, payload)
  VALUES (p_session_id, 'dashboard', 'bye', jsonb_build_object('reason', 'revoked_by_admin'));
  RETURN TRUE;
END;
$$;

CREATE OR REPLACE FUNCTION public.request_support_controller(
  p_session_id UUID,
  p_admin_id UUID
)
RETURNS BOOLEAN
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
  v_updated INTEGER;
BEGIN
  UPDATE public.support_sessions
  SET controller_requested = false,
      controller_claimed_by = NULL,
      controller_claimed_at = NULL
  WHERE created_by = p_admin_id
    AND support_mode = 'ai'
    AND status IN ('pending', 'active');

  UPDATE public.support_sessions
  SET controller_requested = true
  WHERE id = p_session_id
    AND created_by = p_admin_id
    AND support_mode = 'ai'
    AND status IN ('pending', 'active');
  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated = 1;
END;
$$;

REVOKE ALL ON FUNCTION public.request_support_controller(UUID, UUID) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.request_support_controller(UUID, UUID) TO service_role;
