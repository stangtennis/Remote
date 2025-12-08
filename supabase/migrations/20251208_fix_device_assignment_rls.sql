-- Migration: Fix device_assignments RLS policy
-- Date: 2025-12-08
-- Description: Allow admins and super_admins to manage device assignments

-- Drop existing restrictive policy
DROP POLICY IF EXISTS "Admins can manage device assignments" ON device_assignments;

-- Create new policy that allows both admin and super_admin roles
CREATE POLICY "Admins can manage device assignments"
ON device_assignments FOR ALL
TO authenticated
USING (
    EXISTS (
        SELECT 1 FROM user_approvals
        WHERE user_id::uuid = auth.uid()
        AND role IN ('admin', 'super_admin')
    )
)
WITH CHECK (
    EXISTS (
        SELECT 1 FROM user_approvals
        WHERE user_id::uuid = auth.uid()
        AND role IN ('admin', 'super_admin')
    )
);

-- Also allow anon to insert (for device self-registration flow)
DROP POLICY IF EXISTS "Allow device assignment insert" ON device_assignments;
CREATE POLICY "Allow device assignment insert"
ON device_assignments FOR INSERT
TO anon, authenticated
WITH CHECK (true);

-- Verify the policies
SELECT schemaname, tablename, policyname, permissive, roles, cmd, qual 
FROM pg_policies 
WHERE tablename = 'device_assignments';
