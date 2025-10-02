-- Add turn_config column to remote_sessions to store TURN server credentials
ALTER TABLE public.remote_sessions 
ADD COLUMN IF NOT EXISTS turn_config jsonb;

COMMENT ON COLUMN public.remote_sessions.turn_config 
IS 'TURN server configuration including ICE servers with credentials';
