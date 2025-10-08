-- User Approval System
-- Only approved users can access the dashboard and register devices

-- ===========================================
-- 1. Create user_approvals table
-- ===========================================

CREATE TABLE IF NOT EXISTS public.user_approvals (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  email text NOT NULL,
  approved boolean DEFAULT false,
  approved_by uuid REFERENCES auth.users(id),
  approved_at timestamptz,
  requested_at timestamptz DEFAULT now(),
  notes text,
  UNIQUE(user_id)
);

-- Index for faster lookups
CREATE INDEX IF NOT EXISTS idx_user_approvals_user_id ON public.user_approvals(user_id);
CREATE INDEX IF NOT EXISTS idx_user_approvals_approved ON public.user_approvals(approved);

-- ===========================================
-- 2. Enable RLS on user_approvals
-- ===========================================

ALTER TABLE public.user_approvals ENABLE ROW LEVEL SECURITY;

-- Users can view their own approval status
DROP POLICY IF EXISTS "Users can view own approval status" ON public.user_approvals;
CREATE POLICY "Users can view own approval status"
ON public.user_approvals
FOR SELECT
TO authenticated
USING (auth.uid() = user_id);

-- ===========================================
-- 3. Create function to check if user is approved
-- ===========================================

CREATE OR REPLACE FUNCTION public.is_user_approved(check_user_id uuid)
RETURNS boolean
LANGUAGE sql
SECURITY DEFINER
STABLE
AS $$
  SELECT COALESCE(
    (SELECT approved FROM public.user_approvals WHERE user_id = check_user_id),
    false
  );
$$;

-- ===========================================
-- 4. Update existing policies to check approval
-- ===========================================

-- REMOTE_DEVICES: Only approved users can view devices
DROP POLICY IF EXISTS "Users can view own devices" ON public.remote_devices;
CREATE POLICY "Users can view own devices"
ON public.remote_devices
FOR SELECT
TO authenticated
USING (auth.uid() = owner_id AND public.is_user_approved(auth.uid()));

-- REMOTE_DEVICES: Only approved users can insert devices
DROP POLICY IF EXISTS "Users can insert own devices" ON public.remote_devices;
CREATE POLICY "Users can insert own devices"
ON public.remote_devices
FOR INSERT
TO authenticated
WITH CHECK (auth.uid() = owner_id AND public.is_user_approved(auth.uid()));

-- REMOTE_DEVICES: Only approved users can update devices
DROP POLICY IF EXISTS "Users can update own devices" ON public.remote_devices;
CREATE POLICY "Users can update own devices"
ON public.remote_devices
FOR UPDATE
TO authenticated
USING (auth.uid() = owner_id AND public.is_user_approved(auth.uid()))
WITH CHECK (auth.uid() = owner_id AND public.is_user_approved(auth.uid()));

-- REMOTE_DEVICES: Only approved users can delete devices
DROP POLICY IF EXISTS "Users can delete own devices" ON public.remote_devices;
CREATE POLICY "Users can delete own devices"
ON public.remote_devices
FOR DELETE
TO authenticated
USING (auth.uid() = owner_id AND public.is_user_approved(auth.uid()));

-- REMOTE_SESSIONS: Only approved users can view sessions
DROP POLICY IF EXISTS "Users can view own sessions" ON public.remote_sessions;
CREATE POLICY "Users can view own sessions"
ON public.remote_sessions
FOR SELECT
TO authenticated
USING (
  device_id IN (
    SELECT device_id FROM public.remote_devices WHERE owner_id = auth.uid()
  ) AND public.is_user_approved(auth.uid())
);

-- REMOTE_SESSIONS: Only approved users can insert sessions
DROP POLICY IF EXISTS "Users can insert own sessions" ON public.remote_sessions;
CREATE POLICY "Users can insert own sessions"
ON public.remote_sessions
FOR INSERT
TO authenticated
WITH CHECK (
  device_id IN (
    SELECT device_id FROM public.remote_devices WHERE owner_id = auth.uid()
  ) AND public.is_user_approved(auth.uid())
);

-- REMOTE_SESSIONS: Only approved users can update sessions
DROP POLICY IF EXISTS "Users can update own sessions" ON public.remote_sessions;
CREATE POLICY "Users can update own sessions"
ON public.remote_sessions
FOR UPDATE
TO authenticated
USING (
  device_id IN (
    SELECT device_id FROM public.remote_devices WHERE owner_id = auth.uid()
  ) AND public.is_user_approved(auth.uid())
)
WITH CHECK (
  device_id IN (
    SELECT device_id FROM public.remote_devices WHERE owner_id = auth.uid()
  ) AND public.is_user_approved(auth.uid())
);

-- ===========================================
-- 5. Create trigger to auto-create approval records
-- ===========================================

CREATE OR REPLACE FUNCTION public.handle_new_user()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
BEGIN
  INSERT INTO public.user_approvals (user_id, email, approved)
  VALUES (NEW.id, NEW.email, false)
  ON CONFLICT (user_id) DO NOTHING;
  
  RETURN NEW;
END;
$$;

-- Trigger on auth.users (requires admin to set this up)
-- Note: This trigger is created in auth schema which may require superuser
-- Alternative: Create approval record manually or via Edge Function

DROP TRIGGER IF EXISTS on_auth_user_created ON auth.users;
CREATE TRIGGER on_auth_user_created
  AFTER INSERT ON auth.users
  FOR EACH ROW
  EXECUTE FUNCTION public.handle_new_user();

-- ===========================================
-- 6. Create helper function to approve users
-- ===========================================

CREATE OR REPLACE FUNCTION public.approve_user(
  target_user_id uuid,
  approval_notes text DEFAULT NULL
)
RETURNS boolean
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
BEGIN
  -- Update approval status
  UPDATE public.user_approvals
  SET 
    approved = true,
    approved_by = auth.uid(),
    approved_at = now(),
    notes = approval_notes
  WHERE user_id = target_user_id;
  
  -- Return success
  RETURN FOUND;
END;
$$;

-- ===========================================
-- 7. Comments
-- ===========================================

COMMENT ON TABLE public.user_approvals IS 'Tracks user approval status. Only approved users can access the system.';
COMMENT ON FUNCTION public.is_user_approved IS 'Returns true if user is approved, false otherwise.';
COMMENT ON FUNCTION public.approve_user IS 'Approves a user. Can only be called by authenticated users (typically admins).';

-- ===========================================
-- 8. Insert approval for existing users (OPTIONAL)
-- ===========================================

-- Auto-approve all existing users (comment out if you want to manually approve)
-- INSERT INTO public.user_approvals (user_id, email, approved, approved_at)
-- SELECT id, email, true, now()
-- FROM auth.users
-- ON CONFLICT (user_id) DO UPDATE SET approved = true, approved_at = now();
