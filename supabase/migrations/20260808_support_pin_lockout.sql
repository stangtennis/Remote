-- ============================================================================
-- Support PIN brute-force lockout — 2026-08-08
-- The support-signal edge function 'validate' action accepted a 6-digit PIN
-- with no rate limiting, making the ~1M space brute-forceable. This adds an
-- IP-keyed attempt log that the edge function checks before each PIN lookup.
-- ============================================================================

CREATE TABLE IF NOT EXISTS public.support_pin_attempts (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  source text NOT NULL,
  created_at timestamptz DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_support_pin_attempts_source_time
ON public.support_pin_attempts(source, created_at);

-- Only the service_role (edge functions) should touch this table.
ALTER TABLE public.support_pin_attempts ENABLE ROW LEVEL SECURITY;
REVOKE ALL ON public.support_pin_attempts FROM anon, authenticated, PUBLIC;

-- Cleanup function (avoids nested-quote issues inside cron.schedule).
CREATE OR REPLACE FUNCTION public.cleanup_pin_attempts()
RETURNS void AS $$
BEGIN
  DELETE FROM public.support_pin_attempts
  WHERE created_at < now() - interval '1 hour';
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Schedule hourly cleanup (guarded — no-op if pg_cron absent).
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_cron') THEN
    PERFORM cron.unschedule('support-pin-attempts-cleanup');
    PERFORM cron.schedule(
      'support-pin-attempts-cleanup',
      '0 * * * *',
      'SELECT public.cleanup_pin_attempts()'
    );
  END IF;
EXCEPTION WHEN OTHERS THEN NULL;
END $$;
