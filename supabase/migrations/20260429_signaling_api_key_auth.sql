-- v3.0.9: Allow agents to read/write signaling tables via x-device-key.
--
-- v3.0.4 made heartbeat work without JWT (api_key path), but signaling
-- still failed: session_signaling RLS required auth.uid()/session_token,
-- and remote_sessions had only JWT-based SELECT. Result: when JWT
-- expired the agent stayed online (heartbeat fine) but couldn't pick up
-- WebRTC offers from the dashboard. User had to physically restore
-- credentials every time the refresh token rotated out.
--
-- This migration extends api_key-based authorization to:
--   - public.session_signaling (SELECT + INSERT for own device's sessions)
--   - public.remote_sessions (SELECT for own device)
--
-- After this, an agent can sit offline for arbitrarily long, come back,
-- and the dashboard can reconnect without any token gymnastics.

-- ─── session_signaling ────────────────────────────────────────────────
-- Agent reads incoming signals (offer + ICE from dashboard)
CREATE POLICY "Device reads signaling via api_key" ON public.session_signaling
  FOR SELECT USING (
    EXISTS (
      SELECT 1 FROM public.remote_devices d
      WHERE d.api_key = current_setting('request.headers', true)::json->>'x-device-key'
        AND (
          d.device_id IN (
            SELECT w.device_id FROM public.webrtc_sessions w
            WHERE (w.session_id)::uuid = session_signaling.session_id
          )
          OR d.device_id IN (
            SELECT r.device_id FROM public.remote_sessions r
            WHERE r.id = session_signaling.session_id
          )
        )
    )
  );

-- Agent writes outgoing signals (answer + ICE)
CREATE POLICY "Device writes signaling via api_key" ON public.session_signaling
  FOR INSERT WITH CHECK (
    EXISTS (
      SELECT 1 FROM public.remote_devices d
      WHERE d.api_key = current_setting('request.headers', true)::json->>'x-device-key'
        AND (
          d.device_id IN (
            SELECT w.device_id FROM public.webrtc_sessions w
            WHERE (w.session_id)::uuid = session_signaling.session_id
          )
          OR d.device_id IN (
            SELECT r.device_id FROM public.remote_sessions r
            WHERE r.id = session_signaling.session_id
          )
        )
    )
  );

-- ─── remote_sessions ──────────────────────────────────────────────────
-- Agent reads its own session rows (used to map session_id → device_id
-- when handling dashboard offers).
CREATE POLICY "Device reads own remote_sessions via api_key" ON public.remote_sessions
  FOR SELECT USING (
    device_id IN (
      SELECT d.device_id FROM public.remote_devices d
      WHERE d.api_key = current_setting('request.headers', true)::json->>'x-device-key'
    )
  );
