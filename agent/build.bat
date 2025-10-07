@echo off
REM Build script for Remote Desktop Agent with CGO support (robotgo)
REM Requires: GCC (MinGW-w64 recommended)

echo 🔨 Building Remote Desktop Agent with input control...
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

REM Build the agent
echo.
echo 🔧 Building with CGO_ENABLED=1...
cd /d "%~dp0"
go build -ldflags "-s -w -H windowsgui" -o remote-agent.exe ./cmd/remote-agent

if %ERRORLEVEL% EQU 0 (
    echo.
    echo ✅ Build successful!
    echo 📦 Binary: remote-agent.exe
    echo.
    echo 🚀 Run with: .\remote-agent.exe
) else (
    echo.
    echo ❌ Build failed!
    exit /b 1
)
