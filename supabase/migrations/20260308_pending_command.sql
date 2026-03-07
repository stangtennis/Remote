-- Add pending_command column for out-of-band commands (e.g. force_update without WebRTC)
ALTER TABLE public.remote_devices ADD COLUMN IF NOT EXISTS pending_command text DEFAULT NULL;

-- Allow device owner to update pending_command (dashboard sets it, agent clears it)
-- RLS already allows owner to UPDATE their own devices, so no new policy needed.
