# 项目结构

当前采用一个 Go module、一个桌面入口及三个内部包。根目录保留 `main.go` 与 `mygo.json`，符合现有 MyGo 构建方式；当前只有一个可执行应用，入口无需额外迁移到 `cmd/`。

## 模块职责

| 位置 | 职责 |
| --- | --- |
| `main.go` | 解析 `-data-dir`，启动正式应用，报告启动错误 |
| `internal/desktop/window.go` | MyGo 应用生命周期和窗口创建；包对外提供 `Run` |
| `internal/desktop/app.go` | 界面状态及配置、收藏、最近使用等操作 |
| `internal/desktop/view.go` | 主布局、主题与全局反馈 |
| `internal/desktop/sidebar.go` | 连接库、搜索和筛选 |
| `internal/desktop/workspace.go` | 工作区标签与连接详情 |
| `internal/desktop/editor.go` | 连接编辑表单、校验反馈及保存交互 |
| `internal/desktop/files.go` | 右侧文件与传输区域；当前尚未接入 SFTP |
| `internal/library/profile.go` | 连接配置模型与字段校验 |
| `internal/library/filter.go` | 搜索、分组、收藏及最近使用筛选 |
| `internal/library/store.go` | 配置加载、CRUD、可靠写入和并发实例保护 |
| `internal/library/lock_*.go` | 按系统选择文件锁实现 |
| `internal/connection/` | SSH / SFTP 协议基础与回归测试，待 P2 扩展并接入界面 |

拆文件用于集中每项职责，界面仍在同一包内；不为每个视觉区域创建额外包，也不增加只有转发作用的抽象层。

## 依赖与扩展

当前正式入口依赖 `desktop`，`desktop` 使用 `library`；`library` 不依赖 UI，`connection` 不依赖 UI 或连接库。配置文件读写及文件锁只由连接库处理，界面不直接序列化配置。

P2 接入网络后，UI 负责触发操作与显示状态，协议模块处理认证、握手和远程通道。会话管理、凭据库和传输调度在各自功能开始时建立明确职责，不提前创建空包或通用 `utils` 包。

## 工具与资料

`scripts/` 只保留正式构建、运行和产品 UI 验证脚本。`automation.go` 是仅在 `e2e` 构建中启用的产品测试入口，保留用于真实窗口验证。

`docs/` 保存规格、实现状态、约定和可复核资料；`build/`、`artifacts/` 为被 Git 忽略的生成物。历史原型代码已移除，旧性能数据仍保留其环境与测量局限，不能用作当前版本的性能结果。
