@echo off
REM Windows build script for Remote Desktop project
REM Run from project root: build-windows.bat [version]

set VERSION=%1
if "%VERSION%"=="" set VERSION=dev

echo Building Remote Desktop v%VERSION%
echo ==================================

if not exist builds mkdir builds

echo.
echo Building Controller (Windows)...
cd controller
go build -ldflags "-s -w -H windowsgui" -o ..\builds\controller-%VERSION%.exe .
if %ERRORLEVEL% EQU 0 (echo Controller built successfully) else (echo Controller build failed)
cd ..

echo.
echo Building Agent (Windows GUI)...
cd agent
go build -ldflags "-s -w -H windowsgui" -o ..\builds\remote-agent-%VERSION%.exe .\cmd\remote-agent
if %ERRORLEVEL% EQU 0 (echo Agent built successfully) else (echo Agent build failed)
cd ..

echo.
echo Building Agent Console (Windows)...
cd agent
go build -ldflags "-s -w" -o ..\builds\remote-agent-console-%VERSION%.exe .\cmd\remote-agent
if %ERRORLEVEL% EQU 0 (echo Agent Console built successfully) else (echo Agent Console build failed)
cd ..

echo.
echo ==================================
echo Build output in .\builds\
dir builds\*%VERSION%*
