-- Auto-log device online/offline changes via DB trigger
CREATE OR REPLACE FUNCTION log_device_online_change() RETURNS trigger AS $$
BEGIN
  IF OLD.is_online IS DISTINCT FROM NEW.is_online THEN
    INSERT INTO public.audit_logs (device_id, event, severity, details)
    VALUES (
      NEW.device_id,
      CASE WHEN NEW.is_online THEN 'DEVICE_ONLINE' ELSE 'DEVICE_OFFLINE' END,
      'info',
      jsonb_build_object('device_name', NEW.device_name)
    );
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

CREATE TRIGGER trigger_device_online_change
  AFTER UPDATE OF is_online ON public.remote_devices
  FOR EACH ROW EXECUTE FUNCTION log_device_online_change();

-- Index for actor-queries
CREATE INDEX IF NOT EXISTS idx_audit_actor ON public.audit_logs(actor);

-- Allow authenticated users to read audit logs (not just admins)
DROP POLICY IF EXISTS "Admins read audit logs" ON public.audit_logs;
CREATE POLICY "Authenticated users read audit logs" ON public.audit_logs
  FOR SELECT USING (auth.uid() IS NOT NULL);
