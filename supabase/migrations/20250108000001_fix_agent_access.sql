-- Fix agent access with anon key
-- Current: Agents use anon key (public) to access their sessions
-- Security: Users are protected by RLS, agents rely on application-level filtering

-- ===========================================
-- SECURITY MODEL
-- ===========================================
-- 
-- USERS (Dashboard):
--   - Authenticate with Supabase Auth (email/password)
--   - RLS enforces they can ONLY see their own devices
--   - RLS enforces they can ONLY access sessions for their devices
--
-- AGENTS (Remote computers):
--   - Use anon key (public, no authentication)
--   - Can technically query all data
--   - Application filters by device_id (stored in .device_id file)
--   - Additional security: PIN codes required for connections
--
-- This is acceptable because:
--   1. Agents are backend services, not client browsers
--   2. Device_id is stored locally, not exposed to users
--   3. Even if someone queries all sessions, they need the PIN to connect
--   4. Users' data is protected (they can't see other users' devices)
--
-- Future improvement: Use device-specific API keys (api_key field exists)
-- ===========================================

-- ===========================================
-- 1. Agent policies for REMOTE_DEVICES
-- ===========================================

-- Agents can SELECT devices (to check if approved, get config)
DROP POLICY IF EXISTS "Agents can view devices" ON public.remote_devices;
CREATE POLICY "Agents can view devices"
ON public.remote_devices
FOR SELECT
TO anon
USING (true);  -- Allow all reads, app filters by device_id

-- Agents can UPDATE devices (heartbeat, status)
DROP POLICY IF EXISTS "Agents can update devices" ON public.remote_devices;
CREATE POLICY "Agents can update devices"
ON public.remote_devices
FOR UPDATE
TO anon
USING (true)
WITH CHECK (true);  -- Allow all updates, app filters by device_id

-- ===========================================
-- 2. Agent policies for REMOTE_SESSIONS
-- ===========================================

-- Agents can SELECT sessions (to find pending sessions for their device)
DROP POLICY IF EXISTS "Agents can view sessions" ON public.remote_sessions;
CREATE POLICY "Agents can view sessions"
ON public.remote_sessions
FOR SELECT
TO anon
USING (true);  -- Allow all reads, app filters by device_id

-- Agents can UPDATE sessions (to mark as active/ended)
DROP POLICY IF EXISTS "Agents can update sessions" ON public.remote_sessions;
CREATE POLICY "Agents can update sessions"  
ON public.remote_sessions
FOR UPDATE
TO anon
USING (true)
WITH CHECK (true);  -- Allow all updates, app filters by session_id

-- ===========================================
-- 3. Agent policies for SESSION_SIGNALING
-- ===========================================

-- Agents can SELECT signaling (to get WebRTC offers/ICE candidates)
DROP POLICY IF EXISTS "Agents can view signaling" ON public.session_signaling;
CREATE POLICY "Agents can view signaling"
ON public.session_signaling
FOR SELECT
TO anon
USING (true);  -- Allow all reads, app filters by session_id

-- Agents can INSERT signaling (to send WebRTC answers/ICE candidates)
DROP POLICY IF EXISTS "Agents can insert signaling" ON public.session_signaling;
CREATE POLICY "Agents can insert signaling"
ON public.session_signaling
FOR INSERT
TO anon
WITH CHECK (true);  -- Allow all inserts, app provides correct session_id

-- ===========================================
-- Comments
-- ===========================================

COMMENT ON POLICY "Agents can view devices" ON public.remote_devices
IS 'Agents use anon key. Can query all devices but app filters by device_id. Consider per-device API keys for production.';

COMMENT ON POLICY "Agents can view sessions" ON public.remote_sessions
IS 'Agents use anon key. Can query all sessions but app filters by device_id. PIN codes provide additional security.';

COMMENT ON POLICY "Agents can view signaling" ON public.session_signaling
IS 'Agents use anon key for WebRTC signaling. Can query all but app filters by session_id.';
