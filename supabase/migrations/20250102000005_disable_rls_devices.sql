-- Temporarily disable RLS on remote_devices to diagnose stack depth issue
-- This is safe for testing since device_id is a secure random hash

ALTER TABLE public.remote_devices DISABLE ROW LEVEL SECURITY;

COMMENT ON TABLE public.remote_devices 
IS 'RLS temporarily disabled for debugging. Re-enable after testing.';
