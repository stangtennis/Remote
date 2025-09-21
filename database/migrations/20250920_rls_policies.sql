-- RLS Policies draft (to be refined and enabled after flows are wired)
-- Created: 2025-09-20

-- IMPORTANT: Do NOT enable RLS without appropriate policies or you may lock yourself out.
-- The statements below are provided as a starting point and are commented out.

-- Example: enable RLS
-- alter table public.remote_devices enable row level security;
-- alter table public.remote_sessions enable row level security;
-- alter table public.session_signaling enable row level security;
-- alter table public.audit_logs enable row level security;

-- Example policies (broad; tighten later):
-- create policy "remote_devices_select_public"
--   on public.remote_devices for select
--   using (true);

-- create policy "remote_devices_update_service"
--   on public.remote_devices for update
--   using (true) with check (true);
-- -- NOTE: Service role bypasses RLS, but this keeps dev parity when using anon keys.

-- create policy "remote_sessions_select_participants"
--   on public.remote_sessions for select
--   using (true);

-- create policy "remote_sessions_insert_public"
--   on public.remote_sessions for insert
--   with check (true);

-- create policy "session_signaling_rw_participants"
--   on public.session_signaling for all
--   using (true) with check (true);

-- create policy "audit_logs_insert_service"
--   on public.audit_logs for insert
--   with check (true);

-- TODO:
-- - Replace (true) with token/PIN-bound logic once Edge Function issues tokens
-- - Optionally create a view for dashboard-safe columns if you need column filtering
