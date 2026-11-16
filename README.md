# SSHub

基于 MyGo 原生 UI 的 SSH / SFTP 桌面工具，优先支持 Windows，目标是启动快、占用低、交互流畅。

当前完成 P1：三栏应用骨架、连接配置增删改查、分组、搜索、收藏、最近使用和本机持久化。文件列表与传输队列位于右侧，左右面板可收起并拖动调整宽度。

SSH 连接及 SFTP 传输尚未接入正式界面。“打开工作区”创建待连接标签，不发起网络连接；认证方式目前只保存配置，不会要求或保存密码。功能范围与开发顺序见 [产品规格](docs/specs/20261116-sshub-v1/spec.md) 和 [实施计划](docs/specs/20261116-sshub-v1/plan.md)。

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

也可直接运行 `build/windows-amd64/SSHub.exe`，运行时不需要 Go。运行脚本在 EXE 不存在时自动构建；修改代码后需要重新构建。

配置默认保存在 Windows 的 `%APPDATA%\SSHub\connections.json`。新建或编辑时输入分组即可建立分组；最近使用以“打开工作区”为准。配置损坏或版本不支持时显示错误并保留文件，同一数据目录只允许一个实例写入。

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
```

Go 测试验证配置持久化、100 条连接筛选、双实例锁与外部修改保护，以及本地临时 SSH / SFTP 服务的认证、终端收发、resize、目录读取和主机密钥校验。协议模块为下一阶段提供基础能力，尚不代表产品界面已支持完整认证和文件传输。

`verify-library-ui.ps1` 构建带官方 automation 的调试版本，并在独立数据目录中验证中文字段、连接配置增删改查、端口校验、认证配置、分组 / 收藏 / 最近筛选、侧栏收起、重启持久化及损坏文件保护。测试不连接你的服务器。

调试验证后可重新运行 `./scripts/build.ps1` 生成正式构建。构建脚本只对当前进程设置 Go 备用源，结束后恢复环境；可通过 `-Proxy` 指定来源。

产品性能目标尚待后续阶段正式测量；已有的 [历史性能记录](docs/performance/20261116-mygo-baseline.md) 仅供选型参考，其对应的原型代码和脚本已移除。

## 项目结构

```text
main.go                     正式应用启动与命令行参数
mygo.json                   MyGo 构建配置
internal/
  desktop/                  原生窗口、状态、布局与各区域交互
  library/                  连接配置模型、筛选、存储与系统文件锁
  connection/               SSH / SFTP 协议基础与测试
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
