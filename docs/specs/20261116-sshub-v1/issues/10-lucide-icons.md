Status: resolved
Type: task
Blocked by: 05, 06

# 统一 Lucide 界面图标

## Scope

按用户要求将界面中的操作符号替换为 Lucide SVG，覆盖标题栏、新建／筛选、收藏、标签状态及关闭、更多菜单、终端搜索和传输方向；文件列表使用文件／文件夹／符号链接及上级目录图标。保持紧凑布局与菜单／右键操作，不新增常驻功能栏。

## Acceptance

- 固定官方资源来源及许可，SVG 内嵌、仅解析一次，运行时离线可用。
- 常规 16 DIP、标签关闭／收藏 14 DIP，主题颜色可读；保留中文辅助名称、提示及 automation key。
- 验证实际浅深主题、窗口控制／菜单、终端搜索与文件列表／传输显示；只运行与本次改动相称的检查，不重复性能和全量测试。

## Answer

已引入 18 个官方 Lucide SVG，固定来源提交及完整 ISC／Feather MIT 告知，见[资源与映射](../../../../internal/desktop/icons/lucide/README.md)及[第三方许可](../../../third-party/README.md)。资源通过 `go:embed` 内嵌，由 `icons.go` 在初始化时解析一次；`ui.Icon` 原生绘制并跟随主题。常规图标 16 DIP，标签关闭和收藏 14 DIP，连接状态圆环 8 DIP；28 DIP 操作按钮保留中文名称、提示及原有 key，窗口按钮保留 46×34 DIP 与 Windows 贴靠命中区域。文件列表左对齐类型图标，传输方向改为 SVG，辅助名称明确“上传／下载”。

验证：`gofmt`、`go test -tags mygo_nowebview ./internal/desktop`、同包 `go vet` 及 MyGo UI 生命周期 vet 通过；新调试构建位于 `build/lucide/windows-amd64/`。SFTP 文件管理专项通过，随后专项操作新建／取消、收藏、筛选菜单、应用／文件／队列／窗格菜单、搜索上一处／下一处／关闭、最大化／还原及在线退出；浅深主题截图逐一视觉核对。真实上传选择器选取中文文件后，任务完成且远端哈希一致。证据位于 `artifacts/sftp-ui/20261117-095321/`，包括 `lucide-light-search.png`、`lucide-dark-search.png`、`lucide-{light,dark}-files.png` 及快照。临时应用和回环服务器均已正常退出。

上传选择器首次鼠标定位被工具拒绝，改用其“打开”快捷键成功；下载目录选择器的自动化索引／焦点不可靠，取消后结束该项，因此本轮没有下载完成态或符号链接图标的单独窗口截图。所有 SVG 已随初始化解析；不将已有下载协议测试当作本次图标显示实测。更新集成脚本中上传任务辅助名称的断言并通过脚本解析，未重复跨服务器完整专项、全仓检查、正式构建或性能采样；历史性能和发行包仍对应此前的 EXE。
