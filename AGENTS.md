# SSHub 项目约定

## 当前状态与资料

- 默认简体中文沟通，文本使用 UTF-8 无 BOM；代码标识符使用英文。
- 当前为 Windows x64 首版功能迭代与验收阶段，连接管理、SSH、多标签终端、SFTP、设置和配置迁移已实现。重启冷启动及全新 Windows 环境验收暂缓，不视为首版全部验收完成；不主动重启电脑。
- 按需读取 [产品规格与计划](docs/specs/20261116-sshub-v1/plan.md)、[任务地图](docs/specs/20261116-sshub-v1/map.md)、[架构说明](docs/architecture.md) 和 [剩余验收](docs/verification/remaining-acceptance.md)。旧规格与当前交互存在差异时，以用户最新确认及对应任务记录为准。
- 任务和规格使用本地 Markdown，保存在 `docs/specs/`，遵循 [任务跟踪约定](docs/agents/issue-tracker.md)。领域术语使用根目录 `GLOSSARY.md`，架构决策位于 `docs/adr/`，遵循 [领域文档约定](docs/agents/domain.md)。

## 技术与结构

- 使用 Go 1.27.4、MyGo v0.10.0 和官方 Ghostty 终端，保持原生模式及 `mygo_nowebview` 构建标签。固定依赖版本；升级需验证受影响的原生集成与资源表现。
- 根目录 `main.go` 只负责启动；界面与应用状态放在 `internal/desktop`，连接配置与偏好放在 `internal/library`，窗格及标签布局放在 `internal/workspace`，SSH / SFTP 协议放在 `internal/connection`，文件树传输放在 `internal/transfer`。UI 不实现 SSH 协议或终端模拟器。
- 优先在现有包内按职责拆文件，新增包应对应独立职责。界面状态在 UI 线程更新，网络、凭据及耗时操作在后台执行；保留取消、代次校验与共享连接的资源所有权。
- 配置、布局保持向后兼容，损坏数据或外部修改不能静默覆盖。`workspace.json` 是兼容保留的标签布局文件名，不代表产品存在“工作区”实体。

## 产品与界面

- 面向用户使用“连接”“标签”“终端窗格”，不恢复“打开工作区”入口。发起连接即打开标签；同一配置可打开多个会话，重启恢复标签但不自动连接。
- 保持紧凑界面和内置标题栏，中间仅标签与终端，管理操作优先放右键／按需菜单。连接面板和文件面板开关位于顶部菜单旁，传输队列包含在右侧文件面板内。
- 新布局默认隐藏文件面板，首次打开才初始化当前会话的 SFTP；恢复已有面板显隐偏好，隐藏面板不打断传输。
- 复用统一控件样式、GitHub Light / Dark 配色及内嵌 Lucide SVG 图标。界面与终端字体字号独立，保留键盘操作、辅助名称和窄窗口可用性。
- 新建和复制连接默认密码认证，编辑保留原认证方式。复制创建独立配置身份，不继承认证资料；分组可选择已有值或输入新值，并支持右键改组。凭据移除操作称为“删除凭据”。
- 关闭会话可记住“不再提示”，设置中可恢复；未完成传输的处置选择与退出应用确认保持独立。编辑、复制、改组及配置迁移不改变已有标签或传输任务的目标快照。

## 开发与验证

- 正式构建使用 `./scripts/build.ps1`，运行使用 `./scripts/run.ps1`。运行脚本仅在 EXE 缺失时构建，修改代码后需先重新构建；Windows 分发须保留同目录的 `ghostty-vt.dll`。
- 开发时可在设置 `GOFLAGS=-tags=mygo_nowebview` 的终端运行 `go tool mygo dev`，修改后会重新构建并重启；不要将开发版或带 automation 的调试版作为性能／发行结果。
- UI 专项使用 `./scripts/build.ps1 -DebugBuild -OutputDirectory build/<名称>`，再运行对应 `verify-*-ui.ps1 -SkipBuild -BuildDirectory build/<名称>`。调试构建包含 `e2e` automation，正式程序不开放该入口。
- 默认 test-after。按改动规模、影响范围和风险选择最小充分验证，不默认每次运行全量测试、全部 UI 回归或正式构建。仅修改文档时检查真实 diff 和文档链接；Go 改动对修改文件执行 `gofmt`，按需运行受影响包或具体用例的 test／vet；局部界面改动只验证相关交互与窗口。
- Go 检查使用原生标签，例如 `go test -tags mygo_nowebview ./internal/desktop`、`go vet -tags mygo_nowebview ./internal/desktop`。UI 代码按需在相同 `GOFLAGS` 下运行 `go tool mygo vet ./internal/desktop`。连接管理、SSH、标签布局、SFTP、设置、关闭确认分别有对应窗口专项脚本，按影响范围选用。
- 跨模块、公共 contract、安全或资源生命周期改动应覆盖相关回归；影响全局或准备发行时再运行全量检查。已通过的检查不无故重复；出现新改动、失败或未解决的疑点时再扩大验证，并在交付时说明实际验证范围及关键未测项。
- 测试使用独立数据目录、本地临时服务器与临时密钥，不操作用户真实服务器或覆盖用户配置。不跳过 SSH 主机密钥校验，不自动信任未知主机，不把凭据写入配置、日志或截图。

## 性能、生成物与提交

- 性能采样使用正式构建及 `scripts/measure-*.ps1`；诊断仅通过显式 `SSHUB_DIAGNOSTICS` 输出路径启用。结果写到 `docs/performance/`，附环境、构建、负载、计量定义和局限；历史结果不能当作当前版本复测结果。
- 不将输入接收速率当作渲染吞吐、UI build 时间当作帧率，或首帧显示时间当作可操作延迟。重启冷启动与干净环境验收不能由缓存启动或 PATH 隔离代替。
- `build/`、`artifacts/` 和 `.mygo/` 是生成物，不纳入版本控制。详细设计、操作边界和验证证据放在 `docs/`；README 保持简洁，面向 GitHub 访客，不堆砌实现日志与逐项验收结果。
- 提交前检查真实 diff，不混入无关变更。提交信息使用 `<type>(scope): <summary>`，类型与范围为英文，摘要使用简体中文动词开头、不加句号且不超过 50 字。
