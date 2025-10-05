@echo off
echo ========================================
echo Remote Agent Duplicate Check
echo ========================================
echo.

echo 1. Checking running processes...
powershell -Command "Get-Process -Name remote-agent -ErrorAction SilentlyContinue | Format-Table Id,Path,StartTime"
echo.

echo 2. Checking for startup task...
schtasks /query /tn "RemoteDesktopAgent" /fo LIST 2>nul
if %ERRORLEVEL% EQU 0 (
    echo ⚠️  Startup task FOUND
) else (
    echo ✅ No startup task
)
echo.

echo 3. Checking Windows Service...
sc query RemoteDesktopAgent
echo.

echo 4. Process count:
powershell -Command "$count = (Get-Process -Name remote-agent -ErrorAction SilentlyContinue).Count; Write-Host \"remote-agent.exe instances running: $count\""
echo.

pause
