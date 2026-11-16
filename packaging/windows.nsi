Unicode True
!include "MUI2.nsh"
!include "x64.nsh"
!ifndef APP_VERSION
  !define APP_VERSION "0.3.1"
!endif
!ifndef APP_DIR
  !define APP_DIR "../build/win"
!endif
!ifndef OUTPUT_FILE
  !define OUTPUT_FILE "../build/mygo-player-${APP_VERSION}-windows-amd64-setup.exe"
!endif
Name "MyGo Player"
OutFile "${OUTPUT_FILE}"
InstallDir "$LOCALAPPDATA\Programs\MyGoPlayer"
RequestExecutionLevel user
SetCompressor /SOLID lzma
!define MUI_ABORTWARNING
!define MUI_FINISHPAGE_RUN "$INSTDIR\mygo-player.exe"
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "SimpChinese"
Function .onInit
  ${IfNot} ${RunningX64}
    MessageBox MB_ICONSTOP "此安装包需要 64 位 Windows。"
    Abort
  ${EndIf}
FunctionEnd
Section "MyGo Player" SEC_APP
  SetOutPath "$INSTDIR"
  File /r "${APP_DIR}/*"
  WriteUninstaller "$INSTDIR\Uninstall.exe"
  CreateDirectory "$SMPROGRAMS\MyGo Player"
  CreateShortcut "$SMPROGRAMS\MyGo Player\MyGo Player.lnk" "$INSTDIR\mygo-player.exe"
  CreateShortcut "$SMPROGRAMS\MyGo Player\安装或修复 WebView2.lnk" "$INSTDIR\MicrosoftEdgeWebview2Setup.exe" "/install"
  CreateShortcut "$DESKTOP\MyGo Player.lnk" "$INSTDIR\mygo-player.exe"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\MyGoPlayer" "DisplayName" "MyGo Player"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\MyGoPlayer" "DisplayVersion" "${APP_VERSION}"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\MyGoPlayer" "UninstallString" '$\"$INSTDIR\Uninstall.exe$\"'
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\MyGoPlayer" "DisplayIcon" "$INSTDIR\mygo-player.exe"
  MessageBox MB_YESNO|MB_ICONQUESTION "播放器需要 Microsoft WebView2。是否现在安装或检查 WebView2？（需要联网，已有运行时会跳过安装。）" IDNO done
  ExecWait '"$INSTDIR\MicrosoftEdgeWebview2Setup.exe" /silent /install' $0
  done:
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
  RMDir /r "$INSTDIR\licenses"
  Delete "$INSTDIR\Uninstall.exe"
  RMDir "$INSTDIR"
  DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\MyGoPlayer"
SectionEnd
