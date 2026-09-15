-- Non-secret Cloudflare service-token issuance metadata for AI-support enrollment.

ALTER TABLE public.device_enrollment_tokens
  ADD COLUMN IF NOT EXISTS cloudflare_service_token_id TEXT;

ALTER TABLE public.device_enrollment_tokens
  DROP CONSTRAINT IF EXISTS device_enrollment_tokens_cloudflare_service_token_id_check;

ALTER TABLE public.device_enrollment_tokens
  ADD CONSTRAINT device_enrollment_tokens_cloudflare_service_token_id_check
  CHECK (
    cloudflare_service_token_id IS NULL
    OR cloudflare_service_token_id ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  );

COMMENT ON COLUMN public.device_enrollment_tokens.cloudflare_service_token_id IS
  'Non-secret Cloudflare service-token issuance metadata for AI-support enrollment; the service-token secret is never stored.';
