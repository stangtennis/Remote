-- v3.0.4: Make agent → server auth survive long offline periods.
--
-- Previously the agent used the user's JWT (1h access token, refresh token
-- rotated by GoTrue every refresh). If the agent crashed or lost network
-- between receiving a new refresh token and saving it to disk, the saved
-- token was permanently invalid → agent could never come back online without
-- a physical visit. WIN-TEST hit this after a 5-day offline period.
--
-- Fix: every device gets a stable per-device api_key. Agent uses
-- `x-device-key` header for heartbeat/audit/pending_command — no expiry,
-- no rotation. JWT is still used for the initial registration (to set
-- owner_id) and for interactive controller sessions.

-- 1. Backfill api_key for existing devices missing one
UPDATE public.remote_devices
SET api_key = encode(gen_random_bytes(32), 'hex')
WHERE api_key IS NULL OR api_key = '';

-- 2. Default for new rows
ALTER TABLE public.remote_devices
ALTER COLUMN api_key SET DEFAULT encode(gen_random_bytes(32), 'hex');

-- 3. Enforce NOT NULL going forward
ALTER TABLE public.remote_devices
ALTER COLUMN api_key SET NOT NULL;

-- 4. SELECT policy via api_key — required so PATCH ... Prefer:
--    return=representation can return the row (PostgREST evaluates SELECT
--    on the returned row in addition to UPDATE on the original).
CREATE POLICY "Device reads own row via api_key" ON public.remote_devices
  FOR SELECT USING (
    api_key = current_setting('request.headers', true)::json->>'x-device-key'
  );

-- 5. UPDATE policy via api_key. The original "Devices can update own status"
--    policy from the 2025-01-01 schema was dropped by a later migration; we
--    re-add it here so heartbeat can keep updating last_seen / is_online
--    without a valid user JWT.
CREATE POLICY "Device updates own row via api_key" ON public.remote_devices
  FOR UPDATE
  USING (api_key = current_setting('request.headers', true)::json->>'x-device-key')
  WITH CHECK (api_key = current_setting('request.headers', true)::json->>'x-device-key');

-- 6. Tighten audit_logs INSERT: replace the permissive `WITH CHECK (true)`
--    policy with two scoped policies — devices via api_key, users via JWT.
--    Service role still bypasses RLS for trusted writes.
DROP POLICY IF EXISTS "System can write audit logs" ON public.audit_logs;

CREATE POLICY "Device writes own audit via api_key" ON public.audit_logs
  FOR INSERT WITH CHECK (
    device_id IS NOT NULL
    AND EXISTS (
      SELECT 1 FROM public.remote_devices d
      WHERE d.device_id = audit_logs.device_id
        AND d.api_key = current_setting('request.headers', true)::json->>'x-device-key'
    )
  );

CREATE POLICY "Authenticated users write audit logs" ON public.audit_logs
  FOR INSERT WITH CHECK (auth.uid() IS NOT NULL);
