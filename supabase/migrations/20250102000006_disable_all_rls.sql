-- Temporarily disable RLS on all tables to diagnose stack depth issue

ALTER TABLE public.remote_sessions DISABLE ROW LEVEL SECURITY;
ALTER TABLE public.session_signaling DISABLE ROW LEVEL SECURITY;
ALTER TABLE public.audit_logs DISABLE ROW LEVEL SECURITY;

COMMENT ON TABLE public.remote_sessions 
IS 'RLS temporarily disabled for debugging. Re-enable after testing.';

COMMENT ON TABLE public.session_signaling 
IS 'RLS temporarily disabled for debugging. Re-enable after testing.';

COMMENT ON TABLE public.audit_logs 
IS 'RLS temporarily disabled for debugging. Re-enable after testing.';
