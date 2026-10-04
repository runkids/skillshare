---
sidebar_position: 12
---

# 跨 AI 工具共享记忆

将决策、经验与项目背景集中在一个 Markdown 笔记文件夹。先为工具连接阅读指引，再在新会话中验证 Agent 是否读取相关笔记。

本教程使用 global mode，并已将 Claude 与 Codex 配置为 targets：

```bash
skillshare ui -g
```

所有截图的 UI、笔记与对话框均为英文，使用 `/tmp/skillshare-memory-docs` 下的隔离演示 home。你的路径会不同。

## 1. 创建记忆

打开 **Extras → Memory**，点击 **Create memory**。

![英文 Memory 页签的初始状态](/img/memory-empty-demo.png)

这会注册名为 `memory` 的 source-only extra，并创建缺少的起始笔记：`INDEX.md` 是简短入口；`LEARNED.md` 记录经验的日期、背景、结论与证据。现有笔记与配置会保留。

![包含 INDEX.md 与 LEARNED.md 的 Memory](/img/memory-starters-demo.png)

默认 global 文件夹为 `~/.config/skillshare/extras/memory/`，也会显示在 **Extras → Folders & files**，起初没有 targets。本机 Agent 直接读取此来源，无需把笔记副本 sync 到各工具。

## 2. 连接工具

在 **Use with agents** 点击 **Connect to agents**。自行选择工具并为每个工具选择更新模式，再点击 **Review changes**。

- `passive`（默认）：Agent 会读取笔记，只在你要求时更新。
- `active`：Agent 也会保存之后的会话仍用得到的事实，例如你明确表示的偏好、附理由的决定或已确认的陷阱。它会略过一次性的细节与猜测；拿不准时先提议并等你同意；优先更新现有笔记而不是重复新增，并告诉你存了什么。

读取同一个文件的工具共用一个块，所以切换其中一个会一起切换。要改已配置工具的模式，再打开一次 **Connect to agents** 切换即可，变更同样会经过预览。

![英文连接对话框中的 instructions 文件更改预览](/img/memory-connect-demo.png)

检查每个文件的差异（删除行标 `−`，新增行标 `+`），再点击 **Apply changes**。Skillshare 将受管理的阅读指引块加入工具现有的 instructions 文件，或工具已读取的共享来源；若文件尚不存在，也可创建。块带有 scope 与内容 hash 标记，其余内容、现有分配与连接模式均保留。修改现有文件前会备份。若其他工具也读取同一文件，预览会提示，也会警告已知的字符上限。

![英文 Memory 页签显示已配置的工具](/img/memory-connected-demo.png)

**Configured** 表示工具的读取链已有当前指引，不代表 Agent 已读取。**Not configured**、**Outdated** 与 **Needs attention** 描述的是 instructions 文件。完整但过期的块可经再次检查后更新；手动修改或标记格式错误的块会保留，需要手动修复。若工具从不同文件读到两种模式的块，也会显示需要处理：请把读取这些文件的工具设为同一种模式。从多个文件读到块的工具，要先移除多余的块才能切换模式。未同步的共享 instructions 要先 sync；无法读取的 instructions 文件会跳过。若预览后文件改变，必须重新检查再应用。

使用 **Open AGENTS.md** 检查或修复 instructions。连接预览是 dashboard 流程，没有新增 CLI 连接命令。

## 3. 新增笔记与索引链接

点击 **New note**，在 **File name** 输入 `wiki/architecture.md`，保留 **Link from INDEX.md** 勾选，再点击 **Create**。当 `INDEX.md` 可读取时才显示此选项，且默认勾选。

![英文新增笔记对话框中的嵌套路径与索引选项](/img/memory-folder-demo.png)

Skillshare 自动创建缺少的子文件夹，并在 `INDEX.md` 文件末尾附加相对 Markdown 链接。索引更新会检查 version 并备份。若添加链接失败，笔记仍保留，并显示部分完成的警告。选择未索引的笔记，点击 **Add to INDEX** 可重试；也可自行编辑索引。保持索引简短。失效链接会显示警告，不会自动移除。

笔记须为 UTF-8 `.md` 文件，大小不超过 1 MiB。不支持的笔记仍列出并标记 **Unsupported file**，其他正常笔记仍可使用。来源内的隐藏文件、隐藏文件夹与符号链接不包含在内。

选择笔记，点击 **Edit**，添加以下英文演示内容并 **Save**：

```markdown
# Architecture decisions

## Shared memory

Claude and Codex read the same Markdown notes from Skillshare.
Keep durable decisions here and verify facts that may have changed.

## Retrieval

Read INDEX.md first, then only the notes relevant to the current task.
Update notes when the user asks you to remember a decision.
```

![英文笔记的 Markdown 预览](/img/memory-note-demo.png)

左侧树视图支持嵌套文件夹，上方有搜索框；右侧可切换 **Preview** / **Source**，较长的笔记会先折叠，点击 **Show all** 展开。笔记名称旁是 **Edit** 与 **More actions** 菜单，包含 **Copy file path**、**History**、**Move or rename** 与 **Delete note**；**Use with agents** 位于笔记下方。相对链接可在查看器中打开现有笔记。

![展开 wiki 的英文双栏 Memory 查看器](/img/memory-tree-demo.png)

## 4. 搜索、编辑与恢复

在 **Search names and content** 输入 `Retrieval`。搜索包含子文件夹中的笔记路径与内容，不区分大小写。清除搜索即可显示全部笔记。

![英文搜索显示嵌套笔记](/img/memory-search-demo.png)

你也可使用文本编辑器。重新加载 dashboard 可查看外部更改。若编辑中的笔记被修改，过期的保存会被拒绝，草稿会保留。编辑器显示 **Latest saved version** 供比较，并提供 **Copy draft**。自行比较或合并后，点击 **Save my draft** 并确认替换。保存会使用更新后的 version 并备份已保存内容；若再次发生外部修改，仍会产生冲突。

![英文编辑器保留草稿并显示最新保存版本](/img/memory-conflict-demo.png)

**History** 会打开 **Backup Files**，以笔记的绝对路径筛选。可预览、恢复版本，再重新加载 Memory。已删除的笔记也能在同一页恢复。

![wiki/workflow-check.md 的英文 Backup Files 恢复预览](/img/memory-restore-demo.png)

## 5. 在新 Agent 会话中验证

在相关笔记添加临时值，例如 `memory-check: demo-7429`，并保存。在已连接的工具打开新会话。**Copy verification prompt** 提供以下提示：

将鼠标移到 **Copy verification prompt** 上，可在复制前预览完整内容。

![English verification prompt tooltip](/img/memory-verification-demo.png)

> 请读取 instructions 指定的共享记忆 INDEX.md，以及与本次任务相关的笔记。报告笔记的完整路径及我加入的临时验证值。请使用文件读取工具，让我能检查读取事件。

检查实际 read tool event，核对完整路径与临时值。在另一个已连接工具重复验证，再移除临时值。这是手动验证，Skillshare 没有保证可用的读取 telemetry。Agent 自称读过或显示 **Configured** 都不足以证明读取。

需要记录经验时，请要求 Agent 更新 `LEARNED.md` 的背景、结论与证据。两种模式都会在每个 task 开始时读 `INDEX.md`。笔记由用户管理：`passive` 指引让 Agent 提出值得记的事实、只在用户要求时更新；`active` 指引则让 Agent 按上述规则把这类事实保存在这里，而不是工具自己的记忆。本功能不启用 native automatic memory、自动学习或 Obsidian 集成。

## Project mode

先初始化项目的 Skillshare 配置，再执行：

```bash
skillshare extras memory init -p
skillshare ui -p
```

默认来源为 `.skillshare/extras/memory/`；使用可见配置目录时为 `skillshare/extras/memory/`。现有 extras source overrides 仍适用。Project mode 提供相同的连接、索引、编辑与恢复流程。Repo 内来源的指引使用相对于 **project root** 的路径，即使 instructions 文件在子文件夹中也一样。Project 外的 override 使用绝对路径。移动绝对来源或更改位置后，请重新生成并检查指引。

## 高级替代方式：自行复制指引

打开 **Copy guidance**，选择 `passive` 或 `active`，再把复制的块粘贴到 Agent 会读取的 instructions 文件。**Open AGENTS.md** 可打开现有编辑器。

![英文 Copy guidance 预览](/img/memory-guidance-demo.png)

也可在 **Extras → AGENTS.md** 创建共享 instructions，使用该页现有的连接流程。请检查 [跨工具共享一份 AGENTS.md](./sharing-instructions.md) 的连接模式与替换警告。

![英文共享 instructions 页面](/img/memory-agents-demo.png)

保留 scope 与 hash 标记。手动修改生成块的正文会使其标为已修改，后续连接预览将保留此内容。

## CLI 替代方式

```bash
skillshare extras memory init -g
printf '# Architecture decisions\n\nRead relevant notes on demand.\n' |
  skillshare extras memory write wiki/architecture.md --from - -g
skillshare extras memory list --search architecture -g
skillshare extras memory show wiki/architecture.md -g
skillshare extras memory instructions -g
skillshare extras memory instructions --update-mode active -g
```

CLI 只输出阅读指引（未指定 `--update-mode active` 时为 `passive`），需要自行粘贴；不会连接工具或新增索引链接。更新笔记须提供当前的 `--version`，详见 [`extras memory` 参考](../../reference/commands/extras.md#extras-memory)。

## 重命名或移动笔记

选中笔记，打开 **More actions** 后点击 **Move or rename**，输入新的相对 `.md` 路径。将 `wiki/architecture.md` 改为 `wiki/design.md` 是重命名；改为 `projects/design.md` 则移到其他文件夹。缺少的文件夹会自动创建，点击 **Move** 应用。

![英文 Move or rename 对话框，输入新的文件夹路径](/img/memory-move-demo.png)

内容和权限会保留；目标已存在或版本过期时会拒绝操作。移动前会备份来源，可用 **Restore in Backup Files** 查看旧路径的历史；在那里恢复会重建旧笔记，移动后的笔记仍保留。

Markdown 链接不会自动更新，包括笔记内的相对链接。请自行修复 `INDEX.md` 和其他笔记；失效的索引链接会显示在浏览器上方。Agent 指引指向来源根目录的 `INDEX.md`，请保留该位置。

## 删除笔记

在 **More actions** 菜单或编辑器点击 **Delete note**，确认文件名称。未保存编辑会丢弃。Skillshare 检查已保存版本并备份后，只删除该笔记，保留文件夹与其他笔记。请自行更新 `INDEX.md` 中失效的链接。删除后的 **Restore in Backup Files** 可打开已筛选的历史版本。

![英文删除笔记确认](/img/memory-delete-demo.png)

CLI 也需要刚读取的 version：

```bash
version=$(skillshare extras memory show wiki/architecture.md --json -g | jq -r '.version')
skillshare extras memory delete wiki/architecture.md --version "$version" -g
```

使用 [`backup files`](../../reference/commands/backup.md) 恢复：先执行 `skillshare backup files show <absolute-note-path>`，再执行 `skillshare backup files restore <absolute-note-path> <id>`。
