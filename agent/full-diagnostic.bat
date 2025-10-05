@echo off
echo ========================================
echo Remote Agent Full Diagnostic
echo ========================================
echo.
echo Timestamp: %date% %time%
echo.

echo === 1. RUNNING PROCESSES ===
powershell -Command "Get-Process -Name remote-agent -ErrorAction SilentlyContinue | Format-Table Id,Path,StartTime,UserName -AutoSize"
echo.

echo === 2. STARTUP TASK ===
schtasks /query /tn "RemoteDesktopAgent" /fo LIST 2>nul
if %ERRORLEVEL% EQU 0 (
    echo ⚠️  WARNING: Startup task exists!
    echo Run remove-startup.bat to remove it
) else (
    echo ✅ No startup task found
)
echo.

echo === 3. WINDOWS SERVICE ===
sc query RemoteDesktopAgent
echo.

echo === 4. SERVICE CONFIGURATION ===
sc qc RemoteDesktopAgent
echo.

echo === 5. LAST 20 LOG LINES ===
if exist "%~dp0agent.log" (
    powershell -Command "Get-Content '%~dp0agent.log' -Tail 20"
) else (
    echo ⚠️  Log file not found!
)
echo.

echo === 6. DEVICE ID ===
if exist "%~dp0.device_id" (
    type "%~dp0.device_id"
) else (
    echo ⚠️  Device ID file not found!
)
echo.

echo === 7. SUMMARY ===
powershell -Command "$count = (Get-Process -Name remote-agent -ErrorAction SilentlyContinue).Count; Write-Host \"Total remote-agent.exe processes: $count\"; if ($count -gt 1) { Write-Host \"⚠️  WARNING: Multiple instances detected!\" -ForegroundColor Yellow } elseif ($count -eq 1) { Write-Host \"✅ Single instance running\" -ForegroundColor Green } else { Write-Host \"❌ No instances running\" -ForegroundColor Red }"
echo.

echo ========================================
echo.
pause
