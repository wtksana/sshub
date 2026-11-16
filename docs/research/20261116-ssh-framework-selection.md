# SSH/SFTP 桌面工具：性能与 AI 开发选型

查阅日期：2026-11-16，Asia/Hong_Kong。用户目标：启动快、占用低、性能好，全程 AI 开发，不将用户个人语言经验作为选型权重。本文未构建原型或做跨框架实测。

**主推候选：MyGo native UI + 官方 Ghostty terminal 插件 + Go SSH/SFTP 后端。** 理由是这条现有集成路线不启动 WebView，终端输出可以在 Go/native 侧直接进入模拟器，避免持续终端流跨 Rust/JS 或 Go/JS 桥接；它与用户的启动/资源优先级更一致。这是架构判断，并非已经证明成品比 Tauri 更快。若将平台成熟度、生态资料和稳定交付优先级提升到性能之前，则选择 **Tauri 2 稳定版 + Rust 后端 + xterm.js**。[1][2][3]

## 三条路线对比

| 路线 | 适合此需求的优势 | 主要成本与不确定性 | 建议 |
|---|---|---|---|
| MyGo native + libghostty-vt | 无 WebView；已有官方终端视图/Conn/Resize；Go 端直接连接 SSH；UI、协议和状态可在同一语言内验证。[1][2] | MyGo 只有约七周历史，0.x API 与平台行为仍快速变化；Ghostty 模拟器成熟不等于 MyGo 的渲染/输入/FFI 集成同样经历长期验证；终端另有动态库。[1][2] | 性能/占用优先的首选候选，先验收真实工作负载。 |
| Tauri 2 + Rust + xterm.js | Tauri 有稳定 2.x、完整打包与权限体系；xterm.js 已用于 VS Code、Tabby 等，提供 WebGL renderer、CJK/IME、vim/tmux 支持；前端测试和检查工具丰富。[3][4][5] | 仍需系统 WebView、JS 运行时、渲染器及终端流 IPC；需完成 Rust SSH/SFTP 与前端的生命周期、背压和二进制通路；不能用 Rust 无 GC 推断整机占用更低。[3][6][7] | 项目风险优先的备选；性能有机会满足需求，不应先验排除。 |
| GPUI + Rust + 自行接终端 | GPU UI，不以 WebView 作为主渲染路径，已有 Zed 这种复杂应用实践；当前文档涵盖 macOS、Linux/FreeBSD 和 Windows。[8] | GPUI 明确 pre-1.0、经常 breaking；通用桌面 UI 框架不等于开箱 SSH 终端。Zed 的 terminal/terminal_view 有大量项目依赖，且 GPL-3.0-or-later，不能把它们当成 GPUI 自带的 Apache-2.0 可插拔终端。[8][9][10] | 本项目暂不主推；除非愿意承担终端整合与框架维护工程。 |

## 两个容易误判的点

**Rust 没有 GC，并不能直接证明 Tauri 客户端更快。** 终端体验受模拟器、字体与 Unicode 处理、布局/绘制、流量批次、IPC 拷贝、滚动缓冲和 SSH 网络条件共同影响。Tauri 的系统 WebView 仍运行 JS；native Go 路线也仍有 GC、GPU/FFI 和缓冲区成本。应比较整套应用、整棵进程树及相同工作负载，不能只比较后端语言或主进程 RSS。[2][3][4]

**Tauri 2 的流式 IPC 不是只能 JSON/base64。** 官方要求高吞吐流使用 Channels，events 不针对低延迟/高吞吐设计；`Channel<InvokeResponseBody>` 发送 `Raw` 或 `Response` 可以让前端拿到 `ArrayBuffer`。`Channel<Vec<u8>>` 则会作为 JSON 数组发送。应批量输送、按 xterm.js 消费速度控制背压，并让 SFTP 文件字节在后端网络与磁盘之间直接流动。[6][7] 优化后桥接成本仍存在，但不能把它夸大成每条方案都必须进行 base64 编解码。

版本核对也有陷阱：GitHub `/releases/latest` 当前返回 `tauri-v3.0.0-alpha.6`，不能据此选择 v3 为正式稳定版。按正式 `tauri-v2.*` 标签筛选，当前 2.x 最新为 **v2.12.5，2026-10-21**；本建议明确针对 2.x。[11][12]

## 全程 AI 开发改变了什么

不再需要根据用户懂 Go 还是 Rust 选择，但需要更重视 **AI 能否查到准确版本的接口、运行验证、定位故障，以及在升级后重放测试**。更长的使用历史、清晰文档和稳定 API 是 Tauri/xterm.js 的优势；这不保证 AI 不会生成错误代码，而是降低依赖最新源码自证的比例。

MyGo 也有实际可用的验证能力：`ui.Tester` 可以无窗口验证 native UI；v0.9.1 官方 automation 插件支持列窗口、读取元素树、输入、截图和等待空闲，可组成跨平台 AI 操作—观察—修复闭环。[13][14] 因此不能简单断言“自绘 UI 不支持 AI 测试”。它仍要求将插件限定为开发/测试用途，并安排真实系统验证；无窗口单元测试不能替代输入法、GPU、安装签名和终端交互测试。

推荐的开发组织是：协议与传输逻辑独立于 UI；固定框架/终端版本；优先实现一个能连真实测试服务器的最小垂直切片，再扩展连接管理。不要让 AI 同时自行实现终端模拟器、密码库和 SSH 协议。

## 采用门槛

先以 MyGo native 路线做真实原型，验证冷启动到首帧、窗口可交互、首个 SSH 提示符三个不同时间点；记录空闲、单会话和多会话的整树内存。用相同字体/窗口/历史行数测试持续输出、tmux/vim、中文输入、分屏调整，并在传输大文件/大量小文件时检查输入延迟、取消和断网收尾。

没有用户明确阈值时，不虚构“必须低于多少 MB/毫秒”。若发现目标系统的输入、内存、性能或安装发布存在无法接受的问题，再用同一个场景跑 Tauri 2 最小原型。性能差异不足以抵消框架维护成本时，改选 Tauri 2 是合理结果；在验收前，结论是 **MyGo native 更符合当前优先级，而不是已经实测胜出**。

## 许可证

MyGo 主仓库与 Ghostty 为 MIT；Tauri 核心为 MIT/Apache-2.0，xterm.js 为 MIT；GPUI 为 Apache-2.0。Zed 的 `terminal` 和 `terminal_view` 各自标为 GPL-3.0-or-later，不能以 GPUI 的许可证概括复制出来的 Zed 应用代码。随包依赖、字体和动态库仍需依据所选确切版本记录许可。[5][9][10][15][16]

## 第一手来源

1. [前一份 MyGo 调研：版本、历史、发布、SSH/SFTP 适配](./20261116-mygo-evaluation.md)；[MyGo v0.9.1 文档](https://github.com/egoist/mygo/blob/v0.9.1/docs/README.md)
2. [MyGo v0.9.1 Terminal](https://github.com/egoist/mygo/blob/v0.9.1/docs/plugins/terminal.md)
3. [Tauri 官方 README/架构与系统 WebView](https://github.com/tauri-apps/tauri/tree/tauri-v2.12.5)
4. [xterm.js 官方 README：功能、现实应用与 renderer](https://github.com/xtermjs/xterm.js)
5. [xterm.js MIT 许可](https://github.com/xtermjs/xterm.js/blob/master/LICENSE)
6. [Tauri 2 Calling the Frontend：events 与 Channels](https://v2.tauri.app/develop/calling-frontend/)
7. [Tauri v2.12.5 channel.rs：顺序与二进制载荷](https://github.com/tauri-apps/tauri/blob/tauri-v2.12.5/crates/tauri/src/ipc/channel.rs)；[官方 Calling Rust：ArrayBuffer](https://v2.tauri.app/develop/calling-rust/)
8. [GPUI 官方 README：pre-1.0、平台与 API](https://github.com/zed-industries/zed/blob/main/crates/gpui/README.md)
9. [GPUI Cargo.toml / Apache-2.0](https://github.com/zed-industries/zed/blob/main/crates/gpui/Cargo.toml)
10. [Zed terminal Cargo.toml](https://github.com/zed-industries/zed/blob/main/crates/terminal/Cargo.toml)；[terminal_view Cargo.toml](https://github.com/zed-industries/zed/blob/main/crates/terminal_view/Cargo.toml)
11. [Tauri v2.12.5 release](https://github.com/tauri-apps/tauri/releases/tag/tauri-v2.12.5)
12. [Tauri v3.0.0-alpha.6 release](https://github.com/tauri-apps/tauri/releases/tag/tauri-v3.0.0-alpha.6)；[GitHub releases API](https://api.github.com/repos/tauri-apps/tauri/releases?per_page=100)
13. [MyGo v0.9.1 Automation](https://github.com/egoist/mygo/blob/v0.9.1/docs/plugins/automation.md)
14. [MyGo v0.9.1 UI Testing](https://github.com/egoist/mygo/blob/v0.9.1/docs/ui/testing.md)
15. [MyGo LICENSE](https://github.com/egoist/mygo/blob/v0.9.1/LICENSE)；[Ghostty LICENSE](https://github.com/ghostty-org/ghostty/blob/main/LICENSE)
16. [Tauri LICENSE-MIT](https://github.com/tauri-apps/tauri/blob/tauri-v2.12.5/LICENSE_MIT)；[Tauri LICENSE-APACHE](https://github.com/tauri-apps/tauri/blob/tauri-v2.12.5/LICENSE_APACHE-2.0)
