@echo off
echo ========================================
echo Remote Agent Log Viewer
echo ========================================
echo.
echo Log file: %~dp0agent.log
echo.
echo Press Ctrl+C to stop watching...
echo.

powershell -Command "Get-Content '%~dp0agent.log' -Wait -Tail 50"
