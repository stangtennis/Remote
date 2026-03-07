-- Device tags (many-to-many)
CREATE TABLE IF NOT EXISTS public.device_tags (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  device_id text NOT NULL REFERENCES public.remote_devices(device_id) ON DELETE CASCADE,
  tag text NOT NULL,
  created_by uuid REFERENCES auth.users(id),
  created_at timestamptz DEFAULT now(),
  UNIQUE(device_id, tag)
);

CREATE INDEX idx_device_tags_device ON public.device_tags(device_id);
CREATE INDEX idx_device_tags_tag ON public.device_tags(tag);

ALTER TABLE public.device_tags ENABLE ROW LEVEL SECURITY;

-- All authenticated users can read tags
CREATE POLICY "Authenticated users can read tags" ON public.device_tags
  FOR SELECT USING (auth.uid() IS NOT NULL);

-- Users can manage tags they created; admins can manage all
CREATE POLICY "Users can insert tags" ON public.device_tags
  FOR INSERT WITH CHECK (auth.uid() IS NOT NULL);

CREATE POLICY "Users can delete own tags" ON public.device_tags
  FOR DELETE USING (created_by = auth.uid());

-- User device favorites (per user)
CREATE TABLE IF NOT EXISTS public.user_device_favorites (
  user_id uuid NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  device_id text NOT NULL REFERENCES public.remote_devices(device_id) ON DELETE CASCADE,
  created_at timestamptz DEFAULT now(),
  PRIMARY KEY (user_id, device_id)
);

ALTER TABLE public.user_device_favorites ENABLE ROW LEVEL SECURITY;

-- Users can only see and manage their own favorites
CREATE POLICY "Users manage own favorites" ON public.user_device_favorites
  FOR ALL USING (user_id = auth.uid());

COMMENT ON TABLE public.device_tags IS 'Tags for organizing and filtering devices';
COMMENT ON TABLE public.user_device_favorites IS 'Per-user device favorites for quick access';
