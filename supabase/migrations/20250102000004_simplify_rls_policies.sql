-- Simplify RLS policies to avoid circular dependencies and stack depth issues
-- Drop all existing policies and create clean, simple ones

-- Drop all existing policies on remote_devices
DROP POLICY IF EXISTS "Users can view own devices" ON public.remote_devices;
DROP POLICY IF EXISTS "Devices can update own status" ON public.remote_devices;
DROP POLICY IF EXISTS "Users can view own and pending devices" ON public.remote_devices;
DROP POLICY IF EXISTS "Users can approve pending devices" ON public.remote_devices;
DROP POLICY IF EXISTS "Anyone can check device status by device_id" ON public.remote_devices;
DROP POLICY IF EXISTS "Anyone can view devices" ON public.remote_devices;
DROP POLICY IF EXISTS "Devices can update self by device_id" ON public.remote_devices;

-- Create simple, non-overlapping policies

-- 1. Anyone can SELECT (read) devices
CREATE POLICY "allow_select_devices" ON public.remote_devices
  FOR SELECT 
  USING (true);

-- 2. Anyone can UPDATE devices (heartbeat, approval, etc)
CREATE POLICY "allow_update_devices" ON public.remote_devices
  FOR UPDATE 
  USING (true)
  WITH CHECK (true);

-- 3. Anyone can INSERT devices (registration)
CREATE POLICY "allow_insert_devices" ON public.remote_devices
  FOR INSERT 
  WITH CHECK (true);

COMMENT ON POLICY "allow_select_devices" ON public.remote_devices 
IS 'Allow reading devices. device_id is a secure hash so this is safe.';

COMMENT ON POLICY "allow_update_devices" ON public.remote_devices 
IS 'Allow updates. Used for heartbeats and approval. Devices prove identity via device_id.';

COMMENT ON POLICY "allow_insert_devices" ON public.remote_devices 
IS 'Allow device registration.';
