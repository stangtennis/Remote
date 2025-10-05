@echo off
echo ========================================
echo Service Status Monitor
echo ========================================
echo.
echo Watching RemoteDesktopAgent service...
echo Press Ctrl+C to stop
echo.

powershell -Command "while ($true) { $status = (Get-Service RemoteDesktopAgent -ErrorAction SilentlyContinue).Status; $time = Get-Date -Format 'HH:mm:ss'; if ($status) { Write-Host \"$time - Service: $status\" } else { Write-Host \"$time - Service: NOT FOUND\" }; Start-Sleep -Seconds 2 }"
