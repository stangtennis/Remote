-- Published support knowledge for the AI support MCP interface.
-- Keep client secrets, credentials, tokens, and raw command output out of this table.

CREATE TABLE IF NOT EXISTS public.support_knowledge (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  title TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
  summary TEXT NOT NULL DEFAULT '' CHECK (char_length(summary) <= 500),
  content TEXT NOT NULL CHECK (char_length(content) BETWEEN 1 AND 20000),
  category TEXT NOT NULL DEFAULT 'general' CHECK (category IN ('general', 'installation', 'network', 'windows', 'webrtc', 'security', 'runbook')),
  tags TEXT[] NOT NULL DEFAULT '{}',
  source TEXT,
  is_published BOOLEAN NOT NULL DEFAULT false,
  created_by UUID NOT NULL DEFAULT auth.uid() REFERENCES auth.users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS support_knowledge_published_idx
  ON public.support_knowledge(is_published, updated_at DESC);

CREATE OR REPLACE FUNCTION public.touch_support_knowledge_updated_at()
RETURNS TRIGGER
LANGUAGE plpgsql
SET search_path = public, pg_temp
AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS support_knowledge_updated_at ON public.support_knowledge;
CREATE TRIGGER support_knowledge_updated_at
  BEFORE UPDATE ON public.support_knowledge
  FOR EACH ROW EXECUTE FUNCTION public.touch_support_knowledge_updated_at();

ALTER TABLE public.support_knowledge ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS "Authenticated can read published support knowledge" ON public.support_knowledge;
CREATE POLICY "Authenticated can read published support knowledge"
  ON public.support_knowledge FOR SELECT TO authenticated
  USING (is_published = true OR public.is_admin());

DROP POLICY IF EXISTS "Admins can create support knowledge" ON public.support_knowledge;
CREATE POLICY "Admins can create support knowledge"
  ON public.support_knowledge FOR INSERT TO authenticated
  WITH CHECK (public.is_admin() AND created_by = auth.uid());

DROP POLICY IF EXISTS "Admins can update support knowledge" ON public.support_knowledge;
CREATE POLICY "Admins can update support knowledge"
  ON public.support_knowledge FOR UPDATE TO authenticated
  USING (public.is_admin())
  WITH CHECK (public.is_admin());

DROP POLICY IF EXISTS "Admins can delete support knowledge" ON public.support_knowledge;
CREATE POLICY "Admins can delete support knowledge"
  ON public.support_knowledge FOR DELETE TO authenticated
  USING (public.is_admin());

COMMENT ON TABLE public.support_knowledge IS
  'Published support runbooks and troubleshooting knowledge. Never store secrets, credentials, tokens, or raw command output.';
