-- File Transfer System Database Schema
-- Supports secure file transfers between devices

-- File transfers table for tracking transfer sessions
CREATE TABLE IF NOT EXISTS file_transfers (
    id TEXT PRIMARY KEY,
    source_device_id UUID REFERENCES remote_devices(id) ON DELETE CASCADE,
    target_device_id UUID REFERENCES remote_devices(id) ON DELETE CASCADE,
    file_name TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    file_type TEXT,
    status TEXT NOT NULL CHECK (status IN ('pending', 'active', 'completed', 'failed', 'cancelled')),
    progress INTEGER DEFAULT 0 CHECK (progress >= 0 AND progress <= 100),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    metadata JSONB DEFAULT '{}',
    error_message TEXT
);

-- File transfer chunks table for tracking individual chunks
CREATE TABLE IF NOT EXISTS file_transfer_chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transfer_id TEXT REFERENCES file_transfers(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,
    chunk_size INTEGER NOT NULL,
    chunk_hash TEXT,
    storage_path TEXT NOT NULL,
    uploaded_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(transfer_id, chunk_index)
);

-- File shares table for public file sharing
CREATE TABLE IF NOT EXISTS file_shares (
    id TEXT PRIMARY KEY,
    device_id UUID REFERENCES remote_devices(id) ON DELETE CASCADE,
    file_name TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    file_type TEXT,
    share_type TEXT NOT NULL CHECK (share_type IN ('public', 'private', 'device')),
    access_code TEXT,
    download_count INTEGER DEFAULT 0,
    max_downloads INTEGER,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    metadata JSONB DEFAULT '{}'
);

-- File transfer logs for audit trail
CREATE TABLE IF NOT EXISTS file_transfer_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transfer_id TEXT REFERENCES file_transfers(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    event_data JSONB DEFAULT '{}',
    device_id UUID REFERENCES remote_devices(id),
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_file_transfers_source_device ON file_transfers(source_device_id);
CREATE INDEX IF NOT EXISTS idx_file_transfers_target_device ON file_transfers(target_device_id);
CREATE INDEX IF NOT EXISTS idx_file_transfers_status ON file_transfers(status);
CREATE INDEX IF NOT EXISTS idx_file_transfers_created_at ON file_transfers(created_at);
CREATE INDEX IF NOT EXISTS idx_file_transfers_expires_at ON file_transfers(expires_at);

CREATE INDEX IF NOT EXISTS idx_file_transfer_chunks_transfer_id ON file_transfer_chunks(transfer_id);
CREATE INDEX IF NOT EXISTS idx_file_transfer_chunks_index ON file_transfer_chunks(transfer_id, chunk_index);

CREATE INDEX IF NOT EXISTS idx_file_shares_device_id ON file_shares(device_id);
CREATE INDEX IF NOT EXISTS idx_file_shares_expires_at ON file_shares(expires_at);

CREATE INDEX IF NOT EXISTS idx_file_transfer_logs_transfer_id ON file_transfer_logs(transfer_id);
CREATE INDEX IF NOT EXISTS idx_file_transfer_logs_created_at ON file_transfer_logs(created_at);

-- Enable Row Level Security (RLS)
ALTER TABLE file_transfers ENABLE ROW LEVEL SECURITY;
ALTER TABLE file_transfer_chunks ENABLE ROW LEVEL SECURITY;
ALTER TABLE file_shares ENABLE ROW LEVEL SECURITY;
ALTER TABLE file_transfer_logs ENABLE ROW LEVEL SECURITY;

-- RLS Policies for file_transfers
CREATE POLICY "Users can view their own transfers" ON file_transfers
    FOR SELECT USING (
        source_device_id IN (
            SELECT id FROM remote_devices WHERE owner_id = auth.uid()
        ) OR 
        target_device_id IN (
            SELECT id FROM remote_devices WHERE owner_id = auth.uid()
        )
    );

CREATE POLICY "Users can create transfers from their devices" ON file_transfers
    FOR INSERT WITH CHECK (
        source_device_id IN (
            SELECT id FROM remote_devices WHERE owner_id = auth.uid()
        )
    );

CREATE POLICY "Users can update their own transfers" ON file_transfers
    FOR UPDATE USING (
        source_device_id IN (
            SELECT id FROM remote_devices WHERE owner_id = auth.uid()
        ) OR 
        target_device_id IN (
            SELECT id FROM remote_devices WHERE owner_id = auth.uid()
        )
    );

-- RLS Policies for file_transfer_chunks
CREATE POLICY "Users can access chunks for their transfers" ON file_transfer_chunks
    FOR ALL USING (
        transfer_id IN (
            SELECT id FROM file_transfers WHERE 
            source_device_id IN (
                SELECT id FROM remote_devices WHERE owner_id = auth.uid()
            ) OR 
            target_device_id IN (
                SELECT id FROM remote_devices WHERE owner_id = auth.uid()
            )
        )
    );

-- RLS Policies for file_shares
CREATE POLICY "Users can manage their own shares" ON file_shares
    FOR ALL USING (
        device_id IN (
            SELECT id FROM remote_devices WHERE owner_id = auth.uid()
        )
    );

CREATE POLICY "Public shares are viewable by everyone" ON file_shares
    FOR SELECT USING (share_type = 'public' AND expires_at > NOW());

-- RLS Policies for file_transfer_logs
CREATE POLICY "Users can view logs for their transfers" ON file_transfer_logs
    FOR SELECT USING (
        transfer_id IN (
            SELECT id FROM file_transfers WHERE 
            source_device_id IN (
                SELECT id FROM remote_devices WHERE owner_id = auth.uid()
            ) OR 
            target_device_id IN (
                SELECT id FROM remote_devices WHERE owner_id = auth.uid()
            )
        )
    );

-- Enable realtime for file transfer tables
ALTER PUBLICATION supabase_realtime ADD TABLE file_transfers;
ALTER PUBLICATION supabase_realtime ADD TABLE file_transfer_chunks;
ALTER PUBLICATION supabase_realtime ADD TABLE file_shares;

-- Functions for cleanup and maintenance
CREATE OR REPLACE FUNCTION cleanup_expired_transfers()
RETURNS void AS $$
BEGIN
    -- Delete expired transfers and their chunks
    DELETE FROM file_transfers WHERE expires_at < NOW();
    
    -- Delete expired shares
    DELETE FROM file_shares WHERE expires_at < NOW();
    
    -- Clean up orphaned chunks
    DELETE FROM file_transfer_chunks 
    WHERE transfer_id NOT IN (SELECT id FROM file_transfers);
    
    -- Clean up old logs (keep last 30 days)
    DELETE FROM file_transfer_logs 
    WHERE created_at < NOW() - INTERVAL '30 days';
END;
$$ LANGUAGE plpgsql;

-- Function to update transfer progress
CREATE OR REPLACE FUNCTION update_transfer_progress(
    transfer_session_id TEXT,
    new_progress INTEGER,
    new_status TEXT DEFAULT NULL
)
RETURNS void AS $$
BEGIN
    UPDATE file_transfers 
    SET 
        progress = new_progress,
        status = COALESCE(new_status, status),
        updated_at = NOW()
    WHERE id = transfer_session_id;
    
    -- Log the progress update
    INSERT INTO file_transfer_logs (transfer_id, event_type, event_data)
    VALUES (
        transfer_session_id, 
        'progress_update', 
        json_build_object('progress', new_progress, 'status', COALESCE(new_status, 'active'))
    );
END;
$$ LANGUAGE plpgsql;

-- Create storage bucket for file transfers
INSERT INTO storage.buckets (id, name, public, file_size_limit, allowed_mime_types)
VALUES (
    'file-transfers',
    'file-transfers',
    false,
    1073741824, -- 1GB limit per file
    ARRAY['application/octet-stream', 'text/plain', 'image/*', 'video/*', 'audio/*', 'application/*']
) ON CONFLICT (id) DO NOTHING;

-- Storage policies for file-transfers bucket
CREATE POLICY "Users can upload to their transfer sessions" ON storage.objects
    FOR INSERT WITH CHECK (
        bucket_id = 'file-transfers' AND
        (storage.foldername(name))[1] IN (
            SELECT id FROM file_transfers WHERE 
            source_device_id IN (
                SELECT id FROM remote_devices WHERE owner_id = auth.uid()
            )
        )
    );

CREATE POLICY "Users can download from their transfer sessions" ON storage.objects
    FOR SELECT USING (
        bucket_id = 'file-transfers' AND
        (storage.foldername(name))[1] IN (
            SELECT id FROM file_transfers WHERE 
            source_device_id IN (
                SELECT id FROM remote_devices WHERE owner_id = auth.uid()
            ) OR 
            target_device_id IN (
                SELECT id FROM remote_devices WHERE owner_id = auth.uid()
            )
        )
    );

CREATE POLICY "Users can delete their transfer files" ON storage.objects
    FOR DELETE USING (
        bucket_id = 'file-transfers' AND
        (storage.foldername(name))[1] IN (
            SELECT id FROM file_transfers WHERE 
            source_device_id IN (
                SELECT id FROM remote_devices WHERE owner_id = auth.uid()
            )
        )
    );

-- Schedule cleanup function to run daily
-- Note: This requires pg_cron extension to be enabled
-- SELECT cron.schedule('cleanup-expired-transfers', '0 2 * * *', 'SELECT cleanup_expired_transfers();');

COMMENT ON TABLE file_transfers IS 'File transfer sessions between devices';
COMMENT ON TABLE file_transfer_chunks IS 'Individual chunks for large file transfers';
COMMENT ON TABLE file_shares IS 'Public file sharing with access controls';
COMMENT ON TABLE file_transfer_logs IS 'Audit trail for file transfer activities';
