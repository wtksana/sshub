# Windows 原生集成专项

日期：2026-11-16。使用当前 Windows 11 x64 开发机、MyGo v0.9.1 调试构建 `build/p6/windows-amd64/SSHub.exe`，两台仅监听回环地址、临时密钥及独立数据目录的 SSH/SFTP 测试服务。未连接用户服务器。这是功能验证，不用于替代正式构建性能数据。

原始记录和截图：`artifacts/integration-ui/20261116-212807/`。可随仓库复核的结果 JSON 位于同名目录 [20261116-native-integration/](20261116-native-integration/)。

## 跨服务器上传与关闭终端

通过真正的 Windows 多选文件选择器，选择含中文和空格的约 16.1 MiB 文件及另一个文本文件，上传到 A 服务器 `/inbox`。测试服务每次写入延迟 80 ms，切换焦点前快照同时确认 A 的首项“传输中”、次项“等待中”。

随后切换到 B 标签及其 `/other` 文件面板，关闭 A 标签，在确认框选择“关闭终端并继续传输”。A 标签消失后两个任务均完成；分别读取 A 的目标文件计算 SHA256，与源文件一致，并确认 A 根目录、B 根目录及 B `/other` 均未出现误传文件。

| 文件 | 实际目标 | SHA256 |
| --- | --- | --- |
| 持续上传 中文.bin | A:/inbox/持续上传 中文.bin | A538215662C665E422FEDFAE809D197E1DAA285156B7BB9F1F192E87F8088C46 |
| 排队上传.txt | A:/inbox/排队上传.txt | D390A9A17F21B3932C7127A0DE5392BEE489DE4A6D6329AA715F3DDDCAE4268C |

证据：`before-focus-switch.txt`、`transfers-after-close.png`、`transfers.json`。此场景验证本地上传任务在切换服务器时的目标绑定，不表示实现服务器之间的直接文件复制。

## Windows 剪贴板往返

在新的 B 服务器终端中，将随机标记、中文和空格写入 Windows 剪贴板，通过 `Ctrl+Shift+V` 粘贴。SSH 测试服务回显后，使用产品终端搜索确认恰好一处匹配；全选输出，再将系统剪贴板改为哨兵文本，执行复制，确认哨兵被替换。复制结果规范 CRLF 并去除首尾空白后，与欢迎行及完整随机标记逐字符一致。

分别通过 `Ctrl+Insert` 和真正的终端右键菜单 Copy 验证复制；后者由 computer-use 驱动 Windows 原生 UI。测试结束恢复原剪贴板文本，证据不保存用户剪贴板。结果：`clipboard.json`、`clipboard-selection.png`。

MyGo 调试版检查器占用 `Ctrl+Shift+C`：首次用该快捷键会开启检查器，不能据此判断正式版复制失败。已关闭检查器，改用 Windows 复制快捷键及右键菜单完成验证。本记录不声称正式版 `Ctrl+Shift+C` 已单独实测。

## 系统外观实时切换

SSHub 使用默认“跟随系统”，全程保持同一进程与在线 SSH 会话。在 Windows 设置的“个性化→颜色”中将模式从浅色切到深色，观察应用标题栏、连接库、文件面板及终端实时变深；再切回浅色，观察全部恢复。深色与浅色期间分别向隔离回显服务输入中文标记，并通过终端搜索确认一处匹配。

结果：`system-theme.json`、`system-dark.png`、`system-dark-echo.png`、`system-light-echo.png`。原 Windows 浅色模式已恢复；测试应用和两个临时服务均正常退出。

## 复现

在交互 PowerShell 终端执行：

```powershell
./scripts/verify-integration-ui.ps1 -NativeActions
# 若已有对应调试构建，可加 -SkipBuild -BuildDirectory build/p6
```

脚本打开原生选择器后暂停。依据 `artifacts/integration-ui/active.json` 中的 `sources` 选择两个生成文件，返回终端立即回车；脚本自动切换 B、关闭 A 并验证传输归属和哈希。首次暂停后不要等待传输完成才继续，否则活动／排队状态断言会拒绝该次验收。

第二次暂停时，可在另一 PowerShell 终端执行：

```powershell
./scripts/verify-terminal-clipboard.ps1
./scripts/verify-terminal-clipboard.ps1 -NativeCopy
```

`-NativeCopy` 暂停时在产品终端右键菜单中选择 Copy，再回车继续校验。外观切换需要通过实际 Windows 设置手动或由 computer-use 执行，并恢复原模式。最后回到首个终端回车，正常退出并清理测试进程。

仍未覆盖系统 OLE 拖放、终端中文输入法组合输入、真实 OpenSSH／Unix 权限及 vim/top。测试使用原生选择器的文件上传，不能替代系统拖放验收。
