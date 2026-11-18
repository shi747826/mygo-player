# MyGo Player 验证记录

## 0.1.0 Android 播放页手势重做

- 播放页改为全屏沉浸：视频按比例居中，四周用封面模糊背景填充，不再出现大片纯黑；控件默认隐藏，仅在点按或按住时出现，约 3 秒无操作淡出。
- 上滑切换下一集（跟手位移，超过屏高 20% 或速度足够即吸附切换），下滑回到上一集；到首集/末集时给出轻提示并回弹。桌面调试时滚轮与上下键同步。
- 按住画面约 380ms 进入 2× 快进，松手恢复原倍速并继续播放；轻点切换播放/暂停并短暂显示进度条。
- 选集改为底部胶囊 + 底部面板，30 集分段、当前集高亮并定位；移除常驻大播放键、上一集/下一集按钮与常驻进度条。
- 修复 Android WebView 布局视口比可视区域高 34px 时底部控件与面板被系统手势条遮挡的问题：固定层与底部面板改用 `innerHeight` 计算的 `--app-h`。
- 模拟器验证：上滑第 2→3 集、下滑第 3→2 集；长按显示 2× 且 `playbackRate=2`，松手回到 1；轻点后控件出现并在 3.4 秒内隐藏；选集面板底边与可视高度一致（842px），当前集高亮。

## 0.1.0 Android 手机版

环境：Apple Silicon macOS、Go 1.27.3、MyGo CLI 0.10.0、JDK 17.0.21、Android SDK 36（build-tools 36.0.0、NDK 28.2.13676358），模拟器为 Android 15 / API 35 arm64。

构建与检查：

- `go test -race ./...` 与 `go vet ./...` 通过（安卓模块）；原生模块 `mygo native check` 通过。
- Gradle 构建产出 `mygo-player-0.1.0.apk`/`.aab`；`apksigner verify` 通过（v2 签名），ZIP 完整性、`arm64-v8a` 库、`versionName=0.1.0`、`minSdk=26` 均校验。
- APK 清单包含由原生模块合并的 `networkSecurityConfig`：基础禁止明文，仅 `localhost` 与 `127.0.0.1` 允许；`INTERNET` 权限存在。

模拟器实测：

- 首次启动读取来源清单，发现页双列瀑布流加载真实目录；封面经宿主代理从 HEIF 转成 JPEG 后正常显示，集数角标在封面左上角，标题单行省略。
- 点封面进入竖屏播放页，`<video>` 播放解密后的 H.265 MP4：进度从 0:22 前进到 5:03 的时间轴上，切到第 2 集后重新从头播放。
- 选集面板按 30 集分段（1–30、31–60…），当前集高亮，打开时定位当前段。
- 播放页的受保护流在 Go 内解密：单元测试与真实流（23 MB）离线解密后由 FFmpeg 解码抽帧验证；需要独立音轨或子样本加密的流会返回明确的不支持提示。
- 未验证：真实手机、Android 平板、横屏旋转、后台恢复、Play 上架签名、Windows/Linux 上的移动端构建。

## 0.3.8 播放器禁用网页文本选择

- 播放器统一应用 `-webkit-user-select: none`、`user-select: none` 和 `-webkit-user-drag: none`，剧名、集数、时间及菜单文字不再被长按拖动选中。
- 仅调整样式，不拦截鼠标或指针事件；沿用进度拖动、音量输入、菜单点击和键盘快捷键的现有交互检查。
- 使用原生 WKWebView 加载实际控件 HTML/CSS，检查文字、按钮与滑块的选择和拖拽样式，确认滑块保持启用。

## 0.3.7 卡片间距微调

- 剧名与操作按钮的间距由 4 DIP 增至 12 DIP，增加 8 DIP 留白；继续保持剧名单行省略与封面集数角标。
- 虚拟列表高度同步增加 8 DIP，使用已有卡片高度和列表末尾回归检查验证。

## 0.3.6 紧凑剧集卡片

- 剧名单行显示、超长自动省略，取消原先固定双行标题和底部元信息占位；封面下方固定为 20 DIP 剧名和 30 DIP 操作行。
- 总集数移到封面左上角的半透明深色角标，未知集数不显示；观看进度移到封面底部，避免不同状态的卡片高度不一致。
- 虚拟滚动的卡片高度由封面加 102 DIP 调整为加 62 DIP，并计入可视区域的残缺行，保证列表末尾不漏渲染。
- 回归检查覆盖两种卡片宽度、长短剧名、已看/未看、未知/四位数集数，实际布局高度与虚拟列表估算保持一致；保留滚动到末尾及渲染数量上界测试。

## 0.3.5 播放窗口标题栏适配

- 软件内窗口全屏保留 40 DIP 深色标题栏：剧名位于系统按钮旁，集数位于右侧；长标题单行截断，按原生按钮左右边界留出空间。
- 标题栏独立于 WebView 布局，视频从栏下方开始，保留完整画面比例；窗口播放隐藏视频内重复标题，系统全屏移除窗口标题栏并恢复视频标题浮层。
- 整条标题栏（包括剧名区域）支持原生窗口拖拽；Windows 原生窗口按钮随播放/普通布局切换深浅配色。拖拽后由应用获得焦点时，Esc 仍能退出窗口播放并恢复原侧栏状态。
- UI 测试覆盖长剧名、集数、左/右/双侧原生按钮避让、加载层边界、退出与系统全屏；浏览器控制脚本测试覆盖标题显隐。
- 原生 Mac 使用合成测试视频验证：窗口栏高 40，WebView 从 y=40 开始；连续拖拽使窗口从 `(480,139)` 移动到 `(580,199)`，播放布局保持；F 进入系统全屏后视频占满 `2560×1440`、窗口栏消失，再按 F 恢复窗口播放标题栏。
- Windows 仅完成交叉编译与包/图标检查，尚未进行 Windows 原生播放与拖拽验证。

## 0.3.4 紧凑插件选择与可折叠侧栏

- 插件选择框收窄到最多 100 DIP；同一行紧接 28 DIP 的侧栏展开/收起按钮。平台窗口按钮、插件选择和侧栏按钮互相避让，侧栏隐藏后仍保留顶部两个控件。
- 移除“资料库”小标题，缩小顶部和搜索框下方留白，导航菜单整体上移约 33 DIP。
- Cmd/Ctrl+F 会展开侧栏并聚焦搜索；目录分页根据侧栏是否展开计算可用宽度。
- UI 测试覆盖紧凑控件边界、原生按钮避让、目录纵向位置、侧栏隐藏与恢复、隐藏时的搜索快捷键和插件菜单。

## 0.3.3 紧凑窗口布局

- 使用 MyGo 隐藏标题栏并保留系统窗口按钮，macOS 按钮定位为 `(14, 12)`，Windows 原生按钮区高度 40；整个内容区绘制到窗口顶部。
- 移除侧栏应用图标、应用名称、展开/折叠按钮及对应状态；插件选择紧邻系统按钮。通过 `Context.TitleBar()` 适配左右按钮预留区域。
- 右侧移除返回/前进/刷新工具栏和来源副标题，页面标题属于连续白色内容区。返回/前进保留快捷键，刷新改用 Cmd/Ctrl+R，搜索快捷键保留 Cmd/Ctrl+F。
- UI 测试覆盖 900/1280 宽窗口、左侧/右侧/双侧系统按钮、插件下拉菜单点击、详情关闭按钮避让，以及搜索、导航和播放器窗口全屏恢复；原生 Mac 窗口截图用于检查实际布局。
- Windows 继续验证交叉编译产物和图标资源，尚未在 Windows 真机检查显示效果。

## 0.3.2 Windows 图标修复

- 原因：之前只提供 macOS ICNS，Windows 程序没有 PE 图标资源；MyGo 从程序的图标组 ID 1 读取窗口图标，因此快捷方式和窗口均缺少应用图标。
- 新增与现有应用图案一致的 ICO，包含 16/24/32/48/64/128/256 像素尺寸。Windows 构建生成平台专用 COFF 资源，并嵌入主程序；NSIS 安装/卸载程序使用同一 ICO，快捷方式显式选择主程序图标索引 0，安装后通知 Shell 刷新缓存。
- 新增标准库脚本检查最终 PE 的资源树、图标组及各尺寸图像内容，确认与源 ICO 一致；该检查加入打包脚本，防止生成缺少应用图标的安装包。
- 产物仍未在真实 Windows 桌面验证显示效果；资源检查可证明图标已嵌入，不能替代原生界面验证。

## 0.3.1 Windows x64 打包

- 使用 Go 1.27.3、`GOOS=windows GOARCH=amd64 CGO_ENABLED=0`、`mygo_noinspector` 和 `-H=windowsgui` 完成交叉编译。
- `scripts/build-windows.sh` 生成便携 ZIP 与 NSIS 安装程序，携带 FFmpeg 9.0.2、官方 WebView2 联网安装引导程序、使用说明及许可文件。
- Windows 主程序和 FFmpeg 的 PE Machine 为 `0x8664`；WebView2 引导程序通过 PE 格式检查。便携 ZIP 的 CRC 和必需文件检查通过。NSIS 安装程序编译成功，版本使用应用清单中的 0.3.1。
- FFmpeg 依赖 wheel 的 SHA-256 为 `f27ca4cb85e2d4946deedbce1a81245dfd942a2bcf94b53676fb3956aa95bf1b`，与 PyPI 元数据一致；WebView2 引导程序来自 Microsoft 官方下载链接。
- Windows 包对应的实际程序、WebView2 播放、插件启动、安装/卸载和系统代理均未在真实 Windows 桌面测试。Windows 下载器目前只读取环境代理，不读取 Windows 系统代理；程序未做 Windows 代码签名。

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
