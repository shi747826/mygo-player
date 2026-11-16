# MyGo Player

基于 [MyGo](https://github.com/egoist/mygo) 的原生桌面视频播放器，界面与交互改编自 [mygo-music](https://github.com/egoist/mygo-music)。播放器、选集、进度、倍速、音量、画质、自动连播与本地资料库由宿主统一提供；来源通过独立可执行插件接入。

## 安装来源

0.3 起，播放器仓库只保留宿主和协议，不再包含来源实现或随附来源程序。首次打开进入「插件」页，在输入框粘贴 GitHub 仓库链接并点击「安装」，完成后「开启」并选择「使用」。也支持本地 ZIP/文件夹路径。

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

目前没有插件市场、自动升级或 MyGo Music JS/TS 兼容层。卸载保留按来源隔离的收藏、历史及插件私有数据。

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

WebView2 引导程序来自 <https://go.microsoft.com/fwlink/p/?LinkId=2124703>。请使用 Windows x64 FFmpeg，来源与许可证见第三方声明。Windows GitHub 下载器使用 `HTTP_PROXY` / `HTTPS_PROXY` 并遵守 `NO_PROXY`；当前未读取 Windows 系统代理。

## 数据与升级

默认根目录为 `os.UserConfigDir()/mygo-player`，macOS 为 `~/Library/Application Support/mygo-player`。`-data` 可指定独立目录。

- `libraries/<id>/library.json`：收藏、历史和续播进度。
- `plugins/<id>/`：安装的插件包。
- `plugins/enabled.json`：启用状态。
- `plugin-data/<id>/`：插件私有账号、设备、缓存或媒体文件；本地视频插件默认读取其中的 `media` 文件夹。
- `preferences.json`：播放器偏好。

0.3 保留 0.2 的插件 ID 和资料目录；已安装在用户目录中的插件继续可用。旧应用中的随附插件不再包含在新应用中，需要从独立仓库安装。0.1 更早的数据仍保留，不自动迁移。

代码与安装包不含登录态、观看记录或影视内容。上游服务可用性取决于来源。应用采用 [AGPL-3.0-only](LICENSE)，依赖见 [第三方声明](THIRD_PARTY_NOTICES.md)，实测结果见 [VALIDATION.md](VALIDATION.md)。
