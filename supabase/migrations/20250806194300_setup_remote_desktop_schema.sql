-- Remote Desktop System - Complete Schema Migration
-- Phase 1: Supabase Realtime Schema Updates
-- Enable realtime for existing tables and add presence tracking

-- Enable realtime for existing tables
ALTER PUBLICATION supabase_realtime ADD TABLE remote_devices;
ALTER PUBLICATION supabase_realtime ADD TABLE remote_sessions;
ALTER PUBLICATION supabase_realtime ADD TABLE connection_logs;
ALTER PUBLICATION supabase_realtime ADD TABLE device_permissions;

-- Add presence tracking table for global device status
CREATE TABLE IF NOT EXISTS device_presence (
    device_id TEXT PRIMARY KEY,
    status TEXT NOT NULL CHECK (status IN ('online', 'offline', 'busy', 'controlled')),
    last_seen TIMESTAMPTZ DEFAULT NOW(),
    connection_info JSONB DEFAULT '{}',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Enable RLS for device presence
ALTER TABLE device_presence ENABLE ROW LEVEL SECURITY;

-- Create policies for device presence (allow all for now, will restrict later)
CREATE POLICY "Public access for device presence" ON device_presence
    FOR ALL USING (TRUE);

-- Add realtime channels table for managing communication channels
CREATE TABLE IF NOT EXISTS realtime_channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_name TEXT NOT NULL,
    channel_type TEXT NOT NULL CHECK (channel_type IN ('device', 'session', 'control', 'stream')),
    device_id TEXT,
    session_id UUID,
    created_by UUID,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    UNIQUE(channel_name)
);

-- Enable RLS for realtime channels
ALTER TABLE realtime_channels ENABLE ROW LEVEL SECURITY;

-- Create policies for realtime channels
CREATE POLICY "Users can manage their own channels" ON realtime_channels
    FOR ALL USING (TRUE); -- Simplified for now

-- Add indexes for performance
CREATE INDEX IF NOT EXISTS idx_device_presence_status ON device_presence(status);
CREATE INDEX IF NOT EXISTS idx_device_presence_last_seen ON device_presence(last_seen);
CREATE INDEX IF NOT EXISTS idx_realtime_channels_device ON realtime_channels(device_id);
CREATE INDEX IF NOT EXISTS idx_realtime_channels_session ON realtime_channels(session_id);
CREATE INDEX IF NOT EXISTS idx_realtime_channels_type ON realtime_channels(channel_type);

-- Enable realtime for new tables
ALTER PUBLICATION supabase_realtime ADD TABLE device_presence;
ALTER PUBLICATION supabase_realtime ADD TABLE realtime_channels;

-- Agent tracking schema
CREATE TABLE IF NOT EXISTS agent_generations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    platform TEXT NOT NULL CHECK (platform IN ('windows', 'macos', 'linux')),
    device_name TEXT NOT NULL,
    device_token TEXT NOT NULL,
    org_id TEXT DEFAULT 'default',
    generated_at TIMESTAMPTZ DEFAULT NOW(),
    generated_by UUID,
    download_count INTEGER DEFAULT 0,
    last_downloaded TIMESTAMPTZ,
    metadata JSONB DEFAULT '{}',
    status TEXT DEFAULT 'active' CHECK (status IN ('active', 'revoked', 'expired'))
);

-- Enable RLS for agent generations
ALTER TABLE agent_generations ENABLE ROW LEVEL SECURITY;

-- Create policies for agent generations
CREATE POLICY "Public access for agent generations" ON agent_generations
    FOR ALL USING (TRUE);

-- Add indexes for agent tracking
CREATE INDEX IF NOT EXISTS idx_agent_generations_platform ON agent_generations(platform);
CREATE INDEX IF NOT EXISTS idx_agent_generations_device_token ON agent_generations(device_token);
CREATE INDEX IF NOT EXISTS idx_agent_generations_status ON agent_generations(status);
CREATE INDEX IF NOT EXISTS idx_agent_generations_generated_at ON agent_generations(generated_at);

-- Enable realtime for agent tracking
ALTER PUBLICATION supabase_realtime ADD TABLE agent_generations;