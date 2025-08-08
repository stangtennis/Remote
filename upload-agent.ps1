# PowerShell script to upload agent to Supabase Storage using REST API
param(
    [string]$FilePath = "public\RemoteDesktopAgent.exe",
    [string]$BucketName = "agents",
    [string]$FileName = "RemoteDesktopAgent.exe"
)

# Supabase configuration
$supabaseUrl = "https://ptrtibzwokjcjjxvjpin.supabase.co"
$supabaseKey = "REDACTED_JWT"

Write-Host "🚀 Starting agent upload to Supabase Storage..." -ForegroundColor Green
Write-Host ""

# Check if file exists
if (-not (Test-Path $FilePath)) {
    Write-Host "❌ File not found: $FilePath" -ForegroundColor Red
    exit 1
}

$fileInfo = Get-Item $FilePath
$fileSizeMB = [math]::Round($fileInfo.Length / 1MB, 2)
Write-Host "📁 File: $($fileInfo.FullName)" -ForegroundColor Cyan
Write-Host "📊 Size: $fileSizeMB MB" -ForegroundColor Cyan
Write-Host ""

# Upload URL
$uploadUrl = "$supabaseUrl/storage/v1/object/$BucketName/$FileName"

Write-Host "🌐 Upload URL: $uploadUrl" -ForegroundColor Yellow
Write-Host "🔑 Using API Key: $($supabaseKey.Substring(0, 20))..." -ForegroundColor Yellow
Write-Host ""

try {
    # Create headers
    $headers = @{
        "Authorization" = "Bearer $supabaseKey"
        "apikey" = $supabaseKey
        "Content-Type" = "application/octet-stream"
        "x-upsert" = "true"
    }

    Write-Host "📤 Uploading file..." -ForegroundColor Yellow
    
    # Use Invoke-RestMethod for upload
    $response = Invoke-RestMethod -Uri $uploadUrl -Method Post -InFile $FilePath -Headers $headers -ContentType "application/octet-stream"
    
    Write-Host "✅ Upload successful!" -ForegroundColor Green
    Write-Host "📍 Response: $($response | ConvertTo-Json -Depth 2)" -ForegroundColor Green
    Write-Host ""
    Write-Host "🌍 Public URL: $supabaseUrl/storage/v1/object/public/$BucketName/$FileName" -ForegroundColor Green
    
} catch {
    Write-Host "❌ Upload failed!" -ForegroundColor Red
    Write-Host "Error: $($_.Exception.Message)" -ForegroundColor Red
    
    if ($_.Exception.Response) {
        $statusCode = $_.Exception.Response.StatusCode
        Write-Host "Status Code: $statusCode" -ForegroundColor Red
        
        try {
            $errorStream = $_.Exception.Response.GetResponseStream()
            $reader = New-Object System.IO.StreamReader($errorStream)
            $errorBody = $reader.ReadToEnd()
            Write-Host "Response Body: $errorBody" -ForegroundColor Red
        } catch {
            Write-Host "Could not read error response body" -ForegroundColor Red
        }
    }
    
    exit 1
}

Write-Host ""
Write-Host "🎉 Agent upload completed successfully!" -ForegroundColor Green
