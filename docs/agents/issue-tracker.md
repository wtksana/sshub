# 任务跟踪：本地 Markdown

## 文件约定

- 每个功能一个目录：`docs/specs/<yyyyMMdd>-<feature-slug>/`。
- 日期使用创建日期，之后保持不变。
- 功能规格：`spec.md`。
- 实现任务：`issues/<NN>-<slug>.md`，从 01 编号，每个任务独立文件。
- 任务顶部使用 `Status: open`、`Status: claimed` 或 `Status: resolved`。
- 评论和讨论追加到文件末尾的 `## Comments`。

## 技能操作

- “发布到任务跟踪器”：创建对应规格或任务文件。
- “读取相关任务”：读取用户提供路径或编号对应的文件。
- 不创建远程 Issue。

## Wayfinder

- 工作地图：功能目录下的 `map.md`，记录 Notes、
  Decisions-so-far 和 Fog。
- 子任务沿用 `issues/<NN>-<slug>.md`。
- `Type:` 为 research、prototype、grilling 或 task。
- `Blocked by: NN, NN` 声明依赖；依赖全部 resolved 才能开始。
- 按编号选择 open 且无阻塞的任务，开始前保存 Status: claimed。
- 完成后追加 `## Answer`，设置 Status: resolved，并将结论摘要
  和文件链接追加到地图的 Decisions-so-far。
