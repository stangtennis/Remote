-- Fix: Drop the recursive trigger that causes stack depth errors
-- The expire_sessions trigger causes infinite recursion because:
-- 1. Trigger fires AFTER INSERT/UPDATE on remote_sessions
-- 2. Trigger function does UPDATE on remote_sessions
-- 3. UPDATE triggers the same trigger again → infinite loop

DROP TRIGGER IF EXISTS trigger_expire_sessions ON public.remote_sessions;

-- We'll handle session expiration differently (periodic cleanup job or client-side)
-- For now, sessions will just expire naturally based on expires_at timestamp
