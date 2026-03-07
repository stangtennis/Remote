-- Public Support Link: settings table + is_public column on support_sessions

-- 1. Create support_settings table (single-row config)
CREATE TABLE IF NOT EXISTS support_settings (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  public_link_enabled BOOLEAN NOT NULL DEFAULT false,
  updated_by UUID REFERENCES auth.users(id),
  updated_at TIMESTAMPTZ DEFAULT now()
);

-- Start with one row
INSERT INTO support_settings (public_link_enabled) VALUES (false);

-- RLS
ALTER TABLE support_settings ENABLE ROW LEVEL SECURITY;

-- Anyone can read (anon checks if public link is enabled)
CREATE POLICY "Anyone can read support_settings"
  ON support_settings FOR SELECT USING (true);

-- Only admin can update
CREATE POLICY "Admin can update support_settings"
  ON support_settings FOR UPDATE USING (
    auth.uid() IN (
      SELECT user_id FROM user_approvals
      WHERE role IN ('admin', 'super_admin')
    )
  );

-- Enable realtime on support_settings
ALTER PUBLICATION supabase_realtime ADD TABLE support_settings;

-- 2. Add is_public column to support_sessions
ALTER TABLE support_sessions
  ADD COLUMN IF NOT EXISTS is_public BOOLEAN DEFAULT false;

-- Make created_by nullable (public sessions have no creator)
ALTER TABLE support_sessions
  ALTER COLUMN created_by DROP NOT NULL;

-- Allow anon to insert public support sessions (via service role in edge function)
-- The edge function uses service_role key which bypasses RLS, so no anon policy needed.
-- But we need anon to read/write signaling for public sessions:
CREATE POLICY "Anon can read public support signaling"
  ON session_signaling FOR SELECT TO anon
  USING (
    session_id IN (SELECT id FROM support_sessions WHERE is_public = true)
  );

CREATE POLICY "Anon can insert public support signaling"
  ON session_signaling FOR INSERT TO anon
  WITH CHECK (
    session_id IN (SELECT id FROM support_sessions WHERE is_public = true)
  );
