# Cloudy for macOS

网易云音乐的 **macOS 原生桌面客户端**：用 Go 和 [MyGo](https://github.com/egoist/mygo) 写成，界面由 GPU 自绘，不用 WebView、不打包 Chromium，启动快、占内存小。液态玻璃播放栏、逐字歌词、桌面歌词、私人 FM、播客转写、下载、音效这些功能都在。

> **这个仓库是什么**：它是对 [nya-owo/cloudy](https://github.com/nya-owo/cloudy) 的**逆向移植版**。上游是一个只发布 Windows 安装包的第三方网易云客户端，我读它的源码、理清平台相关的分支，把它编译成 macOS 应用，并补上 macOS 上缺的那部分。功能逻辑没有改动，改的是「Windows 之外」的假设。

## 逆向说明

上游 `nya-owo/cloudy` 的 README 明确写着「目前只提供 Windows 版本」，`RELEASING.md` 也把 macOS 相关步骤删掉了。但读代码能看出来，**macOS 的代码路径其实一直在里面**，只是不再发布：

| 已经存在的 macOS 支持 | 位置 |
| --- | --- |
| 刘海播放器（顶部胶囊） | `internal/app/notch_darwin.go`、`ui_notch.go` |
| 红绿灯内缩、隐藏标题栏 | `internal/app/app.go` 的 `TrafficLightPosition`、`TitleBarHidden` |
| 菜单栏模式（关掉 Dock 图标） | `tray.go` 的 `SetActivationPolicy` |
| 关闭窗口继续播放、Dock 唤回 | `app.go` 的 `OnClose` / `syncDock` |
| 媒体键、控制中心、锁屏「正在播放」 | `internal/player/player.go` 的 `mygo.NowPlaying` |
| CoreAudio 音频输出（AudioQueue） | MyGo `audio/output_darwin.go` |
| Mac 中文字体、`.icns`、签名与 DMG | MyGo `cmd/mygo` |

所以这次移植不是重写，而是：**编译 → 在真机上跑起来 → 把只在 Windows 成立的假设逐个改成按平台分支 → 打包**。上游版本基于 `main@a5691e2`（v1.3.2）。

## 与上游的差别

| 改动 | 为什么 |
| --- | --- |
| 以 macOS 为目标打包，产出 `.app` + `.dmg`（ad-hoc 签名） | 上游只出 Windows 安装包 |
| **输出设备选择**（macOS 现在能选听哪副耳机/音箱） | MyGo 的 `audio.Outputs()` 只在 Windows 实现过；这里补了 CoreAudio 设备枚举，并让 AudioQueue 绑定到选定设备，设备拔掉时自动回到默认，插回来又切回去 |
| 字体列表换成 macOS 的中文字体 | 上游列的是微软雅黑、等线、宋体这些 Windows 字体，Mac 上一个都没有 |
| 路径比较按 APFS 的大小写不敏感处理 | 上游只对 Windows 忽略大小写；macOS 的默认文件系统同样不区分 |
| 快捷键显示 `⌃⌥P` 而不是 `Ctrl+Alt+P` | Mac 键盘上写的是符号 |
| Dock 图标按 macOS 网格重绘（824/1024 + squircle + 投影） | Windows 图标铺满方形，放进 Dock 会比别的图标大一圈 |
| `scripts/build-macos.sh` | 一条命令出包 |

为了让音频那部分能改，`third_party/mygo` 放了一份 MyGo 的源码副本（MIT，来自 `nya-owo/mygo`，上游为 `egoist/mygo`），`go.mod` 用 `replace` 指向它。**相对上游只动了音频：**

- `audio/outputs_darwin.go`（新增）：用 CoreAudio 列出输出设备，ID 用设备 UID，标出系统默认；
- `audio/outputs_other.go`：构建标签排除 darwin；
- `audio/output_darwin.go`：`openDevice` 按 `SetOutput` 的选择设置 `kAudioQueueProperty_CurrentDevice`，设备消失时回落到默认，`write` 里检测切换/回归并重开队列（与 Windows 的实现同构）；
- `audio/outputs_darwin_test.go`（新增）：真机验证枚举与绑定。

## 构建

需要 Go 1.27 或更高版本（MyGo 用 `go tool` 管理生成物）。

```sh
./scripts/build-macos.sh
```

产物：

```
build/darwin-arm64/Cloudy.app
build/darwin-arm64/cloudy-1.3.2-arm64.dmg
```

脚本会先按 macOS 的图标网格生成 `icon/AppIcon-1024.png`，再用 `mygo build` 打 `.app` 和 `.dmg`，默认 ad-hoc 签名（本机能跑）。要分发给别人的 Mac，设开发者证书，MyGo 会用同一个身份签名并公证：

```sh
DEVELOPER_ID="Developer ID Application: 你的名字 (TEAMID)" ./scripts/build-macos.sh
```

Intel 与 Apple Silicon 都要，就把平台换成 `darwin/universal`：

```sh
go tool mygo build -platform darwin/universal -sign -
```

已构建好的包也可以直接下：见 [Releases](../../releases)。系统要求 macOS 13 或更高。

### 持续集成

GitHub Actions 的工作流放在 `ci/`：`test.yml` 在 macOS runner 上跑 gofmt、vet 和测试，`release.yml` 在打 tag 时构建 `.app` 与 `.dmg` 并开一个 draft release。本仓库的推送凭据没有 `workflow` 权限，所以它们还没有放进 `.github/workflows/`；授权后即可启用：

```sh
gh auth refresh -s workflow
mkdir -p .github/workflows && git mv ci/*.yml .github/workflows/ && git commit -m "Run CI" && git push
```

## 已验证

- `go test ./...` 在 macOS 上全绿（含 250 多个测试，约 7 分钟）；
- 真机跑起来能拉每日推荐、排行榜、歌单与封面，`open -a Cloudy 某首歌.mp3` 能本地播放；
- 菜单栏出现系统「正在播放」，媒体键可用；
- 输出设备：枚举到本机 `MacBook Pro扬声器`（default），指定它后 AudioQueue 绑定成功，指定不存在的设备时回落到默认，播放不受影响。

## 已知限制

- **检查更新会报错**：上游 Release 只有 Windows 资产，没有 `update-darwin-arm64.json`。想要自更新得自己发一份 macOS 的更新源（MyGo 的 updater 支持 macOS，含签名与增量）。
- **Linux 上没有输出设备列表**：仍是 MyGo 的原状（`outputs_other.go`），只播到系统默认设备。
- **Xcode 26 的 `actool` 不可用**时，MyGo 会跳过 `icon/AppIcon.icon`（Icon Composer 的液态玻璃图标），改用 `icon/AppIcon-1024.png` 生成的 `.icns`。装了 Xcode 26 就会用上原生图标。
- 极少数界面状态（例如设置页的某些行）只在 Windows 有过视觉回归测试，Mac 上如有出入，欢迎提 issue。

## 目录

```
main.go                 入口
internal/app            UI 与状态（视图、设置、播放栏、歌词、下载……）
internal/netease        网易云 eapi：登录、歌曲、歌单、搜索、评论
internal/player         播放、缓存、音效、淡入淡出、断流重试
internal/lyrics         逐字歌词
internal/speech         本机语音转写（sherpa-onnx + Qwen3-ASR）
internal/download       下载
internal/local          本地音乐
third_party/mygo        MyGo 源码副本，含 macOS 音频输出设备的补丁
icon/macos              按 macOS 图标网格生成 App 图标
scripts/build-macos.sh  macOS 构建
```

## 授权

本项目沿用上游的 [AGPL-3.0](LICENSE)。`third_party/mygo` 是 [MyGo](https://github.com/egoist/mygo) 的副本，MIT 授权，其许可证见该目录下的 `LICENSE`。

本项目与网易云音乐官方无关，音乐版权归各权利人所有；登录凭据保存在系统钥匙串，不上传到任何第三方。
