-- v3.1.0: Security hardening from comprehensive audit
--
-- Fixes:
-- 1. audit_logs has a "System can insert audit logs" policy with
--    WITH CHECK (true) — any holder of the Supabase anon key can poison
--    audit logs. Drop it; service_role bypasses RLS anyway and
--    legitimate writers go through the api_key or JWT paths.
--
-- 2. The "Authenticated users write audit logs" policy lets any
--    approved user write audit rows for any device_id (or NULL).
--    Tighten to require device ownership (or NULL only for system
--    events).
--
-- 3. Add api_key revocation: a revoked_at column on remote_devices and
--    a check function that all api_key-gated policies will use. A
--    leaked .credentials file can now be invalidated by setting
--    revoked_at = now().

BEGIN;

-- ─── audit_logs hardening ─────────────────────────────────────────────
DROP POLICY IF EXISTS "System can insert audit logs" ON public.audit_logs;
DROP POLICY IF EXISTS "Authenticated users write audit logs" ON public.audit_logs;

-- Authenticated users can only write rows they "own": either tied to a
-- device they own, or actor matches their auth.uid(), or no device_id
-- and the row is tagged as a global event by the user themselves.
CREATE POLICY "Authenticated users write own audit rows" ON public.audit_logs
  FOR INSERT WITH CHECK (
    auth.uid() IS NOT NULL
    AND (actor IS NULL OR actor = auth.uid())
    AND (
      device_id IS NULL
      OR EXISTS (
        SELECT 1 FROM public.remote_devices d
        WHERE d.device_id = audit_logs.device_id
          AND d.owner_id = auth.uid()
      )
    )
  );

-- ─── api_key revocation ───────────────────────────────────────────────
ALTER TABLE public.remote_devices
  ADD COLUMN IF NOT EXISTS api_key_revoked_at timestamptz;

COMMENT ON COLUMN public.remote_devices.api_key_revoked_at IS
  'When non-null, the api_key for this device is invalid. Heartbeat / signaling / audit policies all check this.';

-- Helper: a stable place to express "this header matches an active key"
CREATE OR REPLACE FUNCTION public.device_id_for_active_api_key()
RETURNS text
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public
AS $$
  SELECT device_id FROM public.remote_devices
  WHERE api_key = current_setting('request.headers', true)::json->>'x-device-key'
    AND api_key_revoked_at IS NULL
  LIMIT 1
$$;

REVOKE ALL ON FUNCTION public.device_id_for_active_api_key() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.device_id_for_active_api_key() TO anon, authenticated;

-- ─── Re-create api_key policies to use the revocation-aware helper ────

-- remote_devices
DROP POLICY IF EXISTS "Device reads own row via api_key" ON public.remote_devices;
DROP POLICY IF EXISTS "Device updates own row via api_key" ON public.remote_devices;

CREATE POLICY "Device reads own row via api_key" ON public.remote_devices
  FOR SELECT USING (device_id = public.device_id_for_active_api_key());

CREATE POLICY "Device updates own row via api_key" ON public.remote_devices
  FOR UPDATE
  USING (device_id = public.device_id_for_active_api_key())
  WITH CHECK (device_id = public.device_id_for_active_api_key());

-- audit_logs
DROP POLICY IF EXISTS "Device writes own audit via api_key" ON public.audit_logs;

CREATE POLICY "Device writes own audit via api_key" ON public.audit_logs
  FOR INSERT WITH CHECK (
    device_id IS NOT NULL
    AND device_id = public.device_id_for_active_api_key()
  );

-- session_signaling
DROP POLICY IF EXISTS "Device reads signaling via api_key" ON public.session_signaling;
DROP POLICY IF EXISTS "Device writes signaling via api_key" ON public.session_signaling;

CREATE POLICY "Device reads signaling via api_key" ON public.session_signaling
  FOR SELECT USING (
    public.device_id_for_active_api_key() IS NOT NULL
    AND (
      EXISTS (
        SELECT 1 FROM public.webrtc_sessions w
        WHERE (w.session_id)::uuid = session_signaling.session_id
          AND w.device_id = public.device_id_for_active_api_key()
      )
      OR EXISTS (
        SELECT 1 FROM public.remote_sessions r
        WHERE r.id = session_signaling.session_id
          AND r.device_id = public.device_id_for_active_api_key()
      )
    )
  );

-- INSERT: device may only write its own answers/ICE — refuse if from_side
-- claims to be 'dashboard' or 'controller'.
CREATE POLICY "Device writes signaling via api_key" ON public.session_signaling
  FOR INSERT WITH CHECK (
    public.device_id_for_active_api_key() IS NOT NULL
    AND (from_side IS NULL OR from_side = 'agent')
    AND (
      EXISTS (
        SELECT 1 FROM public.webrtc_sessions w
        WHERE (w.session_id)::uuid = session_signaling.session_id
          AND w.device_id = public.device_id_for_active_api_key()
      )
      OR EXISTS (
        SELECT 1 FROM public.remote_sessions r
        WHERE r.id = session_signaling.session_id
          AND r.device_id = public.device_id_for_active_api_key()
      )
    )
  );

-- remote_sessions
DROP POLICY IF EXISTS "Device reads own remote_sessions via api_key" ON public.remote_sessions;

CREATE POLICY "Device reads own remote_sessions via api_key" ON public.remote_sessions
  FOR SELECT USING (device_id = public.device_id_for_active_api_key());

COMMIT;
