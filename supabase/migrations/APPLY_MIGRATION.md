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
