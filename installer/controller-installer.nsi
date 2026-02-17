; Remote Desktop Controller Installer
; NSIS Script for Windows

!include "MUI2.nsh"

; General
Name "Remote Desktop Controller"
OutFile "RemoteDesktopController-Setup.exe"
InstallDir "$PROGRAMFILES64\Remote Desktop Controller"
InstallDirRegKey HKLM "Software\RemoteDesktopController" "InstallDir"
RequestExecutionLevel admin

; Version info
!define VERSION "2.65.0"
VIProductVersion "2.65.0.0"
VIAddVersionKey "ProductName" "Remote Desktop Controller"
VIAddVersionKey "CompanyName" "StangTennis"
VIAddVersionKey "FileDescription" "Remote Desktop Controller with H.264 support"
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
    ReadRegStr $0 HKLM "Software\RemoteDesktopController" "Version"
    StrCmp $0 "" done

    ; In silent mode, skip the dialog and go straight to upgrade
    IfSilent upgrade

    ; Show upgrade message
    MessageBox MB_YESNO|MB_ICONQUESTION "Remote Desktop Controller v$0 er allerede installeret.$\n$\nVil du opgradere til v${VERSION}?" IDYES upgrade IDNO abort

    abort:
        Abort

    upgrade:
        ; Graceful shutdown first (without /F)
        nsExec::ExecToLog 'taskkill /IM controller.exe'
        Sleep 3000
        ; Force kill if still running
        nsExec::ExecToLog 'taskkill /F /IM controller.exe'
        Sleep 1000

    done:
FunctionEnd

; Installer Section
Section "Install"
    SetOutPath "$INSTDIR"
    
    ; Overwrite files without asking (upgrade mode)
    SetOverwrite on
    
    ; Stop controller if running (graceful first, then force)
    nsExec::ExecToLog 'taskkill /IM controller.exe'
    Sleep 2000
    nsExec::ExecToLog 'taskkill /F /IM controller.exe'
    Sleep 500
    
    ; Main application
    File "controller.exe"
    
    ; FFmpeg for H.264 decoding
    File "ffmpeg.exe"
    
    ; Create shortcuts
    CreateDirectory "$SMPROGRAMS\Remote Desktop Controller"
    CreateShortcut "$SMPROGRAMS\Remote Desktop Controller\Remote Desktop Controller.lnk" "$INSTDIR\controller.exe"
    CreateShortcut "$SMPROGRAMS\Remote Desktop Controller\Uninstall.lnk" "$INSTDIR\uninstall.exe"
    CreateShortcut "$DESKTOP\Remote Desktop Controller.lnk" "$INSTDIR\controller.exe"
    
    ; Write registry keys
    WriteRegStr HKLM "Software\RemoteDesktopController" "InstallDir" "$INSTDIR"
    WriteRegStr HKLM "Software\RemoteDesktopController" "Version" "${VERSION}"
    
    ; Add/Remove Programs entry
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RemoteDesktopController" "DisplayName" "Remote Desktop Controller"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RemoteDesktopController" "UninstallString" "$INSTDIR\uninstall.exe"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RemoteDesktopController" "DisplayVersion" "${VERSION}"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RemoteDesktopController" "Publisher" "StangTennis"
    WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RemoteDesktopController" "NoModify" 1
    WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RemoteDesktopController" "NoRepair" 1
    
    ; Create uninstaller
    WriteUninstaller "$INSTDIR\uninstall.exe"
    
    ; Start controller after install
    Exec "$INSTDIR\controller.exe"
SectionEnd

; Uninstaller Section
Section "Uninstall"
    ; Stop controller if running
    nsExec::ExecToLog 'taskkill /IM controller.exe'
    Sleep 2000
    nsExec::ExecToLog 'taskkill /F /IM controller.exe'
    Sleep 500

    ; Remove files
    Delete "$INSTDIR\controller.exe"
    Delete "$INSTDIR\ffmpeg.exe"
    Delete "$INSTDIR\uninstall.exe"
    
    ; Remove shortcuts
    Delete "$SMPROGRAMS\Remote Desktop Controller\Remote Desktop Controller.lnk"
    Delete "$SMPROGRAMS\Remote Desktop Controller\Uninstall.lnk"
    Delete "$DESKTOP\Remote Desktop Controller.lnk"
    RMDir "$SMPROGRAMS\Remote Desktop Controller"
    
    ; Remove install directory
    RMDir "$INSTDIR"
    
    ; Remove registry keys
    DeleteRegKey HKLM "Software\RemoteDesktopController"
    DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RemoteDesktopController"
SectionEnd
