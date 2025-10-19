@echo off
REM Install native messaging host for Windows

echo Installing Remote Desktop Control Native Host...

REM Get the current directory (where the .bat file is located)
set "INSTALL_DIR=%~dp0"
set "MANIFEST=%INSTALL_DIR%com.remote.desktop.control.json"

REM Update manifest with absolute path using PowerShell
powershell -NoProfile -ExecutionPolicy Bypass -Command "$content = Get-Content '%MANIFEST%' -Raw; $newPath = '%INSTALL_DIR%remote-desktop-control.exe'.Replace('\', '\\'); $content = $content -replace '\"path\": \"remote-desktop-control.exe\"', ('\"path\": \"' + $newPath + '\"'); Set-Content '%MANIFEST%' -Value $content -NoNewline"

REM Register with Chrome
echo Registering with Google Chrome...
reg add "HKCU\Software\Google\Chrome\NativeMessagingHosts\com.remote.desktop.control" /ve /t REG_SZ /d "%MANIFEST%" /f >nul 2>&1

REM Register with Edge
echo Registering with Microsoft Edge...
reg add "HKCU\Software\Microsoft\Edge\NativeMessagingHosts\com.remote.desktop.control" /ve /t REG_SZ /d "%MANIFEST%" /f >nul 2>&1

echo.
echo Installation complete!
echo.
echo Next steps:
echo 1. Load the browser extension in Chrome (chrome://extensions/)
echo 2. Copy the Extension ID
echo 3. Update com.remote.desktop.control.json with the Extension ID
echo 4. Run this script again
echo 5. Open the agent page and test!
echo.
pause
