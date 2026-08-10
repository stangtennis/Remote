-- ============================================================================
-- device_tags: drop legacy permissive policies — 2026-08-12
--
-- The live DB carried legacy permissive policies (device_tags_select / _insert /
-- _delete) with USING (true) / WITH CHECK (true), created under different names
-- than the 20260308 originals. Because RLS OR's policies together, they kept the
-- cross-tenant hole open even after 20260807 added scoped replacements. Drop them
-- and add a scoped delete policy.
-- ============================================================================

DROP POLICY IF EXISTS device_tags_select ON public.device_tags;
DROP POLICY IF EXISTS device_tags_insert ON public.device_tags;
DROP POLICY IF EXISTS device_tags_delete ON public.device_tags;

-- Only the tag creator (or an admin) may delete their own tags.
CREATE POLICY "Users can delete own tags"
ON public.device_tags
FOR DELETE TO authenticated
USING (created_by = auth.uid() OR public.is_admin());
