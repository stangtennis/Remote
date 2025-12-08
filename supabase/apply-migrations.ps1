# Apply all SQL migrations to local Supabase database
# Run this script to set up the database schema on your local Supabase instance

$DB_HOST = "192.168.1.92"
$DB_PORT = "5432"
$DB_NAME = "postgres"
$DB_USER = "postgres"
$DB_PASSWORD = "postgres"

Write-Host "🚀 Applying SQL migrations to local Supabase..." -ForegroundColor Cyan
Write-Host "Database: ${DB_HOST}:${DB_PORT}/${DB_NAME}" -ForegroundColor Gray
Write-Host ""

# Get all migration files in order
$migrations = Get-ChildItem -Path ".\migrations\*.sql" | Sort-Object Name

if ($migrations.Count -eq 0) {
    Write-Host "❌ No migration files found in .\migrations\" -ForegroundColor Red
    exit 1
}

Write-Host "Found $($migrations.Count) migration files" -ForegroundColor Green
Write-Host ""

$successCount = 0
$failCount = 0

foreach ($migration in $migrations) {
    Write-Host "📄 Applying: $($migration.Name)" -ForegroundColor Yellow
    
    # Set PGPASSWORD environment variable for this command
    $env:PGPASSWORD = $DB_PASSWORD
    
    # Apply migration using psql
    $result = & psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -f $migration.FullName 2>&1
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "   ✅ Success" -ForegroundColor Green
        $successCount++
    } else {
        Write-Host "   ❌ Failed: $result" -ForegroundColor Red
        $failCount++
    }
    
    Write-Host ""
}

# Clear password from environment
Remove-Item Env:\PGPASSWORD -ErrorAction SilentlyContinue

Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Gray
Write-Host "✅ Successful: $successCount" -ForegroundColor Green
Write-Host "❌ Failed: $failCount" -ForegroundColor Red
Write-Host ""

if ($failCount -eq 0) {
    Write-Host "🎉 All migrations applied successfully!" -ForegroundColor Green
} else {
    Write-Host "⚠️  Some migrations failed. Check errors above." -ForegroundColor Yellow
}
