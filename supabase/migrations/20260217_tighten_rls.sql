-- Tighten RLS: Switch agent from anon to authenticated JWT tokens
-- Date: 2026-02-17 (updated 2026-02-18)
-- Description: Remove ALL broad anon/public policies, add owner-scoped authenticated policies.
--              Keep anon access only for Quick Support guest signaling.
--              Signaling policies check both webrtc_sessions AND remote_sessions.

-- ============================================================================
-- Helper: Create a reusable function for "user has access to device" check
-- This covers: device owner, assigned users, and admins.
-- ============================================================================

CREATE OR REPLACE FUNCTION public.user_has_device_access(p_device_id text)
RETURNS boolean AS $$
BEGIN
    RETURN EXISTS (
        SELECT 1 FROM remote_devices WHERE device_id = p_device_id AND owner_id = auth.uid()
    ) OR EXISTS (
        SELECT 1 FROM device_assignments
        WHERE device_id = p_device_id AND user_id = auth.uid() AND revoked_at IS NULL
    ) OR EXISTS (
        SELECT 1 FROM user_approvals
        WHERE user_id::uuid = auth.uid() AND role IN ('admin', 'super_admin')
    );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER STABLE;

-- Grant execute to authenticated users only
REVOKE EXECUTE ON FUNCTION public.user_has_device_access FROM anon;
REVOKE EXECUTE ON FUNCTION public.user_has_device_access FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.user_has_device_access TO authenticated;

-- ============================================================================
-- 1. REMOTE_DEVICES: Drop ALL broad policies, keep authenticated owner-scoped
-- ============================================================================

-- Drop broad public policies (CRITICAL: these allow unrestricted access)
DROP POLICY IF EXISTS "allow_select_devices" ON remote_devices;
DROP POLICY IF EXISTS "allow_update_devices" ON remote_devices;
DROP POLICY IF EXISTS "allow_delete_devices" ON remote_devices;
DROP POLICY IF EXISTS "allow_insert_devices" ON remote_devices;

-- Drop broad anon policies
DROP POLICY IF EXISTS "Agents can view devices" ON remote_devices;
DROP POLICY IF EXISTS "Agents can update devices" ON remote_devices;
DROP POLICY IF EXISTS "Agents can update device status" ON remote_devices;
DROP POLICY IF EXISTS "Agents can update own device" ON remote_devices;
DROP POLICY IF EXISTS "Devices can register themselves" ON remote_devices;
DROP POLICY IF EXISTS "Devices can update their status" ON remote_devices;

-- Keep existing authenticated policies (from earlier migrations):
-- "Users can view own devices" (SELECT, owner_id = auth.uid() AND is_user_approved)
-- "Users can update own devices" (UPDATE, owner_id = auth.uid() AND is_user_approved)
-- "Users can insert own devices" (INSERT, authenticated)
-- "Users can delete own devices" (DELETE, owner_id = auth.uid() AND is_user_approved)
-- "Users can view their assigned devices" (SELECT, device_assignments + admin check)
-- "Admins can manage all devices" (ALL, admin/super_admin)

-- ============================================================================
-- 2. WEBRTC_SESSIONS: Replace anon with authenticated policies
--    Access: device owner + assigned users + admins (via helper function)
-- ============================================================================

-- Drop any old policies
DROP POLICY IF EXISTS "Agents can view device sessions" ON webrtc_sessions;
DROP POLICY IF EXISTS "Agents can update device sessions" ON webrtc_sessions;
DROP POLICY IF EXISTS "Users can manage own sessions" ON webrtc_sessions;
DROP POLICY IF EXISTS "allow_select_webrtc" ON webrtc_sessions;
DROP POLICY IF EXISTS "allow_insert_webrtc" ON webrtc_sessions;
DROP POLICY IF EXISTS "allow_update_webrtc" ON webrtc_sessions;

-- Authenticated: SELECT sessions
DROP POLICY IF EXISTS "auth_select_webrtc_sessions" ON webrtc_sessions;
CREATE POLICY "auth_select_webrtc_sessions"
ON webrtc_sessions FOR SELECT
TO authenticated
USING (public.user_has_device_access(device_id));

-- Authenticated: INSERT sessions (controller creates sessions)
DROP POLICY IF EXISTS "auth_insert_webrtc_sessions" ON webrtc_sessions;
CREATE POLICY "auth_insert_webrtc_sessions"
ON webrtc_sessions FOR INSERT
TO authenticated
WITH CHECK (public.user_has_device_access(device_id));

-- Authenticated: UPDATE sessions (agent answers)
DROP POLICY IF EXISTS "auth_update_webrtc_sessions" ON webrtc_sessions;
CREATE POLICY "auth_update_webrtc_sessions"
ON webrtc_sessions FOR UPDATE
TO authenticated
USING (public.user_has_device_access(device_id))
WITH CHECK (public.user_has_device_access(device_id));

-- Authenticated: DELETE sessions
DROP POLICY IF EXISTS "auth_delete_webrtc_sessions" ON webrtc_sessions;
CREATE POLICY "auth_delete_webrtc_sessions"
ON webrtc_sessions FOR DELETE
TO authenticated
USING (public.user_has_device_access(device_id));

-- ============================================================================
-- 3. REMOTE_SESSIONS: Replace anon with authenticated policies
-- ============================================================================

-- Drop broad anon policies
DROP POLICY IF EXISTS "Agents can update sessions" ON remote_sessions;
DROP POLICY IF EXISTS "Agents can view sessions" ON remote_sessions;
DROP POLICY IF EXISTS "Agents can view device sessions" ON remote_sessions;
DROP POLICY IF EXISTS "Agents can update device sessions" ON remote_sessions;
DROP POLICY IF EXISTS "Users can create sessions" ON remote_sessions;

-- Authenticated: SELECT sessions (device access or session creator)
DROP POLICY IF EXISTS "auth_select_remote_sessions" ON remote_sessions;
CREATE POLICY "auth_select_remote_sessions"
ON remote_sessions FOR SELECT
TO authenticated
USING (
    public.user_has_device_access(device_id)
    OR created_by = auth.uid()
);

-- Authenticated: INSERT sessions (dashboard creates sessions)
DROP POLICY IF EXISTS "auth_insert_remote_sessions" ON remote_sessions;
CREATE POLICY "auth_insert_remote_sessions"
ON remote_sessions FOR INSERT
TO authenticated
WITH CHECK (created_by = auth.uid());

-- Authenticated: UPDATE sessions (agent updates status, dashboard updates)
DROP POLICY IF EXISTS "auth_update_remote_sessions" ON remote_sessions;
CREATE POLICY "auth_update_remote_sessions"
ON remote_sessions FOR UPDATE
TO authenticated
USING (
    public.user_has_device_access(device_id)
    OR created_by = auth.uid()
)
WITH CHECK (
    public.user_has_device_access(device_id)
    OR created_by = auth.uid()
);

-- ============================================================================
-- 4. SESSION_SIGNALING: Replace broad anon with scoped policies
--    Check BOTH webrtc_sessions (agent signaling) AND remote_sessions (dashboard)
--    Keep anon for Quick Support only.
--    Note: webrtc_sessions.session_id is TEXT, session_signaling.session_id is UUID
-- ============================================================================

-- Drop ALL old broad policies
DROP POLICY IF EXISTS "Agents can insert signaling" ON session_signaling;
DROP POLICY IF EXISTS "Agents can read signaling" ON session_signaling;
DROP POLICY IF EXISTS "Agents can view signaling" ON session_signaling;
DROP POLICY IF EXISTS "Agents can delete signaling" ON session_signaling;
DROP POLICY IF EXISTS "Users can read support signaling" ON session_signaling;
DROP POLICY IF EXISTS "Users can insert support signaling" ON session_signaling;
DROP POLICY IF EXISTS "Users can insert signaling" ON session_signaling;
DROP POLICY IF EXISTS "Users can view own signaling" ON session_signaling;
DROP POLICY IF EXISTS "Session participants can signal" ON session_signaling;

-- Authenticated: SELECT signaling
DROP POLICY IF EXISTS "auth_select_signaling" ON session_signaling;
CREATE POLICY "auth_select_signaling"
ON session_signaling FOR SELECT
TO authenticated
USING (
    -- WebRTC sessions (agent signaling) - cast text→uuid
    session_id IN (
        SELECT session_id::uuid FROM webrtc_sessions
        WHERE public.user_has_device_access(device_id)
    )
    -- Remote sessions (dashboard)
    OR session_id IN (
        SELECT id FROM remote_sessions
        WHERE public.user_has_device_access(device_id)
        OR created_by = auth.uid()
    )
    -- Support sessions created by this user
    OR session_id IN (
        SELECT id FROM support_sessions WHERE created_by = auth.uid()
    )
);

-- Authenticated: INSERT signaling messages
DROP POLICY IF EXISTS "auth_insert_signaling" ON session_signaling;
CREATE POLICY "auth_insert_signaling"
ON session_signaling FOR INSERT
TO authenticated
WITH CHECK (
    session_id IN (
        SELECT session_id::uuid FROM webrtc_sessions
        WHERE public.user_has_device_access(device_id)
    )
    OR session_id IN (
        SELECT id FROM remote_sessions
        WHERE public.user_has_device_access(device_id)
        OR created_by = auth.uid()
    )
    OR session_id IN (
        SELECT id FROM support_sessions WHERE created_by = auth.uid()
    )
);

-- Authenticated: DELETE signaling messages (cleanup)
DROP POLICY IF EXISTS "auth_delete_signaling" ON session_signaling;
CREATE POLICY "auth_delete_signaling"
ON session_signaling FOR DELETE
TO authenticated
USING (
    session_id IN (
        SELECT session_id::uuid FROM webrtc_sessions
        WHERE public.user_has_device_access(device_id)
    )
    OR session_id IN (
        SELECT id FROM remote_sessions
        WHERE public.user_has_device_access(device_id)
        OR created_by = auth.uid()
    )
    OR session_id IN (
        SELECT id FROM support_sessions WHERE created_by = auth.uid()
    )
);

-- ============================================================================
-- 5. QUICK SUPPORT: Anon policies for guest users
--    Guests need to read support_sessions (by token) and exchange signaling
-- ============================================================================

-- Anon: SELECT support_sessions by token (guest lookup)
DROP POLICY IF EXISTS "anon_select_support_sessions" ON support_sessions;
CREATE POLICY "anon_select_support_sessions"
ON support_sessions FOR SELECT
TO anon
USING (status IN ('pending', 'active'));

-- Anon: UPDATE support_sessions status (guest marks as active/ended)
DROP POLICY IF EXISTS "anon_update_support_sessions" ON support_sessions;
CREATE POLICY "anon_update_support_sessions"
ON support_sessions FOR UPDATE
TO anon
USING (status IN ('pending', 'active'))
WITH CHECK (status IN ('active', 'ended'));

-- Anon: SELECT signaling for support sessions only
DROP POLICY IF EXISTS "anon_select_support_signaling" ON session_signaling;
CREATE POLICY "anon_select_support_signaling"
ON session_signaling FOR SELECT
TO anon
USING (
    session_id IN (SELECT id FROM support_sessions WHERE status IN ('pending', 'active'))
);

-- Anon: INSERT signaling for support sessions only
DROP POLICY IF EXISTS "anon_insert_support_signaling" ON session_signaling;
CREATE POLICY "anon_insert_support_signaling"
ON session_signaling FOR INSERT
TO anon
WITH CHECK (
    session_id IN (SELECT id FROM support_sessions WHERE status IN ('pending', 'active'))
);

-- ============================================================================
-- 6. Comments
-- ============================================================================

COMMENT ON TABLE public.remote_devices IS
    'RLS: Authenticated owner-scoped + admin policies. No anon access.';

COMMENT ON TABLE public.webrtc_sessions IS
    'RLS: Authenticated via user_has_device_access(). No anon access.';

COMMENT ON TABLE public.remote_sessions IS
    'RLS: Authenticated via user_has_device_access(). No anon access.';

COMMENT ON TABLE public.session_signaling IS
    'RLS: Authenticated for webrtc+remote+support sessions. Anon for Quick Support only.';
