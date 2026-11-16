# MyGo Player 验证记录

## 0.3.1 GitHub 安装修复

- 修复从 Finder 启动时未读取 macOS 系统代理的问题；支持 HTTP/HTTPS/SOCKS，环境变量代理优先，并遵守 NO_PROXY。
- Finder 精简 PATH 下补查用户 `~/.local/bin/gh`，保留 Homebrew 路径；凭据和系统代理命令限时两秒，限制后代进程继承管道造成的等待。
- HTTP 连接与 TLS 握手限时十秒，响应头限时十五秒，保留五分钟总安装限时；超时和取消会显示可重试提示。
- 安装页显示凭据、版本查询、下载、校验阶段，并提供「取消安装」按钮。状态提示位于安装输入框下方。
- `go test -race ./...` 与 `go vet ./...` 通过；新增测试覆盖系统代理、环境代理优先、NO_PROXY、SOCKS 回退、Finder CLI 查找、命令/输出管道限时、连接超时、安装各阶段取消，以及页面取消按钮。

- 真实桌面环境模拟：清除 HTTP_PROXY/HTTPS_PROXY/ALL_PROXY 及大小写变体、NO_PROXY、GH_TOKEN、GITHUB_TOKEN，设置 PATH=/usr/bin:/bin，使用系统代理和用户目录 CLI 登录态，15.24 秒内完成三个私有 Release 的下载安装与协议握手，五个进度阶段均成功。
- 0.3.1 macOS arm64 应用构建及 `codesign --verify --deep --strict` 通过。

## 0.3.0 验证记录

日期：2026-11-16。环境：macOS Apple Silicon arm64、Go 1.27.3、MyGo v0.8.3。所有网络和播放测试使用独立临时数据目录，不读取原应用账号。

## 验证范围

以下为当时已记录的宿主验证结果，不表示每次文档更新都重新执行了网络或媒体测试。来源插件独立开发、构建和发布；本记录只保留宿主的安装、协议兼容、构建和共用播放链路证据，不作为插件业务实现的验证报告。

## 宿主与构建

- `go test -race ./...` 通过：宿主 UI/状态/本地库/WebView、通用插件运行时、GitHub 安装器、公开协议，以及独立 module 协议 fixture。
- `go vet ./...` 通过。
- `go list -deps .` 仅包含 MyGo 及传递依赖；没有来源业务、账号、弹幕、播放准备或来源 SDK 包。
- `scripts/build.sh` 只构建宿主，不再构建或复制来源。产物包含应用、图标、FFmpeg（可选）和许可，`Contents/Resources` 中没有插件目录。
- `codesign --verify --deep --strict 'build/MyGo Player.app'` 通过；Bundle ID `dev.local.mygo-player`，版本 0.3.0，ad-hoc 签名。
- 构建脚本会完整替换已知应用包并拒绝重定向的 build/app 路径，防止旧构建中的随附插件残留到新版本。
- 独立协议 fixture 在测试宿主已编译后编译另一个临时 module，验证安装、发现/搜索/详情/播放地址、可选能力、释放、重载、启停和卸载，全程不重编译宿主。

## GitHub 链接安装

自动化测试覆盖：

- 链接：`github.com/owner/repo`、仓库 URL、`/releases/latest`、`/releases/latest/download/<asset>.zip`、`/releases/tag/<tag>`、`/releases/download/<tag>/<asset>.zip`。
- 拒绝 http、非 GitHub 主机、伪造域名/端口、用户信息、query/fragment、路径越界、源码归档 URL 和编码绕过。
- 从 Release 自动选择当前 GOOS/GOARCH 的 ZIP；兼容 darwin/macos、arm64/aarch64、amd64/x64/x86_64 等常见写法；拒绝错平台、双重平台标记和无平台资产。
- 私有 Release 通过 asset API 和 `application/octet-stream` 下载；测试校验 `Authorization` 只发送给 GitHub API 主机，重定向的 CDN 请求不带 token。
- 401/403/404、无 Release、无兼容资产、超大/不符大小、非 ZIP、取消、重复安装失败保持原包等错误路径。
- 下载后复用与本地 ZIP 相同的 256MiB、10,000 文件、路径越界、符号链接、清单和可执行文件校验，不下载源码或在用户机器上编译。

真实私有仓库验证：对三个兼容插件 Release 分别通过 `Manager.InstallFrom` 安装，均成功读取 0.3.0 清单并完成 JSONL 握手；随后以其中一个已安装插件启动最终宿主进行原生播放：

```text
SMOKE catalog: 15 results
SMOKE episodes: 21
SMOKE seek verified; waiting for automatic next episode
SMOKE automatic next verified: episode 2
WebView media ready: duration=181.0s seek=12.0s position=1.2s
SMOKE PASS: WebView video decoded, playback advanced, seek acknowledged
```

这证明私有 GitHub 链接安装、平台资产选择、凭据使用、包验证、独立进程启动和原生播放链路可用。测试调用与插件页「安装」相同的 `Manager.InstallFrom`；原生窗口中的输入和点击流程未做端到端自动化。

## 限制与兼容

- 安装私有仓库需要本机 `gh auth login`，或设置 `GH_TOKEN` / `GITHUB_TOKEN`。应用只在请求时读取，不写入配置或日志；凭据不会随重定向传给下载 CDN。
- 仅实际验证 macOS Apple Silicon。GitHub 安装器支持常见的 GOOS/GOARCH 名称匹配，但没有在 Windows/Linux 上做原生产包验证。
- 没有插件市场、自动更新、签名身份校验或 MyGo Music JS/TS 兼容层。插件以当前用户权限运行，不是系统沙箱。
- 0.3 不再随附插件。0.2 已安装到用户数据目录的插件仍可发现；旧版内置插件不会从新应用包恢复，需要从独立仓库安装。资料库、账号和插件私有数据目录保持不变。
- 原生播放证据来自单个已安装插件，不能外推到所有来源或所有媒体。其他来源未在本轮重新验证真实媒体解码；历史播放曾受上游 `anonymous_limited` 限制，宿主协议或离线测试通过不代表上游始终可用。
