; Remote Desktop Agent Installer
; NSIS Script for Windows

!include "MUI2.nsh"

; General
Name "Remote Desktop Agent"
OutFile "RemoteDesktopAgent-Setup.exe"
InstallDir "$PROGRAMFILES64\Remote Desktop Agent"
InstallDirRegKey HKLM "Software\RemoteDesktopAgent" "InstallDir"
RequestExecutionLevel admin

; Version info - will be replaced by build script
!define VERSION "2.65.0"
VIProductVersion "2.65.0.0"
VIAddVersionKey "ProductName" "Remote Desktop Agent"
VIAddVersionKey "CompanyName" "StangTennis"
VIAddVersionKey "FileDescription" "Remote Desktop Agent med H.264 support og auto-opdatering"
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "ProductVersion" "${VERSION}"

; Interface Settings
!define MUI_ABORTWARNING
!define MUI_ICON "${NSISDIR}\Contrib\Graphics\Icons\modern-install.ico"
!define MUI_UNICON "${NSISDIR}\Contrib\Graphics\Icons\modern-uninstall.ico"

; Pages
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "LICENSE.txt"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

; Languages
!insertmacro MUI_LANGUAGE "Danish"

; Check for existing installation
Function .onInit
    ; Check if already installed
    ReadRegStr $0 HKLM "Software\RemoteDesktopAgent" "Version"
    StrCmp $0 "" done

    ; In silent mode, skip the dialog and go straight to upgrade
    IfSilent upgrade

    ; Show upgrade message
    MessageBox MB_YESNO|MB_ICONQUESTION "Remote Desktop Agent v$0 er allerede installeret.$\n$\nVil du opgradere til v${VERSION}?" IDYES upgrade IDNO abort

    abort:
        Abort

    upgrade:
        ; Stop service if running
        nsExec::ExecToLog 'sc stop RemoteDesktopAgent'
        Sleep 2000
        ; Graceful shutdown first (without /F) for legacy process mode
        nsExec::ExecToLog 'taskkill /IM remote-agent.exe'
        nsExec::ExecToLog 'taskkill /IM remote-agent-console.exe'
        Sleep 3000
        ; Force kill if still running
        nsExec::ExecToLog 'taskkill /F /IM remote-agent.exe'
        nsExec::ExecToLog 'taskkill /F /IM remote-agent-console.exe'
        Sleep 1000

    done:
FunctionEnd

; Installer Section
Section "Install"
    SetOutPath "$INSTDIR"
    
    ; Overwrite files without asking (upgrade mode)
    SetOverwrite on
    
    ; Stop existing agent if running (graceful first, then force)
    nsExec::ExecToLog 'taskkill /IM remote-agent.exe'
    nsExec::ExecToLog 'taskkill /IM remote-agent-console.exe'
    Sleep 2000
    nsExec::ExecToLog 'taskkill /F /IM remote-agent.exe'
    nsExec::ExecToLog 'taskkill /F /IM remote-agent-console.exe'
    Sleep 500
    
    ; Main application (GUI version)
    File "remote-agent.exe"
    
    ; Console version for debugging
    File "remote-agent-console.exe"
    
    ; OpenH264 DLL for H.264 encoding
    File /nonfatal "openh264-2.1.1-win64.dll"
    
    ; Create shortcuts
    CreateDirectory "$SMPROGRAMS\Remote Desktop Agent"
    CreateShortcut "$SMPROGRAMS\Remote Desktop Agent\Remote Desktop Agent.lnk" "$INSTDIR\remote-agent.exe"
    CreateShortcut "$SMPROGRAMS\Remote Desktop Agent\Remote Desktop Agent (Console).lnk" "$INSTDIR\remote-agent-console.exe"
    CreateShortcut "$SMPROGRAMS\Remote Desktop Agent\Uninstall.lnk" "$INSTDIR\uninstall.exe"
    
    ; Desktop shortcut
    CreateShortcut "$DESKTOP\Remote Desktop Agent.lnk" "$INSTDIR\remote-agent.exe"

    ; Remove old startup shortcut (replaced by Windows Service)
    Delete "$SMSTARTUP\Remote Desktop Agent.lnk"

    ; Stop and remove existing service (if upgrading)
    nsExec::ExecToLog 'sc stop RemoteDesktopAgent'
    Sleep 2000
    nsExec::ExecToLog 'sc delete RemoteDesktopAgent'
    Sleep 1000

    ; Register as Windows Service (LocalSystem = has SeTcbPrivilege for Session 0 capture)
    nsExec::ExecToLog 'sc create RemoteDesktopAgent binPath= "$INSTDIR\remote-agent.exe" start= auto obj= LocalSystem DisplayName= "Remote Desktop Agent"'
    nsExec::ExecToLog 'sc description RemoteDesktopAgent "Remote Desktop Agent - WebRTC remote desktop"'
    nsExec::ExecToLog 'sc failure RemoteDesktopAgent reset= 86400 actions= restart/5000/restart/10000/restart/30000'
    
    ; Write registry keys
    WriteRegStr HKLM "Software\RemoteDesktopAgent" "InstallDir" "$INSTDIR"
    WriteRegStr HKLM "Software\RemoteDesktopAgent" "Version" "${VERSION}"
    
    ; Add/Remove Programs entry
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RemoteDesktopAgent" "DisplayName" "Remote Desktop Agent"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RemoteDesktopAgent" "UninstallString" "$INSTDIR\uninstall.exe"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RemoteDesktopAgent" "DisplayVersion" "${VERSION}"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RemoteDesktopAgent" "Publisher" "StangTennis"
    WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RemoteDesktopAgent" "NoModify" 1
    WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RemoteDesktopAgent" "NoRepair" 1
    
    ; Create uninstaller
    WriteUninstaller "$INSTDIR\uninstall.exe"
    
    ; Start service after install
    nsExec::ExecToLog 'sc start RemoteDesktopAgent'
SectionEnd

; Uninstaller Section
Section "Uninstall"
    ; Stop and remove Windows Service
    nsExec::ExecToLog 'sc stop RemoteDesktopAgent'
    Sleep 2000
    nsExec::ExecToLog 'sc delete RemoteDesktopAgent'

    ; Stop agent if running as process (legacy)
    nsExec::ExecToLog 'taskkill /F /IM remote-agent.exe'
    nsExec::ExecToLog 'taskkill /F /IM remote-agent-console.exe'

    ; Remove old startup shortcut
    Delete "$SMSTARTUP\Remote Desktop Agent.lnk"

    ; Remove files
    Delete "$INSTDIR\remote-agent.exe"
    Delete "$INSTDIR\remote-agent-console.exe"
    Delete "$INSTDIR\openh264-2.1.1-win64.dll"
    Delete "$INSTDIR\uninstall.exe"
    Delete "$INSTDIR\*.log"
    
    ; Remove shortcuts
    Delete "$SMPROGRAMS\Remote Desktop Agent\Remote Desktop Agent.lnk"
    Delete "$SMPROGRAMS\Remote Desktop Agent\Remote Desktop Agent (Console).lnk"
    Delete "$SMPROGRAMS\Remote Desktop Agent\Uninstall.lnk"
    Delete "$DESKTOP\Remote Desktop Agent.lnk"
    Delete "$SMSTARTUP\Remote Desktop Agent.lnk"
    RMDir "$SMPROGRAMS\Remote Desktop Agent"
    
    ; Remove install directory
    RMDir "$INSTDIR"
    
    ; Remove registry keys
    DeleteRegKey HKLM "Software\RemoteDesktopAgent"
    DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RemoteDesktopAgent"
SectionEnd
