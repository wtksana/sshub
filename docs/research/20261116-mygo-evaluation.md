# MyGo 用于 SSH/SFTP 服务器管理工具的选型调研

查阅日期：2026-11-16（Asia/Hong_Kong）。依据官方仓库、发布记录、源码和官方文档；未安装运行 MyGo，未做独立性能测试。稳定版事实优先固定到 `v0.9.1`（提交 `13a0b3e7b3e161454889ff8e1bbf16ab6ffbbe0d`），社区数字是查询当时快照。

## 结论

**MyGo 是值得做原型验证的候选，尤其适合以 Go 为主、希望轻量跨平台并愿意接受早期框架升级成本的个人工具。用于需要长期稳定交付的 SSH/SFTP 产品，应先通过真实终端、文件传输和目标系统验证，再决定采用。** 这是工程判断，不是官方保证。

最关键的发现是：MyGo 已不只是 WebView 外壳。`v0.9.1` 同时支持系统 WebView 与 Go 自绘 native UI，并已有基于 Ghostty 的官方终端插件；后者明确支持自定义 `Conn` 连接 SSH 等字节流。因此，“MyGo 原生没有终端，要自行从零实现终端模拟”这一判断已不成立。但 SSH 协议、SFTP、连接配置、主机密钥校验和传输队列仍是应用职责。[1][2][3]

## 已核实的框架能力

| 方面 | 第一手事实 | 对本工具的意义（判断） |
|---|---|---|
| 定位 | Go 桌面框架；Web 页面运行于 WKWebView / WebKitGTK / WebView2；native UI 由 MyGo 绘制，使用 Metal / Direct3D 11 / OpenGL 等，而不是每个控件映射到系统原生控件。[1][4] | 可选熟悉的 Web 前端，也可纯 Go 开发；“native”不应理解为全部系统原生控件。 |
| 构建 | 核心不用 cgo，可跨平台编译；`v0.9.1/go.mod` 要求 Go 1.27.3。Web 模板工具链采用 Bun，native 项目只需 Go；Bun 不随应用发布。[1][4][5] | Go 后端整合方便；需接受较新 Go 工具链，不能无条件按旧版 Go 环境规划。 |
| 终端 | 官方 native terminal 使用 libghostty-vt，支持 Unicode、全屏应用 alternate screen、宽字符、输入法、鼠标报告、bracketed paste、OSC 8 链接、历史滚动及尺寸变更。[2] | 已有较完整终端起点，但 tmux/vim、中文输入、高 DPI 和大量会话仍要实际测试。 |
| SSH 接入点 | `terminal.Options.Conn` 是 `io.ReadWriteCloser`；有 `Resize(cols, rows int) error` 的实现还会收到尺寸通知。[2][3] | 用 Go SSH 库适配即可走 native 路线；不是一个开箱即用 SSH 管理模块。 |
| Web IPC | 自动生成 TypeScript 客户端，支持类型化服务、事件与 channel；`[]byte` 经 JSON 转为 base64；channel 保序，页面待消费值超过 1 MiB 时 `Send` 等待。[6] | 管理表单、进度很适合；Web 终端持续高吞吐有序列化/桥接成本，需批量与背压。 |
| 桌面能力 | 有窗口、菜单、托盘、文件对话框、拖放/剪贴板、快捷键等；native UI 文档包含列表、表格、树、分栏、标签页。[1][7] | 服务器侧栏、文件列表、多标签/分屏和传输进度有组件基础。 |
| 凭据 | `mygo.Secrets` 使用 macOS Keychain、Windows Credential Manager、Linux Secret Service；单个值最多 2560 字节。[7] | 密码/口令/加密主密钥可保存；不要假定任意私钥文件都能直接放进去。 |
| 发布 | 支持 macOS app/dmg、Windows 安装器、Linux deb/rpm/PKGBUILD 等；签名更新、增量更新已有文档和工具。[8][9] | 少写一部分发布基础设施，但正式签名、证书、打包和平台测试仍需项目负责。 |
| 许可 | 主仓库 MIT，保留版权与许可声明的条件下允许使用、修改、分发和销售。[10] | 可商用/闭源；插件、字体和随包动态库的第三方许可仍应逐项确认。 |

## 平台与“轻量”的边界

核心文档列明：macOS 12+；Linux x64/arm64，native UI 需要 GTK 3，Web UI 还需 WebKitGTK 4.1（可回退4.0）；Windows 10/11 x64/arm64，Web UI 需要 WebView2 Runtime。native-only 不需要 WebView2/WebKitGTK。Linux 托盘还需 AppIndicator 类库。[1]

**带官方终端的要求更高**：libghostty-vt 要求 macOS 13+、Linux glibc 2.28+、Windows 10 1809+（x64/arm64）。它是运行时动态库，`mygo build` 会按平台打入资源；普通 `go run`/`go test` 缺缓存时会下载并校验 SHA-256，打包后的应用不会下载。[2] 因而不能把核心宣传的“单个 Go 二进制、无 cgo”直接等同为“SSH 客户端最终绝对只有一个文件、没有本机库”。无 cgo 与无动态库是两件事。

跨平台编译也不等同于在任意系统完成全部发行流程：macOS 签名和 dmg 制作要在 macOS；其他系统生成 Windows 安装器需要 NSIS；应用另加 cgo 依赖时也会带回目标平台 C 工具链要求。[8]

官方架构文档提供约 7 MB hello-world 和 macOS 数十 MB 等特定测试结果，但同时区分物理占用、WebView 子进程、分辨率、native 窗口和 Mesa 等条件。[4] 本调研不把这些数字当作实际 SSH/SFTP 成品的大小、内存或与 Electron/Tauri 的独立对比结果。终端动态库、历史缓冲、多个会话和文件列表都可能改变占用。

## 维护状态与成熟度

查询时仓库创建时间为 **2026-09-26**，即约七周历史；约 **6317 stars、397 forks、125 个 open issues/PR 合计**，最新推送时间 `2026-11-16T01:40:45Z`。GitHub `open_issues_count` 不能直接当作 bug 数量。[11]

最新正式标记 release 为 **v0.9.1，2026-11-15**；此前 v0.9.0 为 11 月 11 日、v0.8.3 为 11 月 7 日。更新非常活跃，但版本号仍在 0.x，项目历史很短。`v0.9.0` 发布说明确实包含破坏性改动：移除 `sqlite.LibraryPath` 与 `MYGO_SQLITE_LIBRARY`，改走 native modules；native modules 明确标记 experimental 并提示 API 可能破坏性调整。[12][13]

CI 定义覆盖 macOS/Linux/Windows、Go build/vet/test、TypeScript tests/typecheck，以及真实后端 GUI 测试；还可见 Windows ARM64 的专门测试配置。这是工程投入的正面证据，**不是所有实际机器与桌面环境稳定的保证，也不是本次已经跑过 CI 的声明**。[14]

最近提交仍在修复 Windows 退出时进程残留与 macOS 公证票据打包等基础平台问题；同时可见未关闭的单实例锁竞争、Tester 资源释放和 GNOME/XWayland 交互问题。issues 是报告与讨论，不能把标题直接当作所有用户都会复现的已确认缺陷；但足以说明需要预算平台兼容与升级验证时间。[15][16][17][18]

贡献者接口中 egoist 的提交贡献数明显领先，社区已有多个外部贡献者。它能说明当前核心维护集中，不能据此精确计算维护风险或忽略其他形式的贡献。[19]

官方 showcase 已收录作者自己的终端 Kero、GoRex，以及社区的 **Universal S3 UI**（S3/SFTP/FTP，native UI、系统凭据库、拖拽上传）。这些是很接近需求的参考应用，说明不是只有 hello-world；showcase 是项目方维护的目录，应用描述也不是独立审计或长期生产稳定性证明。Universal S3 UI 的 README 安全提示还说明没有安全凭据库时可能回退明文配置，适合参考可行性，不能直接照搬凭据策略。[20]

终端曾有逐行重建导致性能问题的报告；对应 PR #1687 已于 2026-11-11 合并，不应把这条历史问题描述为当前仍无修复。合并记录也不能代替本项目在实际工作负载下的性能验证。[24]

## SSH/SFTP 项目该如何分工

推荐让连接、协议和传输逻辑保持为普通 Go 包，只在边缘适配 MyGo，降低未来替换 UI 框架的成本：

- **UI**：MyGo native + terminal 插件，或 MyGo WebView + Web 终端组件；先选一条路线，不要为“技术齐全”混用两套终端。
- **连接层**：`golang.org/x/crypto/ssh` 负责连接、认证、PTY/shell、端口转发等；`knownhosts` 负责已知主机密钥校验；SFTP 可评估独立的 `github.com/pkg/sftp`。这些不是 MyGo 内置服务。[21][22][23]
- **native terminal 适配**：`ssh.Session` 自身不是 `io.ReadWriteCloser`，需要组合 stdin/stdout 等管道、Close 和错误/退出通知。`Resize(cols, rows)` 适配 `Session.WindowChange(rows, cols)` 时注意参数顺序。PTY/环境变量与多会话生命周期也要明确管理。[3][21]
- **业务层**：服务器分组、SSH config 导入、代理/跳板机、密码/私钥/agent 认证、断线提示与重新连接、传输并发/取消/重试、覆盖确认、权限/符号链接/大目录等都由应用设计。
- **凭据**：小口令可以进 Secrets；较大私钥可以保留文件引用，或加密存储并将加密密钥放系统凭据库。Linux 无 libsecret/会话总线/Secret Service 时接口会报 unsupported，应准备仅内存保存或每次输入的路径。[7]
- **数据路径**：SFTP 文件字节在 Go 后端直接读写磁盘/网络，UI 只接收节流后的进度和目录数据；不应让大文件经 JSON/base64 往返前端。这是根据 IPC 与 SFTP 特征得出的设计建议。[6][23]

MyGo 的可信页面检查可以阻止非可信网页调用 Go，但不代替应用的 host-key 校验和安全边界。若用 Web UI，避免把服务器返回的文件名、日志等当 HTML 执行，不要随意给远程内容 `TrustedOrigins: ["*"]`；若用 native terminal，也应明确 OSC 52/5522 剪贴板读写策略。终端读剪贴板默认拒绝，写剪贴板另有控制项。[2][4]

## 采用前最有价值的验证

以下是建议，尚未执行：

1. 在实际目标系统完成密码、加密私钥、agent、首次主机指纹确认、指纹变化拒绝、跳板机连接。
2. 验证 vim/tmux/htop、中文输入法、组合字符/emoji、高 DPI、快捷键、复制粘贴、分屏调整和全屏程序。
3. 让多个会话持续输出并同时传输大文件/大量小文件，观察输入延迟、CPU、内存与取消响应；测应用自己的边界，不采用框架宣传的内存数字作为验收值。
4. 验证断网/休眠唤醒/服务器关闭时 UI、连接和后台任务都能正确收尾；“重新 SSH 连接”本身不能恢复已终止的远程 shell，保活会话要另用 tmux 等机制设计。
5. 用真正安装包测试离线启动、动态库/凭据库依赖、签名/公证、卸载和自动更新；固定 MyGo 版本后再验证升级。

若是个人自用，愿意跟进框架且偏好 Go，MyGo 已具备足够吸引力；若即将给大量用户发行、不能承担框架迭代，建议同步用其他更成熟桌面框架做最小对比原型，再依据终端体验、发布链与维护成本选择。后一项是选型策略，本报告没有对其他框架做同深度实测。

## 来源

1. [MyGo v0.9.1 README 与文档概览/平台要求](https://github.com/egoist/mygo/blob/v0.9.1/docs/README.md)
2. [v0.9.1 Terminal 官方文档](https://github.com/egoist/mygo/blob/v0.9.1/docs/plugins/terminal.md)
3. [v0.9.1 terminal.Options 源码](https://github.com/egoist/mygo/blob/v0.9.1/plugins/terminal/terminal.go)
4. [v0.9.1 Architecture](https://github.com/egoist/mygo/blob/v0.9.1/docs/architecture.md)
5. [v0.9.1 go.mod](https://github.com/egoist/mygo/blob/v0.9.1/go.mod)
6. [v0.9.1 Bindings](https://github.com/egoist/mygo/blob/v0.9.1/docs/bindings.md)
7. [v0.9.1 Desktop APIs，Secrets](https://github.com/egoist/mygo/blob/v0.9.1/docs/native.md#secrets)
8. [v0.9.1 Distribution](https://github.com/egoist/mygo/blob/v0.9.1/docs/distribution.md)
9. [v0.9.1 Auto-updates](https://github.com/egoist/mygo/blob/v0.9.1/docs/updates.md)
10. [v0.9.1 LICENSE](https://github.com/egoist/mygo/blob/v0.9.1/LICENSE)
11. [GitHub 仓库元数据 API](https://api.github.com/repos/egoist/mygo)
12. [v0.9.1 Release](https://github.com/egoist/mygo/releases/tag/v0.9.1)
13. [v0.9.0 Release，Breaking changes](https://github.com/egoist/mygo/releases/tag/v0.9.0)
14. [v0.9.1 CI 定义](https://github.com/egoist/mygo/blob/v0.9.1/.github/workflows/ci.yml)
15. [Windows 退出进程残留修复 d6b9a0b](https://github.com/egoist/mygo/commit/d6b9a0bd5638dfc4473ace865d34a564646dfb8b)
16. [macOS 公证票据修复 ea02bc6](https://github.com/egoist/mygo/commit/ea02bc6b0afe268d34d926d599b098ca7a39c04c)
17. [Issue #1906：单实例锁](https://github.com/egoist/mygo/issues/1906)
18. [Issue #1902：Tester 资源释放](https://github.com/egoist/mygo/issues/1902)；[Issue #1890：XWayland 菜单](https://github.com/egoist/mygo/issues/1890)
19. [GitHub Contributors API](https://api.github.com/repos/egoist/mygo/contributors?per_page=100)
20. [MyGo 官方 showcase 源数据](https://github.com/egoist/mygo-showcase/blob/main/showcase.yaml)，[Universal S3 UI 项目](https://github.com/pteich/us3ui)
21. [Go SSH 官方包文档](https://pkg.go.dev/golang.org/x/crypto/ssh)
22. [Go knownhosts 官方包文档](https://pkg.go.dev/golang.org/x/crypto/ssh/knownhosts)
23. [pkg/sftp 项目及包文档](https://github.com/pkg/sftp)
24. [终端性能修复 PR #1687](https://github.com/egoist/mygo/pull/1687)
