# Remote Desktop Agent - Total Cleanup Script
# Run as Administrator in PowerShell

Write-Host "=== Remote Desktop Agent - Total Cleanup ===" -ForegroundColor Yellow
Write-Host ""

# Stop ALLE remote desktop processer
Write-Host "Stopping processes..." -ForegroundColor Cyan
Get-Process | Where-Object {$_.Name -like "*remote*" -or $_.Name -like "*agent*" -or $_.Name -like "*RemoteAgent*"} | Stop-Process -Force -ErrorAction SilentlyContinue

# Stop og slet service
Write-Host "Stopping and removing service..." -ForegroundColor Cyan
sc.exe stop RemoteDesktopAgent 2>$null
sc.exe delete RemoteDesktopAgent 2>$null

# Slet ALLE data mapper
Write-Host "Deleting data folders..." -ForegroundColor Cyan
Remove-Item "C:\ProgramData\RemoteDesktopAgent" -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item "C:\ProgramData\RemoteDesktop" -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item "$env:APPDATA\RemoteDesktopAgent" -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item "$env:APPDATA\RemoteDesktop" -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item "$env:LOCALAPPDATA\RemoteDesktopAgent" -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item "$env:LOCALAPPDATA\Fyne" -Recurse -Force -ErrorAction SilentlyContinue

# Slet registry
Write-Host "Cleaning registry..." -ForegroundColor Cyan
reg delete "HKLM\SOFTWARE\RemoteDesktop" /f 2>$null
reg delete "HKCU\SOFTWARE\RemoteDesktop" /f 2>$null
reg delete "HKCU\SOFTWARE\RemoteDesktopAgent" /f 2>$null

# Slet ALLE exe filer i Downloads
Write-Host "Deleting downloaded files..." -ForegroundColor Cyan
Remove-Item "$env:USERPROFILE\Downloads\*remote*.exe" -Force -ErrorAction SilentlyContinue
Remove-Item "$env:USERPROFILE\Downloads\*controller*.exe" -Force -ErrorAction SilentlyContinue
Remove-Item "$env:USERPROFILE\Downloads\*agent*.exe" -Force -ErrorAction SilentlyContinue
Remove-Item "$env:USERPROFILE\Downloads\*Remote*.exe" -Force -ErrorAction SilentlyContinue
Remove-Item "$env:USERPROFILE\Downloads\*.log" -Force -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "=== Cleanup Complete ===" -ForegroundColor Green
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Yellow
Write-Host "1. Restart your computer" -ForegroundColor White
Write-Host "2. Download ONLY 'remote-agent.exe' from v2.10.0" -ForegroundColor White
Write-Host "3. Run from CMD: remote-agent.exe" -ForegroundColor White
Write-Host ""
Write-Host "Press any key to exit..."
$null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
