@echo off
echo ========================================
echo Supabase Storage Upload - Working Solution
echo ========================================
echo.

REM Configuration based on Stack Overflow working solution
set SUPABASE_REF=ptrtibzwokjcjjxvjpin
set SUPABASE_KEY=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6InB0cnRpYnp3b2tqY2pqeHZqcGluIiwicm9sZSI6InNlcnZpY2Vfcm9sZSIsImlhdCI6MTc1NDQzMTU3MSwiZXhwIjoyMDcwMDA3NTcxfQ.bbj8zqF7AESiJwxJjDynhPYVBuAoujVBP3Op5lBaWBo
set BUCKET_NAME=agents
set FILE_NAME=RemoteDesktopAgent.exe
set LOCAL_FILE=public\%FILE_NAME%

echo Project Reference: %SUPABASE_REF%
echo Bucket: %BUCKET_NAME%
echo File: %FILE_NAME%
echo Local Path: %LOCAL_FILE%
echo.

REM Check if file exists
if not exist "%LOCAL_FILE%" (
    echo ERROR: File not found: %LOCAL_FILE%
    echo.
    pause
    exit /b 1
)

REM Show file info
for %%A in ("%LOCAL_FILE%") do (
    echo File found: %%~nxA
    echo Size: %%~zA bytes
)
echo.

REM Construct the upload URL using the working format from Stack Overflow
set UPLOAD_URL=https://%SUPABASE_REF%.supabase.co/storage/v1/object/%BUCKET_NAME%/%FILE_NAME%

echo Upload URL: %UPLOAD_URL%
echo API Key: %SUPABASE_KEY:~0,20%...
echo.

echo Starting upload...
echo.

REM Use the exact working curl command from Stack Overflow
curl -X POST "%UPLOAD_URL%" --data-binary "@%LOCAL_FILE%" -H "apikey: %SUPABASE_KEY%" -H "Authorization: Bearer %SUPABASE_KEY%" -H "Content-Type: application/octet-stream" -w "HTTP Status: %%{http_code}\nTime: %%{time_total}s\n" -v

set CURL_EXIT_CODE=%ERRORLEVEL%
echo.
echo ========================================

if %CURL_EXIT_CODE% EQU 0 (
    echo Upload completed successfully!
    echo.
    echo Public download URL:
    echo https://%SUPABASE_REF%.supabase.co/storage/v1/object/public/%BUCKET_NAME%/%FILE_NAME%
    echo.
    echo This URL should now work in your agent generators!
) else (
    echo Upload failed with exit code: %CURL_EXIT_CODE%
    echo.
    echo Troubleshooting tips:
    echo - Check if the 'agents' bucket exists in Supabase Storage
    echo - Verify API key permissions for storage uploads
    echo - Check RLS policies on storage.objects table
)

echo.
echo ========================================
pause
