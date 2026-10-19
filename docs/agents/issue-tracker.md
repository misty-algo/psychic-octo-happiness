# Issue tracker：GitHub

本仓库的 issue 和 PRD 记录在 GitHub Issues 中。所有 issue tracker 操作都使用 `gh` CLI。

## 约定

- **创建 issue**：`gh issue create --title "..." --body "..."`。多行正文使用 heredoc。
- **读取 issue**：`gh issue view <number> --comments`，同时获取评论和标签。
- **列出 issue**：`gh issue list --state open --json number,title,body,labels,comments`，并按需要添加 `--label` 和 `--state`。
- **评论 issue**：`gh issue comment <number> --body "..."`
- **添加或移除标签**：`gh issue edit <number> --add-label "..."` / `--remove-label "..."`
- **关闭 issue**：`gh issue close <number> --comment "..."`

在仓库克隆目录内运行时，`gh` 从 `git remote -v` 自动推断仓库。

## Pull request 作为 triage 入口

**PRs as a request surface：no.**

`triage` 只处理 GitHub Issues，不读取、标记或关闭 Pull Requests。

## 当技能说“publish to the issue tracker”

创建 GitHub issue。

## 当技能说“fetch the relevant ticket”

运行 `gh issue view <number> --comments`。

## Wayfinding 操作

`wayfinder` 使用一个 map issue 管理多个 child issue：

- **Map**：带有 `wayfinder:map` 标签的单个 issue，正文保存 Notes、Decisions-so-far 和 Fog。
- **Child ticket**：优先使用 GitHub sub-issue 关联到 map。仓库不支持 sub-issue 时，使用 map 正文中的 task list，并在 child 正文顶部写 `Part of #<map>`。
- **Ticket 类型**：使用 `wayfinder:research`、`wayfinder:prototype`、`wayfinder:grilling` 或 `wayfinder:task` 标签。
- **阻塞关系**：优先使用 GitHub 原生 issue dependencies。不可用时，在 child 正文顶部写 `Blocked by: #<n>, #<n>`。
- **Frontier**：从 map 的未关闭 child 中排除仍有未关闭 blocker 或已有 assignee 的 issue，按 map 顺序选择第一个。
- **Claim**：`gh issue edit <number> --add-assignee @me`。这是执行会话的第一次写操作。
- **Resolve**：评论处理结果、关闭 child，并把上下文链接追加到 map 的 Decisions-so-far。
