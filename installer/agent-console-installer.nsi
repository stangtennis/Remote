; Remote Desktop Agent Console Installer
; NSIS Script for Windows - Console version med logs

!include "MUI2.nsh"

; General
Name "Remote Desktop Agent Console"
OutFile "RemoteDesktopAgentConsole-Setup.exe"
InstallDir "$PROGRAMFILES64\Remote Desktop Agent Console"
InstallDirRegKey HKLM "Software\RemoteDesktopAgentConsole" "InstallDir"
RequestExecutionLevel admin

; Version info - will be replaced by build script
!define VERSION "2.62.6"
VIProductVersion "2.62.6.0"
VIAddVersionKey "ProductName" "Remote Desktop Agent Console"
VIAddVersionKey "CompanyName" "StangTennis"
VIAddVersionKey "FileDescription" "Remote Desktop Agent Console med H.264 support og logs"
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "ProductVersion" "${VERSION}"
VIAddVersionKey "LegalCopyright" "StangTennis"

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
    ReadRegStr $0 HKLM "Software\RemoteDesktopAgentConsole" "Version"
    StrCmp $0 "" done
    
    ; Show upgrade message
    MessageBox MB_YESNO|MB_ICONQUESTION "Remote Desktop Agent Console v$0 er allerede installeret.$\n$\nVil du opgradere til v${VERSION}?" IDYES upgrade IDNO abort
    
    abort:
        Abort
    
    upgrade:
        ; Close running agent
        nsExec::ExecToLog 'taskkill /F /IM remote-agent-console.exe'
        Sleep 1000
    
    done:
FunctionEnd

; Installer Section
Section "Install"
    SetOutPath "$INSTDIR"
    
    ; Overwrite files without asking (upgrade mode)
    SetOverwrite on
    
    ; Stop agent if running
    nsExec::ExecToLog 'taskkill /F /IM remote-agent-console.exe'
    Sleep 500
    
    ; Console version (with logs)
    File "remote-agent-console.exe"
    
    ; OpenH264 DLL for H.264 encoding
    File "openh264-2.1.1-win64.dll"
    
    ; Create shortcuts
    CreateDirectory "$SMPROGRAMS\Remote Desktop Agent Console"
    CreateShortcut "$SMPROGRAMS\Remote Desktop Agent Console\Remote Desktop Agent Console.lnk" "$INSTDIR\remote-agent-console.exe"
    CreateShortcut "$SMPROGRAMS\Remote Desktop Agent Console\Uninstall.lnk" "$INSTDIR\uninstall.exe"
    
    ; Desktop shortcut
    CreateShortcut "$DESKTOP\Remote Desktop Agent Console.lnk" "$INSTDIR\remote-agent-console.exe"
    
    ; Write registry keys
    WriteRegStr HKLM "Software\RemoteDesktopAgentConsole" "InstallDir" "$INSTDIR"
    WriteRegStr HKLM "Software\RemoteDesktopAgentConsole" "Version" "${VERSION}"
    
    ; Add/Remove Programs entry
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RemoteDesktopAgentConsole" "DisplayName" "Remote Desktop Agent Console"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RemoteDesktopAgentConsole" "UninstallString" "$INSTDIR\uninstall.exe"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RemoteDesktopAgentConsole" "DisplayVersion" "${VERSION}"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RemoteDesktopAgentConsole" "Publisher" "StangTennis"
    WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RemoteDesktopAgentConsole" "NoModify" 1
    WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RemoteDesktopAgentConsole" "NoRepair" 1
    
    ; Create uninstaller
    WriteUninstaller "$INSTDIR\uninstall.exe"
    
    ; Start agent console after install
    Exec "$INSTDIR\remote-agent-console.exe"
SectionEnd

; Uninstaller Section
Section "Uninstall"
    ; Stop agent if running
    nsExec::ExecToLog 'taskkill /F /IM remote-agent-console.exe'
    
    ; Remove files
    Delete "$INSTDIR\remote-agent-console.exe"
    Delete "$INSTDIR\openh264-2.1.1-win64.dll"
    Delete "$INSTDIR\uninstall.exe"
    Delete "$INSTDIR\*.log"
    
    ; Remove shortcuts
    Delete "$SMPROGRAMS\Remote Desktop Agent Console\Remote Desktop Agent Console.lnk"
    Delete "$SMPROGRAMS\Remote Desktop Agent Console\Uninstall.lnk"
    Delete "$DESKTOP\Remote Desktop Agent Console.lnk"
    RMDir "$SMPROGRAMS\Remote Desktop Agent Console"
    
    ; Remove install directory
    RMDir "$INSTDIR"
    
    ; Remove registry keys
    DeleteRegKey HKLM "Software\RemoteDesktopAgentConsole"
    DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RemoteDesktopAgentConsole"
SectionEnd
