-- Complete Database Table Recreation for CompleteRemoteAgent v7.0.0
-- This creates the correct table structure that matches the agent's expectations

-- Drop existing table and recreate with correct schema
DROP TABLE IF EXISTS public.remote_devices CASCADE;

-- Create remote_devices table with correct column names
CREATE TABLE public.remote_devices (
    device_id TEXT PRIMARY KEY,           -- Agent uses device_id, not id
    device_name TEXT NOT NULL,
    org_id TEXT DEFAULT 'default',
    platform TEXT,                       -- Agent sends os.platform()
    architecture TEXT DEFAULT 'x64',     -- Agent sends os.arch()
    cpu_count INTEGER DEFAULT 1,         -- Agent sends os.cpus().length
    total_memory INTEGER DEFAULT 0,      -- Agent sends total memory in GB
    agent_version TEXT DEFAULT '7.0.0',  -- Agent sends version
    is_online BOOLEAN DEFAULT false,     -- Agent updates online status
    last_seen TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    local_ip TEXT DEFAULT '127.0.0.1',   -- Agent sends local IP
    local_port INTEGER DEFAULT 8080,     -- Agent sends port
    has_native_modules BOOLEAN DEFAULT false,  -- Agent capability
    connected_clients INTEGER DEFAULT 0,  -- Number of connected dashboards
    mode TEXT DEFAULT 'one-time',        -- Agent mode (service/one-time)
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for better performance
CREATE INDEX idx_remote_devices_device_id ON public.remote_devices(device_id);
CREATE INDEX idx_remote_devices_is_online ON public.remote_devices(is_online);
CREATE INDEX idx_remote_devices_last_seen ON public.remote_devices(last_seen);
CREATE INDEX idx_remote_devices_device_name ON public.remote_devices(device_name);
CREATE INDEX idx_remote_devices_org_id ON public.remote_devices(org_id);

-- Enable Row Level Security (RLS)
ALTER TABLE public.remote_devices ENABLE ROW LEVEL SECURITY;

-- Create RLS policies for remote_devices table
-- Allow anonymous users to read all devices (for dashboard)
CREATE POLICY "Allow anonymous read access" ON public.remote_devices
    FOR SELECT USING (true);

-- Allow anonymous users to insert new devices (for agent registration)
CREATE POLICY "Allow anonymous insert access" ON public.remote_devices
    FOR INSERT WITH CHECK (true);

-- Allow anonymous users to update devices (for heartbeats and status updates)
CREATE POLICY "Allow anonymous update access" ON public.remote_devices
    FOR UPDATE USING (true);

-- Allow anonymous users to delete devices (for device removal)
CREATE POLICY "Allow anonymous delete access" ON public.remote_devices
    FOR DELETE USING (true);

-- Create updated_at trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger to automatically update updated_at
CREATE TRIGGER update_remote_devices_updated_at 
    BEFORE UPDATE ON public.remote_devices 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Insert a test device to verify structure
INSERT INTO public.remote_devices (
    device_id, 
    device_name, 
    platform, 
    architecture, 
    agent_version,
    is_online
) VALUES (
    'test_device_123',
    'Test Device',
    'win32',
    'x64',
    '7.0.0',
    false
) ON CONFLICT (device_id) DO NOTHING;

-- Verify the table structure
SELECT column_name, data_type, is_nullable, column_default
FROM information_schema.columns 
WHERE table_name = 'remote_devices' 
  AND table_schema = 'public'
ORDER BY ordinal_position;

-- Show table info
\d public.remote_devices;
