-- SEC-7: Replace over-broad audit_logs read policy.
--
-- "Authenticated users read audit logs" (20260308_activity_log.sql:26) used
-- `USING (auth.uid() IS NOT NULL)` — i.e. ANY authenticated user could read
-- ALL audit rows for every device/session/user, including SHELL_EXEC details
-- and actor UUIDs of other tenants. The 20260429 hardening pass tightened the
-- analogous WRITE policy but missed this SELECT.
--
-- Fix: drop the broad policy and add an admin-scoped one. Non-admin users keep
-- their existing own-row read access via:
--   * "Users can view own audit logs"  (20250108000000_enable_security.sql:180, actor = uid)
--   * "Device owners read own audit logs" (20260428_audit_logs_owner_read.sql:5)
-- so the dashboard activity panel still works (admins see everything; regular
-- users see their own and their devices' events).

DROP POLICY IF EXISTS "Authenticated users read audit logs" ON public.audit_logs;

CREATE POLICY "Admins read all audit logs"
ON public.audit_logs
FOR SELECT TO authenticated
USING (public.is_admin(auth.uid()));
