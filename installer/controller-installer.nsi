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
!define VERSION "2.99.25"
VIProductVersion "2.99.25.0"
VIAddVersionKey "ProductName" "Remote Desktop Controller"
VIAddVersionKey "CompanyName" "StangTennis"
VIAddVersionKey "FileDescription" "Remote Desktop Controller with H.264 support"
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "ProductVersion" "${VERSION}"
VIAddVersionKey "LegalCopyright" "StangTennis"

; Interface Settings
!define MUI_ABORTWARNING
!define MUI_ICON "controller.ico"
!define MUI_UNICON "controller.ico"

; Welcome page branding
!define MUI_WELCOMEPAGE_TITLE_3LINES
!define MUI_WELCOMEPAGE_TITLE "Remote Desktop Controller${\n}Velkommen"
!define MUI_WELCOMEPAGE_TEXT "Denne guide hjælper dig med at installere Remote Desktop Controller v${VERSION}.$\r$\n$\r$\nControlleren bruges til at forbinde til og fjernstyre dine enheder via WebRTC.$\r$\n$\r$\nKlik Næste for at fortsætte."

; Finish page — run app + link til dashboard
!define MUI_FINISHPAGE_TITLE_3LINES
!define MUI_FINISHPAGE_TITLE "Installation fuldført"
!define MUI_FINISHPAGE_TEXT "Remote Desktop Controller v${VERSION} er installeret.$\r$\n$\r$\nDu kan nu forbinde til dine enheder."
!define MUI_FINISHPAGE_RUN "$INSTDIR\controller.exe"
!define MUI_FINISHPAGE_RUN_TEXT "Start Remote Desktop Controller"
!define MUI_FINISHPAGE_LINK "Åbn web-dashboard"
!define MUI_FINISHPAGE_LINK_LOCATION "https://dashboard.hawkeye123.dk"
!define MUI_FINISHPAGE_SHOWREADME ""
!define MUI_FINISHPAGE_SHOWREADME_NOTCHECKED
!define MUI_FINISHPAGE_SHOWREADME_TEXT "Vis README på GitHub"
!define MUI_FINISHPAGE_SHOWREADME_FUNCTION OpenReadme

; Uninstaller confirmation wording
!define MUI_UNCONFIRMPAGE_TEXT_TOP "Afinstaller Remote Desktop Controller v${VERSION}.$\r$\n$\r$\nInstallationen og tilhørende genveje fjernes."

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
SectionEnd

; Helper function — open README in browser
Function OpenReadme
    ExecShell "open" "https://github.com/stangtennis/Remote#readme"
FunctionEnd

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
