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
echo MinGW-w64 (GCC) Installation
echo ========================================
echo.
echo This will install MinGW-w64 which is required
echo to build the Remote Desktop Agent.
echo.
pause

echo.
echo Installing MinGW-w64 via Chocolatey...
choco install mingw -y

if %ERRORLEVEL% EQU 0 (
    echo.
    echo ========================================
    echo ✅ MinGW-w64 installed successfully!
    echo ========================================
    echo.
    echo Please CLOSE THIS WINDOW and open a NEW command prompt
    echo for the PATH changes to take effect.
    echo.
    echo Then you can run: .\build.bat
    echo.
) else (
    echo.
    echo ========================================
    echo ❌ Installation failed!
    echo ========================================
    echo.
    echo Alternative: Download manually from:
    echo https://sourceforge.net/projects/mingw-w64/
    echo.
    echo Choose: x86_64, posix, seh
    echo Install to: C:\mingw64
    echo Add to PATH: C:\mingw64\bin
    echo.
)

pause
