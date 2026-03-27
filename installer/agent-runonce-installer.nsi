; Remote Desktop Agent - Run Once (Portable)
; Lightweight installer — no service, no auto-start
; Just installs exe + DLLs and runs the agent once

!include "MUI2.nsh"

Name "Remote Desktop Agent (Run Once)"
OutFile "RemoteDesktopAgent-RunOnce-Setup.exe"
InstallDir "$LOCALAPPDATA\Remote Desktop Agent"
RequestExecutionLevel user

; Version info
!define VERSION "2.99.36"
VIProductVersion "2.99.36.0"
VIAddVersionKey "ProductName" "Remote Desktop Agent (Run Once)"
VIAddVersionKey "CompanyName" "StangTennis"
VIAddVersionKey "FileDescription" "Portable Remote Desktop Agent — kør uden installation"
VIAddVersionKey "FileVersion" "${VERSION}"

; Interface
!define MUI_ABORTWARNING
!define MUI_ICON "agent.ico"

; Simple pages
!insertmacro MUI_PAGE_INSTFILES
!define MUI_FINISHPAGE_RUN "$INSTDIR\remote-agent-console.exe"
!define MUI_FINISHPAGE_RUN_TEXT "Start Remote Desktop Agent nu"
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_LANGUAGE "Danish"

Section "Install"
    SetOutPath "$INSTDIR"
    SetOverwrite on

    ; Console agent (no GUI subsystem — shows login dialog, then runs)
    File "remote-agent-console.exe"

    ; Required DLLs
    File /nonfatal "openh264-2.1.1-win64.dll"
    File /nonfatal "libturbojpeg.dll"

    ; Desktop shortcut
    CreateShortcut "$DESKTOP\Remote Desktop Agent (Run Once).lnk" "$INSTDIR\remote-agent-console.exe"
SectionEnd
