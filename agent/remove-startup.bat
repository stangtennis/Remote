@echo off
echo ========================================
echo Remote Desktop Agent - Remove Startup
echo ========================================
echo.

:: Check for admin rights
net session >nul 2>&1
if %errorLevel% neq 0 (
    echo ERROR: This script requires Administrator privileges.
    echo Please right-click and select "Run as Administrator"
    echo.
    pause
    exit /b 1
)

echo Removing startup task...
schtasks /delete /tn "RemoteDesktopAgent" /f

if %errorLevel% equ 0 (
    echo.
    echo SUCCESS! Startup task removed.
    echo The agent will no longer start automatically.
) else (
    echo.
    echo Task not found or already removed.
)

echo.
pause
