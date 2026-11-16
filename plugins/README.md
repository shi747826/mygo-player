# 可执行来源插件协议 v2

插件为独立仓库中的程序，不需要注册到 Player 源码。0.3 构建只包含宿主，不随附来源。宿主扫描应用旁的 plugins、macOS Resources/plugins，以及数据目录 plugins；用户包覆盖同 ID 的外部分发包。安装后默认关闭，开启时才启动进程。API：`Manager.Install(path)` 安装本地目录/ZIP，`InstallFrom(ctx,input)` 自动识别本地路径或 GitHub 链接，`InstallGitHub(ctx,url)` 安装 GitHub Release，`Uninstall(id)` / `Reload()` 卸载/重载。

GitHub 仓库首页与 `/releases/latest` 安装最新正式 Release；`/releases/tag/<tag>` 固定版本；`/releases/download/<tag>/<asset>.zip` 选择指定资产，`/releases/latest/download/<asset>.zip` 跟随最新 Release 的同名资产。插件作者应在 Release 上传 `<id>-<GOOS>-<GOARCH>.zip`，如 `my-source-darwin-arm64.zip`，ZIP 根布局与本地包相同。宿主匹配本机系统/架构，不在用户机器上构建源码。私有仓库使用 `GH_TOKEN` / `GITHUB_TOKEN` 或 `gh auth login` 凭据；不把凭据转发到下载 CDN。安装器限制下载 256MiB，并复用以下解压及清单验证。

## 包与清单

ZIP 根目录应直接包含清单和可执行文件：

```text
plugin.json
my-source
```

```json
{
  "id":"my-source",
  "name":"我的来源",
  "version":"1.0.0",
  "runtime":{"type":"exec","protocolVersion":2,"executable":"my-source"},
  "capabilities":["discover","search","detail","resolve"],
  "navigation":[{"label":"推荐","value":"推荐"}],
  "filters":[]
}
```

ID 为小写字母/数字以及单连字符分隔的 slug。executable 是包内相对路径，args 可提供固定参数；宿主直接执行程序，不通过 shell。包不允许符号链接、路径越界或绝对 executable。ZIP 安装限制为解压后 256MiB、10,000 文件。可执行文件需匹配目标平台。

宿主向进程传递 `MYGO_PLUGIN_DATA_DIR=<dataDir>/plugin-data/<id>` 作为独立持久目录，`MYGO_PLAYER_FFMPEG` 为宿主找到的 FFmpeg 路径。工作目录是插件包目录。插件只能把 JSONL 写到 stdout；日志写 stderr。宿主不会解释来源 SDK、认证 cookie 或加密材料。

`capabilities` 决定可见功能：discover、search、rank、detail、resolve、seasons、covers、resetDevice、calendar、danmaku，以及可选 account、favoritesSync、reservations。基础视频来源至少实现 detail/resolve，一般再提供 discover/search。未实现方法返回 `unsupported`。导航 value 使用 推荐/搜索/榜单；收藏/历史由宿主提供。filters 的 value 为 channel 或 sort，options 为 label/value 对。

## JSON Lines

每行长度小于 8MiB。请求可以并发，响应可乱序，通过唯一非零 ID 对应。内置 Go Serve 最多并发 128 个请求。

```json
{"version":2,"id":1,"method":"Ping","params":{}}
{"version":2,"id":1,"result":2}
{"version":2,"id":2,"method":"Detail","params":{"ID":"show-1"}}
{"version":2,"id":2,"result":{"Series":{"id":"show-1","title":"示例","episodes":1},"Episodes":[{"id":"episode-1","title":"第 1 集","number":1}]}}
{"version":2,"id":3,"method":"Resolve","params":{"ID":"episode-1","Quality":"自动"}}
{"version":2,"id":3,"result":{"URL":"http://127.0.0.1:12345/media/session","Definition":"1080p","Qualities":["1080p"],"SessionID":"opaque-session"}}
```

取消消息：`{"version":2,"cancel":3}`。插件应取消相应 context，包括网络请求和长轮询。宿主关闭 stdin 后，插件取消所有请求、清理代理/媒体/临时文件并退出；两秒内未退出会被终止并回收。异常退出让全部挂起请求失败。

错误形如 `{"version":2,"id":3,"error":{"message":"...","kinds":["unsupported"]}}`；具体分类见公开 `pluginapi/errors.go`。不要在错误或日志中输出 cookie、签名或播放密钥。

| 方法 | params | result |
| --- | --- | --- |
| Ping | {} | 2 |
| Discover | Channel, Cursor, Sort, PageSize | Page{Items,Cursor,HasMore} |
| Search | Search:{Query,Cursor,PageSize} | Page |
| Rank | {} | Page |
| Detail | ID | {Series,Episodes} |
| Seasons | ID | []Series |
| Covers | IDs | map[string]string |
| Resolve | ID,Quality | Media{URL,Definition,Qualities,SessionID} |
| Release | SessionID | null |
| Danmaku | SeriesID,ID（分集 ID） | [{time:秒,text:文本}] |
| ResetDevice | {} | null |
| Calendar | Days | map[string]UpcomingEntry |

作品与分集 ID 为不透明字符串。宿主不会拆分、转换整数或假设来源格式。模型的精确 JSON 字段以 `pluginapi/types.go` 为准，Series/Episode 为小写字段，Page/Media 为首字母大写。媒体 URL 必须可供系统 WebView 直接播放；需要请求头、解密、DRM 材料或分离音轨时，插件自行准备或提供 loopback 代理。URL 在会话 Release 或进程退出前保持有效。

宿主统一实现选集、进度、倍速、音量、全屏、画质切换、续播、自动连播和预加载。画质切换重新调用 Resolve。SessionID 可为空；非空时宿主在停止/替换/清理预加载时调用 Release，插件应幂等释放资源。插件还需在退出时清理未释放会话。

弹幕完全可选。声明 danmaku 后，宿主请求时间轴并使用共用渲染器；未声明时不请求数据，隐藏开关和弹幕层。宿主限制最多 1200 条并过滤非法时间与空文本。

## 可选账号扩展

通过同一 JSONL 通道调用，认证持久化和实际凭据归插件。AccountProfile 返回 `{user_id,nickname,avatar}`；AccountLoginStart 返回 `{Token,Image,Expires}`（Token 是临时不透明句柄，Image 为 base64，Expires 为 RFC3339）；AccountLoginWait 参数 `{Token}`，长轮询返回 confirmed/expired，接受取消；AccountLogout 无结果。AccountFavorites / AccountReservations 返回 `[{SeriesID,Title,Cover,Episodes,Online,PublishAt}]`。后两项分别由 favoritesSync/reservations 能力控制。

## 独立开发验证

公开 `pluginapi` 只依赖 Go 标准库，入口调用 `pluginapi.Serve(os.Stdin,os.Stdout,handler)`，自动处理握手、并发和取消。其他语言也可按上述协议实现可执行程序。

插件项目应自行维护实现代码、许可、协议 SDK 快照、测试及 Release 工作流，不依赖宿主 internal 或同级工作区。插件业务测试在各自项目执行，宿主测试关注安装、协议兼容和共用播放器行为。

宿主保留一个最小协议测试 fixture，在测试二进制已编译后编译另一个临时 module，安装 ZIP，验证中立调用、可选能力、释放、重载、启停和卸载。它不是产品来源实现。GitHub 安装测试用 HTTP fixture 覆盖 Release 选择、私有资产、错误与取消；实际私有 Release 安装结果记录在验证文档。

```sh
go test -race ./internal/plugins
go test . -run '^TestExternalExecutableModule' -v
```

真实 GitHub 安装测试需显式设置 `MYGO_PLAYER_LIVE_GITHUB=1`，并以 `MYGO_PLAYER_LIVE_PLUGINS` 提供 JSON 数组，例如 `[{"ID":"example","URL":"https://github.com/owner/repo","Version":"1.0.0"}]`。ID 和版本应与目标包清单一致；私有仓库还需本机凭据。可选 `MYGO_PLAYER_LIVE_APP` 指定待验证的宿主应用，同时以 `MYGO_PLAYER_LIVE_SOURCE` 指定已安装来源 ID。测试配置应使用独立数据目录，不在仓库中记录凭据。

当前没有市场、自动下载升级或 MyGo Music JS/TS 兼容层。
