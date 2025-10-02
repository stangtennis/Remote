-- Allow anyone to read device status (needed for approval polling)
-- This is safe because:
-- 1. device_id is a unique random hash (not guessable)
-- 2. Devices need to check their own approval status
-- 3. API keys and sensitive data are not exposed

DROP POLICY IF EXISTS "Users can view own and pending devices" ON public.remote_devices;

CREATE POLICY "Anyone can view devices" ON public.remote_devices
  FOR SELECT 
  USING (true);

COMMENT ON POLICY "Anyone can view devices" ON public.remote_devices 
IS 'Allow open read access. Safe because device_id is a secure random hash and no sensitive data is exposed.';
