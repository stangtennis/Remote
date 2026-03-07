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

:: Install dependencies
echo Installing dependencies...
pip install -r requirements.txt
if %ERRORLEVEL% neq 0 (
    echo Failed to install dependencies
    pause
    exit /b 1
)

echo.

echo Starting admin panel...
python -u admin/admin.py --server %SERVER_ADDRESS%
if %ERRORLEVEL% neq 0 (
    echo Failed to start admin panel
    pause
    exit /b 1
)

:: Deactivate virtual environment
deactivate

pause
