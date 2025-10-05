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
echo Remote Desktop Agent - Service Uninstall
echo ========================================
echo.

echo Stopping service...
sc stop RemoteDesktopAgent

timeout /t 2 /nobreak >nul

echo Removing service...
sc delete RemoteDesktopAgent

if %errorLevel% equ 0 (
    echo.
    echo ========================================
    echo   SUCCESS! Service removed
    echo ========================================
    echo.
    echo The Remote Desktop Agent service has been uninstalled
    echo.
) else (
    echo.
    echo ERROR: Failed to remove service
    echo.
)

pause
