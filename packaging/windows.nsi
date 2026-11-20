Unicode True
!include "MUI2.nsh"
!include "x64.nsh"
!ifndef APP_VERSION
  !define APP_VERSION "0.3.14"
!endif
!ifndef APP_DIR
  !define APP_DIR "../build/win"
!endif
!ifndef OUTPUT_FILE
  !define OUTPUT_FILE "../build/mygo-player-${APP_VERSION}-windows-amd64-setup.exe"
!endif
!define WEBVIEW2_BOOTSTRAPPER "MicrosoftEdgeWebview2Setup.exe"
!define EDGE_CLIENT_STATE "Software\Microsoft\EdgeUpdate\ClientState"

Name "MyGo Player"
OutFile "${OUTPUT_FILE}"
InstallDir "D:\MyGoPlayer"
RequestExecutionLevel user
SetCompressor /SOLID lzma
SetOverwrite try
!define MUI_ICON "AppIcon.ico"
!define MUI_UNICON "AppIcon.ico"
!define MUI_ABORTWARNING
!define MUI_FINISHPAGE_RUN "$INSTDIR\mygo-player.exe"
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "SimpChinese"
Var SetupLog
Var SetupMessage

; Diagnostics also work with standard NSIS builds that lack LogSet.
Function LogSetup
  DetailPrint "$SetupMessage"
  ClearErrors
  FileOpen $9 "$SetupLog" a
  ${IfNot} ${Errors}
    FileWriteUTF16LE $9 "$SetupMessage$\r$\n"
    FileClose $9
  ${EndIf}
  ClearErrors
FunctionEnd

Function ReportSetupError
  Call LogSetup
  MessageBox MB_OK|MB_ICONSTOP "$SetupMessage$\r$\n$\r$\n诊断日志：$SetupLog" /SD IDOK
FunctionEnd

Function .onInit
  StrCpy $SetupLog "$TEMP\MyGoPlayer-setup.log"
  ClearErrors
  FileOpen $9 "$SetupLog" w
  ${IfNot} ${Errors}
    FileWriteUTF16LE /BOM $9 "MyGo Player ${APP_VERSION} 安装日志$\r$\n"
    FileClose $9
  ${EndIf}
  ${IfNot} ${RunningX64}
    StrCpy $SetupMessage "此安装包需要 64 位 Windows。"
    Call ReportSetupError
    SetErrorLevel 1
    Quit
  ${EndIf}
FunctionEnd

; Match the runtime lookup in MyGo's Windows backend, including per-user installs.
; Edge Update can retain its registration after the runtime files are gone.
Function CheckWebView2Files
  StrCpy $1 0
  ${If} $0 != ""
  ${AndIf} ${FileExists} "$0\msedgewebview2.exe"
  ${AndIf} ${FileExists} "$0\EBWebView\x64\EmbeddedBrowserWebView.dll"
    StrCpy $1 1
  ${EndIf}
FunctionEnd

!macro CheckWebView2Channel ID
  ReadRegStr $0 HKLM "${EDGE_CLIENT_STATE}\${ID}" "EBWebView"
  Call CheckWebView2Files
  ${If} $1 == 1
    Return
  ${EndIf}
  ReadRegStr $0 HKCU "${EDGE_CLIENT_STATE}\${ID}" "EBWebView"
  Call CheckWebView2Files
  ${If} $1 == 1
    Return
  ${EndIf}
!macroend

Function FindRegisteredWebView2
  SetRegView 32
  !insertmacro CheckWebView2Channel "{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" ; Evergreen
  !insertmacro CheckWebView2Channel "{2CD8A007-E189-409D-A2C8-9AF4EF3C72AA}" ; Beta
  !insertmacro CheckWebView2Channel "{0D50BFEC-CD6A-4F9A-964C-C7416E3ACB10}" ; Dev
  !insertmacro CheckWebView2Channel "{65C35B14-6C1D-4122-AC46-7148CC9D6497}" ; Canary
FunctionEnd

Function EnsureWebView2
  ReadEnvStr $0 "WEBVIEW2_BROWSER_EXECUTABLE_FOLDER"
  ${If} $0 != ""
    Call CheckWebView2Files
    ${If} $1 == 0
      StrCpy $SetupMessage "WEBVIEW2_BROWSER_EXECUTABLE_FOLDER 指向的文件夹缺少完整的 x64 WebView2 Runtime。请修正或取消此环境变量后重试。"
      Goto failed
    ${EndIf}
  ${Else}
    Call FindRegisteredWebView2
  ${EndIf}
  ${If} $1 == 1
    StrCpy $SetupMessage "已找到 WebView2 Runtime，跳过安装。"
    Call LogSetup
    Return
  ${EndIf}

  StrCpy $SetupMessage "正在联网安装 Microsoft Edge WebView2 Runtime…"
  Call LogSetup
  ClearErrors
  InitPluginsDir
  SetOutPath "$PLUGINSDIR"
  File /oname=${WEBVIEW2_BOOTSTRAPPER} "${APP_DIR}/${WEBVIEW2_BOOTSTRAPPER}"
  ${If} ${Errors}
    StrCpy $SetupMessage "无法解压 WebView2 安装引导程序。请检查临时目录是否可写及磁盘空间。"
    Goto failed
  ${EndIf}
  ExecWait '"$PLUGINSDIR\${WEBVIEW2_BOOTSTRAPPER}" /silent /install' $2
  ${If} ${Errors}
    StrCpy $SetupMessage "无法启动 WebView2 安装引导程序。请检查系统或安全软件是否阻止了运行。"
    Goto failed
  ${EndIf}
  IntFmt $3 "0x%08X" $2
  StrCpy $SetupMessage "WebView2 安装引导程序退出码：$2 ($3)。"
  Call LogSetup
  Call FindRegisteredWebView2
  ${If} $1 == 0
    StrCpy $SetupMessage "WebView2 安装后仍不可用（引导程序退出码 $2 / $3）。请检查网络连接后重试。"
    Goto failed
  ${EndIf}
  StrCpy $SetupMessage "已确认 WebView2 Runtime 可用。"
  Call LogSetup
  Return

  failed:
    Call ReportSetupError
    SetErrorLevel 3
    Abort
FunctionEnd

Section "MyGo Player" SEC_APP
  StrCpy $SetupMessage "安装目录：$INSTDIR"
  Call LogSetup
  Call EnsureWebView2

  ClearErrors
  SetOutPath "$INSTDIR"
  File /r /x "${WEBVIEW2_BOOTSTRAPPER}" "${APP_DIR}/*"
  WriteUninstaller "$INSTDIR\Uninstall.exe"
  ${If} ${Errors}
    StrCpy $SetupMessage "无法将 MyGo Player 写入 $INSTDIR。请确认驱动器可用、目录可写且磁盘空间充足，并关闭正在运行的播放器后重试。"
    Goto failed
  ${EndIf}
  CreateDirectory "$SMPROGRAMS\MyGo Player"
  CreateShortcut "$SMPROGRAMS\MyGo Player\MyGo Player.lnk" "$INSTDIR\mygo-player.exe" "" "$INSTDIR\mygo-player.exe" 0
  CreateShortcut "$DESKTOP\MyGo Player.lnk" "$INSTDIR\mygo-player.exe" "" "$INSTDIR\mygo-player.exe" 0
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\MyGoPlayer" "DisplayName" "MyGo Player"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\MyGoPlayer" "DisplayVersion" "${APP_VERSION}"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\MyGoPlayer" "UninstallString" '$\"$INSTDIR\Uninstall.exe$\"'
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\MyGoPlayer" "DisplayIcon" '"$INSTDIR\mygo-player.exe",0'
  ${If} ${Errors}
    StrCpy $SetupMessage "无法创建 MyGo Player 快捷方式或注册卸载程序。请检查当前用户的权限后重试。"
    Goto failed
  ${EndIf}
  ; Remove the manual WebView2 shortcut left by earlier versions.
  Delete "$SMPROGRAMS\MyGo Player\安装或修复 WebView2.lnk"
  Delete "$INSTDIR\${WEBVIEW2_BOOTSTRAPPER}"
  System::Call 'shell32::SHChangeNotify(i 0x08000000, i 0, p 0, p 0)'
  StrCpy $SetupMessage "MyGo Player 安装完成。"
  Call LogSetup
  Return

  failed:
    Call ReportSetupError
    SetErrorLevel 4
    Abort
SectionEnd
Section "Uninstall"
  Delete "$DESKTOP\MyGo Player.lnk"
  RMDir /r "$SMPROGRAMS\MyGo Player"
  Delete "$INSTDIR\mygo-player.exe"
  Delete "$INSTDIR\ffmpeg.exe"
  Delete "$INSTDIR\MicrosoftEdgeWebview2Setup.exe"
  Delete "$INSTDIR\THIRD_PARTY_NOTICES.md"
  Delete "$INSTDIR\LICENSE"
  Delete "$INSTDIR\README.txt"
  Delete "$INSTDIR\.mygo-files"
  RMDir /r "$INSTDIR\licenses"
  Delete "$INSTDIR\.mygo-files"
  Delete "$INSTDIR\Uninstall.exe"
  RMDir "$INSTDIR"
  DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\MyGoPlayer"
SectionEnd
