> ⚠️ **免责声明**：本项目源码来自网上大名鼎鼎的**鱼佬**（原作者，[GitHub 主页](https://github.com/fish2018)）。我只是把它拿来打包、测试着玩，方便自己用。

# 红果鉴 / 真果鉴

Flutter 多端独立短剧 / 影视应用

## 最近更新（1.2.0+105）

电视端交互与平台支持更新，Android / Windows / macOS / iOS / 本地服务版同步生效。完整历史见 [CHANGELOG.md](CHANGELOG.md)。

- **新增 macOS 支持（Apple Silicon + Intel 通用包）**：`macos/` 工程与 `DuanjuCore` pod，Go 核心 arm64 / x86_64 两片编成静态库后 `lipo` 成通用库，Dart 侧用 `DynamicLibrary.process()` 取符号（与 iOS 同一套）；产物是 `*-macos-universal.dmg`，带拖拽安装布局、ad-hoc 签名，出包前逐项验签（架构、FFI 入口、必需框架、dmg 内的 app 与 Applications 快捷方式）。系统代理走 `CFNetworkCopySystemProxySettings`，界面按桌面形态工作。
- **电视端搜索框改由原生输入框唤起系统输入法**：Flutter 的 `TextField` 在小米等电视上按确认键唤不出输入法，改用 1×1 隐藏 `EditText` 拉起键盘，输入实时回传、确认即搜索、返回即取消。
- **新增设置「搜索用字母键盘」**（仅电视端）：开启后按确认键弹出内置字母键盘，按拼音首字母输入并实时列出匹配条目，选中即搜。默认走系统输入法。
- **追剧、观看记录、站源搜索支持拼音首字母**：`霸道总裁` 用 `BDZC` 就能搜到（原文子串优先，拼音只做兜底）。
- **首页按返回键不再直接退出**：先提示「再按一次返回退出应用」，2 秒内再按一次才退出；非首页或有子页面时仍是返回上一层。
- **本地服务版（网页端）的收藏夹 / 播放记录支持拼音首字母搜索**：这两个列表的搜索框原先只能按剧名子串找，现在输入 `BDZC` 也能命中《霸道总裁》（原文优先、拼音兜底，规则与 App 侧 `matchesTitle` 一致）。首字母表由 `tool/gen_pinyin_table.dart` 用 lpinyin 生成成 `native/server/web/pinyin.js` 随包发出，网页端不需要联网取字典。
- **本地服务版出包前会自动真跑一遍**：CI 的 `web` job 在 `build_web.py` 之后执行 `scripts/check_web.py`，把两个版本的二进制实际启动、逐个打接口，并校验内嵌网页与仓库里的 `native/server/web` **逐字节一致**、站源裁剪（红果鉴只有红果）确实生效。以后「编过但打不开」「改了前端忘了重新编译」这类问题会在发布前就红掉。

## 使用

| 功能 | 操作 |
| --- | --- |
| 浏览 | 标题栏依次提供排序与筛选、榜单和展开搜索。分类保持独立一行，内容区左右滑动切类；支持默认、名称、自然季号、上线日期、热度、播放量排序及连载状态筛选。列表下滑接近底部自动加载下一页，底部「加载更多」为兜底 |
| 搜索 | 红果合并官网与名称索引，输入停顿 300 毫秒显示最多 10 条联想；韩小圈、剧果、野果、鬼片、A123、爱看机器人调用各自的在线搜索并按分页继续加载。部分失败保留有效结果；最近 20 次搜索按用户保存 |
| 全部源搜索 | 搜索页可勾选「全部源」，同时并发搜索所有已开启的站源（含自定义源），结果按标题 / 年份 / 集数归一去重、同一部剧合并成一条并标出可切换的其它站源。搜索是分批返回的：哪个站源先出结果就先显示，不等最慢的那个；勾选偏好按用户保存在本机 |
| 电视搜索 | 电视端搜索框按确认键由原生输入框唤起系统输入法（解决小米等电视唤不出输入法的问题）。设置里可开「搜索用字母键盘」：弹出内置 A-Z / 数字键盘，按拼音首字母输入并实时列出匹配条目。追剧、观看记录、站源搜索都支持拼音首字母（`霸道总裁` → `BDZC`） |
| 返回键 | 首页按返回键先提示「再按一次返回退出应用」，2 秒内再按一次才退出；在其它页或有子页面时仍是返回上一层 |
| 推荐 | 仅红果分类栏在「全部」后显示「推荐」，可切换真人剧、漫剧、AI 剧并分别记住位置 |
| 详情 | 普通点剧直接进入播放；下载和兜底入口保留详情页。详情页浏览封面、资料、追剧状态与可展开简介；底部固定「立即播放 / 继续播放」和下载入口。选集默认折叠，展开后每组 50 集，可切换范围、定位当前或输入集数跳转；宽屏与电视采用资料、选集分栏 |
| 追剧与历史 | 按想看 / 在看 / 已看 / 有更新筛选；红果已追剧发现新季后进入「有更新」。追剧和历史均可搜索。每个用户独立保存，每 5 秒及退出播放时记录真实进度 |
| 卡片菜单 | 卡片「更多」提供收藏、状态和下载选集；电脑右键，电视菜单键 |
| VIP | 全站源版浏览黄豆或全部站源时显示 VIP 图标，仅过滤黄豆；默认隐藏已确认的 VIP。未知、免费、VIP 分别保存，未知不会覆盖已知状态 |
| 手机播放 | 上滑下一集、下滑上一集；长按 350 毫秒临时 3 倍速。竖屏轻点播放 / 暂停，横屏轻点控制条，双击播放 / 暂停。视频下方为「选集 / 简介 / 下载」Tab |
| 画中画 | Android 手机 / 平板播放器接入画中画按钮；进入前隐藏自绘控制层与弹幕，小窗只保留画面。Android TV、Windows、iOS 暂未接入 |
| 投屏 | Android 端播放设置里的「投屏」（DLNA）：自动搜索同一 Wi-Fi 下的电视 / 盒子 / 投影仪，按当前进度投放，可在面板里暂停、继续、停止。流由手机经局域网转发，投屏期间不要退出播放器；本地下载文件不支持 |
| 画质增强 | 仅 Windows 桌面端显示并运行增强链路（关闭 / 自动 / 省电增强 / 清晰优先）；移动端与电视端隐藏 |
| 弹幕 | 红果在线播放默认开启；播放器侧边圆形「弹」字按钮可关闭、查看状态及失败重试。本地播放不加载弹幕 |
| 下载 | 播放页「下载」Tab 和详情页下载入口均可选择分集与画质；支持批量暂停、继续、重试、删除及清理任务但保留视频。「更新本剧」补新增或缺失分集 |
| 更多 | 站源管理（全站源版可逐个开关站源）、用户管理、设置与备份、界面模式、关于 |

| 站源 | 浏览与播放 | 搜索 |
| --- | --- | --- |
| 红果 | 真人剧、漫剧、AI 剧及分集 | 联网搜索与官网搜索联想 |
| 韩小圈 | MacCMS 模板：最新韩剧 / 韩国电影 / 韩国综艺 / 韩国动漫，多线路分集 | 站源在线搜索 |
| 鬼片 | MacCMS 站点：鬼片 / 电视剧 / 动漫，多线路分集 | RSS 最新条目标题匹配（站点搜索已停用） |
| 青空 | 番剧、剧场动画、特摄及分集 | 站源在线搜索 |
| 黄豆 | 列表、VIP 标记及分集 | 筛选已加载短剧 |
| 剧果 | 热门、最新、接口分类、详情及可播放分集；签名 Cookie 接入在线播放、预加载和下载 | 站源在线搜索，支持继续加载 |
| 野果 | 接口实际分类、目录、详情及真实分集；按分集重新取流，保留 H.264 / H.265 地址 | 站源在线分页搜索 |
| 帝果 | 网页分类、目录、详情及分集；vplayer 签名解析 | 站源在线分页搜索 |
| 黄果视频 / 黄果 AI / 黄果旧版 | 列表、分类、详情及分集 | 筛选已加载短剧 |

站源可用性、清晰度和区域限制取决于源站及网络；应用不解除源站 VIP 或其他授权限制。

## 安装包与平台状态

| 平台 | 包与状态 |
| --- | --- |
| Android 7.0+ | 三架构（arm64-v8a / armeabi-v7a / x86_64）APK；同一签名可覆盖升级，旧证书版本需卸载重装 |
| Windows 10/11 x64 | 完整 ZIP 解压后运行 `hongguojian.exe` / `zhenguojian.exe`，保留所有 DLL 与 `data` |
| macOS 12+ | `*-macos-universal.dmg`，同一份包同时支持 Apple 芯片与 Intel（universal 二进制）；拖进 Applications 即可。未公证，首次打开前在终端执行 `xattr -dr com.apple.quarantine /Applications/红果鉴.app`（真果鉴为 `真果鉴.app`） |
| iOS 15.1+ | 已加入工程、Go 核心链接、媒体依赖、文件管理与构建脚本；iOS 播放页禁用 media_kit_video 硬件纹理加速以规避 libmpv 渲染崩溃；`CFBundleDisplayName` 使用英文 `HongGuoJian` / `ZhenGuoJian` 以通过 iloader 等侧载工具注册 App ID 时的名称校验，中文名写入 `zh-Hans.lproj/InfoPlist.strings`（开发快照，未经 Xcode 构建验证）；待 Xcode 构建与真机验收，无已签名 IPA |
| 本地服务版（Windows / macOS / Linux） | `native/server` 提供 Go HTTP 服务并内嵌网页前端（HLS / FLV / artplayer），由 `scripts/build_web.py` 交叉编译出四份二进制。解压后双击 `start.bat`（Windows）或执行 `sh start.sh`（macOS / Linux），浏览器打开 `http://127.0.0.1:8000/web/`，同一局域网的其它设备可用启动时打印的地址访问。Windows 包内置 `ffmpeg.exe`（加密 H.265 站源需要服务端转码），macOS / Linux 需本机装 ffmpeg |

`INSTALL_FAILED_NO_MATCHING_ABIS` 表示 APK 与设备架构不匹配，请更换对应架构安装包。

### GitHub Actions

工作流只能手动触发，推送和 PR 不再自动运行检查。

发布流程：在 `CHANGELOG.md` 写好同名版本一节（标题如 `## 1.2.0+105`，会写入 Release 说明），推送到 `master`（`main` 也接受），再在 Actions 中手动运行 **Build app packages**。工作流读取 `pubspec.yaml` 的 `version`，得到 tag `app-v{version}`（`+` 替换为 `-`）。同一版本可以重复发布：默认勾选 `republish`，会先删掉同名的旧 Release 与 tag、再按当前提交重建并覆盖同名资产；取消勾选则该版本已存在时直接报错停止，此时应提升 `pubspec.yaml` 的 `version`。然后同时构建各平台的两版（默认与 `--all-sources`），再加上本地服务版：

| 产物 | 内容 |
| --- | --- |
| `*-android` | 三种架构 APK；配好发布证书时用它签名并通过指纹校验，未配置时退回 debug 签名（仅测试用，不能覆盖已安装的正式签名版本） |
| `*-windows` | Windows x64 ZIP |
| `macos` | 两个版本的 `*-macos-universal.dmg`（`hongguojian-…` / `zhenguojian-…`），Apple 芯片与 Intel 通用 |
| `ios-unsigned` | 两个版本的 arm64 iPhone 未签名 IPA（`hongguojian-…` / `zhenguojian-…`），下载后自行签名安装 |
| `web` | `zhenguoweb.zip`：本地服务版合集，内含 `hongguojian.zip` / `zhenguojian.zip` 与 `SHA256SUMS.txt`，两个 zip 都带 Windows 版 `ffmpeg.exe` |

触发时可以勾选/取消 `with_ios`（默认勾选）、`with_macos`（默认勾选）和 `republish`（默认勾选，允许同一版本覆盖重发）。

Android、Windows、web 全部成功后才创建 tag 和 Release；iOS 与 macOS 不阻塞发布（见下文）。勾选 `republish` 时同名旧 Release 会被覆盖并重新指向当前提交；不勾选则不会动已有 Release。

#### 发布时报 `HTTP 403: Resource not accessible by integration`

创建 Release 那一步用的令牌没有写权限。release job 里已经声明了 `permissions: contents: write` 却仍然 403，说明**仓库/组织设置把默认令牌压成了只读**（job 里的声明并不总能盖过这个仓库级默认值）。

处理按顺序试：

1. **Settings → Actions → General → Workflow permissions** 选 **Read and write permissions**（仓库级；组织托管的仓库还要看组织同名设置，组织级是硬上限）；
2. 或者在本仓库 **Settings → Secrets and variables → Actions** 新建 Secret `RELEASE_TOKEN`，放一个带 `repo` 权限的个人访问令牌（工作流会优先用它：`secrets.RELEASE_TOKEN || github.token`）；
3. **改完后必须重新手动触发一次新的运行**（Actions → Run workflow），点 Re-run 重跑不生效——重跑会沿用原运行当时的令牌权限。

跑起来后可以看 job 日志开头 Actions 打印的 `GITHUB_TOKEN Permissions:` 块确认这次实际拿到的权限；工作流里也加了一步专门打印「该仓库 Release 接口要求的权限」。手动发布流程为开发快照，尚未完成首次运行验证。

Android 签名优先使用仓库 Secrets：`ANDROID_KEYSTORE_BASE64`、`ANDROID_KEYSTORE_PASSWORD`、`ANDROID_KEY_ALIAS`、`ANDROID_KEY_PASSWORD`，以及仓库变量 `ANDROID_CERT_SHA256`（证书 SHA-256，冒号和大小写不限）。Secrets 缺失时工作流输出 warning 并改用 debug 签名出包、跳过指纹校验；证书指纹只在发布签名模式下校验。CI 通过环境变量把签名传给 Gradle；本机构建使用 `android/key.properties`。`key.properties` 和证书文件不入库。

#### iOS / macOS 经常「不参与构建」的原因

整条发布流里只有 ios 和 macos 两个 job 用 GitHub 的 macOS runner，而 **macOS 分钟按 10 倍计费**：Free 计划每月 2000 分钟折合 macOS 只有约 200 分钟，一次完整 iOS 构建（Flutter 环境 + Go cgo 交叉编译 + `flutter build ios`）往往就要 20～30 分钟墙钟、200～300 分钟账单；macOS 出包是两个版本、两片架构（arm64 + x86_64），同样不便宜。额度用完后：

- Free 账号默认 spending limit 为 0，**不含信用卡时 job 直接不会启动**（Actions 里显示为跳过/「This job was not started because recent account payments have failed or your spending limit needs to be increased」），而不是报构建错误；
- 这也解释了为什么「老是」发生——每月基本只够跑一次。

排查与处理：

1. 打开 `Settings → Billing and plans → Plans and usage`，看 Actions 用量里 macOS 那一行的剩余；
2. 运行详情页顶部的黄色横幅会写明是账单问题（job 日志本身不会写）；
3. 额度不够时：追加信用卡并提高 spending limit，或改用自托管 macOS runner（不计费）；
4. 只想先出 Android / Windows / 网页版：手动运行时**取消勾选 `with_ios` 与 `with_macos`**，Release 会照常发布，只缺 IPA 与 dmg。

为降低开销，iOS 与 macOS 各自的两个版本都合在同一个 macOS job 里顺序构建（环境初始化、`pub get`、`pod install` 只付一次，Pods 有缓存），并且这两个 job 失败或被跳过都**不会**拦住其它平台的发布——Release 说明里会写明本次未包含 IPA / dmg。

## 日志与排障

「我的 → 设置 → 播放调试日记」会汇总应用事件、播放器事件与**原生核心日志**，可一键复制。原生核心（`native/core`）内部出错时对外只回「本地核心处理失败，请重试」，真正的 panic 堆栈写在：

| 平台 | 位置 |
| --- | --- |
| Android | 应用数据目录 `files/logs/native_stderr.log`（应用内日记页可直接看到） |
| Windows | `%APPDATA%` 下应用数据目录 `logs\native_stderr.log`，也可在应用内日记页复制 |
| iOS | 沙盒 `Library/Application Support/logs/native_stderr.log` |
| macOS | `~/Library/Application Support/com.duanju.hongguojian/logs/`（真果鉴为 `com.duanju.zhenguojian`）；同样不做 stderr 重定向，排障看应用内日记页 |
| 本地服务版 | 直接打印在启动终端，不写文件 |

stderr 重定向由 bridge 的 `DuanjuInstallStderrLog` 完成：Windows 用 `SetStdHandle`（**不能**用 `_dup2`，`os.File.Fd()` 返回的是 HANDLE，传进去会触发 `__fastfail` 直接退出），其它平台不重定向，避免架构相关的编译风险。

遇到「某个源点播就报处理失败」时，先看这里有没有 `native core panic: ...` 与随后的 Goroutine 调用链，再判断是哪个 provider 的哪一行。

其它两个高频问题：

| 现象 | 原因与处理 |
| --- | --- |
| 某个站源的海报全部不显示 | 图床（如豆瓣 `imgN.doubanio.com`）按 Referer 防盗链，并按 TLS 指纹做风控。核心已内置对应 Referer，并在返回内容不是图片时自动换浏览器指纹传输重试；仍拿不到就是源站侧限流或已下架图 |
| 播放报 `tls: failed to verify certificate: x509` | FFI 嵌入时 Go 的 `crypto/tls` 不加载系统信任库，核心现已改用「系统信任库 + 随包 `cacert.pem`」作信任锚。若某个源仍如此，说明它用了其它私有根，需要在核心的信任锚里追加 |

## 开发与构建

Flutter `3.47.4`、Dart `3.12+`、Go `1.24.1+`、Python `3.10+`。
Android 需要 JDK 17、SDK 36、NDK `28.2.13676358`；
Windows 需要 Visual Studio C++ 桌面组件及 MinGW-w64 x64；
iOS 需要 macOS、完整 Xcode 和 CocoaPods；macOS 同样需要 macOS 12+、完整 Xcode 和 CocoaPods（`dmgbuild` + `Pillow` 可选，用于 dmg 的拖拽安装布局）。

构建脚本对子进程默认设置 `GOPROXY=https://goproxy.cn,direct`、`GOSUMDB=off`，同名环境变量可覆盖。

~~~sh
python3 scripts/build_android.py                 # 红果鉴
python3 scripts/build_android.py --all-sources   # 真果鉴
python3 scripts/build_android.py --abi arm64-v8a
python scripts/build_windows.py [--all-sources]
~~~

~~~sh
python3 scripts/build_ios.py
python3 scripts/build_ios.py --all-sources
python3 scripts/build_ios.py --core-only [--simulator]
python3 scripts/build_ios.py --export-options /path/to/ExportOptions.plist
~~~

~~~sh
python3 scripts/build_macos.py                   # 红果鉴（arm64 + x86_64 通用 dmg）
python3 scripts/build_macos.py --all-sources     # 真果鉴
python3 scripts/build_native.py --platform darwin  # 只编 macOS 核心（lipo 成 macos/DuanjuCore/libDuanjuCore.a）
~~~

~~~sh
python3 scripts/build_web.py                     # 两版都构建
python3 scripts/build_web.py --edition hongguojian
python3 scripts/build_web.py --edition zhenguojian
python3 scripts/check_web.py                     # 出包前自检：真跑起来打一遍接口
~~~

产物在 `dist/android`、`dist/windows`、`dist/ios`、`dist/macos`、`dist/web`，红果版以 `hongguojian-` 开头，全站源版以 `zhenguojian-` 开头；本地服务版输出 `dist/web/hongguojian.zip`、`dist/web/zhenguojian.zip` 与合集 `dist/web/zhenguoweb.zip`。

`scripts/check_web.py` 是本地服务版的冒烟自检：把编出来的二进制实际启动，检查主页能打开、内嵌网页与 `native/server/web` 逐字节一致、站源裁剪生效、接口不返回 500、非法请求打不挂服务。默认自动找 `build/web/*/*-linux`（`build_web.py` 的暂存产物），Windows 本机调试时用 `--binary` 指到 exe，`--slug` 指定版本。


首次 Android 调试先编译对应架构核心，再运行：

~~~sh
python3 scripts/build_native.py --platform android --abi arm64-v8a
flutter pub get --enforce-lockfile
flutter run
~~~

调试全站源版：先给 `build_native.py` 加 `--all-sources`，再 `flutter run --dart-define=ALL_SOURCES=true`；iOS 对应 `build_ios.py --core-only --all-sources`。脚本会同步设置 Dart 常量和 Go 编译参数，应用启动时校验二者一致，避免混装原生库。

播放器使用 [media_kit](https://github.com/media-kit/media-kit) / libmpv，下载完成后转成单个 mp4 使用 [FFmpegKit min-gpl](https://github.com/sk3llo/ffmpeg_kit_flutter)（含 GPL 媒体组件），只换封装、不重新编码。FFmpegKit 不参与正常播放。

### 集中检查

~~~sh
dart format --output=none --set-exit-if-changed lib test
dart analyze --fatal-infos lib test
flutter test --dart-define=DISABLE_REMOTE_IMAGES=true
flutter test --dart-define=DISABLE_REMOTE_IMAGES=true --dart-define=ALL_SOURCES=true
cd native
go test -race ./...
go test -race -ldflags="-X duanjuapp/native/core.buildAllSources=true" ./...
~~~

## 目录结构

| 目录 | 内容 |
| --- | --- |
| `lib` | 页面、播放器、本地用户、FFI、下载和媒体处理 |
| `native/core`、`native/bridge` | 独立站源核心、缓存、下载、目录迁移及 C ABI |
| `native/server` | 本地服务版（Go HTTP 服务 + 内嵌网页前端，含直播代发），由 `scripts/build_web.py` 构建 |
| `android`、`windows`、`ios` | 平台工程与必要资源 |
| `assets/video_enhancement`、`packages/media_kit_libs_windows_video` | 增强 Shader 与许可、固定 Windows 媒体依赖插件 |
| `scripts`、`.github/workflows` | 构建与发布 |
| `test` | 自动化测试 |

## 站源开发约定

站源是 Go 原生 provider（`native/core/provider_*.go`），不是运行期加载的 Python 源。新增站源需接入：`provider_huangguo.go`（常量 / 白名单 / `canonicalProviderSource` / `GetHuangguoChapters`）、`provider_media.go`（baseURL / host 反查 / 播放分派）、`app_categories.go`、`app_cover_metadata.go`、`app_runtime.go`（Config 字段 + 目录 / 搜索 / 详情分派 + 空页放行），并在 `lib/models.dart` 登记 `SourceSite`。

注意：Go RE2 正则**不支持 lookahead**。解析多线路播放列表时不能用 `(?=...)` 做分段，否则非捕获组会消耗下一段开头；应改用显式字符串截断（从容器标记之后查找下一段标记）。
