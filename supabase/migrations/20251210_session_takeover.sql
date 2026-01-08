-- Session Takeover System
-- Ensures only ONE active connection per device at a time
-- New connections automatically kick old ones
-- Uses webrtc_sessions table (used by controller)

-- 1. Add controller_id to track who is controlling
ALTER TABLE public.webrtc_sessions 
ADD COLUMN IF NOT EXISTS controller_id text,
ADD COLUMN IF NOT EXISTS controller_type text DEFAULT 'unknown',
ADD COLUMN IF NOT EXISTS kicked_at timestamptz,
ADD COLUMN IF NOT EXISTS kicked_by text;

-- 2. Create active_connections view for easy querying
CREATE OR REPLACE VIEW public.active_connections AS
SELECT 
    session_id,
    device_id,
    controller_id,
    controller_type,
    status,
    created_at
FROM public.webrtc_sessions
WHERE status IN ('pending', 'offer_sent', 'answered', 'connected')
  AND kicked_at IS NULL;

-- 3. Function to claim/takeover a device connection
-- Returns the new session_id and kicks any existing connections
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

-- 4. Function for agent to check if it should disconnect (was kicked)
CREATE OR REPLACE FUNCTION public.check_session_kicked(p_session_id uuid)
RETURNS jsonb AS $$
DECLARE
    v_session record;
BEGIN
    SELECT kicked_at, kicked_by, status 
    INTO v_session
    FROM public.webrtc_sessions 
    WHERE session_id = p_session_id::text;
    
    IF NOT FOUND THEN
        RETURN jsonb_build_object('kicked', false, 'reason', 'session_not_found');
    END IF;
    
    IF v_session.kicked_at IS NOT NULL THEN
        RETURN jsonb_build_object(
            'kicked', true,
            'kicked_at', v_session.kicked_at,
            'kicked_by', v_session.kicked_by,
            'status', v_session.status
        );
    END IF;
    
    RETURN jsonb_build_object('kicked', false, 'status', v_session.status);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 5. Add 'system' as valid from_side for kick messages
ALTER TABLE public.session_signaling 
DROP CONSTRAINT IF EXISTS session_signaling_from_side_check;

ALTER TABLE public.session_signaling 
ADD CONSTRAINT session_signaling_from_side_check 
CHECK (from_side IN ('dashboard', 'agent', 'controller', 'system'));

-- 6. Add 'kick' as valid msg_type
ALTER TABLE public.session_signaling 
DROP CONSTRAINT IF EXISTS session_signaling_msg_type_check;

ALTER TABLE public.session_signaling 
ADD CONSTRAINT session_signaling_msg_type_check 
CHECK (msg_type IN ('offer', 'answer', 'ice', 'kick', 'bye'));

-- 7. Index for fast kicked session lookups
CREATE INDEX IF NOT EXISTS idx_webrtc_sessions_device_active 
ON public.webrtc_sessions(device_id, status) 
WHERE status IN ('pending', 'offer_sent', 'answered', 'connected') AND kicked_at IS NULL;

-- 8. Grant execute permissions
GRANT EXECUTE ON FUNCTION public.claim_device_connection TO anon, authenticated;
GRANT EXECUTE ON FUNCTION public.check_session_kicked TO anon, authenticated;

-- 9. Comment for documentation
COMMENT ON FUNCTION public.claim_device_connection IS 
'Claims exclusive control of a device. Kicks any existing connections and creates a new session.';

COMMENT ON FUNCTION public.check_session_kicked IS 
'Check if a session has been kicked/taken over by another controller.';
