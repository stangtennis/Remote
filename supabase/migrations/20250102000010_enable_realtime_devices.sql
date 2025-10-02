-- Enable Realtime for remote_devices table
-- This allows the dashboard to receive instant updates when devices come online/offline

ALTER PUBLICATION supabase_realtime ADD TABLE remote_devices;

COMMENT ON TABLE public.remote_devices
IS 'Registered remote desktop agents with Realtime enabled for status updates';
