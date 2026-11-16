# Linux OpenSSH、Unix 权限与 top 专项

日期：2026-11-16。Windows 11 x64 上运行 SSHub MyGo v0.9.1 调试应用；服务器为独立 Alpine Linux 虚拟机，验证客户端连接 Linux 服务器的能力，不代表 Linux 桌面客户端已验收。

## 环境与来源

QEMU 便携解压到忽略目录，使用 TCG、512 MiB 客体内存、2 vCPU、磁盘 snapshot 模式；没有安装系统服务或更改 WSL／Hyper-V 配置。用户网络 `restrict=on`，仅向主机 `127.0.0.1` 转发随机 SSH 端口；客体不能通过此网卡访问外部网络。QMP 控制也只监听回环地址。

- [QEMU Windows 构建](https://qemu.weilnetz.de/w64/qemu-w64-setup-20260811.exe)：版本 `11.1.0 (v11.1.0-12130-ge470268ff4)`；下载包 SHA256 `F98A8AEB5F7FAEA9765B6DEE28316C266CD179D80354A2FED8E50176F9A2E59F`，实际 QEMU EXE SHA256 `47D57A6072E0BB3BD98F87926EB129EB1736DFE818C67B3B81EF7CE4EDD0B3CD`。未运行安装程序，使用便携 7-Zip 解包。
- [Alpine NoCloud BIOS tiny 镜像](https://dl-cdn.alpinelinux.org/alpine/v3.23/releases/cloud/nocloud_alpine-3.23.4-x86_64-bios-tiny-r0.qcow2)，[官方 SHA512](https://dl-cdn.alpinelinux.org/alpine/v3.23/releases/cloud/nocloud_alpine-3.23.4-x86_64-bios-tiny-r0.qcow2.sha512)：`d893833178d583b557a98d8097df235290172447f13a18b5060b67705bd77b2e23105c9689a505948589aea9f3a70cb18461b9df03a00103b91567a4ed869c28`，已逐字核对。该摘要校验不是独立签名验证。
- 客体实际版本：Alpine 3.23.4、Linux 6.18.22-0-virt、OpenSSH 10.2p1／OpenSSL 3.5.6、BusyBox 1.37.0；使用真实 `/usr/lib/ssh/sftp-server` 子系统。
- 通过 [7-Zip 26.06](https://github.com/ip7z/7zip/releases/tag/26.06) 解包，pycdlib 1.22.0 只安装到 `artifacts/linux-vm/python`，用于生成 NoCloud ISO。上述工具均不进入产品包。

NoCloud 脚本只在 snapshot 客体内创建测试文件并写入新生成的用户公钥，主机私钥留在忽略目录。通过 `-smbios type=1,serial=ds=nocloud` 明确指定数据源；本镜像未加该提示时未自动识别 ISO，修正后初始化成功。测试前从客体串口独立读取主机公钥指纹；产品确认框必须与其中一个指纹一致才接受，未跳过校验。

## 结果

原始产品验证目录：`artifacts/linux-openssh-ui/20261116-221612/`；客体初始化／版本记录：`artifacts/linux-fixture/20261116-221514/`。无秘密结果见 [linux-openssh-summary.json](20261116-linux-openssh-summary.json)。

- 产品指纹确认、临时 Ed25519 用户私钥认证、真实 PTY shell 通过。输出标记在 shell 中拼接，产品搜索只匹配一处，避免把输入行当执行结果。
- SFTP 进入 `/home/alpine/fixture` 并显示 `Linux 中文.txt`；属性弹窗实际读出 `640`，修改为 `600` 后重新打开仍显示 `600`，远端 `stat -c %a` 独立确认一致。
- 产品新建 `Linux 新建目录`，远端 shell `test -d` 确认目录存在。结束后的独立、严格主机校验 SSH probe 再次确认文件 mode 为 `600`。
- 通过产品终端运行真实 BusyBox `top -d 1`；`linux-top.png` 已实际视觉核对，可见内存／CPU／负载、反色表头、进程行及 `top -d 1` 自身。按 `q` 后 shell 再输出拼接标记，产品搜索确认一处，退出后输入恢复。
- 在线退出确认后产品正常退出；QMP quit 停止虚拟机并丢弃临时磁盘 snapshot。无 QEMU、SSHub、sshd 或测试 fixture 进程残留。宿主系统 ssh-agent 始终停止。

手工搭建后首次验证 `20261116-221344` 也通过；最终记录取自新增 fixture 脚本从原镜像重新启动的全新 snapshot，以确认脚本可复现。未为新增脚本重复 Go 全仓测试或正式十分钟性能；本次运行完整 Linux 专项、fixture 启动／停止、PowerShell／Python 语法和 diff 检查。

## 复现

将上述 QEMU 解压，准备同一 Alpine 镜像，安装 pycdlib 到忽略的目录后执行：

```powershell
$sha = 'd893833178d583b557a98d8097df235290172447f13a18b5060b67705bd77b2e23105c9689a505948589aea9f3a70cb18461b9df03a00103b91567a4ed869c28'
./scripts/start-linux-fixture.ps1 -QemuDirectory <便携QEMU目录> -Image <qcow2文件> -ExpectedImageSHA512 $sha -PythonModulesDirectory <包含pycdlib的目录>
$f = Get-Content -Raw artifacts/linux-fixture/active.json | ConvertFrom-Json
try {
    ./scripts/verify-linux-openssh-ui.ps1 -Port $f.port -Identity $f.identity -HostFingerprints $f.host_fingerprints -SkipBuild
    # 实际打开本次 linux-top.png，核对 top 显示；脚本成功不代替视觉检查
} finally {
    ./scripts/stop-linux-fixture.ps1
}
```

`-SkipBuild` 要求已有 `build/p6` 对应调试构建及 `build/tools/mygo-automation.exe`。fixture 启动失败会清理进程；启动成功后由调用者使用 finally 停止。镜像及工具不纳入 Git，生成密钥、ISO 与 guest 数据均在 artifacts。stop 脚本核对 PID 名称及 QMP 监听端口归属后才发送 quit。

此次没有验收高时延公网、其他发行版、procps 版 top、小时级交互或真实系统 Agent 服务。全新 Linux 客体也不能替代全新 Windows 无工具链机器的产品运行验收；系统拖放、系统 Agent、重启冷启动及干净 Windows 环境仍待补。
