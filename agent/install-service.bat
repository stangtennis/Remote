@echo off
REM Install Remote Desktop Agent as a Windows Service
REM This allows the agent to run at login screen

echo ========================================
echo Remote Desktop Agent - Service Installer
echo ========================================
echo.

REM Check for admin rights
net session >nul 2>&1
if %errorLevel% neq 0 (
    echo ERROR: This script must be run as Administrator!
    echo Right-click and select "Run as Administrator"
    pause
    exit /b 1
)

REM Get the directory where this script is located
set "SCRIPT_DIR=%~dp0"
set "EXE_PATH=%SCRIPT_DIR%remote-agent.exe"

REM Check if executable exists
if not exist "%EXE_PATH%" (
    echo ERROR: remote-agent.exe not found!
    echo Please build the agent first using build.bat
    pause
    exit /b 1
)

echo Installing service...
echo Executable: %EXE_PATH%
echo.

REM Stop and remove existing service if it exists
sc query RemoteDesktopAgent >nul 2>&1
if %errorLevel% equ 0 (
    echo Stopping existing service...
    sc stop RemoteDesktopAgent >nul 2>&1
    timeout /t 2 /nobreak >nul
    echo Removing existing service...
    sc delete RemoteDesktopAgent >nul 2>&1
    timeout /t 2 /nobreak >nul
)

REM Create the service
REM - Run as LocalSystem for Session 0 access
REM - Start automatically on boot
REM - Interact with desktop (for screen capture)
sc create RemoteDesktopAgent binPath= "%EXE_PATH%" start= auto DisplayName= "Remote Desktop Agent" obj= LocalSystem

if %errorLevel% neq 0 (
    echo ERROR: Failed to create service!
    pause
    exit /b 1
)

REM Set service description
sc description RemoteDesktopAgent "Provides remote desktop access with lock screen support"

REM Configure service to restart on failure
sc failure RemoteDesktopAgent reset= 86400 actions= restart/5000/restart/10000/restart/30000

echo.
echo ✅ Service installed successfully!
echo.
echo To start the service now, run:
echo    sc start RemoteDesktopAgent
echo.
echo Or use: net start RemoteDesktopAgent
echo.
echo The service will start automatically on next boot.
echo.
echo To view service status: sc query RemoteDesktopAgent
echo To stop service: sc stop RemoteDesktopAgent
echo To uninstall: run uninstall-service.bat
echo.
pause
