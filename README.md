# SSHub

基于 MyGo 原生 UI 的 SSH / SFTP 桌面工具，优先支持 Windows，目标是启动快、占用低、交互流畅。

窗口使用应用内置标题栏和窗口按钮，支持拖动、双击最大化／还原、最小化和边缘缩放，关闭时仍会确认在线会话。

当前已实现 P1–P5：连接管理、安全 SSH、多会话工作区、SFTP 文件浏览和传输，以及设置与配置迁移。支持独立标签组、左右／上下分屏、标签移动与窗格合并、终端搜索、字号缩放，以及重启恢复布局。文件与传输队列位于右侧，左右面板可通过顶部菜单旁的按钮切换，并拖动调整宽度。新布局默认收起文件面板，打开后才为当前 SSH 会话初始化 SFTP；已保存的面板开关仍会恢复。

支持密码、普通与加密私钥、Windows OpenSSH Agent，以及高级选项中的单层跳板机。双击连接项即可发起连接，也可右键“打开工作区”后手动连接；首次连接需核对主机指纹，指纹变化时拒绝连接。密码及私钥口令默认仅本次使用，主动勾选后保存在 Windows 凭据管理器，可从工作区“忘记凭据”。

连接可取消，断开后保留终端输出，再连接建立新的会话；关闭在线工作区或退出应用需要确认。功能范围与开发顺序见 [产品规格](docs/specs/20261116-sshub-v1/spec.md) 和 [实施计划](docs/specs/20261116-sshub-v1/plan.md)。

右侧文件列表跟随当前终端：路径框回车跳转，双击目录进入；文件右键提供下载、重命名、属性／八进制权限和删除，右上“更多”提供刷新、主目录、新建文件夹及上传。也可将本地文件或文件夹拖入右侧列表上传。文件、文件夹及符号链接使用各自图标；递归传输拒绝跟随链接，递归删除只移除链接本身。

文件标题同行显示服务器状态与项目数，读取目录时项目数位置显示读取状态。列表含名称、修改时间、大小三列，点击表头依次切换升序、降序、默认排列；目录始终优先，默认按名称排列。排序在本地进行，同一会话刷新或跳转目录保留当前排序。修改时间采用本机时区，悬停可查看完整名称及时间；窄面板可横向滚动。

界面统一使用内嵌的 Lucide SVG 图标，随浅深主题着色，运行时无需联网或安装图标字体。图标资源及名称映射见 [图标目录](internal/desktop/icons/lucide/README.md)，许可随便携包分发。

传输队列在文件列表内，全局串行执行，显示所属服务器、进度、平均速度及结果；任务右键可取消或从头重试，必要时使用原配置快照重新连接。关闭终端可选择继续传输；退出有未完成任务时必须确认取消。同名文件默认询问覆盖、跳过或另存，覆盖／跳过可应用到同批次。任务与目标路径在创建时确定，切换标签及编辑连接配置不会改变目标。

文件先写入 `.sshub-*.partial`，关闭写入成功后再发布。失败不替换原文件；若断网导致远程临时文件无法删除，错误会列出其路径。目录传输按文件提交，失败时已完成的文件与已创建目录保留；重试会重新检查冲突，不提供断点续传。新上传文件使用 600 权限，覆盖保留原权限；不支持 chmod 或原子覆盖扩展的服务器会明确失败，不通过删除原文件绕过。暂不保证在其他程序同时修改目标路径时的操作结果。

最多同时显示四个终端窗格；在当前窗格打开连接会新增独立标签，同一服务器可开多个会话。点击终端或标签切换当前窗格，右侧服务器状态随之切换。中间只保留标签与终端；连接项右键提供编辑、收藏和删除，标签右键或标签栏“…”提供连接、断开、搜索、字号、分屏、移动、合并与关闭。“合并窗格”移动该窗格的全部标签，保留已有连接。

终端搜索支持 `Ctrl+Shift+F`，复制／粘贴使用 `Ctrl+Shift+C`／`Ctrl+Shift+V`；字号可通过标签右键菜单或 Ctrl+滚轮调整。`Ctrl+Shift+Enter` 连接当前标签，`Ctrl+B` 切换连接面板，`Ctrl+Shift+E` 切换文件面板。左栏“⋯”提供收藏、最近使用与分组筛选；顶部菜单提供设置、配置导入导出和已信任主机管理；面板显隐按钮直接位于菜单按钮旁。字号与布局按标签保存，恢复时只加载当前连接配置，不恢复 SSH、输出或远程程序，也不自动连接。

顶部应用菜单 →“设置”可选择 GitHub Light、GitHub Dark 或跟随系统，分别配置界面字体／字号（10–20）与终端字体／默认字号（8–32）。界面字体留空时使用系统默认，终端和界面字体互不影响。保存后立即生效，默认保留各标签的字号；勾选“同时将现有标签字号设为默认”才统一重置。外观、界面和终端默认设置保存在 `connections.json`，布局和标签字号仍保存在 `workspace.json`。

“导出配置”生成独立的 SSHub JSON 文件，包含连接、分组、收藏与偏好，不包含密码、口令、私钥内容、系统凭据引用、主机信任、最近使用或工作区布局。目标已存在时需确认覆盖；禁止覆盖应用管理的数据文件。

“导入配置”先校验并预览，同组重名逐项选择跳过、另存或替换；可选择同时导入偏好。确认前不写入，配置变化后需要重新预览；全部连接、跳板引用和偏好校验成功后一次提交。每个导入／替换的连接分配新 ID，密码和私钥口令需重新输入，主机身份仍需正常核对；私钥路径必须在编辑连接中勾选“已核对本机私钥路径”后才能连接。系统 Agent 继续使用本机服务，不迁移 Agent 密钥。

替换不改变已打开标签或传输任务的配置快照；当次进程使用新配置需打开新工作区。保存的标签引用会同步指向新配置，重启仍恢复为待连接状态。跳过同名跳板时可继续引用保留的本地跳板；跳过所依赖的新跳板或形成多层跳板会拒绝整次导入。原始 `connections.json` 不是迁移格式，请使用应用内导出生成迁移文件。

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

`workspace.json` 保存窗格、标签的配置引用、字号、侧栏状态与分隔比例，以及正常退出时的窗口尺寸，不含秘密或终端输出。已删除的配置在恢复时跳过；损坏布局显示提示、保留原文件，本次不覆盖，可将该文件重命名后重启。外部修改会阻止覆盖并报错。布局后台合并写入，正常退出前刷新最后状态。

使用独立目录试用：

```powershell
./scripts/run.ps1 -DataDir 'D:\Temp\sshub-demo'
```

应用仅保留正式入口和 `-data-dir` 参数。

便携 ZIP、第三方许可及校验和由 `./scripts/package.ps1` 生成，输出在 `build/releases/`。解压使用及手动升级说明见 [便携使用说明](docs/portable-readme.md)。首版逐项验收进展见 [验收记录](docs/specs/20261116-sshub-v1/acceptance.md)；仍有待补实测项时，不视为首版验收完成。

正式性能采样使用 `measure-startup.ps1`（10 次缓存首帧显示）及 `measure-product.ps1 -Sessions 1`／`-Sessions 8 -Seconds 600`。这两个脚本要求 PowerShell 7.5+，使用独立数据、回环 SSH 服务和显式启用的无文本诊断；恢复标签需通过真实 UI 手动连接。原始数据在 `artifacts/`，可复核报告保存在 `docs/performance/`。不将首帧时间当作端到端交互延迟，也不将接收速率当作渲染吞吐。

包内 `verify-portable-runtime.ps1` 可用 Windows 自带 PowerShell 5.1 复测文件哈希、启动／退出、换目录保留数据，提供不依赖开发工具的复测入口；不会修改现有用户配置。脚本及限制见[便携说明](docs/portable-readme.md)。

`verify-release.ps1 -Archive <ZIP> -PreviousExecutable <旧版EXE>` 解压并校验每个文件，使用仅含 Windows 路径的子进程 PATH 和隔离 APPDATA 验证启动、正常退出及替换程序后配置／布局保留；不修改当前用户环境，也不等同于全新虚拟机验收。

## 验证

按改动范围选择受影响包和对应窗口检查；以下为可选验证入口，不要求每次全部运行：

```powershell
go test -tags mygo_nowebview ./internal/library ./internal/connection ./internal/transfer ./internal/desktop
go vet -tags mygo_nowebview ./internal/library ./internal/connection ./internal/transfer ./internal/desktop
./scripts/verify-library-ui.ps1
./scripts/verify-ssh-ui.ps1
./scripts/verify-workspace-ui.ps1
./scripts/verify-window-ui.ps1
./scripts/verify-sftp-ui.ps1
./scripts/verify-settings-ui.ps1
```

Go 测试覆盖配置持久化、100 条连接筛选、双实例锁、外部修改保护与跳板引用约束；本地临时 SSH / SFTP 服务覆盖认证、终端收发、resize、目录读取、主机密钥变化、取消握手和跳板两端校验。Windows 测试还覆盖 Agent 命名管道及系统凭据读写与隔离。

`verify-library-ui.ps1` 构建带官方 automation 的调试版本，并在独立数据目录中验证中文字段、连接配置增删改查、端口校验、认证配置、分组 / 收藏 / 最近筛选、侧栏收起、重启持久化及损坏文件保护。测试不连接你的服务器。

`verify-ssh-ui.ps1` 在独立目录运行仅监听回环地址的临时 SSH 服务，验证指纹确认／取消、错误密码重试、密码遮蔽、真实终端中文回显、调整面板、断开、凭据记住／忘记、主机信任管理、跳板加密私钥连接及退出确认。测试辅助入口仅使用 `e2e` 构建，不进入正式程序。

`verify-workspace-ui.ps1` 验证四窗格八条真实 SSH 会话、指针与焦点联动、搜索与字号、标签移动和窗格合并、关闭同配置会话的隔离、断网输出保留和新会话重连、分隔比例及字号恢复、最小窗口布局，以及损坏／外部修改布局保护；新增双击连接、右键未激活标签的操作归属、按需搜索和终端原有复制粘贴菜单验证。八会话属于已验证规模，不是硬限制；四可见窗格为当前限制。

`verify-window-ui.ps1` 在独立数据目录验证内置窗口按钮、拖动、双击最大化／还原、最小化与恢复、实际顶部边缘缩放、Windows 贴靠命中区域、最大化退出后恢复正常窗口尺寸。窗口位置与缩放检查使用 Win32，界面操作使用官方 automation。

`verify-sftp-ui.ps1` 使用以 `os.Root` 隔离的回环 SFTP 服务，验证目录浏览、新建／重命名、路径跳转、拒绝访问、权限修改与删除确认。`-TransferPickers` 在交互终端暂停，供人工或 computer-use 驱动系统文件选择器，临时路径记录在 `artifacts/sftp-ui/active.json`；完成后回车结束测试。已用真实选择器验证中文文件上传下载、同名跳过、任务取消、原标签关闭后重连重试、另存，以及关闭终端继续传输和未完成任务退出提醒。协议测试另覆盖文件夹往返、批量冲突、断网、临时文件清理、远端 ENOSPC 原文件保护与符号链接保护。`-LargeDirectory` 已验证 2,000 条虚拟列表首尾滚动；资源管理器实际拖入中文文件、嵌套文件夹与空目录也已通过，见[拖入专项](docs/verification/20261116-native-drop.md)。

`verify-settings-ui.ps1` 使用独立数据目录验证主题／字体保存、重启、新标签默认字号、导出覆盖确认、坏文件与取消不写入、重名另存／替换／跳过、私钥核对与标签引用恢复，并保存浅深主题截图。`-NativePickers` 已验证真实保存／打开对话框中的中文空格路径导出、重新预览及取消导入。SSH UI 回归另验证在线终端应用深色主题和字体字号后继续收发。

`verify-integration-ui.ps1 -NativeActions` 在交互终端验证真实多选上传：任务运行时切到另一服务器、关闭来源终端，核对活动及排队文件仍落原服务器原目录、内容哈希一致。第二次暂停期间，可运行 `verify-terminal-clipboard.ps1` 或其 `-NativeCopy` 模式，验证中文空格经 Windows 剪贴板粘贴、SSH 回显、搜索和复制后的完整内容。Windows 设置浅→深→浅实时切换也已实测通过，具体步骤、脚本复用及局限见[原生集成专项](docs/verification/20261116-native-integration.md)。

`verify-openssh-ui.ps1 -OpenSSHDirectory <OpenSSH-Win64目录>` 使用临时密钥、独立配置及回环高端口运行真实便携 OpenSSH，不安装系统服务。已验证产品主机指纹核对、私钥认证、Windows PTY shell、SFTP 浏览／新建目录，以及真实 Vim 中文编辑、保存和退出恢复；组件版本与复现步骤见[OpenSSH／Vim 专项](docs/verification/20261116-openssh.md)。

中文输入法组合提交、真实 Linux OpenSSH／Unix chmod 和 BusyBox top、正式版缓存启动可操作均已补齐；见[原生输入与启动](docs/verification/20261116-native-input.md)和[Linux 服务器专项](docs/verification/20261116-linux-openssh.md)。实际[系统 OpenSSH Agent](docs/verification/20261116-system-agent.md)也已验收，临时密钥和服务状态已恢复。[资源管理器拖入](docs/verification/20261116-native-drop.md)已由用户执行并核对内容。剩余重启冷启动和干净 Windows 环境；复测入口见[剩余验收操作](docs/verification/remaining-acceptance.md)，完整状态见[首版验收记录](docs/specs/20261116-sshub-v1/acceptance.md)。

调试验证后可重新运行 `./scripts/build.ps1` 生成正式构建。如果当前程序占用默认构建目录，可用 `./scripts/build.ps1 -OutputDirectory build/titlebar` 生成到独立目录；验证脚本对应使用 `-BuildDirectory build/titlebar`。构建脚本只对当前进程设置 Go 备用源，结束后恢复环境；可通过 `-Proxy` 指定来源。

正式构建已完成十次缓存首帧显示、单终端空闲及八终端十分钟负载采样：首帧平均约 295 ms，空闲工作集峰值 104.8 MiB，八终端峰值 187.3 MiB。另以原生快捷键及截图核对三次缓存启动可操作上界，平均 486.6 ms、最大 526.6 ms。首帧不等同可操作延迟，详见[正式性能报告](docs/performance/20261116-product.md)。[历史性能记录](docs/performance/20261116-mygo-baseline.md) 仅供选型参考，其对应的原型代码和脚本已移除。

## 项目结构

```text
main.go                     正式应用启动与命令行参数
mygo.json                   MyGo 构建配置
internal/
  desktop/                  原生窗口、状态、布局与各区域交互
  library/                  连接配置模型、筛选、存储与系统文件锁
  workspace/                窗格与标签状态、布局校验及可靠持久化
  connection/               SSH 认证、主机信任、跳板及协议测试
  transfer/                 SFTP 文件树传输、冲突、进度与临时文件发布
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
