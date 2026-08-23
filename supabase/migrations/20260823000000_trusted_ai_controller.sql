-- Trusted AI controller sessions.
-- The AI controller may use the normal agent with full controller channels, but
-- it must enter through a server-side capability checked by the Edge Function.

CREATE OR REPLACE FUNCTION public.claim_ai_device_connection(
    p_device_id text,
    p_controller_id text,
    p_actor_id uuid
)
RETURNS jsonb AS $$
DECLARE
    v_old_session record;
    v_new_session_id text;
    v_kicked_count int := 0;
BEGIN
    IF p_device_id IS NULL OR p_controller_id IS NULL OR p_actor_id IS NULL THEN
        RAISE EXCEPTION 'AI session requires device, controller, and actor';
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM public.remote_devices d
        WHERE d.device_id = p_device_id
          AND (
              d.owner_id = p_actor_id
              OR EXISTS (
                  SELECT 1
                  FROM public.device_assignments da
                  WHERE da.device_id = d.device_id
                    AND da.user_id = p_actor_id
                    AND da.revoked_at IS NULL
              )
              OR EXISTS (
                  SELECT 1
                  FROM public.user_approvals ua
                  WHERE ua.user_id = p_actor_id
                    AND ua.role IN ('admin', 'super_admin')
                    AND ua.approved = true
              )
          )
    ) THEN
        RAISE EXCEPTION 'Access denied: no access to device %', p_device_id;
    END IF;

    -- The trigger below checks this request marker. Only this service-role
    -- function is allowed to create an AI-typed session.
    PERFORM set_config('request.jwt.claim.role', 'service_role', true);

    FOR v_old_session IN
        SELECT session_id
        FROM public.webrtc_sessions
        WHERE device_id = p_device_id
          AND status IN ('pending', 'offer_sent', 'answered', 'connected')
          AND kicked_at IS NULL
    LOOP
        UPDATE public.webrtc_sessions
        SET kicked_at = now(),
            kicked_by = p_controller_id,
            status = 'closed',
            updated_at = now()
        WHERE session_id = v_old_session.session_id;
        v_kicked_count := v_kicked_count + 1;
    END LOOP;

    FOR v_old_session IN
        SELECT id::text AS session_id
        FROM public.remote_sessions
        WHERE device_id = p_device_id
          AND status IN ('pending', 'active')
          AND expires_at > now()
    LOOP
        UPDATE public.remote_sessions
        SET status = 'ended', ended_at = now()
        WHERE id::text = v_old_session.session_id;

        INSERT INTO public.session_signaling (
            session_id, from_side, msg_type, payload
        ) VALUES (
            v_old_session.session_id::uuid,
            'system',
            'kick',
            jsonb_build_object(
                'reason', 'taken_over',
                'new_controller_id', p_controller_id,
                'new_controller_type', 'ai'
            )
        );
        v_kicked_count := v_kicked_count + 1;
    END LOOP;

    v_new_session_id := gen_random_uuid()::text;
    INSERT INTO public.webrtc_sessions (
        session_id, device_id, controller_id, controller_type, status
    ) VALUES (
        v_new_session_id, p_device_id, p_controller_id, 'ai', 'pending'
    );

    RETURN jsonb_build_object(
        'success', true,
        'session_id', v_new_session_id,
        'kicked_sessions', v_kicked_count,
        'controller_type', 'ai'
    );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public;

REVOKE ALL ON FUNCTION public.claim_ai_device_connection(text, text, uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION public.claim_ai_device_connection(text, text, uuid) FROM anon;
REVOKE ALL ON FUNCTION public.claim_ai_device_connection(text, text, uuid) FROM authenticated;
GRANT EXECUTE ON FUNCTION public.claim_ai_device_connection(text, text, uuid) TO service_role;

COMMENT ON FUNCTION public.claim_ai_device_connection IS
  'Creates an AI controller session after the trusted AI Edge Function has authenticated the actor.';

CREATE OR REPLACE FUNCTION public.enforce_trusted_ai_controller_type()
RETURNS trigger AS $$
BEGIN
    IF NEW.controller_type = 'ai'
       AND current_setting('request.jwt.claim.role', true) <> 'service_role' THEN
        RAISE EXCEPTION 'AI controller sessions require the trusted AI service path';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS enforce_trusted_ai_controller_type ON public.webrtc_sessions;
CREATE TRIGGER enforce_trusted_ai_controller_type
    BEFORE INSERT OR UPDATE OF controller_type ON public.webrtc_sessions
    FOR EACH ROW
    EXECUTE FUNCTION public.enforce_trusted_ai_controller_type();

-- Agents poll this table with their device API key. The later RLS hardening
-- removed the old broad anon policies but did not replace them here, which
-- leaves agents unable to receive controller offers after JWT expiry.
DROP POLICY IF EXISTS "Device reads own webrtc sessions via api_key" ON public.webrtc_sessions;
CREATE POLICY "Device reads own webrtc sessions via api_key"
ON public.webrtc_sessions FOR SELECT TO anon
USING (
    device_id IN (
        SELECT d.device_id
        FROM public.remote_devices d
        WHERE d.api_key = current_setting('request.headers', true)::json->>'x-device-key'
    )
);

DROP POLICY IF EXISTS "Device updates own webrtc sessions via api_key" ON public.webrtc_sessions;
CREATE POLICY "Device updates own webrtc sessions via api_key"
ON public.webrtc_sessions FOR UPDATE TO anon
USING (
    device_id IN (
        SELECT d.device_id
        FROM public.remote_devices d
        WHERE d.api_key = current_setting('request.headers', true)::json->>'x-device-key'
    )
)
WITH CHECK (
    device_id IN (
        SELECT d.device_id
        FROM public.remote_devices d
        WHERE d.api_key = current_setting('request.headers', true)::json->>'x-device-key'
    )
);
