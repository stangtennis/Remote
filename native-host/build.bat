@echo off
REM Build native host for Windows

echo Building Remote Desktop Control Native Host for Windows...

REM Build the Go executable
go build -ldflags="-s -w" -o remote-desktop-control.exe main.go

if %ERRORLEVEL% EQU 0 (
    echo.
    echo Build successful! File: remote-desktop-control.exe
    echo.
    echo Run install-windows.bat to install the native host.
) else (
    echo.
    echo Build failed!
    pause
    exit /b 1
)

pause
