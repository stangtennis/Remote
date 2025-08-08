-- Create RLS policies to allow uploads to the agents bucket
-- This fixes the "signature verification failed" error
-- Note: RLS is already enabled on storage.objects in hosted Supabase

-- Create bucket first (if it doesn't exist)
INSERT INTO storage.buckets (id, name, public, file_size_limit, allowed_mime_types)
VALUES ('agents', 'agents', true, 52428800, '{"application/octet-stream","application/x-msdownload","application/x-executable"}')
ON CONFLICT (id) DO NOTHING;

-- Create policy to allow INSERT operations on the agents bucket
-- This allows uploads to the agents bucket for authenticated and anon users
CREATE POLICY "Allow uploads to agents bucket" 
ON storage.objects 
FOR INSERT 
TO public 
WITH CHECK (bucket_id = 'agents');

-- Create policy to allow public SELECT operations on the agents bucket
-- This allows public downloads from the agents bucket
CREATE POLICY "Allow public downloads from agents bucket" 
ON storage.objects 
FOR SELECT 
TO public 
USING (bucket_id = 'agents');

-- Create policy to allow UPDATE operations on the agents bucket (for upsert)
-- This allows overwriting files in the agents bucket
CREATE POLICY "Allow updates to agents bucket" 
ON storage.objects 
FOR UPDATE 
TO public 
USING (bucket_id = 'agents') 
WITH CHECK (bucket_id = 'agents');

-- Create policy to allow DELETE operations on the agents bucket
-- This allows deleting files from the agents bucket
CREATE POLICY "Allow deletes from agents bucket" 
ON storage.objects 
FOR DELETE 
TO public 
USING (bucket_id = 'agents');
