# Release v1.1.7

## 🎉 What's New

### 🔐 **User Approval System**
- **Admin controls who can register** - Only approved users can access the dashboard
- **Admin panel** at `/admin.html` to approve pending users
- **Database-level security** with Row Level Security policies
- **Automatic approval records** for new sign-ups
- See `USER_APPROVAL_GUIDE.md` for full documentation

### 🪟 **Enhanced Tray Menu**
- **"Show Console Window"** - Opens PowerShell with live log output
- **"View Log File"** - Opens agent.log in Notepad (fixed!)
- Real-time monitoring from system tray

### 🎮 **Input Control Fixes**
- **Fixed double-clicking** - Removed duplicate event listeners
- **Fixed arrow key double movement** - Added key repeat protection
- **Mouse throttling** - Reduced to 60 FPS for better performance
- **Scroll event fixed** - Changed from `mouse_wheel` to `mouse_scroll`

### 🐛 **Console/Debug Mode**
- **`run-with-console.bat`** - Double-click to see live logs
- **`build-debug.bat`** - Build version with console window
- **`CONSOLE_MODE.md`** - Full documentation

## 📦 Installation

### **Download the Agent:**
1. Download `remote-agent.exe` from this release
2. Copy to a folder on your PC (e.g., `C:\RemoteAgent\`)
3. Double-click to run
4. Enter your email when prompted
5. Wait for admin approval

### **For Admins:**
1. Apply the migration: `supabase/migrations/20250109000000_user_approval_system.sql`
2. Approve yourself via SQL (see `USER_APPROVAL_GUIDE.md`)
3. Login and access admin panel at `/admin.html`

## 🔄 Upgrade from v1.1.6

### **Dashboard:**
- Just reload the page - changes are live
- User approval system is active
- Admin panel available

### **Agent:**
1. Stop the old agent (right-click tray icon → Exit)
2. Replace `remote-agent.exe` with new version
3. Run the new version
4. New tray menu with console options!

## 📋 Full Changelog

### Added
- User approval system with `user_approvals` table
- Admin panel UI (`docs/admin.html`)
- Console mode scripts (`run-with-console.bat`, `build-debug.bat`)
- Enhanced tray menu with console and log viewers
- Key repeat protection for keyboard input
- Mouse movement throttling (60 FPS)
- Duplicate event listener prevention
- Input capture cleanup function
- Complete documentation (`USER_APPROVAL_GUIDE.md`, `CONSOLE_MODE.md`)

### Fixed
- Double-clicking mouse issue (duplicate event listeners)
- Arrow keys moving double (key repeat events)
- Scroll event type (`mouse_wheel` → `mouse_scroll`)
- "View Logs" tray menu item (now uses Notepad)
- Input state tracking for reliable control

### Changed
- RLS policies now check user approval status
- Tray menu reorganized with new options
- Version display updated to v1.1.7

### Security
- Only approved users can access dashboard
- Only approved users can register devices
- Only approved users can create sessions
- Database-level enforcement via RLS
- Admin controls all user access

## 🎯 Migration Guide

If you're upgrading from v1.1.6, follow these steps:

### 1. **Apply Database Migration**
```sql
-- In Supabase SQL Editor
-- Run: supabase/migrations/20250109000000_user_approval_system.sql
```

### 2. **Create Approval Records for Existing Users**
```sql
INSERT INTO public.user_approvals (user_id, email, approved, requested_at)
SELECT id, email, false, created_at
FROM auth.users
ON CONFLICT (user_id) DO NOTHING;
```

### 3. **Approve Yourself**
```sql
UPDATE public.user_approvals
SET approved = true, approved_at = now()
WHERE email = 'your-admin-email@example.com';
```

### 4. **Update Agent**
- Replace `remote-agent.exe` with new version
- Restart the agent

## 📚 Documentation

- **USER_APPROVAL_GUIDE.md** - Complete guide to user approval system
- **CONSOLE_MODE.md** - How to use debug/console mode
- **README.md** - General documentation

## ⚠️ Breaking Changes

**User approval is now required!** 

After upgrading:
- Existing users need approval to access dashboard
- New sign-ups require admin approval
- Make sure to approve yourself first (see Migration Guide)

## 🐛 Known Issues

None at this time. Report issues at: https://github.com/stangtennis/Remote/issues

## 🙏 Credits

Built with:
- Go 1.21+
- Supabase (Database, Auth, Realtime)
- WebRTC (Peer connections)
- Robotgo (Input control)
- Systray (System tray integration)

---

**Full Diff:** https://github.com/stangtennis/Remote/compare/v1.1.6...v1.1.7
