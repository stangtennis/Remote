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
!define VERSION "2.62.17"
VIProductVersion "2.62.17.0"
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
    
    ; Show upgrade message
    MessageBox MB_YESNO|MB_ICONQUESTION "Remote Desktop Agent v$0 er allerede installeret.$\n$\nVil du opgradere til v${VERSION}?" IDYES upgrade IDNO abort
    
    abort:
        Abort
    
    upgrade:
        ; Close running agent
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
    
    ; Stop existing agent if running
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
    
    ; Startup shortcut (auto-start with Windows)
    CreateShortcut "$SMSTARTUP\Remote Desktop Agent.lnk" "$INSTDIR\remote-agent.exe"
    
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
    
    ; Start agent after install
    Exec "$INSTDIR\remote-agent.exe"
SectionEnd

; Uninstaller Section
Section "Uninstall"
    ; Stop agent if running
    nsExec::ExecToLog 'taskkill /F /IM remote-agent.exe'
    nsExec::ExecToLog 'taskkill /F /IM remote-agent-console.exe'
    
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
