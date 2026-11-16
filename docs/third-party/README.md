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
