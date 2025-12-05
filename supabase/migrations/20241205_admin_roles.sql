-- Admin Roles and Invitations System
-- Super Admin: hansemand@gmail.com - full control
-- Admins: Can manage own devices + invite users
-- Users: Can only manage own devices

-- Add role column to user_approvals
ALTER TABLE user_approvals 
ADD COLUMN IF NOT EXISTS role TEXT DEFAULT 'user' CHECK (role IN ('super_admin', 'admin', 'user'));

-- Set hansemand@gmail.com as super_admin
UPDATE user_approvals 
SET role = 'super_admin' 
WHERE user_id IN (
  SELECT id FROM auth.users WHERE email = 'hansemand@gmail.com'
);

-- Create invitations table
CREATE TABLE IF NOT EXISTS user_invitations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email TEXT NOT NULL,
  invited_by UUID REFERENCES auth.users(id),
  role TEXT DEFAULT 'user' CHECK (role IN ('admin', 'user')),
  token TEXT UNIQUE DEFAULT encode(gen_random_bytes(32), 'hex'),
  expires_at TIMESTAMPTZ DEFAULT NOW() + INTERVAL '7 days',
  accepted_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Create device transfers table (for moving devices between admins)
CREATE TABLE IF NOT EXISTS device_transfers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  device_id TEXT NOT NULL REFERENCES remote_devices(device_id),
  from_user_id UUID REFERENCES auth.users(id),
  to_user_id UUID REFERENCES auth.users(id),
  transferred_by UUID REFERENCES auth.users(id),
  transferred_at TIMESTAMPTZ DEFAULT NOW(),
  reason TEXT
);

-- Function to check if user is super admin
CREATE OR REPLACE FUNCTION is_super_admin(user_id UUID)
RETURNS BOOLEAN AS $$
BEGIN
  RETURN EXISTS (
    SELECT 1 FROM user_approvals 
    WHERE user_approvals.user_id = $1 AND role = 'super_admin'
  );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Function to check if user is admin or super admin
CREATE OR REPLACE FUNCTION is_admin(user_id UUID)
RETURNS BOOLEAN AS $$
BEGIN
  RETURN EXISTS (
    SELECT 1 FROM user_approvals 
    WHERE user_approvals.user_id = $1 AND role IN ('super_admin', 'admin')
  );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Function to transfer device to another user (super admin only)
CREATE OR REPLACE FUNCTION transfer_device(
  p_device_id TEXT,
  p_to_user_id UUID,
  p_reason TEXT DEFAULT NULL
)
RETURNS BOOLEAN AS $$
DECLARE
  v_from_user_id UUID;
BEGIN
  -- Check if caller is super admin
  IF NOT is_super_admin(auth.uid()) THEN
    RAISE EXCEPTION 'Only super admins can transfer devices';
  END IF;

  -- Get current owner
  SELECT owner_id INTO v_from_user_id 
  FROM remote_devices 
  WHERE device_id = p_device_id;

  -- Update device owner
  UPDATE remote_devices 
  SET owner_id = p_to_user_id 
  WHERE device_id = p_device_id;

  -- Log transfer
  INSERT INTO device_transfers (device_id, from_user_id, to_user_id, transferred_by, reason)
  VALUES (p_device_id, v_from_user_id, p_to_user_id, auth.uid(), p_reason);

  RETURN TRUE;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Function to send invitation
CREATE OR REPLACE FUNCTION send_invitation(
  p_email TEXT,
  p_role TEXT DEFAULT 'user'
)
RETURNS TEXT AS $$
DECLARE
  v_token TEXT;
BEGIN
  -- Check if caller is admin
  IF NOT is_admin(auth.uid()) THEN
    RAISE EXCEPTION 'Only admins can send invitations';
  END IF;

  -- Only super admins can invite admins
  IF p_role = 'admin' AND NOT is_super_admin(auth.uid()) THEN
    RAISE EXCEPTION 'Only super admins can invite admins';
  END IF;

  -- Create invitation
  INSERT INTO user_invitations (email, invited_by, role)
  VALUES (p_email, auth.uid(), p_role)
  RETURNING token INTO v_token;

  RETURN v_token;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Function to accept invitation
CREATE OR REPLACE FUNCTION accept_invitation(p_token TEXT)
RETURNS BOOLEAN AS $$
DECLARE
  v_invitation RECORD;
BEGIN
  -- Find valid invitation
  SELECT * INTO v_invitation
  FROM user_invitations
  WHERE token = p_token
    AND expires_at > NOW()
    AND accepted_at IS NULL;

  IF NOT FOUND THEN
    RAISE EXCEPTION 'Invalid or expired invitation';
  END IF;

  -- Mark as accepted
  UPDATE user_invitations
  SET accepted_at = NOW()
  WHERE token = p_token;

  -- Auto-approve user with role
  INSERT INTO user_approvals (user_id, approved, role, approved_by)
  VALUES (auth.uid(), true, v_invitation.role, v_invitation.invited_by)
  ON CONFLICT (user_id) DO UPDATE
  SET approved = true, role = v_invitation.role;

  RETURN TRUE;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- RLS policies for invitations
ALTER TABLE user_invitations ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Admins can view invitations they sent"
  ON user_invitations FOR SELECT
  USING (invited_by = auth.uid() OR is_super_admin(auth.uid()));

CREATE POLICY "Admins can create invitations"
  ON user_invitations FOR INSERT
  WITH CHECK (is_admin(auth.uid()));

-- RLS policies for device transfers
ALTER TABLE device_transfers ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Super admins can view all transfers"
  ON device_transfers FOR SELECT
  USING (is_super_admin(auth.uid()));

CREATE POLICY "Super admins can create transfers"
  ON device_transfers FOR INSERT
  WITH CHECK (is_super_admin(auth.uid()));

-- Update remote_devices policy to allow super admin to see all
DROP POLICY IF EXISTS "Users can view own devices" ON remote_devices;
CREATE POLICY "Users can view own devices or super admin sees all"
  ON remote_devices FOR SELECT
  USING (owner_id = auth.uid() OR is_super_admin(auth.uid()));

-- Grant permissions
GRANT EXECUTE ON FUNCTION is_super_admin(UUID) TO authenticated;
GRANT EXECUTE ON FUNCTION is_admin(UUID) TO authenticated;
GRANT EXECUTE ON FUNCTION transfer_device(TEXT, UUID, TEXT) TO authenticated;
GRANT EXECUTE ON FUNCTION send_invitation(TEXT, TEXT) TO authenticated;
GRANT EXECUTE ON FUNCTION accept_invitation(TEXT) TO authenticated;
