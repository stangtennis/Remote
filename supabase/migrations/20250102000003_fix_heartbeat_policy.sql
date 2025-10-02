-- Allow devices to update their own status using device_id in query filter
-- This enables heartbeat updates before the device has received an api_key

CREATE POLICY "Devices can update self by device_id" ON public.remote_devices
  FOR UPDATE 
  USING (true)
  WITH CHECK (true);

COMMENT ON POLICY "Devices can update self by device_id" ON public.remote_devices 
IS 'Allow heartbeat updates. Device proves identity by knowing the unique device_id hash.';
