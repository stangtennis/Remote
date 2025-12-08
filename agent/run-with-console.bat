@echo off
echo ========================================
echo Remote Desktop Agent - Console Mode
echo ========================================
echo.

REM Check if debug version exists
if not exist "%~dp0remote-agent-debug.exe" (
    echo ℹ️  Debug version not found. Building it now...
    echo.
    call "%~dp0build-debug.bat"
    if %ERRORLEVEL% NEQ 0 (
        echo.
        echo ❌ Failed to build debug version.
        pause
        exit /b 1
    )
    echo.
)

echo ✅ Starting agent with console output...
echo.
echo You can see all logs and activity in real-time below.
echo Press Ctrl+C to stop the agent.
echo ========================================
echo.

"%~dp0remote-agent-debug.exe"

echo.
echo ========================================
echo Agent stopped.
echo ========================================
pause
