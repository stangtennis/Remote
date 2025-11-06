# GitHub Release Creation Script for v2.0.0
# This script creates a GitHub release and uploads the executables

$owner = "stangtennis"
$repo = "Remote"
$tag = "v2.0.0"
$name = "v2.0.0 - Maximum Quality Update (2025-11-06)"

# Read release notes
$body = Get-Content -Path "RELEASE_NOTES_v2.0.0.md" -Raw

# GitHub Personal Access Token (you need to set this)
# Create token at: https://github.com/settings/tokens
# Required scopes: repo (Full control of private repositories)
$token = $env:GITHUB_TOKEN

if (-not $token) {
    Write-Host "ERROR: GITHUB_TOKEN environment variable not set!" -ForegroundColor Red
    Write-Host ""
    Write-Host "To create a token:" -ForegroundColor Yellow
    Write-Host "1. Go to: https://github.com/settings/tokens" -ForegroundColor Yellow
    Write-Host "2. Click 'Generate new token (classic)'" -ForegroundColor Yellow
    Write-Host "3. Select 'repo' scope" -ForegroundColor Yellow
    Write-Host "4. Copy the token" -ForegroundColor Yellow
    Write-Host "5. Run: `$env:GITHUB_TOKEN = 'your_token_here'" -ForegroundColor Yellow
    Write-Host "6. Run this script again" -ForegroundColor Yellow
    exit 1
}

Write-Host "Creating GitHub Release v2.0.0..." -ForegroundColor Cyan

# Create release
$headers = @{
    "Accept" = "application/vnd.github+json"
    "Authorization" = "Bearer $token"
    "X-GitHub-Api-Version" = "2022-11-28"
}

$releaseData = @{
    tag_name = $tag
    name = $name
    body = $body
    draft = $false
    prerelease = $false
} | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "https://api.github.com/repos/$owner/$repo/releases" `
        -Method Post `
        -Headers $headers `
        -Body $releaseData `
        -ContentType "application/json"
    
    Write-Host "✅ Release created successfully!" -ForegroundColor Green
    Write-Host "Release ID: $($response.id)" -ForegroundColor Green
    Write-Host "Upload URL: $($response.upload_url)" -ForegroundColor Green
    
    $uploadUrl = $response.upload_url -replace '\{\?name,label\}', ''
    
    # Upload agent executable
    Write-Host ""
    Write-Host "Uploading remote-agent.exe..." -ForegroundColor Cyan
    $agentFile = "agent\remote-agent.exe"
    $agentBytes = [System.IO.File]::ReadAllBytes($agentFile)
    
    $uploadHeaders = @{
        "Accept" = "application/vnd.github+json"
        "Authorization" = "Bearer $token"
        "Content-Type" = "application/octet-stream"
    }
    
    Invoke-RestMethod -Uri "$uploadUrl?name=remote-agent.exe" `
        -Method Post `
        -Headers $uploadHeaders `
        -Body $agentBytes | Out-Null
    
    Write-Host "✅ remote-agent.exe uploaded!" -ForegroundColor Green
    
    # Upload controller executable
    Write-Host ""
    Write-Host "Uploading controller.exe..." -ForegroundColor Cyan
    $controllerFile = "controller\controller.exe"
    $controllerBytes = [System.IO.File]::ReadAllBytes($controllerFile)
    
    Invoke-RestMethod -Uri "$uploadUrl?name=controller.exe" `
        -Method Post `
        -Headers $uploadHeaders `
        -Body $controllerBytes | Out-Null
    
    Write-Host "✅ controller.exe uploaded!" -ForegroundColor Green
    
    Write-Host ""
    Write-Host "🎉 Release v2.0.0 published successfully!" -ForegroundColor Green
    Write-Host "View at: https://github.com/$owner/$repo/releases/tag/$tag" -ForegroundColor Cyan
    
} catch {
    Write-Host "❌ Error creating release:" -ForegroundColor Red
    Write-Host $_.Exception.Message -ForegroundColor Red
    
    if ($_.Exception.Response) {
        $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
        $responseBody = $reader.ReadToEnd()
        Write-Host "Response: $responseBody" -ForegroundColor Red
    }
    exit 1
}
