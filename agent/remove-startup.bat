@echo off

:: Check for admin rights
net session >nul 2>&1
if %errorLevel% neq 0 (
    :: Request administrator privileges
    echo Set UAC = CreateObject^("Shell.Application"^) > "%temp%\getadmin.vbs"
    echo UAC.ShellExecute "%~s0", "", "", "runas", 1 >> "%temp%\getadmin.vbs"
    "%temp%\getadmin.vbs"
    del "%temp%\getadmin.vbs"
    exit /B
)

:: Running as admin now
cd /d "%~dp0"

echo ========================================
echo Remote Desktop Agent - Remove Startup
echo ========================================
echo.

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
