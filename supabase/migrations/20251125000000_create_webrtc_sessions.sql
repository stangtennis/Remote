-- Create webrtc_sessions table for WebRTC signaling
-- This table stores offer/answer SDP directly for simpler signaling

CREATE TABLE IF NOT EXISTS public.webrtc_sessions (
  session_id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  device_id TEXT NOT NULL REFERENCES public.remote_devices(device_id) ON DELETE CASCADE,
  user_id TEXT,
  status TEXT CHECK (status IN ('pending', 'offer_sent', 'answered', 'connected', 'closed')) DEFAULT 'pending',
  offer TEXT,  -- WebRTC SDP offer (JSON-encoded SessionDescription)
  answer TEXT, -- WebRTC SDP answer (JSON-encoded SessionDescription)
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

-- Performance indexes
CREATE INDEX IF NOT EXISTS idx_webrtc_sessions_device ON public.webrtc_sessions(device_id);
CREATE INDEX IF NOT EXISTS idx_webrtc_sessions_status ON public.webrtc_sessions(status);

-- Disable RLS for simplicity (agents need to access without auth)
ALTER TABLE public.webrtc_sessions DISABLE ROW LEVEL SECURITY;

-- Auto-update updated_at
CREATE OR REPLACE FUNCTION update_webrtc_sessions_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_webrtc_sessions_updated_at
  BEFORE UPDATE ON public.webrtc_sessions
  FOR EACH ROW
  EXECUTE FUNCTION update_webrtc_sessions_updated_at();

-- Auto-cleanup old sessions (older than 1 hour)
CREATE OR REPLACE FUNCTION cleanup_old_webrtc_sessions()
RETURNS void AS $$
BEGIN
  DELETE FROM public.webrtc_sessions WHERE created_at < now() - interval '1 hour';
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON TABLE public.webrtc_sessions IS 'WebRTC session signaling with offer/answer SDP';
