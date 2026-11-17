# 日志高亮交付验证

2026-11-17，Windows x64。四项功能任务已完成；正式综合负载验收尚未完成，不能据此宣称整份规格已经全部验收。

## 已通过

- 流式回归：任意分块、中文编码边界、已有 ANSI 前景与其他样式、备用屏幕、保存／恢复前景状态、私有 CSI、未知／截断控制内容、无换行提示符、连续输入、EOF／带数据错误、关闭时已接收及在途数据排空、输入／Resize／Close 透传。
- 配置回归：旧字段缺省、显式禁用、规则校验、复杂正则展开限制、逐批工作／匹配／输出扩张预算、优先级与局部重叠、零宽匹配、深复制、保存失败／取消、外部修改保护、迁移和重启。
- 原生专项：当前会话菜单与键盘开关、原生复制／粘贴／全选／清屏、两会话隔离、标签保持、重连／重启默认关闭、连续日志中文软折行、搜索及原文复制、规则增删重排与颜色、浅深主题、900×620／界面字号 20、历史缓冲新建／握手中／重连生效范围。
- 集成检查：`go test -tags mygo_nowebview ./...`、`go vet -tags mygo_nowebview ./...`、相同标签下 `go tool mygo vet ./internal/desktop` 通过。最终审查修复后重新通过 connection 全包 test／vet 及跨分块回归。变更 PowerShell 脚本语法和 `git diff --check` 通过。
- 两轴审查：Standards 无可行动发现；Spec 发现一项 P2，私有 CSI 被误当 SGR，已修复并由原审查者复核解决；剩余发现为零。

原生专项可复跑脚本：`scripts/verify-log-highlighting-ui.ps1`、`scripts/verify-highlight-settings-ui.ps1`、`scripts/verify-custom-highlight-ui.ps1`、`scripts/verify-scrollback-ui.ps1`。使用调试构建、独立数据目录、回环 SSH 临时密钥及精确固定的主机密钥，不操作用户服务器。最终截图已保留在主仓库的 `artifacts/`：

| 专项 | 证据目录 |
| --- | --- |
| 会话与连续日志 | `artifacts/log-highlighting-ui/20261117-214411/` |
| 内置规则设置 | `artifacts/highlight-settings-ui/20261117-220304/` |
| 自定义规则 | `artifacts/custom-highlight-ui/20261117-222244/` |
| 历史缓冲 | `artifacts/scrollback-ui/20261117-211622/` |

这些生成物不纳入 Git。微基准的原始输出与方法另见[匹配及提示符微基准](../performance/20261117-highlight-matcher.md)：开启时提示符交付 p50 20.402 ms、p95 20.775 ms、最大 25.931 ms；这是流转换的调度及拼片等待，不是 UI 呈现延迟或严格 20 ms 保证。

## 正式构建与尚待验证

最终代码 `120ade66953b20cae3a578a57bb04e8c2a1d5ece` 通过 `scripts/build.ps1 -OutputDirectory build/log-highlighting-release` 正式构建。Go 1.27.4、MyGo v0.9.1，原生标签 `mygo_nowebview,mygo_nodownload,mygo_noinspector`，CGO=0，无 e2e automation。构建信息的 `vcs.modified=true` 如实保留，不将它描述为完全干净的可重现构建。之后仅修改验证文档，不改变该 EXE 的代码。

- EXE SHA-256：`BF52CA24556CBA4C96DFF4136F0168A4854FD3035F85CA04A58CB5584D2023F5`。
- Ghostty DLL SHA-256：`53E8E2DD979702E8F2C48107AF19BE5A76EFC2E88EB7CF1097218E899665DA41`。
- 本机：Windows 11 10.0.26300、Intel Core Ultra X7 358H、16 逻辑处理器、约 31.55 GiB RAM。

已尝试启动 `measure-product.ps1` 的八会话正式版采样。窗口可枚举，但截图为黑屏，首次点击报 `GetCursorPos failed: 拒绝访问。 (0x80070005)`，同时存在 LogonUI；为避免绕过锁屏已停止操作并请求解锁。采样停在 `awaiting_connections`，随后终止脚本并清理测试进程，未开始负载，**没有有效性能结果**。启动准备文件留在 `artifacts/product-performance/20261117-223736/`，不能当作通过证据。

待可交互桌面恢复后，用相同正式 EXE、七项预设加三条自定义规则（“性能”、`message`、`"id":[0-9]+`），执行八会话各 1,000 条逻辑行／秒、关闭／开启各十分钟，以及 2／8／32／64 MiB 档位资源对比。日志每条 101 字节，含 INFO／WARN／ERROR、时间、中文、普通 JSON 文本及原有绿色序号。记录连接／启用／降级数、接收字节、工作集、私有提交、CPU、堆和 goroutine；接收速率不能当作渲染吞吐。

`go test -race` 未执行成功：当前 CGO=0 且无 C 编译器；没有为此安装或改变本机工具链。未重启电脑、未做全新 Windows 验收或小时级泄漏测试，历史性能报告不能代替上述待测项。
