@echo off
echo Requesting administrator privileges...
powershell Start-Process python -ArgumentList "install_certificate.py" -Verb RunAs
pause
