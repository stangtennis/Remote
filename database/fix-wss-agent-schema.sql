-- Comprehensive schema fix for WSS-enabled agent
-- This adds all columns that the WSS agent expects

-- Add missing columns for WSS agent compatibility
ALTER TABLE public.remote_devices 
ADD COLUMN IF NOT EXISTS platform TEXT;

ALTER TABLE public.remote_devices 
ADD COLUMN IF NOT EXISTS architecture TEXT;

ALTER TABLE public.remote_devices 
ADD COLUMN IF NOT EXISTS cpu_count INTEGER;

ALTER TABLE public.remote_devices 
ADD COLUMN IF NOT EXISTS total_memory_gb INTEGER;

-- Ensure all previously added columns exist
ALTER TABLE public.remote_devices 
ADD COLUMN IF NOT EXISTS is_online BOOLEAN DEFAULT false;

ALTER TABLE public.remote_devices 
ADD COLUMN IF NOT EXISTS agent_version TEXT DEFAULT '1.0.0';

ALTER TABLE public.remote_devices 
ADD COLUMN IF NOT EXISTS mode TEXT DEFAULT 'one-time';

ALTER TABLE public.remote_devices 
ADD COLUMN IF NOT EXISTS local_port INTEGER DEFAULT 8080;

ALTER TABLE public.remote_devices 
ADD COLUMN IF NOT EXISTS has_native_modules BOOLEAN DEFAULT false;

-- Update existing operating_system column to also accept platform data
-- (WSS agent uses 'platform' but table has 'operating_system')
UPDATE public.remote_devices 
SET operating_system = platform 
WHERE platform IS NOT NULL AND operating_system IS NULL;
