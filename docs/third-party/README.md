# 第三方许可

`manifest.json` 列出 Windows 正式 EXE 实际链接的 Go 模块及版本，许可文本直接取自各版本 module cache。Go 运行时使用 `Go-LICENSE.txt`；MyGo 内部 shader 编译器沿用 Ebitengine Apache 2.0 许可，另附 `MyGo-shader-LICENSE.txt`。应用字体通过 Windows 系统字体读取，未将系统字体文件打包。

Ghostty DLL 固定在 MyGo v0.9.1 的 `plugins/terminal/mygo-plugin.json` 指定版本：

- 源码：<https://github.com/ghostty-org/ghostty/tree/befcdfd2c3a1cb24d9ec886e93c95b2b5daa7028>
- DLL SHA-256：`53e8e2dd979702e8f2c48107af19be5a76efc2e88eb7cf1097218e899665da41`
- Ghostty MIT 许可见 `Ghostty-LICENSE.txt`。
- `src/build/GhosttyZig.zig` 的 VT 构建引用 uucode Unicode 表／分字；源码固定为 `jacobsandlund/uucode` 的 `9d55524551411b493cca41ca06363625d90aff1e`，附其 MIT、Unicode 和 Hoehrmann 许可。另附 Unicode 数据许可。
- DLL 的 Zig 编译运行时附 `Zig-LICENSE.txt`；SIMD 路径及 Kitty 图像路径所用 simdutf、Highway、Wuffs 的许可一并保留。VT 使用 `GhosttyZig` 构建路径，不能把完整 Ghostty GUI 的字体／窗口依赖误认成 DLL 全部链接项。
- simdutf 按 `pkg/simdutf/build.zig.zon` 的 5.2.8 许可文本；Highway 许可来自 google/highway，实际 vendored shim 标明 `66486a10623fa0d72fe91260f96c892e41aceb06` 且有本地修改；Wuffs 双许可文本来自上游。许可与告知均保留，不宣称拥有这些组件。

升级依赖时需要重新核对 EXE 模块列表、DLL 源码版本、许可与哈希。此目录会随便携包完整分发。

## Lucide 图标

界面使用 Lucide 的 18 个 SVG，来自官方仓库固定提交 [`f8feea7a02ea461c9f2a4b4d851512aae6ec933f`](https://github.com/lucide-icons/lucide/tree/f8feea7a02ea461c9f2a4b4d851512aae6ec933f/icons)。源文件位于 `internal/desktop/icons/lucide/`，未经修改，通过 Go embed 内嵌；MyGo 原生绘制，运行时无需访问 Iconify 或 Lucide。图标名与用途见该目录的 README。

完整上游许可保存在 `Lucide-LICENSE.txt`，包含 Lucide ISC 及源自 Feather 图标的 MIT 告知。它是资源许可，不加入仅记录 Go 链接模块的 `manifest.json`；打包脚本会随本目录分发。

## Devicon 应用图标

应用图标使用 Iconify 的 `devicon:ssh-wordmark`，对应 Devicon 官方 [`ssh-original-wordmark.svg`](https://github.com/devicons/devicon/blob/32fe1d3b3f36ad11f90ab94f7db59738139c2bf6/icons/ssh/ssh-original-wordmark.svg)，固定提交为 `32fe1d3b3f36ad11f90ab94f7db59738139c2bf6`。原始 SVG 保存在 `resources/icon.svg`，保留原色及透明背景，使用 sharp 栅格化为 1024 × 1024 的 `resources/icon.png`；MyGo 根据 `mygo.json` 的 `icon` 配置生成并嵌入各尺寸 Windows 图标，运行时无需访问网络。

上游 MIT 许可保存在 `Devicon-LICENSE.txt`，随本目录分发。
