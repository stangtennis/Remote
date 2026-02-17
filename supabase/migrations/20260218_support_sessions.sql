-- Quick Support: Support Sessions table
-- Allows admin to generate a link for screen sharing via browser

-- 1. Create support_sessions table
CREATE TABLE IF NOT EXISTS support_sessions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  created_by UUID NOT NULL REFERENCES auth.users(id),
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'ended', 'expired')),
  pin TEXT NOT NULL,
  token UUID NOT NULL DEFAULT gen_random_uuid(),
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  ended_at TIMESTAMPTZ
);

-- 2. Index on token for fast lookups
CREATE UNIQUE INDEX IF NOT EXISTS support_sessions_token_idx ON support_sessions(token);

-- 3. RLS: Only authenticated users (admins) can create/view sessions
ALTER TABLE support_sessions ENABLE ROW LEVEL SECURITY;

-- Admins can create support sessions
CREATE POLICY "Admins can insert support sessions"
  ON support_sessions FOR INSERT
  TO authenticated
  WITH CHECK (
    EXISTS (
      SELECT 1 FROM user_approvals
      WHERE user_id = auth.uid()
      AND role IN ('admin', 'super_admin')
    )
  );

-- Admins can view their own support sessions
CREATE POLICY "Admins can view support sessions"
  ON support_sessions FOR SELECT
  TO authenticated
  USING (created_by = auth.uid());

-- Admins can update their own support sessions
CREATE POLICY "Admins can update support sessions"
  ON support_sessions FOR UPDATE
  TO authenticated
  USING (created_by = auth.uid());

-- Service role can do anything (for Edge Functions and cleanup)
-- (service_role bypasses RLS by default, no policy needed)

-- 4. Drop FK on session_signaling so support_sessions can also use it
ALTER TABLE session_signaling
  DROP CONSTRAINT IF EXISTS session_signaling_session_id_fkey;

-- 5. Add RLS policies for support session signaling
CREATE POLICY "Users can read support signaling" ON session_signaling
  FOR SELECT TO authenticated
  USING (session_id IN (SELECT id FROM support_sessions WHERE created_by = auth.uid()));

CREATE POLICY "Users can insert support signaling" ON session_signaling
  FOR INSERT TO authenticated
  WITH CHECK (session_id IN (SELECT id FROM support_sessions WHERE created_by = auth.uid()));

-- 6. Add 'support' to session_signaling from_side constraint
ALTER TABLE session_signaling
  DROP CONSTRAINT IF EXISTS session_signaling_from_side_check;

ALTER TABLE session_signaling
  ADD CONSTRAINT session_signaling_from_side_check
  CHECK (from_side IN ('dashboard', 'agent', 'controller', 'system', 'support'));

-- 5. Enable Realtime on support_sessions
ALTER PUBLICATION supabase_realtime ADD TABLE support_sessions;
