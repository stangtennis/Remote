@echo off
echo Starting Supabase Storage upload with curl...
echo.

set SUPABASE_URL=https://ptrtibzwokjcjjxvjpin.supabase.co
set SUPABASE_KEY=REDACTED_JWT
set FILE_PATH=public\RemoteDesktopAgent.exe
set BUCKET_NAME=agents
set FILE_NAME=RemoteDesktopAgent.exe

echo File: %FILE_PATH%
echo Bucket: %BUCKET_NAME%
echo Target: %FILE_NAME%
echo.

if not exist "%FILE_PATH%" (
    echo Error: File not found: %FILE_PATH%
    pause
    exit /b 1
)

echo File found. Size:
for %%A in ("%FILE_PATH%") do echo %%~zA bytes
echo.

echo Uploading to Supabase Storage...
curl -X POST "%SUPABASE_URL%/storage/v1/object/%BUCKET_NAME%/%FILE_NAME%" ^
     -H "Authorization: Bearer %SUPABASE_KEY%" ^
     -H "apikey: %SUPABASE_KEY%" ^
     -H "Content-Type: application/octet-stream" ^
     -H "x-upsert: true" ^
     --data-binary "@%FILE_PATH%" ^
     -v

echo.
echo Upload command completed.
echo.
echo If successful, the agent should be available at:
echo %SUPABASE_URL%/storage/v1/object/public/%BUCKET_NAME%/%FILE_NAME%
echo.
pause
