-- ============================================================================
-- Deploy corrections — 2026-08-11
-- Fixes discovered during deploy:
--  1. get_user_devices called is_admin(auth.uid()) but the live is_admin() takes
--     NO arguments (reads auth.uid() internally). Recreate with the correct call
--     (otherwise the IDOR-guard RPC throws at runtime).
--  2. search_path hardening on is_admin() with its real signature.
--  3. (Indexes on supabase_admin-owned tables are applied separately as
--     supabase_admin — see deploy notes.)
-- ============================================================================

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

ALTER FUNCTION public.is_admin() SET search_path = public, pg_temp;
