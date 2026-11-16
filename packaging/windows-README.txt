MyGo Player — Windows x64

系统：Windows 10 / 11 x64，需要 Microsoft Edge WebView2 Runtime。

便携包：完整解压后打开 mygo-player.exe。首次使用前，可运行同目录的
MicrosoftEdgeWebview2Setup.exe 安装或修复 WebView2（需要联网）。
安装程序：运行 setup.exe，按提示选择目录；安装结束可安装/检查 WebView2。

发布包附带 ffmpeg.exe，供有媒体处理需求的插件使用。
来源插件在应用「插件」页面通过 GitHub 链接或 ZIP 单独安装；请选择
Windows amd64 安装包。私有仓库需要具备访问权限，并在本机使用
GitHub CLI 执行 gh auth login，或设置 GH_TOKEN / GITHUB_TOKEN。
Windows 下载器遵守 HTTP_PROXY / HTTPS_PROXY 和 NO_PROXY 环境变量。

用户数据默认位于 %APPDATA%\mygo-player，卸载应用不会删除用户资料。

此版本已通过 Windows amd64 交叉编译、PE 格式和打包内容检查；
尚未在真实 Windows 桌面上验证启动、WebView2 播放和安装/卸载。
程序未进行 Windows 代码签名。

对应源代码与许可证：https://github.com/shi747826/mygo-player
