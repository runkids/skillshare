---
sidebar_position: 8
---

# Recipe: Many Projects, One Config

> 通过 global 配置，一次 sync 就把 Skill 和 MCP server 发送到多个项目文件夹。

## Scenario

全局 Target 例如 `~/.claude/skills` 会被每个项目读取，因此所有项目看到的都是同一套 Skill。如果这正是你想要的效果，就不需要这份 recipe。

这份 recipe 适用于项目需要拿到**不同**内容的情况：

- 你安装了很多 Skill，而某个前端项目只需要 `frontend-*`。会话中 Skill 越少，花在描述上的上下文就越少，选错的概率也越低。
- 某个项目不能加载在其他地方都没问题的 MCP server，比如客户的仓库。
- 某个项目需要保留 Skill 的真实副本以便提交，但不想提交任何 Skillshare 配置。
- 你使用的某个工具只读取项目内部的某个文件夹。

[Project mode](/docs/how-to/recipes/skill-per-project-workflow) 同样能让每个项目拥有自己的一套内容：它会在每个项目中保留一份 `.skillshare/config.yaml`，需要在各个文件夹内部分别 sync。global 配置中的 `projects` 能在你机器上的一份文件里做到同样的效果：

| | 全局 Target | 全局 `projects` | Project mode |
|---|---|---|---|
| **谁会拿到 Skill** | 每个项目，相同的一套 | 你列出的文件夹，各自一套 | 仅这一个项目 |
| **配置保存在哪里** | 你的机器 | 你的机器 | 项目的仓库 |
| **队友能不能拿到** | 否 | 否 | 是，通过 clone 获得 |
| **会给项目添加哪些文件** | 无 | 只有被 sync 的 Skill 和 Agent | `.skillshare/` 加上被 sync 的文件 |
| **Sync** | 在任意位置运行一次 `sync` | 在任意位置运行一次 `sync` | 在每个项目内部运行 `sync` |

当配置需要随仓库一起流转时，选择 project mode。当项目只属于你自己、面对无法添加 `.skillshare/` 的客户仓库或开源仓库，或者你想用一次 `sync` 更新所有项目时，选择 `projects`。

## Solution

### Skill 与 Agent：`projects`

```yaml
# ~/.config/skillshare/config.yaml
projects:
  ~/work/project01:
    targets: [claude, codex]
    skills:
      mode: copy
      include:
        - myskill-*
    agents: {}
```

```bash
skillshare sync --dry-run   # 预览
skillshare sync
```

- `targets` 指定该项目使用的工具。Skillshare 会写入每个工具的 project 路径，这里是 `.claude/skills` 和 `.agents/skills`，因此不需要输入路径。
- `skills` 和 `agents` 用来开启对应部分的 sync。留空时会同步全部内容；用 `include` 和 `exclude` 缩小范围。参见 [Filtering skills](/docs/how-to/daily-tasks/filtering-skills)。
- `copy` 会写入真实文件，因此项目可以将它们提交。如果 symlink 就够用，保留默认的 `merge` 即可。

在仪表盘中，**Projects** 页面做的是同一件事：**Add project**，选择 Target，再选择要 sync 的内容。完整字段参见 [`projects`](/docs/reference/targets/configuration#projects)。

### MCP server：`mcp.projects`

MCP server 会被写入每个 Agent 自己的配置文件，因此它们按项目文件夹列出，而不是按路径列出：

```yaml
# ~/.config/skillshare/config.yaml
mcp:
  servers:
    context7:
      command: npx
      args: ["-y", "@upstash/context7-mcp"]
      targets: [opencode]
  projects:
    ~/work/project01:
      servers:
        context7:            # 其他地方都会加载，在这里关闭
          disabled: true
          targets: [opencode]
```

```bash
skillshare sync mcp --dry-run   # 预览每个文件
skillshare sync mcp
```

字段和限制参见 [`mcp`：管理多个项目](/docs/reference/commands/mcp#manage-several-projects-from-the-global-config)。

## Verification

- `skillshare sync` 会报告该项目的 Target，例如 `project01@claude  1 copied`
- `~/work/project01/.claude/skills/` 中只包含 `include` 匹配到的 Skill
- `skillshare sync mcp --dry-run` 为每个项目文件列出一行
- 再次运行 `skillshare sync mcp` 会将每个条目报告为 `unchanged`

## Variations

- **未被工具路径覆盖的文件夹**：Target 只是一个名称加一个路径，所以对于没有任何工具的 project 路径覆盖到的文件夹，`skillshare target add project01 ~/work/project01/some/folder` 仍然有效。当这样的 Target 恰好指向某个工具的 project 路径时，仪表盘的 **Projects** 页面会提示将它转换。
- **提交或忽略**：在 `copy` 模式下，Skillshare 还会在 Target 文件夹中写入 `.skillshare-manifest.json`，用于跟踪它复制了哪些内容。可以将它与 Skill 一起提交，或将它加入 `.gitignore`。
- **路径重叠警告**：如果某个项目文件夹与另一个 Target 已在使用的文件夹相同，`sync` 会打印路径重叠警告。运行 `skillshare doctor` 查看哪些 Target 共用了该路径。
- **多个项目使用同一个 server**：在其中一个项目下用 YAML anchor（`docs: &docs`）定义一次，然后在其他项目中复用（`docs: *docs`）。参见 [`mcp` reference](/docs/reference/commands/mcp#manage-several-projects-from-the-global-config)。
- **共享的项目**：clone 该项目的队友不会得到你的 global 配置。当设置必须随仓库一起传递时，请使用 [project mode](/docs/how-to/recipes/skill-per-project-workflow)。

## Related

- [`target` command reference](/docs/reference/commands/target)
- [`mcp` command reference](/docs/reference/commands/mcp)
- [Sharing MCP servers](/docs/how-to/daily-tasks/sharing-mcp)
