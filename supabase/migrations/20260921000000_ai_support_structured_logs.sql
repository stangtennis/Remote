-- Replace raw persistent SSH command logs with bounded activity events.
-- This migration also sanitizes historical AI-support command payloads.

UPDATE public.audit_logs
SET event = 'AI_SUPPORT_OPERATION',
    details = jsonb_build_object(
      'schema_version', 1,
      'mode', 'command',
      'operation', 'legacy_support_activity',
      'result', 'unknown',
      'compatibility', 'legacy'
    )
WHERE event = 'AI_SUPPORT_COMMAND'
  AND device_id IN (SELECT client_id FROM public.ai_support_clients);

CREATE OR REPLACE FUNCTION public.append_ai_support_log(
  p_client_id TEXT,
  p_activity_log_token_hash TEXT,
  p_event TEXT,
  p_details JSONB
)
RETURNS VOID
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
DECLARE
  v_client public.ai_support_clients%ROWTYPE;
  v_key TEXT;
  v_operation TEXT;
  v_mode TEXT;
  v_result TEXT;
  v_compatibility TEXT;
BEGIN
  SELECT * INTO v_client
  FROM public.ai_support_clients
  WHERE client_id = p_client_id
  FOR UPDATE;

  IF NOT FOUND OR v_client.status <> 'ready' OR v_client.activity_log_token_hash IS DISTINCT FROM p_activity_log_token_hash THEN
    RAISE EXCEPTION 'Invalid AI-support log credentials';
  END IF;
  IF p_event <> 'AI_SUPPORT_OPERATION' OR jsonb_typeof(p_details) <> 'object' THEN
    RAISE EXCEPTION 'Invalid AI-support log event';
  END IF;
  IF p_details ? 'command' THEN
    RAISE EXCEPTION 'Raw AI-support commands are not accepted';
  END IF;
  IF NOT (p_details ?& ARRAY['schema_version', 'mode', 'operation', 'result', 'compatibility']) THEN
    RAISE EXCEPTION 'Incomplete AI-support log event';
  END IF;
  FOR v_key IN SELECT jsonb_object_keys(p_details) LOOP
    IF v_key NOT IN ('schema_version', 'mode', 'operation', 'result', 'exit_code', 'duration_ms', 'compatibility') THEN
      RAISE EXCEPTION 'Unknown AI-support log field';
    END IF;
  END LOOP;

  IF (p_details->>'schema_version')::INTEGER <> 1 THEN
    RAISE EXCEPTION 'Unsupported AI-support log schema';
  END IF;
  v_mode := p_details->>'mode';
  v_operation := p_details->>'operation';
  v_result := p_details->>'result';
  v_compatibility := p_details->>'compatibility';
  IF v_mode NOT IN ('interactive', 'command') OR
     v_operation NOT IN ('interactive_shell', 'system_diagnostics', 'network_diagnostics', 'file_inspection', 'file_change', 'service_change', 'process_change', 'scheduled_task', 'account_change', 'remote_access', 'other_powershell', 'legacy_support_activity') OR
     v_result NOT IN ('success', 'failure', 'unknown') OR
     v_compatibility NOT IN ('structured', 'legacy') THEN
    RAISE EXCEPTION 'Invalid AI-support log values';
  END IF;
  IF p_details ? 'exit_code' AND p_details->'exit_code' <> 'null'::JSONB AND
     (jsonb_typeof(p_details->'exit_code') <> 'number' OR (p_details->>'exit_code')::INTEGER < 0 OR (p_details->>'exit_code')::INTEGER > 255) THEN
    RAISE EXCEPTION 'Invalid AI-support exit code';
  END IF;
  IF p_details ? 'duration_ms' AND p_details->'duration_ms' <> 'null'::JSONB AND
     (jsonb_typeof(p_details->'duration_ms') <> 'number' OR (p_details->>'duration_ms')::BIGINT < 0 OR (p_details->>'duration_ms')::BIGINT > 2147483647) THEN
    RAISE EXCEPTION 'Invalid AI-support duration';
  END IF;

  INSERT INTO public.audit_logs(device_id, actor, event, details, severity)
  VALUES (v_client.client_id, v_client.owner_id, p_event, p_details, CASE WHEN v_result = 'failure' THEN 'warning' ELSE 'info' END);
  UPDATE public.ai_support_clients SET last_seen = now(), updated_at = now() WHERE id = v_client.id;
END;
$$;

REVOKE ALL ON FUNCTION public.append_ai_support_log(TEXT, TEXT, TEXT, JSONB) FROM PUBLIC, authenticated, anon;
GRANT EXECUTE ON FUNCTION public.append_ai_support_log(TEXT, TEXT, TEXT, JSONB) TO service_role;
