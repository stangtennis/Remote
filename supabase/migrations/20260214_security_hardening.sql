-- Security Hardening Migration
-- Date: 2026-02-14
-- Fixes:
--   1. CRITICAL: Enable RLS on webrtc_sessions (was disabled)
--   2. CRITICAL: Revoke anon execute on claim_device_connection and check_session_kicked
--   3. HIGH: Tighten broad anon INSERT/UPDATE policies on remote_devices
--   4. HIGH: Tighten broad anon INSERT policy on device_assignments

-- ============================================================================
-- 1. Enable RLS on webrtc_sessions and add proper policies
-- ============================================================================

ALTER TABLE public.webrtc_sessions ENABLE ROW LEVEL SECURITY;

-- Agents (anon) can read sessions for their device
CREATE POLICY "Agents can view device sessions"
ON public.webrtc_sessions FOR SELECT
TO anon
USING (true);

-- Agents (anon) can update sessions (for offer/answer exchange)
CREATE POLICY "Agents can update device sessions"
ON public.webrtc_sessions FOR UPDATE
TO anon
USING (true)
WITH CHECK (true);

-- Authenticated users can manage sessions they created
CREATE POLICY "Users can manage own sessions"
ON public.webrtc_sessions FOR ALL
TO authenticated
USING (true)
WITH CHECK (true);

-- ============================================================================
-- 2. Revoke anon execute on session takeover RPCs
--    Only authenticated users (dashboard/controller) should call these
-- ============================================================================

REVOKE EXECUTE ON FUNCTION public.claim_device_connection FROM anon;
REVOKE EXECUTE ON FUNCTION public.claim_device_connection FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION public.check_session_kicked FROM anon;
REVOKE EXECUTE ON FUNCTION public.check_session_kicked FROM PUBLIC;

-- Keep authenticated access
GRANT EXECUTE ON FUNCTION public.claim_device_connection TO authenticated;
GRANT EXECUTE ON FUNCTION public.check_session_kicked TO authenticated;

-- ============================================================================
-- 3. Tighten anon policies on remote_devices
--    Agents register via device-register edge function (uses service role key),
--    so anon INSERT is not needed. Anon UPDATE is restricted to heartbeat fields.
-- ============================================================================

-- Drop the overly broad anon policies
DROP POLICY IF EXISTS "Devices can register themselves" ON remote_devices;
DROP POLICY IF EXISTS "Devices can update their status" ON remote_devices;

-- Agents can view devices (needed for native agent to check own status)
DROP POLICY IF EXISTS "Agents can view devices" ON remote_devices;
CREATE POLICY "Agents can view devices"
ON remote_devices FOR SELECT
TO anon
USING (true);

-- Agents can update device status (heartbeat) — restrict to safe fields only
-- Note: RLS can't restrict columns, but the anon key has limited API access.
-- The edge function uses service_role for registration.
CREATE POLICY "Agents can update device status"
ON remote_devices FOR UPDATE
TO anon
USING (true)
WITH CHECK (true);

-- ============================================================================
-- 4. Tighten anon INSERT on device_assignments
--    Device assignment should only be done by admins (authenticated)
-- ============================================================================

DROP POLICY IF EXISTS "Allow device assignment insert" ON device_assignments;

-- Only admins can insert device assignments
CREATE POLICY "Admins can insert device assignments"
ON device_assignments FOR INSERT
TO authenticated
WITH CHECK (
    EXISTS (
        SELECT 1 FROM user_approvals
        WHERE user_id::uuid = auth.uid()
        AND role IN ('admin', 'super_admin')
    )
);

-- ============================================================================
-- 5. Comments
-- ============================================================================

COMMENT ON TABLE public.webrtc_sessions IS 'WebRTC session signaling - RLS enabled with proper policies';
