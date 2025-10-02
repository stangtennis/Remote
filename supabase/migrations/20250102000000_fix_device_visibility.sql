-- Fix RLS policy to allow users to see pending devices (owner_id = NULL)
-- This allows users to approve newly registered devices

-- Drop existing policy
DROP POLICY IF EXISTS "Users can view own devices" ON public.remote_devices;

-- Create new policy that includes pending devices
CREATE POLICY "Users can view own and pending devices" ON public.remote_devices
  FOR SELECT USING (
    owner_id = auth.uid() 
    OR owner_id IS NULL
  );

-- Update comment
COMMENT ON POLICY "Users can view own and pending devices" ON public.remote_devices 
IS 'Allow users to see their approved devices and any unapproved devices awaiting approval';
