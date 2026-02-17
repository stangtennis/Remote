-- Tighten RLS: Switch agent from anon to authenticated JWT tokens
-- Date: 2026-02-17
-- Description: Remove broad anon policies, add owner-scoped authenticated policies.
--              Keep anon access only for Quick Support guest signaling.

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
-- 1. REMOTE_DEVICES: Replace anon with authenticated owner-scoped policies
-- ============================================================================

-- Drop broad anon policies
DROP POLICY IF EXISTS "Agents can view devices" ON remote_devices;
DROP POLICY IF EXISTS "Agents can update device status" ON remote_devices;
DROP POLICY IF EXISTS "Devices can register themselves" ON remote_devices;
DROP POLICY IF EXISTS "Devices can update their status" ON remote_devices;

-- Authenticated users can view their own devices
DROP POLICY IF EXISTS "auth_select_own_devices" ON remote_devices;
CREATE POLICY "auth_select_own_devices"
ON remote_devices FOR SELECT
TO authenticated
USING (owner_id = auth.uid());

-- Authenticated users can update their own devices (heartbeat, status)
DROP POLICY IF EXISTS "auth_update_own_devices" ON remote_devices;
CREATE POLICY "auth_update_own_devices"
ON remote_devices FOR UPDATE
TO authenticated
USING (owner_id = auth.uid())
WITH CHECK (owner_id = auth.uid());

-- Authenticated users can register (insert) devices they own
DROP POLICY IF EXISTS "auth_insert_own_devices" ON remote_devices;
CREATE POLICY "auth_insert_own_devices"
ON remote_devices FOR INSERT
TO authenticated
WITH CHECK (owner_id = auth.uid());

-- Keep existing admin/assignment policies (from 20260108_reenable_rls_devices.sql):
-- "Users can view their assigned devices" and "Admins can manage all devices"
-- These are already correct and cover controller access.

-- ============================================================================
-- 2. WEBRTC_SESSIONS: Replace anon with authenticated policies
--    Access: device owner + assigned users + admins (via helper function)
-- ============================================================================

-- Drop broad anon/authenticated policies
DROP POLICY IF EXISTS "Agents can view device sessions" ON webrtc_sessions;
DROP POLICY IF EXISTS "Agents can update device sessions" ON webrtc_sessions;
DROP POLICY IF EXISTS "Users can manage own sessions" ON webrtc_sessions;

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
--    Keep anon for Quick Support only.
-- ============================================================================

-- Drop broad anon policies and old support policies (superseded by new ones below)
DROP POLICY IF EXISTS "Agents can insert signaling" ON session_signaling;
DROP POLICY IF EXISTS "Agents can read signaling" ON session_signaling;
DROP POLICY IF EXISTS "Agents can delete signaling" ON session_signaling;
DROP POLICY IF EXISTS "Users can read support signaling" ON session_signaling;
DROP POLICY IF EXISTS "Users can insert support signaling" ON session_signaling;

-- Authenticated: SELECT signaling
-- (covers agent polling, dashboard reading, and support sessions)
DROP POLICY IF EXISTS "auth_select_signaling" ON session_signaling;
CREATE POLICY "auth_select_signaling"
ON session_signaling FOR SELECT
TO authenticated
USING (
    -- Session belongs to a device the user has access to (via remote_sessions)
    session_id IN (
        SELECT id FROM remote_sessions
        WHERE public.user_has_device_access(device_id)
        OR created_by = auth.uid()
    )
    -- Or it's a support session created by this user
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
    'RLS: Authenticated for regular sessions. Anon for Quick Support only.';
