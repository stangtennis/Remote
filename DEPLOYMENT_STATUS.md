# Deployment Status

## ✅ Completed

### Database Schema
- ✅ Tables created: `remote_devices`, `remote_sessions`, `session_signaling`, `audit_logs`
- ✅ Indexes created for performance
- ✅ RLS policies enabled
- ✅ Triggers configured (session expiration, signaling cleanup)
- ✅ Audit logging function created

### Edge Functions
- ✅ `session-token` deployed
- ✅ `device-register` deployed  
- ✅ `file-transfer` deployed

View functions: https://supabase.com/dashboard/project/mnqtdugcvfyenjuqruol/functions

---

## ⏳ Pending (Manual Steps)

### 1. Create Storage Buckets
**Go to:** https://supabase.com/dashboard/project/mnqtdugcvfyenjuqruol/storage

**Create 2 buckets:**

#### Bucket 1: `agents`
- **Name:** `agents`
- **Public:** ✅ Yes (public bucket)
- **File size limit:** 52428800 (50 MB)
- **Allowed MIME types:** 
  - `application/x-msdownload`
  - `application/octet-stream`
  - `application/json`

#### Bucket 2: `file-transfers`
- **Name:** `file-transfers`
- **Public:** ❌ No (private)
- **File size limit:** 104857600 (100 MB)
- **Allowed MIME types:** Leave empty (all types)

**Note:** Storage policies are already deployed and will automatically apply once buckets are created.

---

### 2. Set Edge Function Secrets
**Go to:** https://supabase.com/dashboard/project/mnqtdugcvfyenjuqruol/settings/functions

**Add these secrets:**
- `SUPABASE_URL` = https://mnqtdugcvfyenjuqruol.supabase.co
- `SUPABASE_ANON_KEY` = REDACTED_JWT
- `SUPABASE_SERVICE_ROLE_KEY` = REDACTED_JWT

**(Optional - for TURN later):**
- `TURN_PROVIDER` = twilio
- `TWILIO_ACCOUNT_SID` = (your Twilio SID)
- `TWILIO_AUTH_TOKEN` = (your Twilio token)

---

## 🎯 Next Steps

After completing the pending manual steps above:

1. **Test Database** - Verify tables in SQL Editor
2. **Test Edge Functions** - Use the Functions panel to invoke test requests
3. **Build Dashboard** (Fase 1) - Create the web UI for device management
4. **Build Agent** (Fase 2) - Create the Go application for Windows

---

## Database Tables

You can view your tables here:
https://supabase.com/dashboard/project/mnqtdugcvfyenjuqruol/editor

### Schema Overview:
- `remote_devices` - Registered agent devices
- `remote_sessions` - Active/historical sessions
- `session_signaling` - WebRTC signaling messages
- `audit_logs` - Security audit trail

---

## Testing Edge Functions

### Test session-token:
```bash
curl -X POST https://mnqtdugcvfyenjuqruol.supabase.co/functions/v1/session-token \
  -H "Authorization: Bearer YOUR_USER_JWT" \
  -H "Content-Type: application/json" \
  -d '{"device_id": "test-device-123"}'
```

### Test device-register:
```bash
curl -X POST https://mnqtdugcvfyenjuqruol.supabase.co/functions/v1/device-register \
  -H "Content-Type: application/json" \
  -d '{
    "device_id": "hw-12345",
    "platform": "windows",
    "arch": "amd64",
    "device_name": "Test PC"
  }'
```

---

**Last Updated:** 2025-10-01
