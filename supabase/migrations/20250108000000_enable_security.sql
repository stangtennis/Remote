-- Re-enable Row Level Security with proper policies
-- This ensures only authenticated users can access their own devices

-- ===========================================
-- 1. Enable RLS on all tables
-- ===========================================

ALTER TABLE public.remote_devices ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.remote_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.session_signaling ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.audit_logs ENABLE ROW LEVEL SECURITY;

-- ===========================================
-- 2. REMOTE_DEVICES table policies
-- ===========================================

-- Users can only see devices they own
DROP POLICY IF EXISTS "Users can view own devices" ON public.remote_devices;
CREATE POLICY "Users can view own devices"
ON public.remote_devices
FOR SELECT
TO authenticated
USING (auth.uid() = owner_id);

-- Users can insert their own devices
DROP POLICY IF EXISTS "Users can insert own devices" ON public.remote_devices;
CREATE POLICY "Users can insert own devices"
ON public.remote_devices
FOR INSERT
TO authenticated
WITH CHECK (auth.uid() = owner_id);

-- Users can update their own devices
DROP POLICY IF EXISTS "Users can update own devices" ON public.remote_devices;
CREATE POLICY "Users can update own devices"
ON public.remote_devices
FOR UPDATE
TO authenticated
USING (auth.uid() = owner_id)
WITH CHECK (auth.uid() = owner_id);

-- Users can delete their own devices
DROP POLICY IF EXISTS "Users can delete own devices" ON public.remote_devices;
CREATE POLICY "Users can delete own devices"
ON public.remote_devices
FOR DELETE
TO authenticated
USING (auth.uid() = owner_id);

-- Agents can update their own device status (heartbeat)
DROP POLICY IF EXISTS "Agents can update own device" ON public.remote_devices;
CREATE POLICY "Agents can update own device"
ON public.remote_devices
FOR UPDATE
TO anon
USING (device_id = current_setting('request.jwt.claims', true)::json->>'device_id')
WITH CHECK (device_id = current_setting('request.jwt.claims', true)::json->>'device_id');

-- ===========================================
-- 3. REMOTE_SESSIONS table policies
-- ===========================================

-- Users can view sessions for their own devices
DROP POLICY IF EXISTS "Users can view own sessions" ON public.remote_sessions;
CREATE POLICY "Users can view own sessions"
ON public.remote_sessions
FOR SELECT
TO authenticated
USING (
  device_id IN (
    SELECT device_id FROM public.remote_devices WHERE owner_id = auth.uid()
  )
);

-- Users can create sessions for their own devices
DROP POLICY IF EXISTS "Users can create sessions" ON public.remote_sessions;
CREATE POLICY "Users can create sessions"
ON public.remote_sessions
FOR INSERT
TO authenticated
WITH CHECK (
  device_id IN (
    SELECT device_id FROM public.remote_devices WHERE owner_id = auth.uid()
  )
);

-- Users can update their own sessions
DROP POLICY IF EXISTS "Users can update own sessions" ON public.remote_sessions;
CREATE POLICY "Users can update own sessions"
ON public.remote_sessions
FOR UPDATE
TO authenticated
USING (
  device_id IN (
    SELECT device_id FROM public.remote_devices WHERE owner_id = auth.uid()
  )
);

-- Agents can view sessions for their device
DROP POLICY IF EXISTS "Agents can view device sessions" ON public.remote_sessions;
CREATE POLICY "Agents can view device sessions"
ON public.remote_sessions
FOR SELECT
TO anon
USING (device_id = current_setting('request.jwt.claims', true)::json->>'device_id');

-- Agents can update sessions for their device
DROP POLICY IF EXISTS "Agents can update device sessions" ON public.remote_sessions;
CREATE POLICY "Agents can update device sessions"
ON public.remote_sessions
FOR UPDATE
TO anon
USING (device_id = current_setting('request.jwt.claims', true)::json->>'device_id');

-- ===========================================
-- 4. SESSION_SIGNALING table policies
-- ===========================================

-- Users can view signaling for their sessions
DROP POLICY IF EXISTS "Users can view own signaling" ON public.session_signaling;
CREATE POLICY "Users can view own signaling"
ON public.session_signaling
FOR SELECT
TO authenticated
USING (
  session_id IN (
    SELECT rs.id FROM public.remote_sessions rs
    JOIN public.remote_devices d ON rs.device_id = d.device_id
    WHERE d.owner_id = auth.uid()
  )
);

-- Users can insert signaling for their sessions
DROP POLICY IF EXISTS "Users can insert signaling" ON public.session_signaling;
CREATE POLICY "Users can insert signaling"
ON public.session_signaling
FOR INSERT
TO authenticated
WITH CHECK (
  session_id IN (
    SELECT rs.id FROM public.remote_sessions rs
    JOIN public.remote_devices d ON rs.device_id = d.device_id
    WHERE d.owner_id = auth.uid()
  )
);

-- Agents can view signaling for their device sessions
DROP POLICY IF EXISTS "Agents can view signaling" ON public.session_signaling;
CREATE POLICY "Agents can view signaling"
ON public.session_signaling
FOR SELECT
TO anon
USING (
  session_id IN (
    SELECT id FROM public.remote_sessions
    WHERE device_id = current_setting('request.jwt.claims', true)::json->>'device_id'
  )
);

-- Agents can insert signaling for their device sessions
DROP POLICY IF EXISTS "Agents can insert signaling" ON public.session_signaling;
CREATE POLICY "Agents can insert signaling"
ON public.session_signaling
FOR INSERT
TO anon
WITH CHECK (
  session_id IN (
    SELECT id FROM public.remote_sessions
    WHERE device_id = current_setting('request.jwt.claims', true)::json->>'device_id'
  )
);

-- ===========================================
-- 5. AUDIT_LOGS table policies
-- ===========================================

-- Users can view their own audit logs
DROP POLICY IF EXISTS "Users can view own audit logs" ON public.audit_logs;
CREATE POLICY "Users can view own audit logs"
ON public.audit_logs
FOR SELECT
TO authenticated
USING (actor = auth.uid());

-- System can insert audit logs
DROP POLICY IF EXISTS "System can insert audit logs" ON public.audit_logs;
CREATE POLICY "System can insert audit logs"
ON public.audit_logs
FOR INSERT
TO authenticated, anon
WITH CHECK (true);

-- ===========================================
-- Comments
-- ===========================================

COMMENT ON POLICY "Users can view own devices" ON public.remote_devices 
IS 'Users can only see devices they own (linked to their auth.uid)';

COMMENT ON POLICY "Agents can view device sessions" ON public.remote_sessions
IS 'Agents can only view sessions for their specific device_id (from JWT claims)';

COMMENT ON TABLE public.remote_devices 
IS 'RLS enabled. Users can only access devices they own.';

COMMENT ON TABLE public.remote_sessions 
IS 'RLS enabled. Users can only access sessions for their devices.';

COMMENT ON TABLE public.session_signaling 
IS 'RLS enabled. Users and agents can only access signaling for authorized sessions.';

COMMENT ON TABLE public.audit_logs 
IS 'RLS enabled. Users can only view their own audit logs.';
