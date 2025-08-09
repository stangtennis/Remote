-- Create remote_devices table for Supabase Remote Desktop System
-- This table stores all connected remote desktop agents

-- Drop table if exists (for clean recreation)
DROP TABLE IF EXISTS public.remote_devices CASCADE;

-- Create remote_devices table
CREATE TABLE public.remote_devices (
    id TEXT PRIMARY KEY,
    device_name TEXT NOT NULL,
    device_type TEXT DEFAULT 'desktop',
    operating_system TEXT,
    ip_address TEXT,
    status TEXT DEFAULT 'offline',
    last_seen TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    access_key TEXT,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for better performance
CREATE INDEX idx_remote_devices_status ON public.remote_devices(status);
CREATE INDEX idx_remote_devices_last_seen ON public.remote_devices(last_seen);
CREATE INDEX idx_remote_devices_device_name ON public.remote_devices(device_name);

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

-- Create trigger to automatically update updated_at column
CREATE TRIGGER update_remote_devices_updated_at 
    BEFORE UPDATE ON public.remote_devices 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Grant necessary permissions to anon role
GRANT SELECT, INSERT, UPDATE, DELETE ON public.remote_devices TO anon;
GRANT USAGE ON SCHEMA public TO anon;

-- Insert a test device to verify table creation
INSERT INTO public.remote_devices (
    id, 
    device_name, 
    device_type, 
    operating_system, 
    ip_address, 
    status, 
    access_key,
    metadata
) VALUES (
    'test-device-12345',
    'Test Device',
    'desktop',
    'Windows 11',
    '192.168.1.100',
    'offline',
    'test_access_key_12345',
    '{"test": true, "created_by": "schema_setup"}'::jsonb
);

-- Verify table creation
SELECT 'remote_devices table created successfully' as status;
SELECT COUNT(*) as device_count FROM public.remote_devices;
