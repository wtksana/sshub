# Windows 资源管理器原生拖入专项

日期：2026-11-16。使用现有 P6 调试程序、独立配置、临时密钥和回环 SFTP 服务。用户从资源管理器同时将“拖入 中文.txt”和“拖入 文件夹”拖到 SSHub 右侧路径 `/` 的文件列表，并明确回复“已拖入，传输完成”。没有通过选择器、脚本复制或 automation 上传代替该手势。

## 结果

- `verify-sftp-ui.ps1 -SkipBuild -BuildDirectory build/p6 -TransferPickers` 的文件管理专项先通过，随后保持窗口供人工拖放。
- `verify-native-drop.ps1 -PrepareOnly` 仅准备随机内容的中文文件、嵌套目录和空目录，不上传。用户完成真实手势后才执行 `-CheckOnly`。
- 右侧文件列表出现“拖入 文件夹/”和“拖入 中文.txt”；两个队列任务均为“本地 SFTP · 已完成”，分别传输 61 B 和 56 B，均为 1 文件、0 跳过。截图已视觉核对。
- 两个远端文件逐一 SHA256 与本地源文件一致；`拖入 文件夹/子目录/空格 内容.txt` 以及 `拖入 文件夹/空目录` 均存在。无秘密哈希见 [摘要](20261116-native-drop-summary.json)。
- 测试前读取进程 token，SSHub 和两个 Explorer 进程均为 elevated，权限级别一致；未修改 UAC、安全设置或运行权限。
- 验后回车结束专项，应用和 fixture 正常退出，无遗留 SSHub／ssh-fixture 进程。

原始证据：`artifacts/sftp-ui/20261116-235410/`，包含 `native-drop.json`、`native-drop-result.json`、`native-drop-snapshot.txt` 和 `native-drop.png`。

## 证据边界

工具 API 拒绝跨源窗口边界拖动，因此手势由用户执行并确认，agent 核对完成后的产品界面、队列状态、远端内容及目录结构；没有录制拖动过程。内容检查脚本仍如实记录 `native_gesture_observed_by_script=false`，不把文件到达单独当成 OLE 手势证明。此次覆盖同权限级别的当前 Windows、文件与文件夹混合拖入，不扩展为其他权限组合或所有 Windows 版本的结论。

本轮未修改产品代码；仅运行本次 SFTP 管理／拖入专项和文档 diff 检查，未重复全仓测试、正式构建或性能采样。
