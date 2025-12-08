-- Auto-offline detection: Mark devices as offline if last_seen is older than 60 seconds
-- This handles cases where agent crashes or loses network without graceful shutdown

-- Create function to check and update offline devices
CREATE OR REPLACE FUNCTION check_offline_devices()
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
BEGIN
  UPDATE remote_devices
  SET is_online = false
  WHERE is_online = true
    AND last_seen < NOW() - INTERVAL '60 seconds';
END;
$$;

-- Create a scheduled job to run every 30 seconds (requires pg_cron extension)
-- Note: pg_cron must be enabled in Supabase dashboard under Database > Extensions

-- If pg_cron is available:
-- SELECT cron.schedule('check-offline-devices', '30 seconds', 'SELECT check_offline_devices()');

-- Alternative: Create a trigger that checks on any device update
CREATE OR REPLACE FUNCTION trigger_check_offline()
RETURNS TRIGGER
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
BEGIN
  -- When any device updates, also check for stale devices
  PERFORM check_offline_devices();
  RETURN NEW;
END;
$$;

-- Create trigger (fires on any device heartbeat, checking all devices)
DROP TRIGGER IF EXISTS check_offline_on_update ON remote_devices;
CREATE TRIGGER check_offline_on_update
  AFTER UPDATE ON remote_devices
  FOR EACH STATEMENT
  EXECUTE FUNCTION trigger_check_offline();

-- Grant execute permission
GRANT EXECUTE ON FUNCTION check_offline_devices() TO authenticated;
GRANT EXECUTE ON FUNCTION check_offline_devices() TO anon;
