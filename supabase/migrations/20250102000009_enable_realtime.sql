-- Enable Realtime replication for session_signaling table
-- This allows the dashboard to receive agent messages in real-time

ALTER PUBLICATION supabase_realtime ADD TABLE session_signaling;

COMMENT ON TABLE public.session_signaling 
IS 'WebRTC signaling messages with Realtime enabled for instant delivery';
