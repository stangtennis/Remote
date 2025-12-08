@echo off
REM Setup Windows Firewall rules for Remote Desktop Agent
REM Run this as Administrator to allow agent through firewall

echo ========================================
echo Remote Desktop Agent - Firewall Setup
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

REM Find agent executable (try different names)
set "EXE_PATH="
if exist "%SCRIPT_DIR%remote-agent.exe" set "EXE_PATH=%SCRIPT_DIR%remote-agent.exe"
if exist "%SCRIPT_DIR%remote-agent-v2.6.9.exe" set "EXE_PATH=%SCRIPT_DIR%remote-agent-v2.6.9.exe"

REM Check if executable exists
if "%EXE_PATH%"=="" (
    echo ERROR: Agent executable not found!
    echo Please place remote-agent.exe in the same folder as this script.
    pause
    exit /b 1
)

echo Found agent: %EXE_PATH%
echo.

REM Remove existing rules
echo Removing old firewall rules...
netsh advfirewall firewall delete rule name="Remote Desktop Agent" >nul 2>&1

REM Add new rules
echo Adding inbound rule...
netsh advfirewall firewall add rule name="Remote Desktop Agent" dir=in action=allow program="%EXE_PATH%" enable=yes profile=any

echo Adding outbound rule...
netsh advfirewall firewall add rule name="Remote Desktop Agent" dir=out action=allow program="%EXE_PATH%" enable=yes profile=any

echo.
echo ✅ Firewall rules added successfully!
echo.
echo The agent can now communicate through Windows Firewall.
echo You should no longer see security prompts when connecting.
echo.
pause
