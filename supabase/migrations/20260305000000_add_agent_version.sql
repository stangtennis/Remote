-- Add agent_version column to track which version each agent is running
ALTER TABLE public.remote_devices ADD COLUMN IF NOT EXISTS agent_version text;
