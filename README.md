# SSHub

轻量的 SSH / SFTP 桌面客户端，基于 [MyGo](https://github.com/egoist/mygo) 原生 UI 和 [Ghostty](https://ghostty.org/) 终端，专注服务器连接与文件管理，无需 WebView。

目前优先支持 Windows x64，处于首版开发与验收阶段。macOS / Linux 尚未实机验收。

## 功能

- **连接管理**：分组、收藏、搜索、最近使用、复制连接及配置导入导出。
- **SSH 认证**：密码、普通／加密私钥、Windows OpenSSH Agent 和单层跳板机。
- **多标签终端**：分屏、终端搜索、字号调整及标签布局恢复。
- **SFTP 文件管理**：目录浏览、排序、权限修改、拖入上传、文件夹传输和任务队列。
- **外观设置**：GitHub Light / Dark、跟随系统、独立的界面与终端字体设置。
- **本地存储**：主机指纹校验，可选使用 Windows 凭据管理器保存密码与私钥口令。

## 使用

解压完整的 Windows 便携包，运行 `SSHub.exe`；请保留同目录的 `ghostty-vt.dll`，无需安装 Go、Node 或 WebView2。

点击左侧 **+** 添加连接，双击连接项建立会话。连接项和终端标签支持右键操作，顶部按钮切换连接与文件面板。首次连接时需核对服务器指纹。

配置默认保存在 `%APPDATA%\SSHub`。安装、升级与迁移说明见 [便携使用说明](docs/portable-readme.md)。

## 开发

需要 Windows x64、PowerShell 7 和 Go 1.27.4；MyGo 版本由 `go.mod` 固定。

在项目根目录启动开发模式，修改代码后自动重新构建并重启：

```powershell
$env:GOFLAGS = '-tags=mygo_nowebview'
go tool mygo dev
```

构建并运行正式版本：

```powershell
./scripts/build.ps1
./scripts/run.ps1
```

输出位于 `build/windows-amd64/`。修改代码后使用正式版本需重新构建；`run.ps1` 只在程序不存在时自动构建。

生成便携 ZIP：

```powershell
./scripts/package.ps1
```

产物位于 `build/releases/`。开发约定和按改动范围验证的方法见 [AGENTS.md](AGENTS.md)。

## 文档

- [项目架构](docs/architecture.md)
- [产品规格](docs/specs/20261116-sshub-v1/spec.md)
- [验收状态](docs/specs/20261116-sshub-v1/acceptance.md)与[剩余验收](docs/verification/remaining-acceptance.md)
- [性能报告](docs/performance/20261116-product.md)
- [第三方组件与许可](docs/third-party/README.md)
