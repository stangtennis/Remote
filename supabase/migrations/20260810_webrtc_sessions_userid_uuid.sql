-- ============================================================================
-- webrtc_sessions.user_id: text -> uuid — 2026-08-10
-- Enables referential integrity (FK to auth.users) and aligns the column type
-- with every other user_id column (uuid). webrtc_sessions is transient
-- (cleaned hourly by cleanup_old_webrtc_sessions), so if the cast fails on
-- legacy rows just TRUNCATE public.webrtc_sessions and re-run this migration.
-- NULLIF guards against empty-string rows; NULLs pass through unchanged.
-- ============================================================================

ALTER TABLE public.webrtc_sessions
ALTER COLUMN user_id TYPE uuid USING NULLIF(user_id, '')::uuid;
