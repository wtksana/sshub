# SSHub 项目约定

- 默认简体中文沟通，文本使用 UTF-8 无 BOM。
- 当前阶段是首版产品开发，按 `docs/specs/20261116-sshub-v1/plan.md` 分阶段交付，只保留正式应用入口。协议代码放在 `internal/connection`，UI 不实现 SSH 协议或终端模拟器。
- 固定 MyGo 版本；原生模式保持无 WebView。正式构建使用 `./scripts/build.ps1`，运行使用 `./scripts/run.ps1`。
- 改动后执行 `gofmt`、`go test ./...`、`go vet ./...`；涉及界面时运行实际窗口或官方 automation 验证。
- 性能测试使用正式构建；产品性能采样工具随后续阶段接入。性能结果必须附带环境、负载、计量定义及局限，不将输入接收速率当渲染吞吐或将 UI build 时间当帧率。
- 不跳过 SSH 主机密钥校验，不自动信任未知主机。测试使用本地临时服务器与临时密钥。
- `artifacts/` 和 `build/` 是生成物，不纳入版本控制；将可复核结论与测量方法写到 `docs/performance/`。
- 根目录 `main.go` 只负责启动；界面放在 `internal/desktop`，连接配置放在 `internal/library`。优先在现有包内按职责拆文件，新增包应对应独立职责。

## Agent skills

### Issue tracker

任务和规格使用本地 Markdown，保存在 `docs/specs/`。
详见 `docs/agents/issue-tracker.md`。

### Domain docs

采用 single-context：根目录 `GLOSSARY.md` 和 `docs/adr/`。
详见 `docs/agents/domain.md`。
