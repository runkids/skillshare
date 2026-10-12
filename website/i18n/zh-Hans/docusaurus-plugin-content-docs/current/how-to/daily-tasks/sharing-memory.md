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

左侧树视图支持嵌套文件夹，上方有搜索框，旁边的 **Refresh** 按钮会在 Agent 从 dashboard 之外写入笔记后，重新加载列表与当前打开的笔记；右侧可切换 **Preview** / **Source**，较长的笔记会先折叠，点击 **Show all** 展开。笔记名称旁是 **Edit** 与 **More actions** 菜单，包含 **Copy file path**、**History**、**Move or rename** 与 **Delete note**；**Use with agents** 位于笔记下方。相对链接可在查看器中打开现有笔记。

![展开 wiki 的英文双栏 Memory 查看器](/img/memory-tree-demo.png)

## 4. 搜索、编辑与恢复

在 **Search names and content** 输入 `Retrieval`。搜索包含子文件夹中的笔记路径与内容，不区分大小写。清除搜索即可显示全部笔记。

![英文搜索显示嵌套笔记](/img/memory-search-demo.png)

你也可使用文本编辑器。重新加载 dashboard 可查看外部更改。若编辑中的笔记被修改，过期的保存会被拒绝，草稿会保留。编辑器显示 **Latest saved version** 供比较，并提供 **Copy draft**。自行比较或合并后，点击 **Save my draft** 并确认替换。保存会使用更新后的 version 并备份已保存内容；若再次发生外部修改，仍会产生冲突。

![英文编辑器保留草稿并显示最新保存版本](/img/memory-conflict-demo.png)

**History** 会打开 **Backup Files**，以笔记的绝对路径筛选。可预览、恢复版本，再重新加载 Memory。已删除的笔记也能在同一页恢复。

![wiki/workflow-check.md 的英文 Backup Files 恢复预览](/img/memory-restore-demo.png)

## 试用与整理记忆

在 **Use with agents → 记忆工具** 选择 **跨对话试用**：

1. 将第一段提示粘贴到当前对话，请 Agent 在此来源的 `INDEX.md` 添加临时 `memory-check` 行，保留其他内容。
2. 在同一个或另一个已连接的工具中打开新对话，粘贴第二段提示。它仅包含来源路径，不包含验证值。检查实际文件读取事件、完整路径，并将返回值与步骤 1 比较。
3. 粘贴清理提示，仅移除该测试行。此浏览器标签页会在重新打开对话框或刷新时保留测试值。若浏览器存储不可用，请保留此页直到清理完成。

按钮仅复制提示，Skillshare 不会执行 Agent 或将试用标记为通过。明确读取只能验证数据检索，不代表 native automatic loading。

选择 **回顾本次** 可复制检查提示，寻找重复、矛盾、过期信息、失效索引链接和缺少日期、原因或来源的决策。Agent 先提出文件路径、证据及修改前后内容。你在对话中确认具体修改后，才写入、移动或删除笔记。未知日期与无法获取的证据会明确标记。

Dashboard 的 Memory 区域也能打开同一个 **记忆工具** 对话框。将整理提示粘贴到你想回顾的对话；Agent 只使用当前对话、可获取的工具结果及你提供的 session 记录，从纠正与反复错误中提取可复用的经验。参考 [retro 流程](https://github.com/mattpocock/skills/blob/main/skills/engineering/retro/SKILL.md)，先检查已有文档与检查机制，机械性错误优先建议自动检查，并按影响排序。批准后的记忆修改仅限画面显示的记忆来源；项目检查或指引修改需要另行同意。Skillshare 不会获取私人对话历史或执行这次回顾。

选择 **做梦**，比较跨对话保存的笔记与经验、发现反复出现的模式，并提出重复、过期、矛盾或失效链接的整理建议。笔记若有跨对话来源即可支持归纳，不强制保存原始 transcript；证据不足时必须说明结论限制。参考 [Agent Memory Repo](https://cognition.com/agent-memory-repo)，由你使用的 Agent 执行。

三个页签均为 inline，完整提示默认折叠，可按 **查看完整提示** 展开；**复制给 Agent** 会复制包含来源路径与项目／共享范围的完整提示。若 Agent 已安装更新的内置 Skillshare skill，也可直接要求回顾本次或对指定笔记做梦。Skill 会核对解析后的来源、提出具体路径与修改供确认，再通过版本检查及备份写入。不需要新的 journal 或强制 transcript archive；由宿主调度的回顾只提出建议。

记忆工具会停用直到来源范围确认；项目模式也需要项目路径。回顾与做梦提示都要求使用带版本检查和备份的 Skillshare CLI，来源不符、CLI 无法使用或发生冲突时停止写入。

更新的读取指引会在反复纠正或矛盾笔记出现时，用一行建议回顾或做梦，同一对话只提醒一次，不自行开始。Skillshare 不会启动或调度 Agent。

两种更新模式都要求决策与经验附日期、原因及可获取的来源或证据，成功写入后以一行报告存入内容、项目／共享范围及文件路径。例如：“已记住：此项目用 devcontainer 运行测试。项目记忆：testing.md。”沿用普通 Markdown，不强制 metadata schema。编辑器也显示来源提示。

现有工具通过 **Connect to agents → Review changes → Apply changes** 更新指引。完整但过期的区块可检查更新；手动修改过的区块仍保留。Skillshare 不会静默更新工具或现有笔记。


需要记录经验时，请要求 Agent 更新 `LEARNED.md` 的背景、结论与证据。两种模式都会在每个 task 开始时读 `INDEX.md`。笔记由用户管理：`passive` 指引让 Agent 提出值得记的事实、只在用户要求时更新；`active` 指引则让 Agent 按上述规则把这类事实保存在这里，而不是工具自己的记忆：一两句话说得完的事实，直接作为 `INDEX.md` 中 `## Notes` 下的一个条目，较长的笔记才另建文件。本功能不启用 native automatic memory、自动学习或 Obsidian 集成。

## Project mode

先初始化项目的 Skillshare 配置，再执行：

```bash
skillshare extras memory init -p
skillshare ui -p
```

默认来源为 `.skillshare/extras/memory/`；使用可见配置目录时为 `skillshare/extras/memory/`。现有 extras source overrides 仍适用。Project mode 提供相同的连接、索引、编辑与恢复流程。Repo 内来源的指引使用相对于 **project root** 的路径，即使 instructions 文件在子文件夹中也一样。Project 指引会告诉 agent 把这个项目的笔记放在这里，关于你、你的工具或其他项目的笔记不属于这里，因此同时读到 shared memory 指引的 agent 知道每个事实该存哪里。Project 外的 override 使用绝对路径。移动绝对来源或更改位置后，请重新生成并检查指引。

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
