@echo off

:: Configuration
set SERVER_ADDRESS=192.168.1.90:8000
set VENV_PATH=venv

:: Check Python installation
echo Checking Python installation...
python --version >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo Python is not installed or not in PATH
    pause
    exit /b 1
)

echo.

echo Checking virtual environment...
if not exist "%VENV_PATH%\Scripts\activate.bat" (
    echo Virtual environment not found. Creating...
    python -m venv %VENV_PATH%
    if %ERRORLEVEL% neq 0 (
        echo Failed to create virtual environment
        pause
        exit /b 1
    )
)

:: Activate virtual environment
call %VENV_PATH%\Scripts\activate.bat

:: Install dependencies with retry logic
echo Installing dependencies...
set RETRY_COUNT=3
:retry_install
pip install --only-binary=:all: -r requirements.txt
if %ERRORLEVEL% neq 0 (
    set /a RETRY_COUNT-=1
    if %RETRY_COUNT% gtr 0 (
        echo Installation failed. Retrying... (%RETRY_COUNT% attempts remaining)
        goto retry_install
    )
    echo Failed to install dependencies after 3 attempts
    echo Last error output:
    pip install --only-binary=:all: -r requirements.txt
    pause
    exit /b 1
)

echo.

echo Starting client...
python -u client/client.py --server %SERVER_ADDRESS%

:: Deactivate virtual environment
deactivate

pause
