@echo off
REM Build script for Remote Desktop Agent - DEBUG MODE (with console)
REM This version shows the console window so you can see logs in real-time

echo 🔨 Building Remote Desktop Agent - DEBUG MODE (with console)...
echo.

REM Check if GCC is available
where gcc >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo ❌ ERROR: GCC not found!
    echo.
    echo Please install MinGW-w64:
    echo   1. Download from: https://sourceforge.net/projects/mingw-w64/
    echo   2. Or use chocolatey: choco install mingw
    echo   3. Add to PATH: C:\mingw64\bin
    echo.
    exit /b 1
)

echo ✅ GCC found

REM Enable CGO
set CGO_ENABLED=1

REM Build the agent WITHOUT -H windowsgui flag (shows console)
echo.
echo 🔧 Building with console output enabled...
cd /d "%~dp0"
go build -ldflags "-s -w" -o remote-agent-debug.exe ./cmd/remote-agent

if %ERRORLEVEL% EQU 0 (
    echo.
    echo ✅ Build successful!
    echo 📦 Binary: remote-agent-debug.exe
    echo.
    echo 🚀 Run with: .\remote-agent-debug.exe
    echo.
    echo ℹ️  This version shows the console window with live logs.
    echo ℹ️  For production use without console, use build.bat instead.
) else (
    echo.
    echo ❌ Build failed!
    exit /b 1
)
