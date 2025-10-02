-- Allow users to delete devices
-- This enables cleanup of old/duplicate device entries

CREATE POLICY "allow_delete_devices" ON public.remote_devices
  FOR DELETE 
  USING (true);

COMMENT ON POLICY "allow_delete_devices" ON public.remote_devices 
IS 'Allow deleting devices. Safe because device_id is a secure hash.';
