# Supabase Migrations

## Migration Squash Assessment (2026-03-14)

36 migration files, total ~2500 lines. Analysis:

**Squash candidates (contradictory/superseded migrations):**
- RLS toggle chain: `20250102000004_simplify_rls_policies` -> `000005_disable_rls_devices`
  -> `000006_disable_all_rls` -> `20260108_reenable_rls_devices` -> `20260214_security_hardening`
  -> `20260217_tighten_rls`. Six files that iteratively disable then re-enable RLS.
- Device visibility fixes: `20250102000000_fix_device_visibility` -> `000001_fix_device_approval`
  -> `000002_allow_device_self_check` -> `000003_fix_heartbeat_policy` (4 small fix-on-fix files).
- Agent access: `20250108000000_enable_security` -> `20250108000001_fix_agent_access`
  -> `20250112000000_fix_web_agent_policies` (immediate fix chain).

**Recommendation: DO NOT squash in production.**
All 36 migrations have already been applied to the live Supabase database.
Squashing would break `supabase db push` (Supabase tracks applied migrations by filename).
If a fresh database setup is ever needed, consider creating a single consolidated
`initial_schema.sql` for that purpose while keeping existing files intact.

---

# Apply Device Assignment Migration

## 🚀 Quick Apply

### Using Supabase CLI (Recommended)

```bash
# Navigate to project root
cd f:\#Remote

# Apply migration
supabase db push

# Or apply specific migration
supabase migration up
```

### Using Supabase Dashboard

1. Go to **SQL Editor** in Supabase Dashboard
2. Copy contents of `supabase/migrations/20251104_device_assignments.sql`
3. Paste and run

### Manual SQL

```bash
# Connect to your database
psql "postgresql://postgres:[PASSWORD]@db.[PROJECT].supabase.co:5432/postgres"

# Run migration
\i supabase/migrations/20251104_device_assignments.sql
```

---

## ✅ Verify Migration

After applying, verify with these queries:

```sql
-- Check new columns exist
SELECT column_name, data_type 
FROM information_schema.columns 
WHERE table_name = 'remote_devices' 
AND column_name IN ('approved', 'assigned_by', 'assigned_at');

-- Check device_assignments table exists
SELECT COUNT(*) FROM device_assignments;

-- Test get_user_devices function
SELECT * FROM get_user_devices('your-user-id');

-- Test get_unassigned_devices function (as admin)
SELECT * FROM get_unassigned_devices();
```

---

## 🔄 Rollback (if needed)

```sql
-- Drop new table
DROP TABLE IF EXISTS device_assignments CASCADE;

-- Remove new columns
ALTER TABLE remote_devices
DROP COLUMN IF EXISTS approved,
DROP COLUMN IF EXISTS assigned_by,
DROP COLUMN IF EXISTS assigned_at;

-- Drop functions
DROP FUNCTION IF EXISTS get_user_devices(TEXT);
DROP FUNCTION IF EXISTS get_unassigned_devices();
DROP FUNCTION IF EXISTS assign_device(TEXT, TEXT, BOOLEAN, TEXT);
DROP FUNCTION IF EXISTS revoke_device_assignment(TEXT, TEXT);
```

---

## 📊 What This Migration Does

1. ✅ Adds `approved`, `assigned_by`, `assigned_at` to `remote_devices`
2. ✅ Creates `device_assignments` table
3. ✅ Creates indexes for performance
4. ✅ Creates 4 functions for device management
5. ✅ Updates RLS policies
6. ✅ Migrates existing devices
7. ✅ Allows anonymous device registration

---

## 🎯 Next Steps After Migration

1. Update agent to remove login requirement
2. Update admin panel to show device management
3. Update controller to use `get_user_devices()`
4. Test the new workflow
