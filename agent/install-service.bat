@echo off
echo ========================================
echo Remote Desktop Agent - Service Installation
echo ========================================
echo.

:: Check for admin rights
net session >nul 2>&1
if %errorLevel% neq 0 (
    echo Requesting administrator privileges...
    powershell -Command "Start-Process '%~f0' -Verb RunAs"
    exit /b
)

:: Get current directory
set "AGENT_PATH=%~dp0remote-agent.exe"

if not exist "%AGENT_PATH%" (
    echo ERROR: remote-agent.exe not found in current directory
    echo Please run this script from the agent folder
    echo.
    pause
    exit /b 1
)

echo Installing service...
echo Agent path: %AGENT_PATH%
echo.

:: Create the service
sc create RemoteDesktopAgent binPath= "%AGENT_PATH%" start= auto DisplayName= "Remote Desktop Agent" obj= LocalSystem

if %errorLevel% neq 0 (
    echo.
    echo ERROR: Failed to create service
    echo.
    pause
    exit /b 1
)

:: Set service description
sc description RemoteDesktopAgent "Provides remote desktop access with lock screen support via WebRTC"

:: Configure service to interact with desktop (required for login screen access)
sc config RemoteDesktopAgent type= own type= interact

:: Start the service
echo.
echo Starting service...
sc start RemoteDesktopAgent

if %errorLevel% equ 0 (
    echo.
    echo ========================================
    echo   SUCCESS! Service installed and started
    echo ========================================
    echo.
    echo The agent will now run as a Windows Service
    echo It will start automatically on boot
    echo It can capture the login screen when locked
    echo.
    echo Service name: RemoteDesktopAgent
    echo.
    echo To stop:   sc stop RemoteDesktopAgent
    echo To start:  sc start RemoteDesktopAgent
    echo To remove: run uninstall-service.bat
    echo.
) else (
    echo.
    echo Service created but failed to start
    echo Check Windows Event Viewer for errors
    echo.
)

pause
