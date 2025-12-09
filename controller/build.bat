@echo off
echo Building Remote Desktop Controller...

REM Check if windres is available for embedding manifest
where windres >nul 2>&1
if %ERRORLEVEL% EQU 0 (
    echo Embedding admin manifest...
    windres -o controller.syso controller.rc
    if %ERRORLEVEL% NEQ 0 (
        echo Warning: Failed to embed manifest, building without it
    )
) else (
    echo Warning: windres not found, building without embedded manifest
    echo Install MinGW or TDM-GCC to embed the admin manifest
)

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
