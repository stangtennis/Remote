-- Storage policies for Remote Desktop Application
-- NOTE: Create buckets manually via Supabase Dashboard first:
--   1. agents (public, 50MB limit)
--   2. file-transfers (private, 100MB limit)

-- Drop existing policies if they exist
DROP POLICY IF EXISTS "Public can download agents" ON storage.objects;
DROP POLICY IF EXISTS "Service role can upload agents" ON storage.objects;
DROP POLICY IF EXISTS "Users can access own session files" ON storage.objects;
DROP POLICY IF EXISTS "Users can upload to own sessions" ON storage.objects;
DROP POLICY IF EXISTS "Users can delete own session files" ON storage.objects;

-- Storage policies for agents bucket
CREATE POLICY "Public can download agents"
ON storage.objects FOR SELECT
USING (bucket_id = 'agents');

CREATE POLICY "Service role can upload agents"
ON storage.objects FOR INSERT
WITH CHECK (
  bucket_id = 'agents' 
  AND (auth.jwt()->>'role')::text = 'service_role'
);

-- Storage policies for file-transfers bucket
CREATE POLICY "Users can access own session files"
ON storage.objects FOR SELECT
USING (
  bucket_id = 'file-transfers'
  AND auth.uid()::text = (storage.foldername(name))[1]
);

CREATE POLICY "Users can upload to own sessions"
ON storage.objects FOR INSERT
WITH CHECK (
  bucket_id = 'file-transfers'
  AND EXISTS (
    SELECT 1 FROM public.remote_sessions
    WHERE id::text = (storage.foldername(name))[1]
    AND created_by = auth.uid()
  )
);

CREATE POLICY "Users can delete own session files"
ON storage.objects FOR DELETE
USING (
  bucket_id = 'file-transfers'
  AND EXISTS (
    SELECT 1 FROM public.remote_sessions
    WHERE id::text = (storage.foldername(name))[1]
    AND created_by = auth.uid()
  )
);
