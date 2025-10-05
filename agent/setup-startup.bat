@echo off
echo ========================================
echo Remote Desktop Agent - Auto-Startup Setup
echo ========================================
echo.
echo This will configure the agent to start automatically
echo when you log in to Windows (not as a service)
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

echo Creating startup task...
echo Agent path: %AGENT_PATH%
echo.

:: Get current username
for /f "tokens=*" %%u in ('whoami') do set CURRENT_USER=%%u

:: Create scheduled task to run at user logon in interactive session
:: Runs as current user with highest privileges in their desktop session
schtasks /create /tn "RemoteDesktopAgent" /tr "%AGENT_PATH%" /sc onlogon /ru "%CURRENT_USER%" /rl HIGHEST /f

if %errorLevel% equ 0 (
    echo.
    echo ========================================
    echo   SUCCESS! Agent configured to run at startup
    echo ========================================
    echo.
    echo The agent will start automatically when Windows boots.
    echo.
    echo To start it now without rebooting:
    echo   schtasks /run /tn "RemoteDesktopAgent"
    echo.
    echo To remove startup:
    echo   schtasks /delete /tn "RemoteDesktopAgent" /f
    echo.
) else (
    echo.
    echo ERROR: Failed to create startup task
    echo.
)

pause
