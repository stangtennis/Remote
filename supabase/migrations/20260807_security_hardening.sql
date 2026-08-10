-- ============================================================================
-- Security hardening batch — 2026-08-07
-- Addresses gap-analysis findings: IDOR, support-signaling is_public, storage
-- SELECT, device_tags scoping, webrtc_sessions cleanup, token uniqueness,
-- and search_path hardening on SECURITY DEFINER helpers.
-- ============================================================================

-- 1. IDOR: get_user_devices must only return devices for the caller (or admin).
--    Previously any authenticated caller could pass any p_user_id and enumerate
--    another user's devices.
DROP FUNCTION IF EXISTS get_user_devices(UUID);

CREATE OR REPLACE FUNCTION get_user_devices(p_user_id UUID)
RETURNS SETOF remote_devices
SECURITY DEFINER
SET search_path = public
AS $$
BEGIN
    IF p_user_id IS DISTINCT FROM auth.uid()
       AND NOT is_admin() THEN
        RAISE EXCEPTION 'Not allowed to view another user''s devices';
    END IF;

    RETURN QUERY
    SELECT DISTINCT ON (d.device_id) d.*
    FROM remote_devices d
    LEFT JOIN device_assignments da ON d.device_id = da.device_id
    WHERE (
        (da.user_id = p_user_id AND da.revoked_at IS NULL)
        OR d.owner_id = p_user_id
    )
    ORDER BY d.device_id, d.last_seen DESC NULLS LAST;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION get_user_devices IS 'Returns devices owned by or assigned to a user (full row). Restricted to the caller or admins.';

-- 2. Anon support signaling must respect is_public. Previously anon could
--    read/inject SDP+ICE into private (PIN/token) support sessions because the
--    support_sessions hardening (20260716) did not touch this table.
DROP POLICY IF EXISTS "anon_select_support_signaling" ON session_signaling;
CREATE POLICY "anon_select_support_signaling"
ON session_signaling FOR SELECT
TO anon
USING (
    session_id IN (
        SELECT id FROM support_sessions
        WHERE status IN ('pending', 'active') AND is_public = true
    )
);

DROP POLICY IF EXISTS "anon_insert_support_signaling" ON session_signaling;
CREATE POLICY "anon_insert_support_signaling"
ON session_signaling FOR INSERT
TO anon
WITH CHECK (
    session_id IN (
        SELECT id FROM support_sessions
        WHERE status IN ('pending', 'active') AND is_public = true
    )
);

-- 3. Storage file-transfers SELECT: match the sessionId/ path convention used
--    by INSERT/DELETE and the file-transfer edge function. The old policy
--    expected userId/ as the first folder, so uploaded files could never be
--    read back.
DROP POLICY IF EXISTS "Users can access own session files" ON storage.objects;
CREATE POLICY "Users can access own session files"
ON storage.objects FOR SELECT
USING (
    bucket_id = 'file-transfers'
    AND EXISTS (
        SELECT 1 FROM public.remote_sessions
        WHERE id::text = (storage.foldername(name))[1]
        AND created_by = auth.uid()
    )
);

-- 4. device_tags: scope read/write via user_has_device_access. Previously any
--    authenticated user could read or add tags to ANY device (cross-tenant).
--    Drop both the 20260308-named policies AND any legacy device_tags_* policies
--    that may exist from intermediate migrations.
DROP POLICY IF EXISTS "Authenticated users can read tags" ON device_tags;
DROP POLICY IF EXISTS "Users can insert tags" ON device_tags;
DROP POLICY IF EXISTS device_tags_select ON device_tags;
DROP POLICY IF EXISTS device_tags_insert ON device_tags;
DROP POLICY IF EXISTS device_tags_delete ON device_tags;
CREATE POLICY "Authenticated users can read tags"
ON device_tags FOR SELECT
TO authenticated
USING (public.user_has_device_access(device_id));

CREATE POLICY "Users can insert tags"
ON device_tags FOR INSERT
TO authenticated
WITH CHECK (public.user_has_device_access(device_id));

CREATE POLICY "Users can delete own tags"
ON device_tags FOR DELETE
TO authenticated
USING (created_by = auth.uid() OR public.is_admin());

-- 5. webrtc_sessions grows unbounded: the cleanup function existed but was
--    never scheduled. Wire it to pg_cron (guarded — no-op if pg_cron absent).
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_cron') THEN
        PERFORM cron.unschedule('webrtc-sessions-cleanup');
        PERFORM cron.schedule(
            'webrtc-sessions-cleanup',
            '*/10 * * * *',
            'SELECT public.cleanup_old_webrtc_sessions()'
        );
    END IF;
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

-- 6. remote_sessions.token uniqueness. Previously no UNIQUE constraint, so two
--    sessions could share a token. Partial index allows NULLs.
CREATE UNIQUE INDEX IF NOT EXISTS remote_sessions_token_key
ON remote_sessions(token)
WHERE token IS NOT NULL;

-- 7. search_path hardening on the is_admin() SECURITY DEFINER helper (no-arg
--    signature — it reads auth.uid() internally).
ALTER FUNCTION public.is_admin() SET search_path = public, pg_temp;
