-- Remote Desktop Application Database Schema
-- This file contains all the SQL needed to set up the Supabase database

-- Enable necessary extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Users table for authentication and user management
CREATE TABLE users (
  id UUID DEFAULT uuid_generate_v4() PRIMARY KEY,
  email TEXT UNIQUE NOT NULL,
  username TEXT UNIQUE NOT NULL,
  full_name TEXT,
  avatar_url TEXT,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  last_login TIMESTAMP WITH TIME ZONE,
  is_active BOOLEAN DEFAULT true
);

-- Remote computers/devices that can be accessed
CREATE TABLE remote_devices (
  id UUID DEFAULT uuid_generate_v4() PRIMARY KEY,
  owner_id UUID REFERENCES users(id) ON DELETE CASCADE,
  device_name TEXT NOT NULL,
  device_type TEXT DEFAULT 'desktop', -- desktop, laptop, server, etc.
  operating_system TEXT,
  ip_address INET,
  port INTEGER DEFAULT 3389,
  is_online BOOLEAN DEFAULT false,
  last_seen TIMESTAMP WITH TIME ZONE,
  access_key TEXT UNIQUE NOT NULL, -- Unique key for device access
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Remote desktop sessions
CREATE TABLE remote_sessions (
  id UUID DEFAULT uuid_generate_v4() PRIMARY KEY,
  user_id UUID REFERENCES users(id) ON DELETE CASCADE,
  device_id UUID REFERENCES remote_devices(id) ON DELETE CASCADE,
  session_token TEXT UNIQUE NOT NULL,
  status TEXT DEFAULT 'connecting', -- connecting, active, disconnected, failed
  started_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  ended_at TIMESTAMP WITH TIME ZONE,
  duration_seconds INTEGER,
  connection_quality TEXT DEFAULT 'good', -- excellent, good, fair, poor
  screen_resolution TEXT,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Connection logs for monitoring and debugging
CREATE TABLE connection_logs (
  id UUID DEFAULT uuid_generate_v4() PRIMARY KEY,
  session_id UUID REFERENCES remote_sessions(id) ON DELETE CASCADE,
  log_level TEXT DEFAULT 'info', -- error, warn, info, debug
  message TEXT NOT NULL,
  metadata JSONB,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Device permissions (who can access which devices)
CREATE TABLE device_permissions (
  id UUID DEFAULT uuid_generate_v4() PRIMARY KEY,
  device_id UUID REFERENCES remote_devices(id) ON DELETE CASCADE,
  user_id UUID REFERENCES users(id) ON DELETE CASCADE,
  permission_level TEXT DEFAULT 'view', -- view, control, admin
  granted_by UUID REFERENCES users(id),
  granted_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  expires_at TIMESTAMP WITH TIME ZONE,
  is_active BOOLEAN DEFAULT true,
  UNIQUE(device_id, user_id)
);

-- Create indexes for better performance
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_remote_devices_owner ON remote_devices(owner_id);
CREATE INDEX idx_remote_devices_access_key ON remote_devices(access_key);
CREATE INDEX idx_remote_sessions_user ON remote_sessions(user_id);
CREATE INDEX idx_remote_sessions_device ON remote_sessions(device_id);
CREATE INDEX idx_remote_sessions_status ON remote_sessions(status);
CREATE INDEX idx_connection_logs_session ON connection_logs(session_id);
CREATE INDEX idx_device_permissions_device ON device_permissions(device_id);
CREATE INDEX idx_device_permissions_user ON device_permissions(user_id);

-- Create updated_at trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Add updated_at triggers
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_remote_devices_updated_at BEFORE UPDATE ON remote_devices
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Row Level Security (RLS) policies
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE remote_devices ENABLE ROW LEVEL SECURITY;
ALTER TABLE remote_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE connection_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE device_permissions ENABLE ROW LEVEL SECURITY;

-- Users can only see their own profile
CREATE POLICY "Users can view own profile" ON users
    FOR SELECT USING (auth.uid() = id);

CREATE POLICY "Users can update own profile" ON users
    FOR UPDATE USING (auth.uid() = id);

-- Device owners can manage their devices
CREATE POLICY "Users can view own devices" ON remote_devices
    FOR SELECT USING (auth.uid() = owner_id);

CREATE POLICY "Users can insert own devices" ON remote_devices
    FOR INSERT WITH CHECK (auth.uid() = owner_id);

CREATE POLICY "Users can update own devices" ON remote_devices
    FOR UPDATE USING (auth.uid() = owner_id);

CREATE POLICY "Users can delete own devices" ON remote_devices
    FOR DELETE USING (auth.uid() = owner_id);

-- Users can view devices they have permission to access
CREATE POLICY "Users can view permitted devices" ON remote_devices
    FOR SELECT USING (
        EXISTS (
            SELECT 1 FROM device_permissions 
            WHERE device_id = remote_devices.id 
            AND user_id = auth.uid() 
            AND is_active = true
        )
    );

-- Session policies
CREATE POLICY "Users can view own sessions" ON remote_sessions
    FOR SELECT USING (auth.uid() = user_id);

CREATE POLICY "Users can insert own sessions" ON remote_sessions
    FOR INSERT WITH CHECK (auth.uid() = user_id);

-- Insert some sample data
INSERT INTO users (email, username, full_name) VALUES
    ('admin@example.com', 'admin', 'System Administrator'),
    ('user1@example.com', 'user1', 'Test User 1'),
    ('user2@example.com', 'user2', 'Test User 2');

-- Note: In a real application, you would set up Supabase Auth instead of manually inserting users
