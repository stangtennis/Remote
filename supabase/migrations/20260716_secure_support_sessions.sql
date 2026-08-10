-- SEC-1: Tighten anon access to support_sessions.
--
-- Previous policy (20260217_tighten_rls.sql:230) exposed ALL columns of every
-- pending/active support session — including `pin` and `token` — to any caller
-- holding the public anon key, with no token binding. This let anyone harvest
-- active Quick Support PINs/tokens and hijack screen-share sessions.
--
-- Fix: restrict anon SELECT/UPDATE to rows where is_public = true. Private
-- (PIN/token) sessions are now invisible to anon. This does NOT break the guest
-- flow because:
--   * Private guests validate/exchange signaling through the `support-signal`
--     Edge Function, which runs with the service_role key (bypasses RLS).
--   * No frontend code performs a direct `from('support_sessions')` SELECT or
--     UPDATE with the anon key (verified). The only anon consumer is the
--     public Realtime channel filtered on is_public=eq.true, which this policy
--     still permits.
--
-- Public sessions remain anon-accessible by design (they are meant to be public).

DROP POLICY IF EXISTS "anon_select_support_sessions" ON public.support_sessions;
CREATE POLICY "anon_select_support_sessions"
ON public.support_sessions
FOR SELECT TO anon
USING (is_public = true AND status IN ('pending', 'active'));

DROP POLICY IF EXISTS "anon_update_support_sessions" ON public.support_sessions;
CREATE POLICY "anon_update_support_sessions"
ON public.support_sessions
FOR UPDATE TO anon
USING (is_public = true AND status IN ('pending', 'active'))
WITH CHECK (is_public = true AND status IN ('active', 'ended'));
