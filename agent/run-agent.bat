@echo off
echo ========================================
echo Remote Desktop Agent - Manual Run
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

:: Get current directory and user
set "AGENT_PATH=%~dp0remote-agent.exe"
for /f "tokens=*" %%u in ('whoami') do set CURRENT_USER=%%u

echo Starting agent with elevated privileges in your session...
echo This allows full screen capture access.
echo.

:: Just run it directly (already has admin rights from the script)
echo Starting agent...
start "" "%AGENT_PATH%"
timeout /t 1 /nobreak >nul

echo.
echo Agent is running in the background as SYSTEM.
echo Check for the agent window or logs.
echo.
pause
