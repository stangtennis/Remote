@echo off

:: Configuration
set PROCESS_NAME=admin.py

:: Find and terminate specific processes
echo Searching for %PROCESS_NAME% processes...
for /f "tokens=2" %%p in ('tasklist /FI "IMAGENAME eq python.exe" /FI "WINDOWTITLE eq *%PROCESS_NAME%*" /NH') do (
    echo Stopping process PID: %%p
    taskkill /PID %%p /F
    if %ERRORLEVEL% neq 0 (
        echo Failed to stop process PID: %%p
    )
)

echo.
echo All %PROCESS_NAME% processes have been terminated.
pause
