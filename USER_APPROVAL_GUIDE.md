# User Approval System

The Remote Desktop system now requires **administrator approval** for new users. This prevents unauthorized access and gives you control over who can use your system.

---

## 🔐 How It Works

### **1. New User Signs Up**
- User creates account at `https://stangtennis.github.io/Remote/`
- Account is created but **not approved** yet
- User cannot access dashboard until approved

### **2. Admin Approves User**
- Admin logs into `https://stangtennis.github.io/Remote/admin.html`
- Sees list of pending users
- Clicks "Approve User" to grant access

### **3. User Can Access Dashboard**
- Once approved, user can login normally
- User can register devices and create sessions
- Full access to the system

---

## 👤 Setting Up the First Admin

When you first deploy the system, **no users are approved yet**, including yourself! Here's how to approve the first admin user:

### **Option 1: Auto-Approve Yourself (Recommended for First Setup)**

Run this SQL in Supabase SQL Editor:

```sql
-- Approve yourself as the first admin
UPDATE public.user_approvals
SET 
  approved = true,
  approved_at = now(),
  notes = 'First admin - auto-approved'
WHERE email = 'your-email@example.com';
```

Replace `your-email@example.com` with your actual email address.

###

 **Option 2: Auto-Approve ALL Existing Users**

If you already have existing users and want to approve them all:

```sql
-- Auto-approve all existing users
UPDATE public.user_approvals
SET 
  approved = true,
  approved_at = now(),
  notes = 'Auto-approved - existing user'
WHERE approved = false;
```

⚠️ **Warning:** This approves EVERYONE who has signed up!

### **Option 3: Manually Approve via SQL**

You can approve specific users by their email:

```sql
-- Approve specific user
UPDATE public.user_approvals
SET 
  approved = true,
  approved_at = now(),
  notes = 'Manually approved by admin'
WHERE email = 'user@example.com';
```

---

## 📋 Using the Admin Panel

### **Access the Admin Panel:**

1. **Login** to the dashboard normally
2. Click **"🔐 User Approvals"** button in the header
3. You'll see the admin panel

### **Admin Panel Features:**

**📊 Statistics:**
- **Total Users:** All registered users
- **Pending Approval:** Users waiting for approval
- **Approved Users:** Users with access

**🔍 Filters:**
- **All Users:** See everyone
- **Pending:** Only users awaiting approval
- **Approved:** Only approved users

**✅ Approve Users:**
- Each pending user shows:
  - Email address
  - Registration date
  - Status (Pending/Approved)
- Click **"Approve User"** to grant access
- User can immediately login and use the system

---

## 🔄 User Flow with Approval System

### **New User Experience:**

```
1. User signs up
   ↓
2. Email verification (Supabase default)
   ↓
3. User tries to login
   ↓
4. System shows: "⏸️ Your account is pending approval"
   ↓
5. User waits for admin approval
   ↓
6. Admin approves user
   ↓
7. User can now login and use system ✅
```

### **Admin Experience:**

```
1. New user signs up
   ↓
2. Admin sees notification (optional - set up Realtime)
   ↓
3. Admin opens Admin Panel
   ↓
4. Admin sees pending user
   ↓
5. Admin reviews user details
   ↓
6. Admin clicks "Approve User"
   ↓
7. User is approved ✅
```

---

## 🛠️ Technical Details

### **Database Table: `user_approvals`**

```sql
CREATE TABLE public.user_approvals (
  id bigint PRIMARY KEY,
  user_id uuid REFERENCES auth.users(id),
  email text,
  approved boolean DEFAULT false,
  approved_by uuid REFERENCES auth.users(id),
  approved_at timestamptz,
  requested_at timestamptz DEFAULT now(),
  notes text
);
```

### **Automatic Record Creation:**

A trigger automatically creates an approval record when a user signs up:

```sql
-- Trigger: on_auth_user_created
-- Creates user_approvals record with approved = false
```

### **RLS Policies Updated:**

All policies now check `is_user_approved()` function:

```sql
-- Example: Users can only view devices if approved
CREATE POLICY "Users can view own devices"
ON public.remote_devices
FOR SELECT
TO authenticated
USING (
  auth.uid() = owner_id 
  AND public.is_user_approved(auth.uid())
);
```

---

## 📡 API Functions

### **Check if User is Approved:**

```javascript
// JavaScript example
const { data, error } = await supabase
  .from('user_approvals')
  .select('approved')
  .eq('user_id', userId)
  .single();

const isApproved = data?.approved || false;
```

### **Approve a User (Admin only):**

```javascript
// Using the approve_user function
const { data, error } = await supabase.rpc('approve_user', {
  target_user_id: userId,
  approval_notes: 'Approved by admin'
});
```

---

## 🔧 Configuration Options

### **Disable Approval System (Not Recommended):**

If you want to disable the approval system and auto-approve everyone:

```sql
-- Auto-approve all new users by default
ALTER TABLE public.user_approvals 
ALTER COLUMN approved SET DEFAULT true;

-- Update existing users
UPDATE public.user_approvals 
SET approved = true 
WHERE approved = false;
```

### **Enable Email Notifications (Optional):**

You can set up email notifications when new users sign up:

1. Create a Supabase Edge Function
2. Listen to `user_approvals` table inserts
3. Send email to admin
4. Admin gets notified of pending approvals

---

## 🎯 Best Practices

### **1. Approve Promptly:**
- Check admin panel regularly
- Don't keep users waiting too long
- Consider email notifications for new sign-ups

### **2. Review Before Approving:**
- Check email domain
- Verify it's someone you know/expect
- Add notes for why you approved them

### **3. Multiple Admins:**
- Any approved user can access admin panel
- Consider creating specific "admin" role later
- For now, all users can approve others

### **4. Monitor Access:**
- Regularly review approved users
- Remove access if needed (delete from `user_approvals`)
- Check audit logs for suspicious activity

---

## ❓ FAQ

**Q: I signed up but can't login. What's wrong?**  
A: Your account needs approval. Contact the administrator to approve your account.

**Q: How do I know if my account is approved?**  
A: Try logging in. If approved, you'll see the dashboard. If not, you'll see "pending approval" message.

**Q: Can I approve myself?**  
A: Yes, for the first admin. Run the SQL query in Option 1 above.

**Q: Who can approve users?**  
A: Currently, any logged-in user can access the admin panel. Consider adding role-based access later.

**Q: What happens to agents from unapproved users?**  
A: Agents can register, but they won't be visible in the dashboard until the user is approved.

**Q: Can I bulk approve users?**  
A: Yes, use SQL to update multiple users at once.

---

## 🚀 Quick Start Checklist

- [ ] Deploy the migration: `20250109000000_user_approval_system.sql`
- [ ] Sign up for your admin account
- [ ] Run SQL to approve yourself (Option 1 above)
- [ ] Login and access the dashboard
- [ ] Test the admin panel at `/admin.html`
- [ ] Create test user account
- [ ] Approve test user from admin panel
- [ ] Verify test user can login
- [ ] ✅ System is ready!

---

**Your Remote Desktop system now has user approval enabled!** 🎉

Only users you approve can access the dashboard and register devices.
