@echo off
REM Install native messaging host for Windows

echo Installing Remote Desktop Control Native Host...

REM Get the current directory (where the .bat file is located)
set "INSTALL_DIR=%~dp0"
set "MANIFEST=%INSTALL_DIR%com.remote.desktop.control.json"

REM Update manifest with absolute path
powershell -Command "(Get-Content '%MANIFEST%') -replace '\"path\": \"remote-desktop-control.exe\"', ('\"path\": \"' + '%INSTALL_DIR%remote-desktop-control.exe\"'.Replace('\', '\\') + '\"') | Set-Content '%MANIFEST%'"

REM Register with Chrome
echo Registering with Google Chrome...
reg add "HKCU\Software\Google\Chrome\NativeMessagingHosts\com.remote.desktop.control" /ve /t REG_SZ /d "%MANIFEST%" /f

REM Register with Edge
echo Registering with Microsoft Edge...
reg add "HKCU\Software\Microsoft\Edge\NativeMessagingHosts\com.remote.desktop.control" /ve /t REG_SZ /d "%MANIFEST%" /f

echo.
echo Installation complete!
echo.
echo Next steps:
echo 1. Install the browser extension from the Chrome Web Store
echo 2. Open the agent page
echo 3. Remote control should now work!
echo.
pause
