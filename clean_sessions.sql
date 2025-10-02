-- Clean all old sessions before testing
-- Run this in Supabase SQL Editor before testing

-- 1. Delete all signaling messages
DELETE FROM session_signaling;

-- 2. Delete all pending/active sessions
DELETE FROM remote_sessions WHERE status IN ('pending', 'active');

-- 3. Mark all devices as offline (they'll come back online when agent starts)
UPDATE remote_devices SET is_online = false;

-- 4. Verify cleanup
SELECT 'Signaling messages' as table_name, COUNT(*) as count FROM session_signaling
UNION ALL
SELECT 'Pending/Active sessions', COUNT(*) FROM remote_sessions WHERE status IN ('pending', 'active')
UNION ALL
SELECT 'Online devices', COUNT(*) FROM remote_devices WHERE is_online = true;
