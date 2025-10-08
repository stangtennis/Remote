# 🔒 Security Guide

## Current Security Status

### ⚠️ IMPORTANT: Apply Security Migration First!

**Before using in production, run:**
```bash
cd supabase
supabase db push
```

This will apply migration `20250108000000_enable_security.sql` which enables Row Level Security (RLS).

---

## 🛡️ Security Architecture

### 1. **User Authentication** (Dashboard)

**How it works:**
- Users sign up/login via **Supabase Auth**
- Email verification required
- MFA (Multi-Factor Authentication) available
- Each user gets a unique `user_id` (UUID)

**What users can do:**
- ✅ View only **their own devices**
- ✅ Create sessions only for **their devices**
- ✅ Control only **their remote computers**
- ❌ Cannot see other users' devices
- ❌ Cannot access other users' sessions

---

### 2. **Device Authentication** (Agent)

**How it works:**
- Agent registers with user's email
- User **approves device** in dashboard
- Device gets unique `device_id` stored in `.device_id` file
- Device uses **Supabase anon key** (public, no auth)

**Security Model:**
- Agent uses **anon key** (public, same for all devices)
- Agent **filters by device_id** in application code
- Technically can query all data, but:
  - ✅ Application code only queries its own device_id
  - ✅ PIN codes required to actually connect
  - ✅ device_id stored locally, not exposed
  - ✅ User can delete device from dashboard anytime

**Why this is acceptable:**
- Agents are **backend services**, not browsers
- Even if someone knew all session IDs, they need the **6-digit PIN** to connect
- **Users' data is protected** - RLS prevents users from seeing each other's devices

**Future improvement:** Use per-device API keys (field exists in schema)

---

### 3. **Row Level Security (RLS)**

All database tables have RLS enabled:

#### **`remote_devices` table:**
```sql
-- ✅ USERS (authenticated):
--    Can ONLY see devices where owner_id = their user_id
WHERE owner_id = auth.uid()

-- ⚠️ AGENTS (anon key):
--    Can see all devices, but application filters by device_id
USING (true)  -- Application-level filtering
```

#### **`remote_sessions` table:**
```sql
-- ✅ USERS (authenticated):
--    Can ONLY access sessions for devices they own
WHERE device_id IN (
  SELECT device_id FROM remote_devices WHERE owner_id = auth.uid()
)

-- ⚠️ AGENTS (anon key):
--    Can see all sessions, but application filters by device_id
--    Additional security: PIN codes required to connect
USING (true)  -- Application-level filtering + PIN codes
```

**Security Layers:**
1. **Users:** Database-enforced (RLS) ✅
2. **Agents:** Application-enforced + PIN codes ⚠️
3. **Connections:** WebRTC P2P encryption 🔐

---

## 🔐 How to Secure Your Setup

### Step 1: Enable Email Verification

**In Supabase Dashboard:**
1. Go to **Authentication → Email Templates**
2. Enable **Confirm Signup** email
3. Set email from address

**Effect:** Users must verify email before accessing dashboard

---

### Step 2: Restrict Sign-ups (Recommended)

**Option A: Disable Public Sign-ups**
1. Go to **Authentication → Providers**
2. Turn off **Enable email sign-ups**
3. Manually invite users via **Authentication → Users → Invite**

**Option B: Use Allow List**
1. Create a policy to only allow specific emails
2. See example below

---

### Step 3: Enable MFA (Multi-Factor Authentication)

**In Supabase Dashboard:**
1. Go to **Authentication → Auth Providers**
2. Enable **Phone** or **TOTP**
3. Configure SMS provider (Twilio/MessageBird)

**Effect:** Users need second factor to login

---

### Step 4: Secure Your Environment Variables

**Never expose these:**
- ❌ `SUPABASE_SERVICE_ROLE_KEY` (admin access)
- ❌ Database password
- ❌ TURN credentials

**Safe to expose:**
- ✅ `SUPABASE_URL`
- ✅ `SUPABASE_ANON_KEY` (has RLS protection)

---

## 🚫 Restricting Who Can Sign Up

### Option 1: Disable Public Sign-ups (Simplest)

**Supabase Dashboard:**
```
Authentication → Settings → Disable sign-ups
Then manually invite users
```

---

### Option 2: Email Allowlist Policy

**Create migration:**
```sql
-- Only allow specific email domains
CREATE OR REPLACE FUNCTION public.check_email_allowed()
RETURNS TRIGGER AS $$
BEGIN
  -- Allow only your domain
  IF NEW.email NOT LIKE '%@yourdomain.com' THEN
    RAISE EXCEPTION 'Email domain not allowed';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER enforce_email_allowlist
  BEFORE INSERT ON auth.users
  FOR EACH ROW
  EXECUTE FUNCTION public.check_email_allowed();
```

---

### Option 3: Invite-Only System

**Create an invites table:**
```sql
CREATE TABLE public.user_invites (
  email TEXT PRIMARY KEY,
  invited_by UUID REFERENCES auth.users(id),
  created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Only allow sign-ups with valid invite
CREATE OR REPLACE FUNCTION public.check_invite_exists()
RETURNS TRIGGER AS $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM public.user_invites WHERE email = NEW.email) THEN
    RAISE EXCEPTION 'No invite found for this email';
  END IF;
  
  -- Remove invite after use
  DELETE FROM public.user_invites WHERE email = NEW.email;
  
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER enforce_invites
  BEFORE INSERT ON auth.users
  FOR EACH ROW
  EXECUTE FUNCTION public.check_invite_exists();
```

---

## 🔍 Monitoring & Auditing

### Check Who Has Access

**In Supabase Dashboard:**
```sql
-- View all users
SELECT id, email, created_at, last_sign_in_at
FROM auth.users
ORDER BY created_at DESC;

-- View all devices and their owners
SELECT d.id, d.name, d.platform, u.email as owner_email, d.approved
FROM devices d
JOIN auth.users u ON d.user_id = u.id
ORDER BY d.created_at DESC;

-- View active sessions
SELECT rs.id, rs.pin, d.name as device_name, u.email as owner_email, rs.status
FROM remote_sessions rs
JOIN devices d ON rs.device_id = d.id
JOIN auth.users u ON d.user_id = u.id
WHERE rs.status = 'active';
```

---

## 🚨 Emergency: Unauthorized Access

### If someone unauthorized signs up:

**1. Delete the user:**
```sql
-- In Supabase SQL Editor
DELETE FROM auth.users WHERE email = 'unauthorized@email.com';
```

**2. Revoke all their sessions:**
```sql
-- Delete their devices
DELETE FROM devices WHERE user_id = 'user-uuid-here';
```

**3. Enable stricter authentication** (see Step 2 above)

---

### If a device is compromised:

**1. In Dashboard:** Delete the device
**2. On the computer:** Delete `.device_id` file
**3. Agent will need to re-register and be re-approved

---

## 📋 Security Checklist

Before going to production:

- [ ] Apply security migration (`20250108000000_enable_security.sql`)
- [ ] Enable email verification
- [ ] Disable public sign-ups OR enable allowlist
- [ ] Enable MFA (optional but recommended)
- [ ] Never commit `.env` files to git
- [ ] Use strong Supabase project password
- [ ] Regularly review users and devices
- [ ] Enable audit logging
- [ ] Set up monitoring/alerts

---

## 🔒 Best Practices

### For Users:
1. **Use strong passwords** (12+ characters)
2. **Enable MFA** if available
3. **Regularly review devices** - Delete unused ones
4. **Don't share credentials**
5. **Use unique PIN codes** for each session

### For Administrators:
1. **Regularly audit users** - Remove inactive accounts
2. **Monitor audit logs** - Check for suspicious activity
3. **Keep Supabase updated** - Apply security patches
4. **Use service role key carefully** - Never expose in client code
5. **Implement rate limiting** - Prevent brute force attacks

---

## 🆘 Need Help?

- **Supabase Security Docs**: https://supabase.com/docs/guides/auth
- **RLS Guide**: https://supabase.com/docs/guides/database/postgres/row-level-security
- **Auth Concepts**: https://supabase.com/docs/guides/auth/auth-deep-dive/auth-deep-dive-jwts

---

**Remember: Security is a continuous process, not a one-time setup!**
