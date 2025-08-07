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

-- Add updated_at trigger for device_presence
CREATE OR REPLACE FUNCTION update_device_presence_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_device_presence_timestamp
    BEFORE UPDATE ON device_presence
    FOR EACH ROW
    EXECUTE FUNCTION update_device_presence_timestamp();

-- Function to clean up old presence records
CREATE OR REPLACE FUNCTION cleanup_old_presence()
RETURNS void AS $$
BEGIN
    -- Mark devices as offline if not seen for 5 minutes
    UPDATE device_presence 
    SET status = 'offline'
    WHERE status != 'offline' 
    AND last_seen < NOW() - INTERVAL '5 minutes';
    
    -- Delete very old presence records (older than 24 hours)
    DELETE FROM device_presence 
    WHERE last_seen < NOW() - INTERVAL '24 hours';
END;
$$ LANGUAGE plpgsql;

-- Add some helpful views
CREATE OR REPLACE VIEW online_devices AS
SELECT 
    dp.*,
    rd.device_name,
    rd.operating_system,
    rd.metadata as device_metadata
FROM device_presence dp
JOIN remote_devices rd ON dp.device_id = rd.device_id
WHERE dp.status = 'online';

CREATE OR REPLACE VIEW active_sessions_view AS
SELECT 
    rs.*,
    rd.device_name,
    rd.operating_system,
    dp.status as device_status
FROM remote_sessions rs
JOIN remote_devices rd ON rs.device_id = rd.device_id
LEFT JOIN device_presence dp ON rs.device_id = dp.device_id
WHERE rs.status = 'active';

-- Grant necessary permissions
GRANT ALL ON device_presence TO authenticated;
GRANT ALL ON realtime_channels TO authenticated;
GRANT SELECT ON online_devices TO authenticated;
GRANT SELECT ON active_sessions_view TO authenticated;

-- Insert initial test data for development
INSERT INTO device_presence (device_id, status, metadata) 
VALUES ('test-device-001', 'offline', '{"test": true}')
ON CONFLICT (device_id) DO NOTHING;

COMMENT ON TABLE device_presence IS 'Tracks real-time presence and status of connected devices';
COMMENT ON TABLE realtime_channels IS 'Manages Supabase Realtime channels for device communication';
COMMENT ON VIEW online_devices IS 'View of currently online devices with device information';
COMMENT ON VIEW active_sessions_view IS 'View of active remote sessions with device status';
