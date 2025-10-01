# Dashboard Deployment Guide

## 🎉 Dashboard is Ready!

Your Remote Desktop dashboard is built and ready to deploy to GitHub Pages.

## What's Been Built

- ✅ **index.html** - Login/Signup page
- ✅ **dashboard.html** - Main dashboard with device management
- ✅ **Modern CSS** - Dark theme, responsive design
- ✅ **Authentication** - Supabase Auth integration
- ✅ **Device Management** - List, approve, connect to devices
- ✅ **WebRTC Viewer** - Real-time remote desktop viewing
- ✅ **Input Control** - Mouse and keyboard forwarding
- ✅ **Connection Stats** - Real-time quality metrics

## Deploy to GitHub Pages

### Step 1: Push to GitHub

```bash
cd f:\#Remote

# Initialize git if not already
git init
git remote add origin https://github.com/stangtennis/Remote.git

# Add all files
git add .

# Commit
git commit -m "Add dashboard and backend infrastructure"

# Push to main branch
git push -u origin main
```

### Step 2: Enable GitHub Pages

1. Go to: https://github.com/stangtennis/Remote/settings/pages
2. Under **"Build and deployment"**:
   - **Source:** Deploy from a branch
   - **Branch:** main
   - **Folder:** /dashboard
3. Click **Save**

### Step 3: Wait for Deployment

GitHub will build and deploy your site (takes 1-2 minutes).

Your dashboard will be live at:
**https://stangtennis.github.io/Remote/**

## Update Supabase Auth URLs

After deployment, update your Supabase Auth settings:

1. Go to: https://supabase.com/dashboard/project/mnqtdugcvfyenjuqruol/auth/url-configuration
2. Update **Site URL** to: https://stangtennis.github.io/Remote/
3. Add **Redirect URLs**:
   - https://stangtennis.github.io/Remote/dashboard.html
   - http://localhost:5500 (for local testing)
4. Click **Save**

## Test Your Dashboard

1. Open: https://stangtennis.github.io/Remote/
2. Create an account (check email for confirmation)
3. Login to dashboard
4. You should see empty device list (no agents running yet)

## Local Testing (Before Deploy)

To test locally before deploying:

```bash
# Option 1: Use VS Code Live Server
# - Install "Live Server" extension
# - Right-click dashboard/index.html → Open with Live Server

# Option 2: Use Python
cd dashboard
python -m http.server 8000
# Open http://localhost:8000

# Option 3: Use Node.js
npx serve dashboard
```

## Next Steps

After dashboard is deployed:

1. ✅ **Test Login** - Create account and verify email
2. ✅ **Test Dashboard** - Verify device list loads (empty)
3. 🔨 **Build Agent** - Create Windows .exe to populate device list
4. 🔗 **Test Full Flow** - Agent → Dashboard → WebRTC connection

## Troubleshooting

### "Failed to fetch" errors
- Check SUPABASE_URL and ANON_KEY in dashboard/js/auth.js
- Verify Supabase project is active

### CORS errors
- Ensure Supabase Auth URLs are configured correctly
- Add your GitHub Pages URL to redirect URLs

### Can't login/signup
- Check email confirmation (check spam folder)
- Verify Auth is enabled in Supabase dashboard

### Blank page
- Check browser console for errors (F12)
- Verify all files were committed and pushed

## File Checklist

Make sure these files exist before deploying:

- [ ] dashboard/index.html
- [ ] dashboard/dashboard.html
- [ ] dashboard/css/styles.css
- [ ] dashboard/js/auth.js
- [ ] dashboard/js/app.js
- [ ] dashboard/js/devices.js
- [ ] dashboard/js/webrtc.js
- [ ] dashboard/js/signaling.js
- [ ] dashboard/README.md

---

**Ready to deploy?** Run the git commands above! 🚀
