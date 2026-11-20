# MyGo Player

基于 [MyGo](https://github.com/egoist/mygo) 的原生桌面视频播放器，界面与交互改编自 [mygo-music](https://github.com/egoist/mygo-music)。播放器、选集、进度、倍速、音量、画质、自动连播与本地资料库由宿主统一提供；来源通过独立可执行插件接入。

## 安装来源

首次打开进入「插件」页。点击「选择插件 ZIP…」或「选择文件夹…」，在系统选择器中确认后直接安装。也可以粘贴 GitHub 仓库或 Release 链接，点击「从链接安装」。安装后点击「开启」，再点击「使用」切换来源。

兼容插件通过清单声明推荐、搜索、详情、分集和播放等能力；弹幕、账号、收藏同步及预约为可选扩展。网络服务访问、封面解密和媒体代理由插件实现；本地媒体插件也可提供文件扫描和支持 Range 的媒体 URL。

请使用插件作者提供的仓库链接或安装包。插件独立构建和发布，播放器新增来源无需改代码或重编译。

支持的 GitHub 链接形式：

```text
https://github.com/owner/repo
https://github.com/owner/repo/releases/latest
https://github.com/owner/repo/releases/latest/download/my-source-darwin-arm64.zip
https://github.com/owner/repo/releases/tag/v0.3.0
https://github.com/owner/repo/releases/download/v0.3.0/my-source-darwin-arm64.zip
```

仓库链接获取 latest Release；指定 tag 固定版本。安装器自动匹配当前系统/架构的 ZIP 资产，下载后仍使用相同的清单、路径和文件大小校验。没有 Release 或没有兼容安装包会明确报错，不下载源码代替可执行文件。

安装私有仓库插件需要具备仓库权限，并在本机执行一次 `gh auth login`；应用也支持环境变量 `GH_TOKEN` 或 `GITHUB_TOKEN`。凭据不会写入应用配置。公共仓库允许匿名安装。插件是以当前用户身份运行的程序，请安装可信来源。

0.3.1 的 GitHub 安装器支持 macOS 系统 HTTP/HTTPS/SOCKS 代理，显式环境变量代理优先并遵守 NO_PROXY；从 Finder 打开时也会查找常见安装位置的 GitHub CLI。安装过程中显示凭据、版本查询、下载和校验阶段，可点击「取消安装」；连接和凭据读取都有超时。

## 桌面应用内更新

- **播放器**：在「插件」页的「播放器更新」区域点击「检查更新」，然后下载并安装，重启后生效。签名校验通过后才替换应用。Windows x64 与 macOS Apple Silicon 支持此流程。0.3.12 及更早版本需要先安装一次支持更新的正式版。
- **插件**：在已安装插件旁点击「更新」。通过 GitHub 安装的插件会使用保存的仓库链接。之前用 ZIP 安装的插件可填写作者的 GitHub 仓库链接，或用新 ZIP 更新。更新保留启用状态、收藏、历史及插件数据。

私有 GitHub Releases 的更新使用本机已有的 GitHub CLI 登录或环境变量凭据。运行于只读安装位置时，请将应用移到当前用户可写的位置。

## 发布桌面版本

在 `packaging/Info.plist` 更新桌面版本号，提交后推送一致的 `vMAJOR.MINOR.PATCH` 标签。GitHub Actions 会构建 Mac ZIP、Windows 安装程序和便携 ZIP，并将安装包、签名更新包及校验和一并发布。各插件仓库以自身清单版本和标签发布两个平台的 ZIP。

## 开发与协议

```text
pluginapi/            无第三方依赖的公开协议/模型/Serve SDK
internal/plugins/     本地/GitHub 安装、发现、进程与 JSONL 客户端
internal/catalog/     宿主本地收藏/历史存储
internal/webplayer/   共用系统 WebView 与视频控制层
```

协议 2 JSON Lines：清单声明任意有效 ID、能力、导航与过滤项，插件返回中立作品、分集与可播放媒体 URL。SDK、签名、解密、账号及代理留在插件内部。弹幕等能力可选，未声明则隐藏对应入口。协议和插件包说明见 [plugins/README.md](plugins/README.md)。

公开 `pluginapi` 仅依赖 Go 标准库，可以作为注明来源的协议 SDK 快照随独立项目维护；也可以使用其他语言实现 JSONL。插件项目应独立构建，无需播放器工作区或 sibling replace。

## 运行与构建

需要 Go 1.27.3+，无需 Node。提供 macOS Apple Silicon 和 Windows x64 发布包；macOS 已做原生运行验证，Windows 目前仅完成交叉编译和打包检查。

```sh
go mod download
MYGO_PLAYER_BUNDLE_FFMPEG=/absolute/path/to/ffmpeg sh scripts/build.sh
open 'build/MyGo Player.app'
```

所有打包脚本默认把最终安装包/ZIP 输出到 `~/Desktop/MyGo Player 打包/`，中间构建文件保留在 `build/`。可通过 `MYGO_PLAYER_OUTPUT_DIR` 指定其他输出目录。Windows 脚本同时生成便携 ZIP 和安装程序（需要 NSIS），macOS 脚本生成应用及桌面目录中的 ZIP。

构建只产出宿主，不拉取或编译插件。应用使用 ad-hoc 签名，未做 Developer ID 公证。FFmpeg 可选；宿主查找顺序为 `-ffmpeg`、程序同目录、`MYGO_PLAYER_FFMPEG`、PATH，并将找到的路径传给插件，供需要处理媒体的插件使用。

```sh
go test -race ./...
go vet ./...
# 先在独立数据目录安装并启用所需插件，将 my-source 替换为其清单 ID
'./build/MyGo Player.app/Contents/MacOS/mygo-player' \
  -data /tmp/mygo-player-smoke -source my-source -smoke -smoke-next
```

### Windows x64

Release 提供便携 ZIP 和 NSIS 安装程序。Windows 10/11 x64 需要 Microsoft Edge WebView2 Runtime；包内附有官方联网安装引导程序及 FFmpeg。便携 ZIP 完整解压后运行 `mygo-player.exe`；安装程序可创建快捷方式，并提示安装或修复 WebView2。Windows 程序未做代码签名，也尚未进行原生桌面启动、播放及安装/卸载验证。插件需有独立的 Windows amd64 Release 资产。

可在 macOS/Linux 的 shell 或 Windows 的 Git Bash 中构建，需要 Python 3；生成安装程序还需 NSIS：

```sh
WEBVIEW2_BOOTSTRAPPER=/absolute/path/to/MicrosoftEdgeWebview2Setup.exe \
MYGO_PLAYER_BUNDLE_FFMPEG=/absolute/path/to/windows-x64/ffmpeg.exe \
sh scripts/build-windows.sh
```

Windows 图标使用与 macOS 相同的应用图案，`packaging/AppIcon.ico` 包含 16–256 像素多尺寸图像。打包脚本通过固定版本 `github.com/akavel/rsrc@v0.10.2` 生成 `appicon_windows_amd64.syso`，将图标嵌入程序的资源 ID 1，供 MyGo 窗口和 Windows Shell 使用；首次构建需联网下载该工具。安装程序和卸载程序也使用相同图标。构建时会检查实际 EXE 的图标资源与 ICO 内容一致，缺失时中止打包。

WebView2 引导程序来自 <https://go.microsoft.com/fwlink/p/?LinkId=2124703>。请使用 Windows x64 FFmpeg，来源与许可证见第三方声明。Windows GitHub 下载器使用 `HTTP_PROXY` / `HTTPS_PROXY` 并遵守 `NO_PROXY`；当前未读取 Windows 系统代理。

## Android

`android/` 是独立的 Go 模块（MyGo 0.10），只为 Android 构建；桌面版继续使用当前 MyGo 版本与原生 UI，两者互不影响。手机界面是内嵌网页（`android/frontend`）：底部三个标签「发现 / 追剧 / 我的」，发现页双列竖版封面瀑布流，点封面进入竖屏播放页，上下滑动或按钮切集，选集从底部面板按 30 集一段打开；播放页退出即保存进度。

Android 不能执行下载的可执行文件插件，因此手机版在构建时把来源编译进 APK：宿主只提供中性的 `android/sourceapi` 注册接口和 `https://mygo.localhost` 页面，来源仓库各自提供 `mobile` 适配包。接入哪些来源由构建参数决定，仓库本身不包含具体来源实现或名称：

```sh
# Android SDK、NDK、JDK 17 与 MyGo 0.10 CLI；也可用 MYGO_PLAYER_ANDROID_ENV 指定
. ../.tools/android-tools-setup.env
export MYGO_PLAYER_ANDROID_SOURCES=/absolute/path/sources.json
sh scripts/build-android.sh
```

`sources.json` 是独立来源目录的数组，每项为 `{"directory": "/path/to/source-repo"}`（可选 `import` 指定适配包）。脚本在 `build/android-project-*` 临时目录生成注册文件、替换为本机模块路径并调用 `mygo build`，最终把 APK、AAB、构建信息与校验和复制到桌面打包目录；中间目录和生成文件不进入仓库。

手机端的媒体：`<video>` 优先，HLS 回退到内置的 hls.js。来源的本地媒体服务在 `127.0.0.1` 上提供，应用只对 `https://mygo.localhost` 页面放宽混合内容，并通过网络安全配置仅允许 `localhost`/`127.0.0.1` 明文；封面经宿主代理获取，Android 端可把 HEIF 封面转成 JPEG。需要在 FFmpeg 上完成的受保护多轨媒体，若来源能在 Go 内完成解密（例如单轨 cenc MP4）则可直接播放，其余情况会明确提示不支持。

Android 包使用调试密钥签名，未做 Play 商店签名、上架或真机验证以外的兼容性声明。

## 数据与升级

默认根目录为 `os.UserConfigDir()/mygo-player`，macOS 为 `~/Library/Application Support/mygo-player`。`-data` 可指定独立目录。

- `libraries/<id>/library.json`：收藏、历史和续播进度。
- `plugins/<id>/`：安装的插件包。
- `plugins/enabled.json`：启用状态。
- `plugin-data/<id>/`：插件私有账号、设备、缓存或媒体文件；本地视频插件默认读取其中的 `media` 文件夹。
- `preferences.json`：播放器偏好。

0.3 保留 0.2 的插件 ID 和资料目录；已安装在用户目录中的插件继续可用。旧应用中的随附插件不再包含在新应用中，需要从独立仓库安装。0.1 更早的数据仍保留，不自动迁移。

代码与安装包不含登录态、观看记录或影视内容。上游服务可用性取决于来源。应用采用 [AGPL-3.0-only](LICENSE)，依赖见 [第三方声明](THIRD_PARTY_NOTICES.md)，实测结果见 [VALIDATION.md](VALIDATION.md)。
