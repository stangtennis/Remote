-- Keep the direct pg_cron cleanup path aligned with the support Edge Function.
-- This is required because production may run the SQL job without invoking
-- the Edge Function.

CREATE OR REPLACE FUNCTION public.cleanup_old_sessions_direct()
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
  five_minutes_ago TIMESTAMPTZ := now() - interval '5 minutes';
  fifteen_minutes_ago TIMESTAMPTZ := now() - interval '15 minutes';
  thirty_minutes_ago TIMESTAMPTZ := now() - interval '30 minutes';
  ninety_days_ago TIMESTAMPTZ := now() - interval '90 days';
  two_minutes_ago TIMESTAMPTZ := now() - interval '2 minutes';
BEGIN
  DELETE FROM public.session_signaling WHERE created_at < five_minutes_ago;

  UPDATE public.remote_sessions
     SET status = 'expired', ended_at = now()
  WHERE status IN ('pending', 'active') AND created_at < fifteen_minutes_ago;

  DELETE FROM public.remote_sessions
  WHERE status IN ('expired', 'ended') AND created_at < ninety_days_ago;

  UPDATE public.remote_devices
  SET is_online = false
  WHERE is_online = true AND last_seen < two_minutes_ago;

  WITH expired_support AS (
    UPDATE public.support_sessions
    SET status = 'expired', ended_at = now()
    WHERE status IN ('pending', 'active') AND expires_at < now()
    RETURNING id
  )
  INSERT INTO public.session_signaling(session_id, from_side, msg_type, payload)
  SELECT id, 'dashboard', 'bye', jsonb_build_object('reason', 'support_session_expired')
  FROM expired_support;

  DELETE FROM public.support_action_audit WHERE created_at < ninety_days_ago;
  DELETE FROM public.support_sessions
  WHERE status IN ('expired', 'ended') AND created_at < ninety_days_ago;
END;
$$;

COMMENT ON FUNCTION public.cleanup_old_sessions_direct() IS
  'Cleans remote and support sessions, support audit history, signaling, and offline devices every five minutes.';
