# Copy release notes to clipboard for easy pasting
Get-Content -Path "RELEASE_NOTES_v2.0.0.md" -Raw | Set-Clipboard
Write-Host "✅ Release notes copied to clipboard!" -ForegroundColor Green
Write-Host ""
Write-Host "Now paste them into the GitHub release description box." -ForegroundColor Cyan
