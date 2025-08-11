-- Add missing columns to remote_devices table
-- This fixes the registration issue with the stable client

-- Add agent_version column
ALTER TABLE public.remote_devices 
ADD COLUMN IF NOT EXISTS agent_version TEXT DEFAULT '1.0.0';

-- Add other columns that might be missing
ALTER TABLE public.remote_devices 
ADD COLUMN IF NOT EXISTS is_online BOOLEAN DEFAULT false;

ALTER TABLE public.remote_devices 
ADD COLUMN IF NOT EXISTS mode TEXT DEFAULT 'one-time';

ALTER TABLE public.remote_devices 
ADD COLUMN IF NOT EXISTS local_port INTEGER DEFAULT 8080;

ALTER TABLE public.remote_devices 
ADD COLUMN IF NOT EXISTS has_native_modules BOOLEAN DEFAULT false;

ALTER TABLE public.remote_devices 
ADD COLUMN IF NOT EXISTS connected_clients INTEGER DEFAULT 0;

-- Update existing devices to have default values
UPDATE public.remote_devices 
SET 
    agent_version = COALESCE(agent_version, '1.0.0'),
    is_online = COALESCE(is_online, false),
    mode = COALESCE(mode, 'one-time'),
    local_port = COALESCE(local_port, 8080),
    has_native_modules = COALESCE(has_native_modules, false),
    connected_clients = COALESCE(connected_clients, 0)
WHERE agent_version IS NULL 
   OR is_online IS NULL 
   OR mode IS NULL 
   OR local_port IS NULL 
   OR has_native_modules IS NULL 
   OR connected_clients IS NULL;

-- Verify the columns were added
SELECT column_name, data_type, is_nullable, column_default
FROM information_schema.columns 
WHERE table_name = 'remote_devices' 
  AND table_schema = 'public'
ORDER BY ordinal_position;
