-- Allow users to approve pending devices by setting owner_id, approved_by, approved_at

CREATE POLICY "Users can approve pending devices" ON public.remote_devices
  FOR UPDATE 
  USING (owner_id IS NULL)
  WITH CHECK (
    owner_id = auth.uid() 
    AND approved_by = auth.uid()
  );

-- Comment
COMMENT ON POLICY "Users can approve pending devices" ON public.remote_devices 
IS 'Allow authenticated users to claim/approve devices that have no owner (pending approval)';
