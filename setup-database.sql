-- Simplified Database Setup for Remote Desktop Application
-- Run this in your Supabase SQL Editor

-- Enable necessary extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Users table for authentication and user management
CREATE TABLE IF NOT EXISTS users (
  id UUID DEFAULT uuid_generate_v4() PRIMARY KEY,
  email TEXT UNIQUE NOT NULL,
  username TEXT UNIQUE NOT NULL,
  full_name TEXT,
  password_hash TEXT NOT NULL,
  avatar_url TEXT,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  last_login TIMESTAMP WITH TIME ZONE,
  is_active BOOLEAN DEFAULT true
);

-- Remote computers/devices that can be accessed
CREATE TABLE IF NOT EXISTS remote_devices (
  id UUID DEFAULT uuid_generate_v4() PRIMARY KEY,
  owner_id UUID REFERENCES users(id) ON DELETE CASCADE,
  device_name TEXT NOT NULL,
  device_type TEXT DEFAULT 'desktop',
  operating_system TEXT,
  ip_address INET,
  port INTEGER DEFAULT 3389,
  is_online BOOLEAN DEFAULT false,
  last_seen TIMESTAMP WITH TIME ZONE,
  access_key TEXT UNIQUE NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Remote desktop sessions
CREATE TABLE IF NOT EXISTS remote_sessions (
  id UUID DEFAULT uuid_generate_v4() PRIMARY KEY,
  user_id UUID REFERENCES users(id) ON DELETE CASCADE,
  device_id UUID REFERENCES remote_devices(id) ON DELETE CASCADE,
  session_token TEXT UNIQUE NOT NULL,
  status TEXT DEFAULT 'connecting',
  started_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  ended_at TIMESTAMP WITH TIME ZONE,
  duration_seconds INTEGER,
  connection_quality TEXT DEFAULT 'good',
  screen_resolution TEXT,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Connection logs for monitoring and debugging
CREATE TABLE IF NOT EXISTS connection_logs (
  id UUID DEFAULT uuid_generate_v4() PRIMARY KEY,
  session_id UUID REFERENCES remote_sessions(id) ON DELETE CASCADE,
  log_level TEXT DEFAULT 'info',
  message TEXT NOT NULL,
  metadata JSONB,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Device permissions (who can access which devices)
CREATE TABLE IF NOT EXISTS device_permissions (
  id UUID DEFAULT uuid_generate_v4() PRIMARY KEY,
  device_id UUID REFERENCES remote_devices(id) ON DELETE CASCADE,
  user_id UUID REFERENCES users(id) ON DELETE CASCADE,
  permission_level TEXT DEFAULT 'view',
  granted_by UUID REFERENCES users(id),
  granted_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  expires_at TIMESTAMP WITH TIME ZONE,
  is_active BOOLEAN DEFAULT true,
  UNIQUE(device_id, user_id)
);

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_remote_devices_owner ON remote_devices(owner_id);
CREATE INDEX IF NOT EXISTS idx_remote_devices_access_key ON remote_devices(access_key);
CREATE INDEX IF NOT EXISTS idx_remote_sessions_user ON remote_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_remote_sessions_device ON remote_sessions(device_id);
CREATE INDEX IF NOT EXISTS idx_remote_sessions_status ON remote_sessions(status);
CREATE INDEX IF NOT EXISTS idx_connection_logs_session ON connection_logs(session_id);
CREATE INDEX IF NOT EXISTS idx_device_permissions_device ON device_permissions(device_id);
CREATE INDEX IF NOT EXISTS idx_device_permissions_user ON device_permissions(user_id);

-- Insert some sample data for testing
INSERT INTO users (email, username, full_name, password_hash) VALUES
    ('admin@example.com', 'admin', 'System Administrator', '$2b$10$example.hash.here'),
    ('user1@example.com', 'user1', 'Test User 1', '$2b$10$example.hash.here'),
    ('user2@example.com', 'user2', 'Test User 2', '$2b$10$example.hash.here')
ON CONFLICT (email) DO NOTHING;
