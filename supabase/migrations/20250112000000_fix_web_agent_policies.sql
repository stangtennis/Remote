-- Fix RLS policies for web agent access
-- Web agents use anon key and need to update sessions and insert signaling

-- ===========================================
-- REMOTE_SESSIONS: Allow anon to update sessions
-- ===========================================

-- Agents can update session status (accept/reject)
DROP POLICY IF EXISTS "Agents can update sessions" ON public.remote_sessions;
CREATE POLICY "Agents can update sessions"
ON public.remote_sessions
FOR UPDATE
TO anon
USING (true)
WITH CHECK (true);

COMMENT ON POLICY "Agents can update sessions" ON public.remote_sessions
IS 'Agents use anon key. Can update session status (started_at, status, etc.)';

-- ===========================================
-- SESSION_SIGNALING: Allow anon to insert signaling
-- ===========================================

-- Agents can insert signaling messages (offers, ICE candidates)
DROP POLICY IF EXISTS "Agents can insert signaling" ON public.session_signaling;
CREATE POLICY "Agents can insert signaling"
ON public.session_signaling
FOR INSERT
TO anon
WITH CHECK (true);

-- Agents can read signaling messages
DROP POLICY IF EXISTS "Agents can read signaling" ON public.session_signaling;
CREATE POLICY "Agents can read signaling"
ON public.session_signaling
FOR SELECT
TO anon
USING (true);

COMMENT ON POLICY "Agents can insert signaling" ON public.session_signaling
IS 'Agents use anon key. Need to send WebRTC offers and ICE candidates.';

COMMENT ON POLICY "Agents can read signaling" ON public.session_signaling
IS 'Agents use anon key. Need to receive WebRTC answers and ICE candidates.';
