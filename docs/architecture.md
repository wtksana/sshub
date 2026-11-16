# 项目结构

当前采用一个 Go module、一个桌面入口及按职责组织的内部包。根目录保留 `main.go` 与 `mygo.json`，符合现有 MyGo 构建方式；当前只有一个正式应用，入口无需额外迁移到 `cmd/`。

## 模块职责

| 位置 | 职责 |
| --- | --- |
| `main.go` | 解析 `-data-dir`，启动正式应用，报告启动错误 |
| `internal/desktop/window.go` | MyGo 应用生命周期和窗口创建；包对外提供 `Run` |
| `internal/desktop/app.go` | 界面状态及配置、收藏、最近使用等操作 |
| `internal/desktop/view.go` | 主布局、主题与全局反馈 |
| `internal/desktop/sidebar.go` | 连接库、搜索和筛选 |
| `internal/desktop/workspace.go` | 窗格树渲染、平面标签组、状态点及按需搜索 |
| `internal/desktop/layout.go` | 工作区恢复、后台合并保存及退出刷新 |
| `internal/desktop/menus.go` | 连接项／标签右键菜单、窗格菜单、视图及筛选菜单 |
| `internal/desktop/terminal_tools.go` | 官方终端搜索与字号操作 |
| `internal/desktop/session.go` | 异步连接、终端生命周期、取消及断开 |
| `internal/desktop/prompts.go` | 串行主机确认与凭据输入提示 |
| `internal/desktop/credentials.go` | 系统凭据读写与连接身份隔离 |
| `internal/desktop/session_dialogs.go` | 关闭确认及主机信任管理 |
| `internal/desktop/editor.go` | 连接编辑表单、校验反馈及保存交互 |
| `internal/desktop/files.go` | 右侧文件与传输区域；当前尚未接入 SFTP |
| `internal/library/profile.go` | 连接配置模型与字段校验 |
| `internal/library/filter.go` | 搜索、分组、收藏及最近使用筛选 |
| `internal/library/store.go` | 配置加载、CRUD、可靠写入和并发实例保护 |
| `internal/library/lock_*.go` | 按系统选择文件锁实现 |
| `internal/workspace/` | 分屏树、标签引用、激活／移动／关闭／合并语义、严格校验与布局可靠写入 |
| `internal/connection/` | SSH 认证、取消、单层跳板、主机信任与终端通道；SFTP 底层能力及协议回归 |
| `internal/testssh/` | 共享的回环 SSH 测试服务；`cmd/` 只在 `e2e` 构建中提供 UI 测试辅助进程 |

拆文件用于集中每项职责，界面仍在同一包内；不为每个视觉区域创建额外包，也不增加只有转发作用的抽象层。

## 依赖与扩展

当前正式入口依赖 `desktop`，`desktop` 使用 `library`、`workspace` 与 `connection`；三个包均不依赖 UI。连接配置文件读写及数据目录锁由连接库处理；工作区在同一锁的生命周期内读写独立布局文件，界面不自行解析或替换文件。测试服务不进入正式应用的依赖链。

P2 中 UI 负责操作、状态、终端实例与系统凭据集成；协议模块通过上下文和回调处理认证、握手、信任记录与远程通道。网络和凭据调用在后台执行，窗口更新统一派发到 UI 线程；每次连接有独立取消上下文与代次检查，过期结果不能覆盖新会话。工作区持有连接配置及跳板配置快照，编辑保存不改变已打开会话的目标。

主机确认串行显示，避免并发会话覆盖提示。未知密钥仅在用户确认后可靠写入应用自己的 `known_hosts`；变化密钥拒绝连接。系统凭据按数据目录、配置及认证目标隔离，仅在明确勾选且连接成功后保存，失败不回退明文文件。传输调度在 P4 建立，不提前增加空包。

P3 将布局与运行中资源分开：`workspace.State` 只记录窗格、标签 ID、配置 ID 与展示偏好；`desktop.workspace` 按标签 ID 持有真实 SSH 连接、取消上下文和终端实例。移动或合并只改变标签归属，不释放连接；删除标签才释放对应资源。恢复根据配置 ID 读取当前配置并创建待连接状态，已删除配置不从布局复活。

布局写入使用独立 goroutine、容量一的最新快照队列和 250 ms 合并窗口，UI 只提交不可变编码快照；退出前刷新，随后释放数据目录锁。严格限制文件大小、版本、树结构、ID 唯一性和比例范围；坏文件与外部修改不会被静默覆盖。终端搜索仍由 Ghostty 完成，界面不复制输出到另一个搜索缓存。

连接详情与常驻会话工具条已移除，管理动作绑定连接项或标签对象。终端内部保留官方复制粘贴菜单；应用菜单使用框架的窗口内菜单，与真实 UI 自动化共享行为。菜单回调发生在窗格渲染期间，分屏、移动与合并先排队，整棵树渲染及焦点处理结束后执行，避免遍历中改变树结构。

## 工具与资料

`scripts/` 只保留正式构建、运行和产品 UI 验证脚本。`automation.go` 是仅在 `e2e` 构建中启用的产品测试入口，保留用于真实窗口验证。

`docs/` 保存规格、实现状态、约定和可复核资料；`build/`、`artifacts/` 为被 Git 忽略的生成物。历史原型代码已移除，旧性能数据仍保留其环境与测量局限，不能用作当前版本的性能结果。
