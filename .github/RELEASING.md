# 桌面版发布

将发布工作流和版本改动合并，再为该提交推送一个尚未使用的 `vMAJOR.MINOR.PATCH` 标签。`packaging/Info.plist` 中的 `CFBundleShortVersionString` 和 `CFBundleVersion` 须与标签版本一致。手动创建 GitHub Release 会占用该发布版本。

管理员在此仓库的 **Settings → Secrets and variables → Actions** 中配置同一更新签名密钥对。将公钥保存为变量 `MYGO_PLAYER_UPDATE_PUBLIC_KEY`，将私钥保存为密钥 `MYGO_PLAYER_UPDATE_PRIVATE_KEY`。后续版本须继续使用同一密钥对，以便已安装的应用验证新版本。

例如，为已审核的 `0.3.13` 提交发布版本时

```sh
git tag -a v0.3.13 -m 'MyGo Player v0.3.13'
git push origin refs/tags/v0.3.13
```

工作流在 Apple Silicon runner 上测试并打包 macOS 应用，在 Linux runner 上交叉编译 Windows x64 并用 NSIS 打包。GitHub Release 包含以下文件，签名更新资产供应用内更新使用。

- `mygo-player-<版本>-macos-arm64.zip`
- `mygo-player-<版本>-windows-amd64.zip`
- `mygo-player-<版本>-windows-amd64-setup.exe`
- `update-darwin-arm64.json`、`update-darwin-arm64.tar.gz`
- `update-windows-amd64.json`、`update-windows-amd64.tar.gz`
- `SHA256SUMS.txt`

macOS 应用使用 ad-hoc 签名。两端使用与第三方声明一致的固定版本 FFmpeg，工作流在解包前验证 wheel 的 SHA-256。Windows 包还包含从 Microsoft 下载的 WebView2 Evergreen 联网安装引导程序。分发安装包时，应确保接收者能够获取应用和随包分发的 FFmpeg 的对应源代码，相关来源及许可信息随包提供。

两端均完成验证后，发布任务才会上传资产并公开为仓库的 Release。上传中断可在 **Actions → Desktop release** 重新运行失败的任务以继续草稿发布。修正构建或源码并补发已发布版本时，应使用新的版本和标签。私有仓库的 Release 下载沿用仓库访问权限。
