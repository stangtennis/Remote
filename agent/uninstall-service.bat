@echo off
echo ========================================
echo Remote Desktop Agent - Service Uninstall
echo ========================================
echo.

:: Check for admin rights
net session >nul 2>&1
if %errorLevel% neq 0 (
    echo Requesting administrator privileges...
    powershell -Command "Start-Process '%~f0' -Verb RunAs"
    exit /b
)

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
