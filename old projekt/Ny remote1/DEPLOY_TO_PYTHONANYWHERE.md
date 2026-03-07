# Deploying to PythonAnywhere

This guide will help you deploy the remote desktop server to PythonAnywhere's free tier.

## Steps

1. Create a PythonAnywhere Account:
   - Go to https://www.pythonanywhere.com
   - Sign up for a free account
   - No credit card required

2. Upload Your Code:
   ```bash
   # In your PythonAnywhere dashboard:
   # 1. Click on "Files"
   # 2. Create a new directory called "remote-desktop"
   # 3. Upload server.py and requirements.txt
   ```

3. Install Dependencies:
   ```bash
   # Open a PythonAnywhere console and run:
   cd remote-desktop
   pip3 install --user -r requirements.txt
   ```

4. Create a Web App:
   - Go to the "Web" tab in your dashboard
   - Click "Add a new web app"
   - Choose "Manual configuration"
   - Choose Python 3.9 (or latest available)

5. Configure WSGI File:
   - In the web app configuration, find and click on the WSGI configuration file link
   - Replace the contents with:

   ```python
   import sys
   import os
   
   # Add your project directory to Python path
   path = '/home/YOUR_USERNAME/remote-desktop'
   if path not in sys.path:
       sys.path.append(path)
   
   from server import socket_app
   
   # Set environment variable for PythonAnywhere
   os.environ['PYTHONANYWHERE_SITE'] = 'true'
   
   application = socket_app
   ```

6. Configure Web App:
   - In "Static Files" section, leave it empty (we don't need static files)
   - Set "Force HTTPS" to enabled
   - Click the "Reload" button

7. Get Your Server URL:
   - Your server will be available at: `https://YOUR_USERNAME.pythonanywhere.com`
   - Use this URL when connecting clients and admin panel

## Connecting Clients and Admin

1. Connect client:
```bash
python client.py --server https://YOUR_USERNAME.pythonanywhere.com
```

2. Connect admin panel:
```bash
python admin.py --server https://YOUR_USERNAME.pythonanywhere.com
```

## Limitations of Free Tier

1. CPU quota restrictions
2. Limited outbound network access
3. Web apps are put to sleep after inactivity
4. May need to reload every 24 hours

## Troubleshooting

1. If connection fails:
   - Check the error log in PythonAnywhere dashboard
   - Ensure all dependencies are installed
   - Verify WSGI file configuration
   - Make sure to use HTTPS in the server URL

2. If the app is slow:
   - This is normal for free tier
   - Consider upgrading or using a different provider for production

3. If the app stops responding:
   - Free tier apps go to sleep after inactivity
   - Just reload the web app in PythonAnywhere dashboard
