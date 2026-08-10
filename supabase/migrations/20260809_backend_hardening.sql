-- ============================================================================
-- Backend hardening — 2026-08-09
-- accept_invitation race, claim_device_connection identity spoof, missing indexes.
-- ============================================================================

-- 1. accept_invitation: lock the invitation row during redemption (FOR UPDATE)
--    so two concurrent calls cannot both pass the accepted_at IS NULL check.
--    Also adds search_path hardening.
DROP FUNCTION IF EXISTS accept_invitation(text);

CREATE OR REPLACE FUNCTION accept_invitation(p_token TEXT)
RETURNS BOOLEAN AS $$
DECLARE
    v_invitation RECORD;
    v_caller_email TEXT;
BEGIN
    -- Find + LOCK the invitation row so concurrent redemptions serialize.
    SELECT * INTO v_invitation
    FROM user_invitations
    WHERE token = p_token
      AND expires_at > NOW()
      AND accepted_at IS NULL
    FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Invalid or expired invitation';
    END IF;

    -- Bind invitation to the caller's email (defence-in-depth against token leaks)
    SELECT email INTO v_caller_email FROM auth.users WHERE id = auth.uid();
    IF v_caller_email IS NULL THEN
        RAISE EXCEPTION 'Could not resolve caller email';
    END IF;
    IF v_invitation.email IS NOT NULL AND v_invitation.email <> v_caller_email THEN
        RAISE EXCEPTION 'Invitation is for a different email address';
    END IF;

    -- Mark accepted only if still unclaimed (guards against the race window).
    UPDATE user_invitations
    SET accepted_at = NOW()
    WHERE token = p_token AND accepted_at IS NULL;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Invitation was already accepted';
    END IF;

    INSERT INTO user_approvals (user_id, approved, role, approved_by)
    VALUES (auth.uid(), true, v_invitation.role, v_invitation.invited_by)
    ON CONFLICT (user_id) DO UPDATE
    SET approved = true, role = v_invitation.role;

    RETURN TRUE;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, pg_temp;

-- 2. claim_device_connection: derive controller identity from auth.uid() so a
--    caller cannot spoof kicked_by / controller_id in audit/session data.
--    (Signature unchanged — CREATE OR REPLACE.)
CREATE OR REPLACE FUNCTION public.claim_device_connection(
    p_device_id text,
    p_controller_id text,
    p_controller_type text DEFAULT 'dashboard'
)
RETURNS jsonb AS $$
DECLARE
    v_old_sessions record;
    v_new_session_id text;
    v_kicked_count int := 0;
    v_caller text := auth.uid()::text;
BEGIN
    IF NOT public.user_has_device_access(p_device_id) THEN
        RAISE EXCEPTION 'Access denied: no access to device %', p_device_id;
    END IF;

    -- Recorded identity is always the authenticated caller, never the param.
    FOR v_old_sessions IN
        SELECT session_id FROM public.webrtc_sessions
        WHERE device_id = p_device_id
          AND status IN ('pending', 'offer_sent', 'answered', 'connected')
          AND kicked_at IS NULL
    LOOP
        UPDATE public.webrtc_sessions
        SET kicked_at = now(), kicked_by = v_caller, status = 'closed', updated_at = now()
        WHERE session_id = v_old_sessions.session_id;
        v_kicked_count := v_kicked_count + 1;
    END LOOP;

    FOR v_old_sessions IN
        SELECT id::text as session_id
        FROM public.remote_sessions
        WHERE device_id = p_device_id
          AND status IN ('pending', 'active')
          AND expires_at > now()
    LOOP
        UPDATE public.remote_sessions
        SET status = 'ended', ended_at = now()
        WHERE id::text = v_old_sessions.session_id;

        INSERT INTO public.session_signaling (session_id, from_side, msg_type, payload)
        VALUES (
            v_old_sessions.session_id, 'system', 'kick',
            jsonb_build_object('reason', 'taken_over', 'new_controller_id', v_caller, 'new_controller_type', p_controller_type)
        );
        v_kicked_count := v_kicked_count + 1;
    END LOOP;

    v_new_session_id := gen_random_uuid()::text;
    INSERT INTO public.webrtc_sessions (session_id, device_id, controller_id, controller_type, status)
    VALUES (v_new_session_id, p_device_id, v_caller, p_controller_type, 'pending');

    RETURN jsonb_build_object(
        'success', true,
        'session_id', v_new_session_id,
        'kicked_sessions', v_kicked_count,
        'message', CASE
            WHEN v_kicked_count > 0 THEN 'Took over from ' || v_kicked_count || ' existing session(s)'
            ELSE 'New session created'
        END
    );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, pg_temp;

REVOKE EXECUTE ON FUNCTION public.claim_device_connection FROM anon;
GRANT EXECUTE ON FUNCTION public.claim_device_connection TO authenticated;

-- 3. Missing indexes on foreign-key and frequently-queried columns.
CREATE INDEX IF NOT EXISTS idx_device_transfers_device    ON public.device_transfers(device_id);
CREATE INDEX IF NOT EXISTS idx_device_transfers_from_user ON public.device_transfers(from_user_id);
CREATE INDEX IF NOT EXISTS idx_device_transfers_to_user   ON public.device_transfers(to_user_id);
CREATE INDEX IF NOT EXISTS idx_device_transfers_by        ON public.device_transfers(transferred_by);
CREATE INDEX IF NOT EXISTS idx_user_invitations_invited_by ON public.user_invitations(invited_by);
CREATE INDEX IF NOT EXISTS idx_user_invitations_email     ON public.user_invitations(email);
CREATE INDEX IF NOT EXISTS idx_support_sessions_pin       ON public.support_sessions(pin);
