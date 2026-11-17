Unicode true
RequestExecutionLevel user
; Per-file decompression avoids a full payload copy in the system TEMP drive.
SetCompressor lzma
SetCompressorDictSize 32
ManifestDPIAware true
ManifestSupportedOS all

!include MUI2.nsh
!define MUI_ICON "Eden.ico"
!define MUI_UNICON "Eden.ico"
!include LogicLib.nsh
!include FileFunc.nsh
!include x64.nsh
!include WinVer.nsh
!include nsDialogs.nsh

!ifndef APP_DIR
  !error "APP_DIR is required"
!endif
!ifndef VERSION
  !error "VERSION is required"
!endif
!ifndef VERSION_NUMERIC
  !error "VERSION_NUMERIC is required"
!endif
!ifndef OUTPUT_FILE
  !error "OUTPUT_FILE is required"
!endif

!ifdef NSIS_WIN32_MAKENSIS
  !define INSTALL_HELPER_SOURCE "${APP_DIR}\resources\eden-install.exe"
!else
  !define INSTALL_HELPER_SOURCE "${APP_DIR}/resources/eden-install.exe"
!endif

!define APP_ID "io.github.carterleealt.claude-eden"
!define UNINSTALL_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\ClaudeEden"
!define WEBVIEW2_APP_ID "{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"
Name "Claude Eden ${VERSION}"
OutFile "${OUTPUT_FILE}"
InstallDir "$LOCALAPPDATA\Programs\Eden"
InstallDirRegKey HKCU "${UNINSTALL_KEY}" "InstallLocation"
VIProductVersion "${VERSION_NUMERIC}"
VIAddVersionKey /LANG=1033 "ProductName" "Claude Eden"
VIAddVersionKey /LANG=1033 "ProductVersion" "${VERSION}"
VIAddVersionKey /LANG=1033 "FileVersion" "${VERSION}"
VIAddVersionKey /LANG=1033 "FileDescription" "Claude Eden Windows Installer"
VIAddVersionKey /LANG=1033 "LegalCopyright" "Claude Eden contributors"

!ifdef SIGN_SCRIPT
  !uninstfinalize '"${SIGN_PWSH}" -NoProfile -File "${SIGN_SCRIPT}" -Path "%1"' = 0
!endif

!define MUI_CUSTOMFUNCTION_ABORT ConfirmInstallerAbort
!define MUI_CUSTOMFUNCTION_GUIINIT RefreshFontControls
!define MUI_CUSTOMFUNCTION_UNGUIINIT un.RefreshFontControls
!define MUI_FINISHPAGE_RUN "$INSTDIR\Eden.exe"
!define MUI_FINISHPAGE_RUN_NOTCHECKED
!define MUI_FINISHPAGE_TEXT "Claude Eden is installed.$\r$\n$\r$\nIf virtualization is unavailable, enable Windows Hypervisor Platform in Windows Features and restart Windows. Your projects and Claude Eden data are preserved during upgrades and uninstall."
!define MUI_PAGE_CUSTOMFUNCTION_SHOW RefreshFontControls
!insertmacro MUI_PAGE_WELCOME
!define MUI_PAGE_CUSTOMFUNCTION_SHOW RefreshFontControls
!insertmacro MUI_PAGE_LICENSE "${APP_DIR}/LICENSE.txt"
!define MUI_DIRECTORYPAGE_TEXT_TOP "$(EdenDirectoryHelp)"
!define MUI_PAGE_CUSTOMFUNCTION_SHOW RefreshFontControls
!insertmacro MUI_PAGE_DIRECTORY
Page custom ShortcutOptionsPage ShortcutOptionsLeave
!define MUI_PAGE_CUSTOMFUNCTION_SHOW RefreshFontControls
!insertmacro MUI_PAGE_INSTFILES
!define MUI_PAGE_CUSTOMFUNCTION_SHOW RefreshFontControls
!insertmacro MUI_PAGE_FINISH
!define MUI_PAGE_CUSTOMFUNCTION_SHOW un.RefreshFontControls
!insertmacro MUI_UNPAGE_CONFIRM
!define MUI_PAGE_CUSTOMFUNCTION_SHOW un.RefreshFontControls
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "SimpChinese"
; Resource layout reserves 11-point space; controls use 14 px at 96 DPI.
SetFont "Eden UI" 11

LangString EdenDirectoryHelp ${LANG_ENGLISH} "Choose where to install the Claude Eden application. On first launch, you can choose a separate drive for the Linux image, download cache, and virtual disks before starting the environment."
LangString EdenDirectoryHelp ${LANG_SIMPCHINESE} "请选择 Claude Eden 程序的安装目录。首次打开 Claude Eden 后，可以在启动环境前单独选择存放 Linux 镜像、下载缓存和虚拟磁盘的磁盘。"
LangString EdenShortcutsTitle ${LANG_ENGLISH} "Shortcuts"
LangString EdenShortcutsTitle ${LANG_SIMPCHINESE} "快捷方式"
LangString EdenShortcutsHelp ${LANG_ENGLISH} "Choose where to add Claude Eden shortcuts for your Windows account."
LangString EdenShortcutsHelp ${LANG_SIMPCHINESE} "选择为当前 Windows 用户添加的 Claude Eden 快捷方式。"
LangString EdenDesktopShortcut ${LANG_ENGLISH} "Add a desktop shortcut"
LangString EdenDesktopShortcut ${LANG_SIMPCHINESE} "添加桌面快捷方式"
LangString EdenStartMenuShortcuts ${LANG_ENGLISH} "Add to the Start menu"
LangString EdenStartMenuShortcuts ${LANG_SIMPCHINESE} "添加到开始菜单"
LangString EdenAbortQuestion ${LANG_ENGLISH} "Are you sure you want to quit the Claude Eden installer?"
LangString EdenAbortQuestion ${LANG_SIMPCHINESE} "确定要退出 Claude Eden 安装程序吗？"

Var UpdateMode
Var StageDir
Var BackupDir
Var ExistingInstall
Var DesktopShortcut
Var StartMenuShortcuts
Var DesktopShortcutControl
Var StartMenuShortcutsControl
Var InstallationMutex
Var TransactionRecord
Var DiagnosticFile
Var InstallPhase
Var InstallOutcome
Var HelperExit
Var DiagnosticArguments
Var UIFont
Var UIFontBold
Var UIFontInfo
Var UIFontDialogText
Var UIFontDialogFlags
Var UIFontDialogResult
Var UIFontDialogWindow
Var UIFontDialogCallback
Var UIFontDialogHook

!macro LoadPrivateFonts
  InitPluginsDir
  File /oname=$PLUGINSDIR\EdenUI-Regular.ttf "${__FILEDIR__}\..\..\assets\fonts\EdenUI-Regular.ttf"
  File /oname=$PLUGINSDIR\EdenUI-Bold.ttf "${__FILEDIR__}\..\..\assets\fonts\EdenUI-Bold.ttf"
  System::Call 'gdi32::AddFontResourceExW(w "$PLUGINSDIR\EdenUI-Regular.ttf", i 0x10, p 0) i.r0'
  ${If} $0 == 0
    SetErrorLevel 2
    Abort "Unable to load the bundled interface font. Please run the installer again."
  ${EndIf}
  System::Call 'gdi32::AddFontResourceExW(w "$PLUGINSDIR\EdenUI-Bold.ttf", i 0x10, p 0) i.r0'
  ${If} $0 == 0
    SetErrorLevel 2
    Abort "Unable to load the bundled bold font. Please run the installer again."
  ${EndIf}
!macroend

!macro FontControlFunctions PREFIX
Function ${PREFIX}InitializeFontControls
  System::Call 'user32::GetDC(p $HWNDPARENT) p.r1'
  System::Call 'gdi32::GetDeviceCaps(p r1, i 90) i.r0'
  System::Call 'user32::ReleaseDC(p $HWNDPARENT, p r1)'
  IntOp $0 $0 * -14
  IntOp $0 $0 / 96
  System::Call 'gdi32::CreateFontW(i r0, i 0, i 0, i 0, i 400, i 0, i 0, i 0, i 1, i 0, i 0, i 5, i 0, w "Eden UI") p.s'
  Pop $UIFont
  System::Call 'gdi32::CreateFontW(i r0, i 0, i 0, i 0, i 700, i 0, i 0, i 0, i 1, i 0, i 0, i 5, i 0, w "Eden UI") p.s'
  Pop $UIFontBold
  System::Alloc 92
  Pop $UIFontInfo
  Call ${PREFIX}RefreshFontControls
FunctionEnd

Function ${PREFIX}RefreshFontControls
  Push $HWNDPARENT
  Call ${PREFIX}ApplyControlFont
FunctionEnd

Function ${PREFIX}ApplyControlFont
  Exch $0
  Push $1
  Push $2
  Push $3
  SendMessage $0 ${WM_GETFONT} 0 0 $2
  StrCpy $3 400
  ${If} $2 != 0
    System::Call 'gdi32::GetObjectW(p r2, i 92, p $UIFontInfo)'
    IntOp $2 $UIFontInfo + 16
    System::Call '*$2(i.r3)'
  ${EndIf}
  ${If} $3 >= 600
    SendMessage $0 ${WM_SETFONT} $UIFontBold 1
  ${Else}
    SendMessage $0 ${WM_SETFONT} $UIFont 1
  ${EndIf}
  System::Call 'user32::GetWindow(p r0, i 5) p.r1'
  ${DoWhile} $1 != 0
    Push $1
    Call ${PREFIX}ApplyControlFont
    System::Call 'user32::GetWindow(p r1, i 2) p.r1'
  ${Loop}
  Pop $3
  Pop $2
  Pop $1
  Pop $0
FunctionEnd
!macroend
!insertmacro FontControlFunctions ""
!insertmacro FontControlFunctions "un."

; A thread-local hook applies private fonts before the native dialog first paints.
Function ShowFontMessage
  System::Store "s"
  System::Get '(i .r0, p .r1, p .r2) p r3 r4'
  Pop $UIFontDialogCallback
  System::Call 'kernel32::GetCurrentThreadId() i.r5'
  System::Call 'user32::SetWindowsHookExW(i 5, k $UIFontDialogCallback, p 0, i r5) p.s'
  Pop $UIFontDialogHook
  StrCpy $4 ""
  System::Call 'user32::MessageBoxW(p $HWNDPARENT, w "$UIFontDialogText", w "Claude Eden", i $UIFontDialogFlags) i.s'
  font_message_callback:
    StrCpy $5 $4 8
    ${If} $5 == "callback"
      ${If} $0 == 5
        System::Call 'user32::UnhookWindowsHookEx(p $UIFontDialogHook)'
        StrCpy $UIFontDialogHook 0
        StrCpy $UIFontDialogWindow $1
        Call ApplyFontMessage
      ${EndIf}
      System::Call 'user32::CallNextHookEx(p 0, i r0, p r1, p r2) p.r3'
      StrCpy $4 ""
      System::Call $UIFontDialogCallback
      Goto font_message_callback
    ${EndIf}
  Pop $UIFontDialogResult
  ${If} $UIFontDialogHook != 0
    System::Call 'user32::UnhookWindowsHookEx(p $UIFontDialogHook)'
  ${EndIf}
  System::Free $UIFontDialogCallback
  System::Store "l"
FunctionEnd

Function ApplyFontMessage
  System::Store "s"
  StrCpy $0 $UIFontDialogWindow
  Push $0
  Call ApplyControlFont
  GetDlgItem $5 $0 65535
  GetDlgItem $6 $0 6
  ${If} $6 == 0
    GetDlgItem $6 $0 1
  ${EndIf}
  System::Alloc 16
  Pop $8
  ${If} $5 != 0
  ${AndIf} $6 != 0
    ; Use the available text area above the buttons for the bundled face's metrics.
    System::Call 'user32::GetWindowRect(p r6, p r8)'
    System::Call 'user32::MapWindowPoints(p 0, p r0, p r8, i 2)'
    System::Call '*$8(i, i.r7)'
    System::Call 'user32::GetWindowRect(p r5, p r8)'
    System::Call 'user32::MapWindowPoints(p 0, p r0, p r8, i 2)'
    System::Call '*$8(i.r1, i.r2, i.r3, i.r4)'
    IntOp $3 $3 - $1
    IntOp $4 $7 - $2
    IntOp $4 $4 - 16
    System::Call 'user32::SetWindowPos(p r5, p 0, i 0, i 0, i r3, i r4, i 6)'
  ${EndIf}
  System::Free $8
  System::Store "l"
FunctionEnd
Function ConfirmInstallerAbort
  StrCpy $UIFontDialogText "$(EdenAbortQuestion)"
  StrCpy $UIFontDialogFlags 0x34
  Call ShowFontMessage
  ${If} $UIFontDialogResult != 6
    Abort
  ${EndIf}
FunctionEnd

!macro ReleasePrivateFonts
  System::Call 'gdi32::DeleteObject(p $UIFont)'
  System::Call 'gdi32::DeleteObject(p $UIFontBold)'
  System::Free $UIFontInfo
  System::Call 'gdi32::RemoveFontResourceExW(w "$PLUGINSDIR\EdenUI-Regular.ttf", i 0x10, p 0)'
  System::Call 'gdi32::RemoveFontResourceExW(w "$PLUGINSDIR\EdenUI-Bold.ttf", i 0x10, p 0)'
!macroend

Function WriteInstallDiagnostic
  ${If} $DiagnosticFile != ""
    Push $2
    Push $3
    StrCpy $3 0
    IfErrors 0 +2
      StrCpy $3 1
    ClearErrors
    FileOpen $2 "$DiagnosticFile" w
    ${IfNot} ${Errors}
      FileWrite $2 '{"phase":"$InstallPhase","status":"$InstallOutcome","helperExit":"$HelperExit"}$\r$\n'
      FileClose $2
    ${EndIf}
    ${If} $3 == 1
      SetErrors
    ${Else}
      ClearErrors
    ${EndIf}
    Pop $3
    Pop $2
  ${EndIf}
FunctionEnd

!macro InstallPhase NAME
  StrCpy $InstallPhase "${NAME}"
  Call WriteInstallDiagnostic
!macroend

Function un.WriteInstallDiagnostic
  ${If} $DiagnosticFile != ""
    Push $2
    Push $3
    StrCpy $3 0
    IfErrors 0 +2
      StrCpy $3 1
    ClearErrors
    FileOpen $2 "$DiagnosticFile" w
    ${IfNot} ${Errors}
      FileWrite $2 '{"phase":"$InstallPhase","status":"$InstallOutcome","helperExit":"$HelperExit"}$\r$\n'
      FileClose $2
    ${EndIf}
    ${If} $3 == 1
      SetErrors
    ${Else}
      ClearErrors
    ${EndIf}
    Pop $3
    Pop $2
  ${EndIf}
FunctionEnd

!macro UninstallPhase NAME
  StrCpy $InstallPhase "${NAME}"
  Call un.WriteInstallDiagnostic
!macroend

Function un.onUninstFailed
  StrCpy $InstallOutcome "failed"
  Call un.WriteInstallDiagnostic
FunctionEnd

Function un.onUninstSuccess
  StrCpy $InstallOutcome "success"
  !insertmacro UninstallPhase "completed"
FunctionEnd

Function .onInstFailed
  StrCpy $InstallOutcome "failed"
  Call WriteInstallDiagnostic
FunctionEnd

Function .onInstSuccess
  StrCpy $InstallOutcome "success"
  Call WriteInstallDiagnostic
FunctionEnd

!macro AcquireInstallationMutex
  ; NSIS and the Go update helper reject an existing named object and keep
  ; their handle until the transaction ends. No caller waits for ownership.
  System::Call 'kernel32::CreateMutexW(p 0, i 0, w "Global\Eden.Installation") p.r0 ?e'
  Pop $1
  ${If} $0 == 0
    SetErrorLevel 2
    Abort "无法锁定 Claude Eden 安装操作，请核对系统权限后重试。"
  ${EndIf}
  StrCpy $InstallationMutex $0
  ${If} $1 == 183
    SetErrorLevel 2
    Abort "另一个 Claude Eden 安装或卸载仍在进行，请等待完成后重试。"
  ${EndIf}
!macroend

Function .onGUIEnd
  !insertmacro ReleasePrivateFonts
  ${If} $InstallationMutex != ""
    System::Call 'kernel32::CloseHandle(p $InstallationMutex)'
  ${EndIf}
FunctionEnd

Function un.onGUIEnd
  !insertmacro ReleasePrivateFonts
  ${If} $InstallationMutex != ""
    System::Call 'kernel32::CloseHandle(p $InstallationMutex)'
  ${EndIf}
FunctionEnd

Function LoadShortcutOptions
  ; Fresh installs enable both entries. Older installs inherit actual shortcuts.
  StrCpy $DesktopShortcut 1
  StrCpy $StartMenuShortcuts 1
  ReadRegStr $0 HKCU "${UNINSTALL_KEY}" "InstallLocation"
  ${If} $0 != ""
    StrCpy $DesktopShortcut 0
    StrCpy $StartMenuShortcuts 0
    ${If} ${FileExists} "$DESKTOP\Claude Eden.lnk"
      StrCpy $DesktopShortcut 1
    ${EndIf}
    ${If} ${FileExists} "$SMPROGRAMS\Claude Eden\Claude Eden.lnk"
      StrCpy $StartMenuShortcuts 1
    ${EndIf}
  ${EndIf}
  ClearErrors
  ReadRegDWORD $0 HKCU "${UNINSTALL_KEY}" "DesktopShortcut"
  ${IfNot} ${Errors}
    StrCpy $DesktopShortcut $0
  ${EndIf}
  ClearErrors
  ReadRegDWORD $0 HKCU "${UNINSTALL_KEY}" "StartMenuShortcuts"
  ${IfNot} ${Errors}
    StrCpy $StartMenuShortcuts $0
  ${EndIf}
  ${GetParameters} $0
  ClearErrors
  ${GetOptions} $0 "/DESKTOPSHORTCUT=" $1
  ${IfNot} ${Errors}
    ${If} $1 != "0"
    ${AndIf} $1 != "1"
      SetErrorLevel 2
      Abort "DESKTOPSHORTCUT must be 0 or 1."
    ${EndIf}
    StrCpy $DesktopShortcut $1
  ${EndIf}
  ClearErrors
  ${GetOptions} $0 "/STARTMENUSHORTCUT=" $1
  ${IfNot} ${Errors}
    ${If} $1 != "0"
    ${AndIf} $1 != "1"
      SetErrorLevel 2
      Abort "STARTMENUSHORTCUT must be 0 or 1."
    ${EndIf}
    StrCpy $StartMenuShortcuts $1
  ${EndIf}
FunctionEnd

Function ShortcutOptionsPage
  !insertmacro MUI_HEADER_TEXT "$(EdenShortcutsTitle)" "$(EdenShortcutsHelp)"
  nsDialogs::Create 1018
  Pop $0
  ${If} $0 == error
    Abort
  ${EndIf}
  ${NSD_CreateCheckbox} 0 8u 100% 16u "$(EdenDesktopShortcut)"
  Pop $DesktopShortcutControl
  ${NSD_SetState} $DesktopShortcutControl $DesktopShortcut
  ${NSD_CreateCheckbox} 0 36u 100% 16u "$(EdenStartMenuShortcuts)"
  Pop $StartMenuShortcutsControl
  ${NSD_SetState} $StartMenuShortcutsControl $StartMenuShortcuts
  ${NSD_OnBack} ShortcutOptionsLeave
  Call RefreshFontControls
  nsDialogs::Show
FunctionEnd

Function ShortcutOptionsLeave
  ${NSD_GetState} $DesktopShortcutControl $DesktopShortcut
  ${NSD_GetState} $StartMenuShortcutsControl $StartMenuShortcuts
FunctionEnd

Function SetDefaultInstallDirectory
  ; NSIS removes /D= from $CMDLINE, so inspect the original command line.
  System::Call 'kernel32::GetCommandLineW() w .r0'
  ClearErrors
  ${GetOptions} $0 "/D=" $1
  ${IfNot} ${Errors}
    Return
  ${EndIf}
  ; Keep upgrades in the registered location, including installations on C:.
  ReadRegStr $0 HKCU "${UNINSTALL_KEY}" "InstallLocation"
  ${If} $0 != ""
    StrCpy $INSTDIR $0
    Return
  ${EndIf}
  System::Call 'kernel32::GetDriveTypeW(w "D:\") i .r0'
  ${If} $0 == 3
    StrCpy $INSTDIR "D:\Eden"
  ${EndIf}
FunctionEnd

!macro CheckStopped FILE
  ${If} ${FileExists} "$INSTDIR\${FILE}"
    ; Exclusive write access also detects images whose VM/controller has no window.
    ; Process exit can precede release of its executable's sharing lock.
    ; Retry sharing/lock violations briefly; never replace an image still in use.
    Push $1
    Push $2
    StrCpy $1 0
    ${Do}
      System::Call 'kernel32::CreateFileW(w "$INSTDIR\${FILE}", i 0x40000000, i 0, p 0, i 3, i 0, p 0) p .r0 ?e'
      Pop $2
      ${If} $0 != -1
        ${ExitDo}
      ${EndIf}
      ${If} $2 != 32
      ${AndIf} $2 != 33
        ${ExitDo}
      ${EndIf}
      IntOp $1 $1 + 1
      ${If} $1 >= 30
        ${ExitDo}
      ${EndIf}
      Sleep 100
    ${Loop}
    Pop $2
    Pop $1
    ${If} $0 == -1
      StrCpy $HelperExit "application_file_in_use"
      SetErrorLevel 2
      Abort "Quit Claude Eden and stop its environment before installing or uninstalling."
    ${EndIf}
    System::Call 'kernel32::CloseHandle(p r0)'
  ${EndIf}
!macroend

Function .onInit
  !insertmacro LoadPrivateFonts
  Call InitializeFontControls
  StrCpy $InstallOutcome "running"
  StrCpy $HelperExit ""
  ${GetParameters} $0
  ${GetOptions} $0 "/DIAGNOSTICS=" $DiagnosticFile
  StrCpy $DiagnosticArguments ""
  ${If} $DiagnosticFile != ""
    StrCpy $DiagnosticArguments '-Diagnostics "$DiagnosticFile.helper.json"'
  ${EndIf}
  !insertmacro InstallPhase "initializing"
  SetShellVarContext current
  SetRegView 64
  ${IfNot} ${IsNativeAMD64}
    StrCpy $UIFontDialogText "This Claude Eden package requires an Intel/AMD 64-bit version of Windows. Windows on ARM and 32-bit Windows are not supported."
    StrCpy $UIFontDialogFlags 0x10
    Call ShowFontMessage
    SetErrorLevel 1633
    Quit
  ${EndIf}
  ${IfNot} ${AtLeastWin10}
    StrCpy $UIFontDialogText "This Claude Eden package requires Windows 10 x64."
    StrCpy $UIFontDialogFlags 0x10
    Call ShowFontMessage
    SetErrorLevel 1633
    Quit
  ${EndIf}
  System::Call 'kernel32::GetModuleHandleW(w "kernel32.dll") p.r0'
  System::Call 'kernel32::GetProcAddress(p r0, m "CreatePseudoConsole") p.r1'
  System::Call 'kernel32::GetProcAddress(p r0, m "ResizePseudoConsole") p.r2'
  System::Call 'kernel32::GetProcAddress(p r0, m "ClosePseudoConsole") p.r3'
  ${If} $1 == 0
  ${OrIf} $2 == 0
  ${OrIf} $3 == 0
    StrCpy $UIFontDialogText "Windows is missing the ConPTY terminal APIs. Update to a Windows 10 version that provides them (Microsoft introduced ConPTY in version 1809)."
    StrCpy $UIFontDialogFlags 0x10
    Call ShowFontMessage
    SetErrorLevel 1633
    Quit
  ${EndIf}
  Call SetDefaultInstallDirectory
  Call LoadShortcutOptions
  StrCpy $UpdateMode "0"
  ${GetParameters} $0
  ClearErrors
  ${GetOptions} $0 "/UPDATE" $1
  ${IfNot} ${Errors}
    StrCpy $UpdateMode "1"
    SetSilent silent
  ${EndIf}
FunctionEnd

Function CheckDestination
  ${GetRoot} "$INSTDIR" $0
  ${If} $0 == ""
  ${OrIf} $INSTDIR == $0
  ${OrIf} $INSTDIR == "$0\"
    SetErrorLevel 2
    Abort "Choose a dedicated application folder."
  ${EndIf}
  StrCpy $ExistingInstall "0"
  IfFileExists "$INSTDIR\Eden.install-id" existing
  ; IfFileExists with a wildcard also matches an empty directory.
  ${DirState} "$INSTDIR" $0
  StrCmp $0 1 foreign done
  existing:
    FileOpen $0 "$INSTDIR\Eden.install-id" r
    FileRead $0 $1
    FileClose $0
    ${If} $1 != "${APP_ID}"
      Goto foreign
    ${EndIf}
    StrCpy $ExistingInstall "1"
    ; Never stop processes by their name or force-close user workloads.
    !insertmacro CheckStopped "Eden.exe"
    !insertmacro CheckStopped "resources\eden.exe"
    !insertmacro CheckStopped "resources\mihomo.exe"
    !insertmacro CheckStopped "resources\runtime\qemu\qemu-system-x86_64.exe"
    Goto done
  foreign:
    SetErrorLevel 2
    Abort "The destination contains files from another installation. Choose an empty folder."
  done:
    ClearErrors
FunctionEnd

Function RestorePreservedFiles
  nsExec::ExecToStack '"$PLUGINSDIR\eden-install.exe" -Operation restore -Target "$BackupDir" -Source "$StageDir" -Receipt "$TransactionRecord" $DiagnosticArguments'
  Pop $0
  Pop $1
  ${If} $0 != 0
    DetailPrint "$1"
    SetErrorLevel 2
    Abort "安装恢复尚未完成。文件分别保留在 $BackupDir 和 $StageDir，请保留这两个目录后重试。"
  ${EndIf}
  ; The installation folder was kept, so the previous entries move back into it.
  nsExec::ExecToStack '"$PLUGINSDIR\eden-install.exe" -Operation attach -Target "$INSTDIR" -Source "$BackupDir" $DiagnosticArguments'
  Pop $0
  Pop $1
  ${If} $0 != 0
    DetailPrint "$1"
    SetErrorLevel 2
    Abort "原安装目录未能恢复。文件保留在 $BackupDir，新程序保留在 $StageDir。"
  ${EndIf}
  nsExec::ExecToStack '"$PLUGINSDIR\eden-install.exe" -Operation complete -Target "$INSTDIR" -Receipt "$TransactionRecord" $DiagnosticArguments'
  Pop $0
  Pop $1
  ${If} $0 != 0
    SetErrorLevel 2
    Abort "仍有文件需要核对，安装回执保留在 $TransactionRecord。请保留相关目录后处理。"
  ${EndIf}
FunctionEnd

Section "Claude Eden" Main
  !insertmacro InstallPhase "acquiring_installation_lock"
  !insertmacro AcquireInstallationMutex
  StrCpy $TransactionRecord "$INSTDIR.eden-installation.json"
  ${If} ${FileExists} "$TransactionRecord"
    SetErrorLevel 2
    Abort "存在未处理的安装事务记录：$TransactionRecord。请先按记录恢复保留的目录，再重试。"
  ${EndIf}
  !insertmacro InstallPhase "checking_destination"
  Call CheckDestination
  ${GetParent} "$INSTDIR" $0
  CreateDirectory "$0"
  GetTempFileName $StageDir "$0"
  Delete "$StageDir"
  CreateDirectory "$StageDir"
  !insertmacro InstallPhase "unpacking_application"
  SetOutPath "$StageDir"
  ClearErrors
!ifdef NSIS_WIN32_MAKENSIS
  File /r "${APP_DIR}\*"
!else
  File /r "${APP_DIR}/*"
!endif
  WriteUninstaller "$StageDir\Uninstall.exe"
  ${If} ${Errors}
    SetOutPath "$TEMP"
    RMDir /r "$StageDir"
    SetErrorLevel 2
    Abort "Could not unpack Claude Eden. Check free disk space and folder permissions."
  ${EndIf}

  ; Install the bundled offline runtime only when WebView2 is absent.
  !insertmacro InstallPhase "detecting_webview_runtime"
  SetRegView 32
  ReadRegStr $0 HKLM "SOFTWARE\Microsoft\EdgeUpdate\Clients\${WEBVIEW2_APP_ID}" "pv"
  ReadRegStr $1 HKCU "Software\Microsoft\EdgeUpdate\Clients\${WEBVIEW2_APP_ID}" "pv"
  SetRegView 64
  ${If} $0 == ""
  ${OrIf} $0 == "0.0.0.0"
    ${If} $1 == ""
    ${OrIf} $1 == "0.0.0.0"
      DetailPrint "Installing the bundled Microsoft WebView2 runtime..."
      !insertmacro InstallPhase "installing_webview_runtime"
      ExecWait '"$StageDir\resources\runtime\MicrosoftEdgeWebView2RuntimeInstallerX64.exe" /silent /install' $0
      StrCpy $HelperExit $0
      Call WriteInstallDiagnostic
      ${If} $0 != 0
        SetOutPath "$TEMP"
        RMDir /r "$StageDir"
        SetErrorLevel 2
        Abort "Microsoft WebView2 installation failed ($0). Install the runtime and retry."
      ${EndIf}
    ${EndIf}
  ${EndIf}

  SetOutPath "$TEMP"
  StrCpy $BackupDir ""
  ${If} $ExistingInstall == "1"
    ${GetParent} "$INSTDIR" $0
    GetTempFileName $BackupDir "$0"
    Delete "$BackupDir"
  ${EndIf}
  InitPluginsDir
  SetOutPath "$PLUGINSDIR"
  File /oname=eden-install.exe "${INSTALL_HELPER_SOURCE}"
  DetailPrint "正在检查并保留已有项目和修改文件…"
  !insertmacro InstallPhase "preparing_file_transaction"
  ; EdenData is the default storage of a packaged Windows installation. It
  ; stays in place: a VM registered on it keeps its paths and open handles.
  nsExec::ExecToStack '"$PLUGINSDIR\eden-install.exe" -Operation prepare -Target "$StageDir" -Source "$INSTDIR" -Backup "$BackupDir" -Receipt "$TransactionRecord" -KeepInPlace EdenData $DiagnosticArguments'
  Pop $0
  Pop $1
  StrCpy $HelperExit $0
  Call WriteInstallDiagnostic
  DetailPrint "$1"
  ${If} $0 != 0
    SetOutPath "$TEMP"
    RMDir /r "$StageDir"
    SetErrorLevel 2
    Abort "无法安全保留现有文件，安装已停止。原安装目录已保留，请检查磁盘空间、文件占用和安装日志。"
  ${EndIf}
  SetOutPath "$TEMP"
  ${If} $ExistingInstall == "1"
    !insertmacro InstallPhase "moving_previous_application"
    ; The installation folder itself stays; only its program and personal
    ; entries move, so kept data and open handles below it are untouched.
    nsExec::ExecToStack '"$PLUGINSDIR\eden-install.exe" -Operation detach -Target "$BackupDir" -Source "$INSTDIR" -Receipt "$TransactionRecord" $DiagnosticArguments'
    Pop $0
    Pop $1
    StrCpy $HelperExit $0
    Call WriteInstallDiagnostic
    ${If} $0 != 0
      DetailPrint "$1"
      nsExec::ExecToStack '"$PLUGINSDIR\eden-install.exe" -Operation complete -Target "$INSTDIR" -Receipt "$TransactionRecord" $DiagnosticArguments'
      Pop $0
      Pop $1
      RMDir /r "$StageDir"
      SetErrorLevel 2
      Abort "Could not move the installed application. Quit Claude Eden, close programs using files in its folder, and retry."
    ${EndIf}
    DetailPrint "正在迁入个人文件并保留原有链接…"
    !insertmacro InstallPhase "moving_personal_files"
    nsExec::ExecToStack '"$PLUGINSDIR\eden-install.exe" -Operation move -Target "$StageDir" -Source "$BackupDir" -Receipt "$TransactionRecord" $DiagnosticArguments'
    Pop $0
    Pop $1
    StrCpy $HelperExit $0
    Call WriteInstallDiagnostic
    DetailPrint "$1"
    ${If} $0 != 0
      Call RestorePreservedFiles
      RMDir /r "$StageDir"
      SetErrorLevel 2
      Abort "个人文件未能迁入新程序目录，原安装已恢复。请关闭占用文件的程序后重试。"
    ${EndIf}
  ${Else}
    RMDir "$INSTDIR"
  ${EndIf}
  !insertmacro InstallPhase "publishing_application"
  ClearErrors
  ${If} $BackupDir != ""
    ; The kept installation folder receives the staged entries one by one.
    nsExec::ExecToStack '"$PLUGINSDIR\eden-install.exe" -Operation attach -Target "$INSTDIR" -Source "$StageDir" $DiagnosticArguments'
    Pop $0
    Pop $1
    StrCpy $HelperExit $0
    Call WriteInstallDiagnostic
    ${If} $0 != 0
      DetailPrint "$1"
      SetErrors
    ${EndIf}
  ${Else}
    Rename "$StageDir" "$INSTDIR"
  ${EndIf}
  ${If} ${Errors}
    ${If} $BackupDir != ""
      Call RestorePreservedFiles
    ${EndIf}
    RMDir /r "$StageDir"
    SetErrorLevel 2
    Abort "Could not install Claude Eden. The previous installation has been restored."
  ${EndIf}
  ${If} $BackupDir != ""
    !insertmacro InstallPhase "cleaning_previous_application"
    DetailPrint "正在清理已核对的旧程序文件…"
    nsExec::ExecToStack '"$PLUGINSDIR\eden-install.exe" -Operation cleanup -Target "$INSTDIR" -Source "$BackupDir" -Receipt "$TransactionRecord" $DiagnosticArguments'
    Pop $0
    Pop $1
    StrCpy $HelperExit $0
    Call WriteInstallDiagnostic
    DetailPrint "$1"
    ${If} $0 != 0
      DetailPrint "旧目录中仍有需要保留的文件：$BackupDir"
      SetErrorLevel 2
      Abort "安装仍有待恢复的文件。请保留 $TransactionRecord 及其中记录的目录，再处理恢复。"
    ${EndIf}
  ${Else}
    !insertmacro InstallPhase "completing_file_transaction"
    nsExec::ExecToStack '"$PLUGINSDIR\eden-install.exe" -Operation complete -Target "$INSTDIR" -Receipt "$TransactionRecord" $DiagnosticArguments'
    Pop $0
    Pop $1
    StrCpy $HelperExit $0
    Call WriteInstallDiagnostic
    ${If} $0 != 0
      SetErrorLevel 2
      Abort "安装回执未能完成，记录保留在 $TransactionRecord。"
    ${EndIf}
  ${EndIf}
  !insertmacro InstallPhase "registering_installation"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayName" "Claude Eden"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "Publisher" "Claude Eden contributors"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "UninstallString" '$\"$INSTDIR\Uninstall.exe$\"'
  WriteRegStr HKCU "${UNINSTALL_KEY}" "QuietUninstallString" '$\"$INSTDIR\Uninstall.exe$\" /S'
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayIcon" "$INSTDIR\Eden.exe,0"
  WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoRepair" 1
  SetOutPath "$INSTDIR"
  ${If} $DesktopShortcut == 1
    CreateShortcut "$DESKTOP\Claude Eden.lnk" "$INSTDIR\Eden.exe"
  ${Else}
    Delete "$DESKTOP\Claude Eden.lnk"
  ${EndIf}
  ${If} $StartMenuShortcuts == 1
    CreateDirectory "$SMPROGRAMS\Claude Eden"
    CreateShortcut "$SMPROGRAMS\Claude Eden\Claude Eden.lnk" "$INSTDIR\Eden.exe"
    CreateShortcut "$SMPROGRAMS\Claude Eden\Uninstall Claude Eden.lnk" "$INSTDIR\Uninstall.exe"
  ${Else}
    Delete "$SMPROGRAMS\Claude Eden\Claude Eden.lnk"
    Delete "$SMPROGRAMS\Claude Eden\Uninstall Claude Eden.lnk"
    RMDir "$SMPROGRAMS\Claude Eden"
  ${EndIf}
  WriteRegDWORD HKCU "${UNINSTALL_KEY}" "DesktopShortcut" $DesktopShortcut
  WriteRegDWORD HKCU "${UNINSTALL_KEY}" "StartMenuShortcuts" $StartMenuShortcuts
  !insertmacro InstallPhase "completed"
  SetErrorLevel 0
SectionEnd

Function un.onInit
  !insertmacro LoadPrivateFonts
  Call un.InitializeFontControls
  StrCpy $InstallOutcome "running"
  StrCpy $HelperExit ""
  StrCpy $DiagnosticArguments ""
  ${GetParameters} $0
  ${GetOptions} $0 "/DIAGNOSTICS=" $DiagnosticFile
  ${If} $DiagnosticFile != ""
    StrCpy $DiagnosticArguments '-Diagnostics "$DiagnosticFile.helper.json"'
  ${EndIf}
  !insertmacro UninstallPhase "checking_installation_identity"
  SetShellVarContext current
  SetRegView 64
  FileOpen $0 "$INSTDIR\Eden.install-id" r
  FileRead $0 $1
  FileClose $0
  ${If} $1 != "${APP_ID}"
    StrCpy $HelperExit "identity_mismatch"
    Call un.WriteInstallDiagnostic
    SetErrorLevel 2
    Abort "This is not a Claude Eden installation folder."
  ${EndIf}
  !insertmacro UninstallPhase "checking_stopped_processes"
  !insertmacro CheckStopped "Eden.exe"
  !insertmacro CheckStopped "resources\eden.exe"
  !insertmacro CheckStopped "resources\mihomo.exe"
  !insertmacro CheckStopped "resources\runtime\qemu\qemu-system-x86_64.exe"
FunctionEnd

Section "Uninstall"
  !insertmacro UninstallPhase "acquiring_installation_lock"
  !insertmacro AcquireInstallationMutex
  !insertmacro UninstallPhase "checking_pending_transaction"
  ${If} ${FileExists} "$INSTDIR.eden-installation.json"
    StrCpy $HelperExit "pending_install_transaction"
    Call un.WriteInstallDiagnostic
    SetErrorLevel 2
    Abort "存在待恢复的安装事务，请先处理 $INSTDIR.eden-installation.json 中记录的目录。"
  ${EndIf}
  InitPluginsDir
  SetOutPath "$PLUGINSDIR"
  File /oname=eden-install.exe "${INSTALL_HELPER_SOURCE}"
  File /oname=install-files.json "${FILE_MANIFEST}"
  ; Run the embedded Go helper outside the application being removed.
  SetOutPath "$TEMP"
  !insertmacro UninstallPhase "removing_owned_files"
  nsExec::ExecToStack '"$PLUGINSDIR\eden-install.exe" -Operation uninstall -Target "$INSTDIR" -Manifest "$PLUGINSDIR\install-files.json" $DiagnosticArguments'
  Pop $0
  Pop $1
  StrCpy $HelperExit $0
  Call un.WriteInstallDiagnostic
  DetailPrint "$1"
  ${If} $0 != 0
    SetErrorLevel 2
    Abort "部分程序文件正在使用或无法读取，卸载未完成。个人文件已保留，请关闭占用程序后重试。"
  ${EndIf}
  !insertmacro UninstallPhase "removing_shortcuts_and_registration"
  Delete "$DESKTOP\Claude Eden.lnk"
  Delete "$SMPROGRAMS\Claude Eden\Claude Eden.lnk"
  Delete "$SMPROGRAMS\Claude Eden\Uninstall Claude Eden.lnk"
  RMDir "$SMPROGRAMS\Claude Eden"
  DeleteRegKey HKCU "${UNINSTALL_KEY}"
  !insertmacro UninstallPhase "removing_install_directory"
  RMDir "$INSTDIR"
SectionEnd
