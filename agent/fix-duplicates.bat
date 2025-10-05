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
echo Fix Duplicate Agents
echo ========================================
echo.
echo This will:
echo  1. Stop all running agent processes
echo  2. Remove startup task (if exists)
echo  3. Keep only the Windows Service
echo.
pause

echo.
echo Step 1: Stopping all agent processes...
taskkill /F /IM remote-agent.exe 2>nul
timeout /t 2 /nobreak >nul

echo Step 2: Removing startup task...
schtasks /delete /tn "RemoteDesktopAgent" /f 2>nul
if %ERRORLEVEL% EQU 0 (
    echo ✅ Startup task removed
) else (
    echo ℹ️  No startup task found
)

echo.
echo Step 3: Starting Windows Service...
sc start RemoteDesktopAgent

echo.
echo Step 4: Verifying...
timeout /t 3 /nobreak >nul
powershell -Command "$count = (Get-Process -Name remote-agent -ErrorAction SilentlyContinue).Count; Write-Host \"Processes running: $count\"; if ($count -eq 1) { Write-Host \"✅ SUCCESS! Only one instance running\" -ForegroundColor Green } else { Write-Host \"⚠️  Expected 1, got $count\" -ForegroundColor Yellow }"

echo.
echo ========================================
echo.
pause
