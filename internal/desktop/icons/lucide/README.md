# Lucide 图标

来源：官方 [lucide-icons/lucide](https://github.com/lucide-icons/lucide/tree/f8feea7a02ea461c9f2a4b4d851512aae6ec933f/icons)，固定提交 `f8feea7a02ea461c9f2a4b4d851512aae6ec933f`，下载日期 2026-11-17。SVG 保留上游原样，24×24 viewBox、2 单位描边、圆端点及圆连接；许可见仓库 `docs/third-party/Lucide-LICENSE.txt`。

| 用途 | Iconify 名称 |
| --- | --- |
| 应用菜单 | `lucide:menu` |
| 连接面板／文件面板 | `lucide:panel-left`、`lucide:panel-right` |
| 新建连接 | `lucide:plus` |
| 筛选连接 | `lucide:list-filter` |
| 文件、窗格及队列操作 | `lucide:ellipsis` |
| 最小化／最大化／还原／关闭 | `lucide:minus`、`lucide:square`、`lucide:copy`、`lucide:x` |
| 收藏／连接状态 | `lucide:star`、`lucide:circle` |
| 上传／下载 | `lucide:upload`、`lucide:download` |
| 文件夹／文件／符号链接 | `lucide:folder`、`lucide:file`、`lucide:file-symlink` |
| 上级目录 | `lucide:corner-left-up` |
| 上一处／下一处搜索结果 | `lucide:chevron-up`、`lucide:chevron-down` |

由 `internal/desktop/icons.go` 内嵌并在包初始化时解析一次。常规图标 16 DIP，标签关闭和收藏 14 DIP，连接状态 8 DIP；颜色继承按钮文字或当前主题的状态／次级文字色。按钮保留中文辅助名称、提示与原有 automation key，不依赖字体中的符号字形。未添加运行时网络依赖，也未改动 EXE 的应用图标。
