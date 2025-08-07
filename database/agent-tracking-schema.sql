-- Agent Tracking Schema
-- For tracking generated agents and their deployment status

-- Table to track agent generations
CREATE TABLE IF NOT EXISTS agent_generations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    platform TEXT NOT NULL CHECK (platform IN ('windows', 'macos', 'linux')),
    device_name TEXT NOT NULL,
    filename TEXT NOT NULL,
    device_token TEXT NOT NULL UNIQUE,
    generated_at TIMESTAMPTZ DEFAULT NOW(),
    generated_by UUID REFERENCES auth.users(id),
    org_id TEXT DEFAULT 'default',
    config JSONB DEFAULT '{}',
    download_count INTEGER DEFAULT 0,
    last_downloaded TIMESTAMPTZ,
    status TEXT DEFAULT 'generated' CHECK (status IN ('generated', 'downloaded', 'deployed', 'active', 'inactive'))
);

-- Enable RLS
ALTER TABLE agent_generations ENABLE ROW LEVEL SECURITY;

-- Create policies
CREATE POLICY "Users can manage their own generated agents" ON agent_generations
    FOR ALL USING (generated_by = auth.uid());

-- Table to track agent deployments and connections
CREATE TABLE IF NOT EXISTS agent_deployments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_generation_id UUID REFERENCES agent_generations(id) ON DELETE CASCADE,
    device_id TEXT NOT NULL,
    deployed_at TIMESTAMPTZ DEFAULT NOW(),
    first_connection TIMESTAMPTZ,
    last_connection TIMESTAMPTZ,
    connection_count INTEGER DEFAULT 0,
    status TEXT DEFAULT 'deployed' CHECK (status IN ('deployed', 'connected', 'disconnected', 'error')),
    metadata JSONB DEFAULT '{}'
);

-- Enable RLS
ALTER TABLE agent_deployments ENABLE ROW LEVEL SECURITY;

-- Create policies
CREATE POLICY "Users can view their agent deployments" ON agent_deployments
    FOR SELECT USING (
        agent_generation_id IN (
            SELECT id FROM agent_generations WHERE generated_by = auth.uid()
        )
    );

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_agent_generations_generated_by ON agent_generations(generated_by);
CREATE INDEX IF NOT EXISTS idx_agent_generations_platform ON agent_generations(platform);
CREATE INDEX IF NOT EXISTS idx_agent_generations_generated_at ON agent_generations(generated_at);
CREATE INDEX IF NOT EXISTS idx_agent_deployments_agent_generation ON agent_deployments(agent_generation_id);
CREATE INDEX IF NOT EXISTS idx_agent_deployments_device_id ON agent_deployments(device_id);

-- Function to update agent deployment status when device connects
CREATE OR REPLACE FUNCTION update_agent_deployment_on_connection()
RETURNS TRIGGER AS $$
BEGIN
    -- Find matching agent deployment by device_id
    UPDATE agent_deployments 
    SET 
        status = CASE 
            WHEN NEW.status = 'online' THEN 'connected'
            ELSE 'disconnected'
        END,
        last_connection = CASE 
            WHEN NEW.status = 'online' THEN NOW()
            ELSE last_connection
        END,
        first_connection = CASE 
            WHEN first_connection IS NULL AND NEW.status = 'online' THEN NOW()
            ELSE first_connection
        END,
        connection_count = CASE 
            WHEN NEW.status = 'online' AND OLD.status != 'online' THEN connection_count + 1
            ELSE connection_count
        END
    WHERE device_id = NEW.device_id;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to update deployment status when device presence changes
CREATE TRIGGER update_agent_deployment_trigger
    AFTER UPDATE ON device_presence
    FOR EACH ROW
    EXECUTE FUNCTION update_agent_deployment_on_connection();

-- View for agent statistics
CREATE OR REPLACE VIEW agent_statistics AS
SELECT 
    ag.generated_by,
    ag.platform,
    COUNT(*) as total_generated,
    COUNT(ad.id) as total_deployed,
    COUNT(CASE WHEN ad.status = 'connected' THEN 1 END) as currently_connected,
    MAX(ag.generated_at) as last_generation,
    SUM(ag.download_count) as total_downloads
FROM agent_generations ag
LEFT JOIN agent_deployments ad ON ag.id = ad.agent_generation_id
GROUP BY ag.generated_by, ag.platform;

-- Grant permissions
GRANT ALL ON agent_generations TO authenticated;
GRANT ALL ON agent_deployments TO authenticated;
GRANT SELECT ON agent_statistics TO authenticated;

-- Insert some test data
INSERT INTO agent_generations (platform, device_name, filename, device_token, generated_by, config)
VALUES 
    ('windows', 'Test Windows PC', 'RemoteAgent_Test_Windows_PC_2024-01-06.bat', 'test_token_win_001', 
     '00000000-0000-0000-0000-000000000000', '{"autoStart": true, "hideWindow": true}'),
    ('macos', 'Test Mac', 'RemoteAgent_Test_Mac_2024-01-06.sh', 'test_token_mac_001', 
     '00000000-0000-0000-0000-000000000000', '{"autoStart": true, "hideWindow": false}'),
    ('linux', 'Test Linux Server', 'RemoteAgent_Test_Linux_Server_2024-01-06.sh', 'test_token_linux_001', 
     '00000000-0000-0000-0000-000000000000', '{"autoStart": false, "hideWindow": true}')
ON CONFLICT (device_token) DO NOTHING;

COMMENT ON TABLE agent_generations IS 'Tracks generated client agents for download and deployment';
COMMENT ON TABLE agent_deployments IS 'Tracks deployment and connection status of generated agents';
COMMENT ON VIEW agent_statistics IS 'Provides statistics on agent generation and deployment by user and platform';
