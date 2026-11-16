# SSHub

基于 MyGo 原生 UI 的 SSH / SFTP 桌面工具，优先支持 Windows，目标是启动快、占用低、交互流畅。

当前完成 P1 连接管理与 P2 安全 SSH：配置增删改查、分组、搜索、收藏、最近使用，以及真实终端连接。文件与传输队列区域位于右侧，左右面板可收起并拖动调整宽度。

支持密码、普通与加密私钥、Windows OpenSSH Agent，以及高级选项中的单层跳板机。先“打开工作区”，再点击“连接”；首次连接需核对主机指纹，指纹变化时拒绝连接。密码及私钥口令默认仅本次使用，主动勾选后保存在 Windows 凭据管理器，可从工作区“忘记凭据”。

连接可取消，断开后保留终端输出，再连接建立新的会话；关闭在线工作区或退出应用需要确认。当前未接入分屏、布局恢复和 SFTP 文件操作。功能范围与开发顺序见 [产品规格](docs/specs/20261116-sshub-v1/spec.md) 和 [实施计划](docs/specs/20261116-sshub-v1/plan.md)。

## 开发环境

- Windows x64、PowerShell 7、Go 1.27.4，固定 MyGo v0.9.1。
- 不需要 Node、Bun 或 WebView2。
- macOS / Linux 尚未实机验收；当前构建、运行和 UI 验证脚本针对 Windows。

## 构建与运行

在项目根目录执行：

```powershell
./scripts/build.ps1
./scripts/run.ps1
```

也可直接运行 `build/windows-amd64/SSHub.exe`，运行时不需要 Go。分发时需保留同目录的 `ghostty-vt.dll`。运行脚本在 EXE 不存在时自动构建；修改代码后需要重新构建。

配置默认保存在 Windows 的 `%APPDATA%\SSHub\connections.json`。新建或编辑时输入分组即可建立分组；最近使用以“打开工作区”为准。配置损坏或版本不支持时显示错误并保留文件，同一数据目录只允许一个实例写入。

主机信任记录位于同一数据目录的 `known_hosts`，可在“已信任主机”中查看与移除，移除前需确认。系统凭据按数据目录和连接身份隔离，不写入 JSON 或日志；修改服务器地址等身份字段后需重新提供凭据。Agent 默认使用 Windows OpenSSH Agent 命名管道，也支持 `SSH_AUTH_SOCK` 指定的端点。

使用独立目录试用：

```powershell
./scripts/run.ps1 -DataDir 'D:\Temp\sshub-demo'
```

应用仅保留正式入口和 `-data-dir` 参数。

## 验证

```powershell
go test ./...
go vet ./...
./scripts/verify-library-ui.ps1
./scripts/verify-ssh-ui.ps1
```

Go 测试覆盖配置持久化、100 条连接筛选、双实例锁、外部修改保护与跳板引用约束；本地临时 SSH / SFTP 服务覆盖认证、终端收发、resize、目录读取、主机密钥变化、取消握手和跳板两端校验。Windows 测试还覆盖 Agent 命名管道及系统凭据读写与隔离。

`verify-library-ui.ps1` 构建带官方 automation 的调试版本，并在独立数据目录中验证中文字段、连接配置增删改查、端口校验、认证配置、分组 / 收藏 / 最近筛选、侧栏收起、重启持久化及损坏文件保护。测试不连接你的服务器。

`verify-ssh-ui.ps1` 在独立目录运行仅监听回环地址的临时 SSH 服务，验证指纹确认／取消、错误密码重试、密码遮蔽、真实终端中文回显、调整面板、断开、凭据记住／忘记、主机信任管理、跳板加密私钥连接及退出确认。测试辅助入口仅使用 `e2e` 构建，不进入正式程序。

当前验证不代表已完成中文输入法组合输入、vim/top 等交互程序、真实 Windows OpenSSH 服务或长时间负载兼容性验收；这些仍需后续 Windows 实测。

调试验证后可重新运行 `./scripts/build.ps1` 生成正式构建。构建脚本只对当前进程设置 Go 备用源，结束后恢复环境；可通过 `-Proxy` 指定来源。

产品性能目标尚待后续阶段正式测量；已有的 [历史性能记录](docs/performance/20261116-mygo-baseline.md) 仅供选型参考，其对应的原型代码和脚本已移除。

## 项目结构

```text
main.go                     正式应用启动与命令行参数
mygo.json                   MyGo 构建配置
internal/
  desktop/                  原生窗口、状态、布局与各区域交互
  library/                  连接配置模型、筛选、存储与系统文件锁
  connection/               SSH 认证、主机信任、跳板及协议测试
  testssh/                  仅供协议与产品 UI 测试的本地 SSH 服务
scripts/                    正式构建、运行与产品 UI 验证
docs/
  architecture.md           模块职责与依赖约定
  specs/                    产品规格、实施计划与本地任务
  adr/                      架构决策
  agents/                   项目 Agent 工作流
  research/                 历史选型调研
  performance/              带计量口径的性能记录
build/                      构建生成物，不纳入版本控制
artifacts/                  验证生成物，不纳入版本控制
```

详细职责与后续扩展方式见 [项目结构说明](docs/architecture.md)。
