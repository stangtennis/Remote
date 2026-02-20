-- Enable pg_cron extension for scheduled tasks
CREATE EXTENSION IF NOT EXISTS pg_cron;

-- Create a function that calls the cleanup Edge Function
CREATE OR REPLACE FUNCTION trigger_session_cleanup()
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
DECLARE
  function_url text;
  service_key text;
BEGIN
  -- Get Supabase URL and service key from environment
  function_url := current_setting('app.settings.supabase_url', true) || '/functions/v1/session-cleanup';
  service_key := current_setting('app.settings.supabase_service_key', true);
  
  -- Call the Edge Function via HTTP
  -- Note: In production, use pg_net extension or just do cleanup directly in SQL
  PERFORM cleanup_old_sessions_direct();
END;
$$;

-- Direct SQL cleanup function (more reliable than HTTP calls)
CREATE OR REPLACE FUNCTION cleanup_old_sessions_direct()
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
DECLARE
  five_minutes_ago timestamptz := now() - interval '5 minutes';
  fifteen_minutes_ago timestamptz := now() - interval '15 minutes';
  twenty_four_hours_ago timestamptz := now() - interval '24 hours';
  two_minutes_ago timestamptz := now() - interval '2 minutes';
BEGIN
  -- 1. Clean up old signaling messages (older than 5 minutes)
  DELETE FROM public.session_signaling
  WHERE created_at < five_minutes_ago;
  
  RAISE NOTICE '✅ Cleaned up old signaling messages';

  -- 2. Expire old pending/active sessions (older than 15 minutes)
  UPDATE public.remote_sessions
  SET status = 'expired', ended_at = now()
  WHERE status IN ('pending', 'active')
    AND created_at < fifteen_minutes_ago;
  
  RAISE NOTICE '✅ Expired old sessions';

  -- 3. Delete really old expired/ended sessions (older than 24 hours)
  DELETE FROM public.remote_sessions
  WHERE status IN ('expired', 'ended')
    AND created_at < twenty_four_hours_ago;
  
  RAISE NOTICE '✅ Deleted old completed sessions';

  -- 4. Update offline status for devices that haven't been seen in 2 minutes
  UPDATE public.remote_devices
  SET is_online = false
  WHERE is_online = true
    AND last_seen < two_minutes_ago;
  
  RAISE NOTICE '✅ Marked inactive devices as offline';
  
END;
$$;

-- Schedule cleanup to run every 5 minutes
SELECT cron.schedule(
  'session-cleanup',           -- job name
  '*/5 * * * *',               -- every 5 minutes
  'SELECT cleanup_old_sessions_direct();'
);

-- Also create a manual trigger for testing
COMMENT ON FUNCTION cleanup_old_sessions_direct() IS 'Cleans up old sessions, signaling messages, and updates device status. Runs automatically every 5 minutes via pg_cron.';
