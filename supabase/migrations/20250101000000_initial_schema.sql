-- Initial schema for Remote Desktop Application
-- Based on plan.md Section 5

-- Enable necessary extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- remote_devices table
CREATE TABLE IF NOT EXISTS public.remote_devices (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  device_id text NOT NULL UNIQUE,
  device_name text,
  platform text,
  arch text,
  cpu_count int,
  ram_bytes bigint,
  is_online boolean DEFAULT false,
  last_seen timestamptz DEFAULT now(),
  api_key text UNIQUE, -- for device authentication
  approved_by uuid REFERENCES auth.users(id),
  approved_at timestamptz,
  owner_id uuid REFERENCES auth.users(id), -- device owner
  created_at timestamptz DEFAULT now()
);

-- Performance indexes for remote_devices
CREATE INDEX idx_remote_devices_online ON public.remote_devices(is_online, last_seen);
CREATE INDEX idx_remote_devices_owner ON public.remote_devices(owner_id) WHERE owner_id IS NOT NULL;

-- remote_sessions table
CREATE TABLE IF NOT EXISTS public.remote_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  device_id text NOT NULL REFERENCES public.remote_devices(device_id) ON DELETE CASCADE,
  created_by uuid REFERENCES auth.users(id),
  status text CHECK (status IN ('pending','active','ended','expired')) DEFAULT 'pending',
  pin text,
  token text,
  created_at timestamptz DEFAULT now(),
  expires_at timestamptz,
  ended_at timestamptz,
  metrics jsonb DEFAULT '{}'::jsonb, -- {"bitrate": 2500, "fps": 30, "rtt": 45, "packet_loss": 0.1, "connection_type": "P2P"}
  version text DEFAULT 'v1' -- API version
);

-- Performance indexes for remote_sessions
CREATE INDEX idx_sessions_status ON public.remote_sessions(status, expires_at);
CREATE INDEX idx_sessions_device ON public.remote_sessions(device_id, created_at);
CREATE INDEX idx_sessions_user ON public.remote_sessions(created_by) WHERE created_by IS NOT NULL;

-- session_signaling table
CREATE TABLE IF NOT EXISTS public.session_signaling (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  session_id uuid NOT NULL REFERENCES public.remote_sessions(id) ON DELETE CASCADE,
  from_side text CHECK (from_side IN ('dashboard','agent')) NOT NULL,
  msg_type text CHECK (msg_type IN ('offer','answer','ice')) NOT NULL,
  payload jsonb NOT NULL,
  created_at timestamptz DEFAULT now()
);

-- Performance index for session_signaling
CREATE INDEX idx_signaling_session ON public.session_signaling(session_id, created_at);

-- Auto-cleanup old signaling (24h TTL)
CREATE OR REPLACE FUNCTION cleanup_old_signaling()
RETURNS void AS $$
BEGIN
  DELETE FROM public.session_signaling WHERE created_at < now() - interval '24 hours';
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- audit_logs table
CREATE TABLE IF NOT EXISTS public.audit_logs (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  session_id uuid,
  device_id text,
  actor uuid REFERENCES auth.users(id),
  event text NOT NULL, -- error_code taxonomy: AUTH_FAIL, SESSION_START, etc.
  details jsonb,
  severity text CHECK (severity IN ('info','warning','error')) DEFAULT 'info',
  created_at timestamptz DEFAULT now()
);

-- Performance indexes for audit_logs
CREATE INDEX idx_audit_session ON public.audit_logs(session_id) WHERE session_id IS NOT NULL;
CREATE INDEX idx_audit_device ON public.audit_logs(device_id) WHERE device_id IS NOT NULL;
CREATE INDEX idx_audit_time ON public.audit_logs(created_at);

-- Auto-expire sessions trigger
CREATE OR REPLACE FUNCTION expire_sessions()
RETURNS trigger AS $$
BEGIN
  UPDATE public.remote_sessions 
  SET status = 'expired', ended_at = now()
  WHERE expires_at < now() AND status NOT IN ('ended', 'expired');
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_expire_sessions
  AFTER INSERT OR UPDATE ON public.remote_sessions
  FOR EACH STATEMENT
  EXECUTE FUNCTION expire_sessions();

-- Enable Row Level Security
ALTER TABLE public.remote_devices ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.remote_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.session_signaling ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.audit_logs ENABLE ROW LEVEL SECURITY;

-- RLS Policies for remote_devices
CREATE POLICY "Users can view own devices" ON public.remote_devices
  FOR SELECT USING (owner_id = auth.uid());

CREATE POLICY "Devices can update own status" ON public.remote_devices
  FOR UPDATE USING (
    api_key = current_setting('request.headers', true)::json->>'x-device-key'
  );

-- RLS Policies for remote_sessions
CREATE POLICY "Users can view own sessions" ON public.remote_sessions
  FOR SELECT USING (created_by = auth.uid());

CREATE POLICY "Users can create sessions" ON public.remote_sessions
  FOR INSERT WITH CHECK (created_by = auth.uid());

CREATE POLICY "Users can update own sessions" ON public.remote_sessions
  FOR UPDATE USING (created_by = auth.uid());

-- RLS Policies for session_signaling
CREATE POLICY "Session participants can signal" ON public.session_signaling
  FOR ALL USING (
    EXISTS (
      SELECT 1 FROM public.remote_sessions 
      WHERE id = session_id 
      AND (
        created_by = auth.uid() 
        OR token = current_setting('request.jwt.claims', true)::json->>'session_token'
      )
    )
  );

-- RLS Policies for audit_logs
CREATE POLICY "Admins read audit logs" ON public.audit_logs
  FOR SELECT USING (
    (current_setting('request.jwt.claims', true)::json->>'role')::text = 'admin'
  );

CREATE POLICY "System can write audit logs" ON public.audit_logs
  FOR INSERT WITH CHECK (true);

-- Helper function to log audit events
CREATE OR REPLACE FUNCTION log_audit_event(
  p_session_id uuid,
  p_device_id text,
  p_event text,
  p_details jsonb DEFAULT NULL,
  p_severity text DEFAULT 'info'
)
RETURNS void AS $$
BEGIN
  INSERT INTO public.audit_logs (session_id, device_id, actor, event, details, severity)
  VALUES (p_session_id, p_device_id, auth.uid(), p_event, p_details, p_severity);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Comments for documentation
COMMENT ON TABLE public.remote_devices IS 'Stores registered remote desktop agents';
COMMENT ON TABLE public.remote_sessions IS 'Active and historical remote desktop sessions';
COMMENT ON TABLE public.session_signaling IS 'WebRTC signaling messages (SDP/ICE)';
COMMENT ON TABLE public.audit_logs IS 'Audit trail for security and debugging';
