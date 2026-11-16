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
| `internal/desktop/workspace.go` | 工作区标签与连接详情 |
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
| `internal/connection/` | SSH 认证、取消、单层跳板、主机信任与终端通道；SFTP 底层能力及协议回归 |
| `internal/testssh/` | 共享的回环 SSH 测试服务；`cmd/` 只在 `e2e` 构建中提供 UI 测试辅助进程 |

拆文件用于集中每项职责，界面仍在同一包内；不为每个视觉区域创建额外包，也不增加只有转发作用的抽象层。

## 依赖与扩展

当前正式入口依赖 `desktop`，`desktop` 使用 `library` 与 `connection`；`library` 不依赖 UI，`connection` 不依赖 UI 或连接库。配置文件读写及文件锁只由连接库处理，界面不直接序列化配置。测试服务不进入正式应用的依赖链。

P2 中 UI 负责操作、状态、终端实例与系统凭据集成；协议模块通过上下文和回调处理认证、握手、信任记录与远程通道。网络和凭据调用在后台执行，窗口更新统一派发到 UI 线程；每次连接有独立取消上下文与代次检查，过期结果不能覆盖新会话。工作区持有连接配置及跳板配置快照，编辑保存不改变已打开会话的目标。

主机确认串行显示，避免并发会话覆盖提示。未知密钥仅在用户确认后可靠写入应用自己的 `known_hosts`；变化密钥拒绝连接。系统凭据按数据目录、配置及认证目标隔离，仅在明确勾选且连接成功后保存，失败不回退明文文件。传输调度在 P4 建立，不提前增加空包。

## 工具与资料

`scripts/` 只保留正式构建、运行和产品 UI 验证脚本。`automation.go` 是仅在 `e2e` 构建中启用的产品测试入口，保留用于真实窗口验证。

`docs/` 保存规格、实现状态、约定和可复核资料；`build/`、`artifacts/` 为被 Git 忽略的生成物。历史原型代码已移除，旧性能数据仍保留其环境与测量局限，不能用作当前版本的性能结果。
