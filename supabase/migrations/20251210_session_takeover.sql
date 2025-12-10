-- Session Takeover System
-- Ensures only ONE active connection per device at a time
-- New connections automatically kick old ones

-- 1. Add controller_id to track who is controlling
ALTER TABLE public.remote_sessions 
ADD COLUMN IF NOT EXISTS controller_id text,
ADD COLUMN IF NOT EXISTS controller_type text CHECK (controller_type IN ('dashboard', 'controller', 'unknown')) DEFAULT 'unknown',
ADD COLUMN IF NOT EXISTS kicked_at timestamptz,
ADD COLUMN IF NOT EXISTS kicked_by text;

-- 2. Create active_connections view for easy querying
CREATE OR REPLACE VIEW public.active_connections AS
SELECT 
    id as session_id,
    device_id,
    controller_id,
    controller_type,
    status,
    created_at,
    expires_at
FROM public.remote_sessions
WHERE status IN ('pending', 'active')
  AND (expires_at IS NULL OR expires_at > now())
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
    v_new_session_id uuid;
    v_kicked_count int := 0;
BEGIN
    -- Find and kick all existing active sessions for this device
    FOR v_old_sessions IN 
        SELECT id, controller_id, controller_type 
        FROM public.remote_sessions 
        WHERE device_id = p_device_id 
          AND status IN ('pending', 'active')
          AND kicked_at IS NULL
    LOOP
        -- Mark old session as kicked
        UPDATE public.remote_sessions 
        SET 
            kicked_at = now(),
            kicked_by = p_controller_id,
            status = 'ended',
            ended_at = now()
        WHERE id = v_old_sessions.id;
        
        -- Insert kick signal for the old controller to receive
        INSERT INTO public.session_signaling (session_id, from_side, msg_type, payload)
        VALUES (
            v_old_sessions.id,
            'system',
            'kick',
            jsonb_build_object(
                'reason', 'takeover',
                'new_controller_id', p_controller_id,
                'new_controller_type', p_controller_type,
                'kicked_at', now()
            )
        );
        
        v_kicked_count := v_kicked_count + 1;
    END LOOP;

    -- Create new session
    INSERT INTO public.remote_sessions (
        device_id,
        controller_id,
        controller_type,
        status,
        pin,
        expires_at
    ) VALUES (
        p_device_id,
        p_controller_id,
        p_controller_type,
        'pending',
        lpad(floor(random() * 10000)::text, 4, '0'),
        now() + interval '5 minutes'
    )
    RETURNING id INTO v_new_session_id;

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
    FROM public.remote_sessions 
    WHERE id = p_session_id;
    
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
CREATE INDEX IF NOT EXISTS idx_sessions_device_active 
ON public.remote_sessions(device_id, status) 
WHERE status IN ('pending', 'active') AND kicked_at IS NULL;

-- 8. Grant execute permissions
GRANT EXECUTE ON FUNCTION public.claim_device_connection TO anon, authenticated;
GRANT EXECUTE ON FUNCTION public.check_session_kicked TO anon, authenticated;

-- 9. Comment for documentation
COMMENT ON FUNCTION public.claim_device_connection IS 
'Claims exclusive control of a device. Kicks any existing connections and creates a new session.';

COMMENT ON FUNCTION public.check_session_kicked IS 
'Check if a session has been kicked/taken over by another controller.';
