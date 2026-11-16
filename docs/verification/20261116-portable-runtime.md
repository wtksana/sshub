# 便携运行复测入口

日期：2026-11-16。本轮没有修改产品 Go 代码；新增 Windows 自带 PowerShell 5.1 可运行的独立校验脚本，随候选 ZIP 分发，补齐另一台 Windows 机器的复测入口。它不要求检出源码、Go、Node、Python 或安装 PowerShell 7。

`scripts/verify-portable-runtime.ps1` 先校验包内文件 SHA256，再创建临时 APPDATA，使用仅含 Windows 目录的子进程 PATH 运行正式应用；检查 ready/show、连接数零和正常退出，再把程序复制到另一目录复验配置／主题／信任／布局保留。不会读取或修改原用户 SSHub 数据。输出目录须是全新目录且位于程序目录之外，防止递归复制到自己。

当前机器上实际使用 `Windows PowerShell 5.1.26100.9818` 跑完整检查通过，原目录／替换目录均正常退出；无秘密结果见 [portable-runtime-summary.json](20261116-portable-runtime-summary.json)。原始诊断及临时数据在 `artifacts/portable-runtime/20261116-222601/`。宿主仍有 Go／Node／Python，报告中如实列出可发现工具；不把本次结果升级为干净 Windows 机器验收。

首次直接用 `-File` 被宿主执行策略拒绝，未更改执行策略；改用 `-Command` 直接执行已检查的脚本代码完成复测，方式已写入[便携说明](../portable-readme.md)。启动 Windows PowerShell 时移除由当前 PowerShell 7 进程继承的 `PSModulePath`，使其加载系统自身模块；只影响该子进程。脚本对 JSON、诊断及校验清单显式使用 UTF-8，避免 5.1 将中文文件名读成本地编码。

最终候选包：`build/releases/20261116-222601/SSHub-0.1.0-windows-amd64.zip`，SHA256 `b91cdf11c1737a876a5874095389c9f72d94bcab265abf62c29ac333b09c2605`。EXE SHA256 仍为 `f3ab848f26fdc913dac6e4b3b35770fa41a4e266cebe66e0966f6dc43708d0d0`，没有重新构建或重复正式十分钟负载。新包增加校验脚本与更新后的说明，因此 ZIP 哈希与此前不同。现有报告的早期包仍保留为历史证据，不混写其哈希。

另在临时副本中破坏 DLL，校验脚本明确拒绝，在启动前结束。新增 `verify-native-drop.ps1` 的内容检查也验证了缺失上传和内容篡改被拒绝、完整复制数据可通过；这仅验证内容检查器，不是原生系统拖放实测。原始产物在 `artifacts/drop-verifier-check/20261116-222531/`，没有将复制数据作为 OLE 上传证据。

检查范围：实际 PowerShell 5.1 运行、程序启动／退出及换目录、校验失败路径、拖放内容检查器的正反路径、PowerShell 语法及真实 diff。没有运行全仓 Go 测试；所有测试应用均退出。

首版仍待系统拖放、实际系统 Agent 服务、重启冷启动及全新 Windows 环境证据，具体操作见[剩余验收](remaining-acceptance.md)。包内脚本不测试 SSH／Ghostty 实际加载、输入响应或冷启动，须分别核对。
