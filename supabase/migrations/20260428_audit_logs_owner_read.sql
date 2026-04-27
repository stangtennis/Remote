-- Allow device owners to read audit_logs entries for their devices.
-- Until now, only admins could SELECT — but the v3.0.2 shell exec feature
-- writes SHELL_EXEC entries that owners need to review themselves.

CREATE POLICY "Device owners read own audit logs" ON public.audit_logs
  FOR SELECT USING (
    device_id IS NOT NULL AND EXISTS (
      SELECT 1 FROM public.remote_devices d
      WHERE d.device_id = audit_logs.device_id
        AND d.owner_id = auth.uid()
    )
  );

-- Indexes already exist (idx_audit_device, idx_audit_time) — no new indexes needed.
