-- Complete Database Schema Fix for CompleteRemoteAgent v7.0.0
-- This adds ALL missing columns needed for full agent registration

-- Add all missing columns to remote_devices table
ALTER TABLE public.remote_devices 
ADD COLUMN IF NOT EXISTS agent_version TEXT DEFAULT '7.0.0';

ALTER TABLE public.remote_devices 
ADD COLUMN IF NOT EXISTS is_online BOOLEAN DEFAULT false;

ALTER TABLE public.remote_devices 
ADD COLUMN IF NOT EXISTS architecture TEXT DEFAULT 'x64';

ALTER TABLE public.remote_devices 
ADD COLUMN IF NOT EXISTS cpu_count INTEGER DEFAULT 1;

ALTER TABLE public.remote_devices 
ADD COLUMN IF NOT EXISTS total_memory INTEGER DEFAULT 0;

ALTER TABLE public.remote_devices 
ADD COLUMN IF NOT EXISTS local_ip TEXT DEFAULT '127.0.0.1';

ALTER TABLE public.remote_devices 
ADD COLUMN IF NOT EXISTS local_port INTEGER DEFAULT 8080;

ALTER TABLE public.remote_devices 
ADD COLUMN IF NOT EXISTS has_native_modules BOOLEAN DEFAULT false;

ALTER TABLE public.remote_devices 
ADD COLUMN IF NOT EXISTS connected_clients INTEGER DEFAULT 0;

ALTER TABLE public.remote_devices 
ADD COLUMN IF NOT EXISTS mode TEXT DEFAULT 'one-time';

-- Update existing devices to have default values
UPDATE public.remote_devices 
SET 
    agent_version = COALESCE(agent_version, '7.0.0'),
    is_online = COALESCE(is_online, false),
    architecture = COALESCE(architecture, 'x64'),
    cpu_count = COALESCE(cpu_count, 1),
    total_memory = COALESCE(total_memory, 0),
    local_ip = COALESCE(local_ip, '127.0.0.1'),
    local_port = COALESCE(local_port, 8080),
    has_native_modules = COALESCE(has_native_modules, false),
    connected_clients = COALESCE(connected_clients, 0),
    mode = COALESCE(mode, 'one-time')
WHERE agent_version IS NULL 
   OR is_online IS NULL 
   OR architecture IS NULL 
   OR cpu_count IS NULL 
   OR total_memory IS NULL 
   OR local_ip IS NULL 
   OR local_port IS NULL 
   OR has_native_modules IS NULL 
   OR connected_clients IS NULL 
   OR mode IS NULL;

-- Verify all columns exist
SELECT column_name, data_type, is_nullable, column_default
FROM information_schema.columns 
WHERE table_name = 'remote_devices' 
  AND table_schema = 'public'
ORDER BY ordinal_position;

-- Show current table structure
\d public.remote_devices;
