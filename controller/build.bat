@echo off
echo Building Remote Desktop Controller...

REM Build for Windows
go build -ldflags "-s -w -H windowsgui" -o controller.exe

if %ERRORLEVEL% EQU 0 (
    echo.
    echo ✅ Build successful!
    echo.
    echo Output: controller.exe
    echo Size: 
    dir controller.exe | find "controller.exe"
) else (
    echo.
    echo ❌ Build failed!
    exit /b 1
)
