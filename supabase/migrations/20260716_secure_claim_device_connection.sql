-- SEC-2: Add authorization to claim_device_connection().
--
-- The SECURITY DEFINER function (20251210_session_takeover.sql) kicked every
-- active session on a device and created a new one WITHOUT checking that the
-- caller owns/is assigned to the device. It was granted to `anon, authenticated`
-- (20260214_security_hardening.sql:46), so even anonymous callers could
-- disconnect any user's session on any device.
--
-- Fix: require user_has_device_access(p_device_id) (which returns true for
-- owners, assigned users, and admins/super_admins), and revoke EXECUTE from
-- anon. The single caller is the controller (controller/internal/webrtc/
-- signaling.go:45), which sends the authenticated user's JWT and already has a
-- fallback path if the RPC errors (signaling.go:106) — so legitimate use is
-- unaffected and broken/abusive use is rejected.
--
-- Body is reproduced verbatim from 20251210_session_takeover.sql with only the
-- authorization guard added near the top of BEGIN.

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
BEGIN
    -- Authorization: caller must have access to this device.
    IF NOT public.user_has_device_access(p_device_id) THEN
        RAISE EXCEPTION 'Access denied: no access to device %', p_device_id;
    END IF;

    -- Kick all existing active sessions in webrtc_sessions (controller)
    FOR v_old_sessions IN
        SELECT session_id, controller_id, controller_type
        FROM public.webrtc_sessions
        WHERE device_id = p_device_id
          AND status IN ('pending', 'offer_sent', 'answered', 'connected')
          AND kicked_at IS NULL
    LOOP
        UPDATE public.webrtc_sessions
        SET
            kicked_at = now(),
            kicked_by = p_controller_id,
            status = 'closed',
            updated_at = now()
        WHERE session_id = v_old_sessions.session_id;

        v_kicked_count := v_kicked_count + 1;
    END LOOP;

    -- Kick all existing active sessions in remote_sessions (dashboard)
    FOR v_old_sessions IN
        SELECT id::text as session_id, created_by::text as controller_id
        FROM public.remote_sessions
        WHERE device_id = p_device_id
          AND status IN ('pending', 'active')
          AND expires_at > now()
    LOOP
        UPDATE public.remote_sessions
        SET
            status = 'ended',
            ended_at = now()
        WHERE id::text = v_old_sessions.session_id;

        -- Send kick signal to session_signaling for dashboard to detect
        INSERT INTO public.session_signaling (
            session_id,
            from_side,
            msg_type,
            payload
        ) VALUES (
            v_old_sessions.session_id,
            'system',
            'kick',
            jsonb_build_object(
                'reason', 'taken_over',
                'new_controller_id', p_controller_id,
                'new_controller_type', p_controller_type
            )
        );

        v_kicked_count := v_kicked_count + 1;
    END LOOP;

    -- Create new session in webrtc_sessions
    v_new_session_id := gen_random_uuid()::text;

    INSERT INTO public.webrtc_sessions (
        session_id,
        device_id,
        controller_id,
        controller_type,
        status
    ) VALUES (
        v_new_session_id,
        p_device_id,
        p_controller_id,
        p_controller_type,
        'pending'
    );

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
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Restrict execution: only authenticated users (anon can no longer call it).
REVOKE EXECUTE ON FUNCTION public.claim_device_connection FROM anon;
GRANT EXECUTE ON FUNCTION public.claim_device_connection TO authenticated;

COMMENT ON FUNCTION public.claim_device_connection IS
  'Atomically take over device sessions. Caller must have device access (user_has_device_access).';
