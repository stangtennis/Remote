-- AI support sessions require a client-entered admin code and explicit consent.
-- This migration adds the authorization state and a durable action history.

ALTER TABLE public.support_sessions
  ADD COLUMN IF NOT EXISTS support_mode TEXT NOT NULL DEFAULT 'screen',
  ADD COLUMN IF NOT EXISTS requested_scopes TEXT[] NOT NULL DEFAULT ARRAY['screen'],
  ADD COLUMN IF NOT EXISTS requires_client_code BOOLEAN NOT NULL DEFAULT true,
  ADD COLUMN IF NOT EXISTS client_code_verified_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS client_grant_token UUID,
  ADD COLUMN IF NOT EXISTS client_consent_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS client_consent_scopes TEXT[],
  ADD COLUMN IF NOT EXISTS client_label TEXT,
  ADD COLUMN IF NOT EXISTS consent_policy_version TEXT,
  ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS revoked_by UUID REFERENCES auth.users(id),
  ADD COLUMN IF NOT EXISTS revoke_reason TEXT;

ALTER TABLE public.support_sessions
  DROP CONSTRAINT IF EXISTS support_sessions_support_mode_check;
ALTER TABLE public.support_sessions
  ADD CONSTRAINT support_sessions_support_mode_check
  CHECK (support_mode IN ('screen', 'ai'));

CREATE UNIQUE INDEX IF NOT EXISTS support_sessions_client_grant_token_idx
  ON public.support_sessions(client_grant_token)
  WHERE client_grant_token IS NOT NULL;

CREATE TABLE IF NOT EXISTS public.support_action_audit (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  support_session_id UUID NOT NULL REFERENCES public.support_sessions(id) ON DELETE RESTRICT,
  device_id TEXT,
  actor_type TEXT NOT NULL CHECK (actor_type IN ('admin', 'client', 'operator', 'ai', 'system')),
  actor_id UUID,
  action_type TEXT NOT NULL,
  target TEXT,
  status TEXT NOT NULL DEFAULT 'started' CHECK (status IN ('started', 'succeeded', 'failed', 'cancelled')),
  summary TEXT NOT NULL,
  details JSONB NOT NULL DEFAULT '{}'::jsonb,
  verified BOOLEAN NOT NULL DEFAULT false,
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS support_action_audit_session_idx
  ON public.support_action_audit(support_session_id, created_at DESC);
CREATE INDEX IF NOT EXISTS support_action_audit_device_idx
  ON public.support_action_audit(device_id, created_at DESC)
  WHERE device_id IS NOT NULL;

ALTER TABLE public.support_action_audit ENABLE ROW LEVEL SECURITY;

-- Public support is intentionally viewable by the authenticated dashboard as
-- well as the anonymous sharer. Private sessions remain owner/grant scoped.
DROP POLICY IF EXISTS "Authenticated can read public support sessions" ON public.support_sessions;
CREATE POLICY "Authenticated can read public support sessions"
  ON public.support_sessions FOR SELECT TO authenticated
  USING (is_public = true AND status IN ('pending', 'active'));

DROP POLICY IF EXISTS "Authenticated can read public support signaling" ON public.session_signaling;
CREATE POLICY "Authenticated can read public support signaling"
  ON public.session_signaling FOR SELECT TO authenticated
  USING (
    session_id IN (SELECT id FROM public.support_sessions WHERE is_public = true)
  );

DROP POLICY IF EXISTS "Authenticated can insert public support signaling" ON public.session_signaling;
CREATE POLICY "Authenticated can insert public support signaling"
  ON public.session_signaling FOR INSERT TO authenticated
  WITH CHECK (
    session_id IN (SELECT id FROM public.support_sessions WHERE is_public = true)
  );

DROP POLICY IF EXISTS "Admins can view own support action audit" ON public.support_action_audit;
CREATE POLICY "Admins can view own support action audit"
  ON public.support_action_audit FOR SELECT TO authenticated
  USING (
    EXISTS (
      SELECT 1
      FROM public.support_sessions s
      WHERE s.id = support_action_audit.support_session_id
        AND s.created_by = auth.uid()
    )
  );

COMMENT ON TABLE public.support_action_audit IS
  'Operator/AI support history. Do not store passwords, tokens, or raw secrets in details.';

-- Keep signaling and lifecycle changes status-conditional at the database
-- boundary. Edge Functions can otherwise race when revoke and ready arrive
-- concurrently.
CREATE OR REPLACE FUNCTION public.append_support_signal(
  p_session_id UUID,
  p_from_side TEXT,
  p_msg_type TEXT,
  p_payload JSONB
)
RETURNS VOID
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
BEGIN
  IF p_from_side <> 'support' OR p_msg_type NOT IN ('answer', 'ice', 'bye') THEN
    RAISE EXCEPTION 'Invalid support signal';
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM public.support_sessions
    WHERE id = p_session_id
      AND status IN ('pending', 'active')
      AND expires_at > now()
  ) THEN
    RAISE EXCEPTION 'Support session is no longer active';
  END IF;

  INSERT INTO public.session_signaling(session_id, from_side, msg_type, payload)
  VALUES (p_session_id, p_from_side, p_msg_type, p_payload);
END;
$$;

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
      revoke_reason = COALESCE(NULLIF(left(p_reason, 500), ''), 'Revoked by admin')
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

REVOKE ALL ON FUNCTION public.append_support_signal(UUID, TEXT, TEXT, JSONB) FROM PUBLIC;
REVOKE ALL ON FUNCTION public.revoke_support_session(UUID, UUID, TEXT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.append_support_signal(UUID, TEXT, TEXT, JSONB) TO service_role;
GRANT EXECUTE ON FUNCTION public.revoke_support_session(UUID, UUID, TEXT) TO service_role;

-- Preserve the action history when the normal support-session retention job
-- tries to remove old sessions.
ALTER TABLE public.support_action_audit
  DROP CONSTRAINT IF EXISTS support_action_audit_support_session_id_fkey;
ALTER TABLE public.support_action_audit
  ADD CONSTRAINT support_action_audit_support_session_id_fkey
  FOREIGN KEY (support_session_id) REFERENCES public.support_sessions(id) ON DELETE RESTRICT;
