; Remote Desktop portable AI support launcher
; Extracts the temporary client per-user and starts it immediately.

!include "MUI2.nsh"

Name "Remote Desktop AI Support"
OutFile "RemoteDesktopSupport-Setup.exe"
InstallDir "$LOCALAPPDATA\RemoteDesktopSupport"
RequestExecutionLevel user
SilentInstall silent
AutoCloseWindow true
ShowInstDetails nevershow

!define VERSION "2.99.25"
VIProductVersion "2.99.25.0"
VIAddVersionKey "ProductName" "Remote Desktop AI Support"
VIAddVersionKey "CompanyName" "StangTennis"
VIAddVersionKey "FileDescription" "Temporary Remote Desktop AI Support client"
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "ProductVersion" "${VERSION}"
VIAddVersionKey "LegalCopyright" "StangTennis"

Section "Portable support"
    SetOutPath "$INSTDIR"
    SetOverwrite on
    File "remote-support.exe"
    Exec '"$INSTDIR\remote-support.exe"'
SectionEnd
