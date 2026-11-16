# Windows 系统 OpenSSH Agent 专项

日期：2026-11-16。用户明确授权临时启用并启动原为 `Disabled / Stopped` 的系统 `ssh-agent`，仅添加本次临时密钥，验后移除并恢复原服务状态；没有重启电脑。

实际服务为 `C:\Windows\System32\OpenSSH\ssh-agent.exe`，文件版本 `9.5.6.4`、产品版本 `OpenSSH_9.5p2 for Windows`。管理临时密钥使用此前下载的便携 Win32-OpenSSH 10.4p1 Preview 的 `ssh-add.exe`；产品认证使用系统服务的默认命名管道，不使用便携 agent 或 Go 模拟 agent。测试进程移除 `SSH_AUTH_SOCK` 环境变量，确保走默认 `\\.\pipe\openssh-ssh-agent`。

## 已验证

- 原服务状态与启动方式先落盘，临时设为 Manual 后启动。初始系统 Agent 中没有已载入身份。
- 回环 SSH fixture 新生成测试私钥，只把该密钥加入系统 Agent；产品配置认证方式为 `agent`，没有引用私钥路径。产品弹窗中的主机指纹与从临时服务公钥独立计算的指纹一致，核对后才接受。
- 系统 Agent 完成实际签名认证，产品进入“已连接”；右侧 SFTP 显示 `Agent 中文.txt`，真实终端文本回显经搜索确认一处。`system-agent.png` 已视觉核对，包含连接状态、文件列表和中文搜索结果。
- 产品通过在线退出确认正常退出，临时 SSH 服务停止。finally 仅删除本次临时公钥；重新列举身份后集合与运行前一致（本次均为空）。没有执行清空所有密钥。
- 外层 finally 停止系统 Agent 并恢复 Disabled；再次从系统服务读取确认为 `Stopped / Disabled`，与原始记录一致。没有遗留 SSHub 或 SSH fixture 进程。

原始产品证据：`artifacts/system-agent-ui/20261116-234914/`；服务前后记录：`artifacts/system-agent-service/20261116-234914/`。无秘密摘要见 [system-agent-summary.json](20261116-system-agent-summary.json)。

首次运行在添加密钥前被 OpenSSH 的私钥 ACL 检查拒绝，服务已由 finally 恢复，未留下临时身份。已修正脚本：仅对生成的测试私钥取消宽泛继承，保留当前用户、SYSTEM、Administrators 访问权限；未修改用户密钥或系统安全配置。另兼容 `ssh-add -l` 将“无身份”信息写入 stderr 的情况。修正后完整成功路径通过。

## 复现与局限

在系统 Agent 已运行且已有对应调试构建／fixture／automation 工具的环境运行：

```powershell
./scripts/verify-system-agent-ui.ps1 -OpenSSHDirectory <OpenSSH-Win64目录>
```

专项脚本自身不修改服务设置；临时启停只在本次用户授权的外层 try/finally 中执行，并独立记录前后状态。脚本要求实际服务已启动，否则在创建 fixture 或添加密钥前拒绝。后续若原环境不同，需保留原状态和已有密钥，不得直接复用本次“Disabled／Stopped／空集合”的假设。

本次覆盖当前 Windows 系统 Agent、单个临时 Ed25519 身份、回环 SSH／SFTP 和正常退出；未验收智能卡、硬件密钥、大量已载入身份或其他 Agent 版本。本轮仅修改验收脚本和文档，执行完整专项、脚本语法及 diff 检查，未重复全仓 Go 测试或正式性能。
