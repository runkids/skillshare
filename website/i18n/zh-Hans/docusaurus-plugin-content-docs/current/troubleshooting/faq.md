---
sidebar_position: 4
---

# FAQ

关于 skillshare 的常见问题。

## General

### 这不就是 `ln -s` 吗？

从本质上说，是的。但 skillshare 额外处理了：
- 多个 Target 的检测
- Backup/restore
- Merge 模式（每个 Skill 各自的 symlink）
- 跨设备同步
- 损坏 symlink 的恢复

所以你不需要自己动手做这些事。

### 如果我在 Target 目录里修改一个 Skill 会怎样？

因为 Target 都是 symlink，改动会直接作用在 Source 上。所有 Target 会立刻看到这个变化。

### 如何保留某个 CLI 专属的 Skill？

使用 `merge` 模式（默认）。Target 中的本地 Skill 不会被覆盖或同步。

```bash
skillshare target claude --mode merge
skillshare sync
```

然后直接在 `~/.claude/skills/` 中创建 Skill——它们不会被动到。

### 我在用 dotfiles 管理工具（stow/chezmoi/yadm）——skillshare 会破坏我的 symlink 吗？

不会。skillshare 会检测 Source 和 Target 目录中的外部 symlink 并保留它们。所有命令——sync、update、uninstall、list、diff、install——都会解析 symlink 并对底层目录进行操作，而不会移除这些链接本身。详情参见 [Dotfiles Manager Compatibility](/docs/reference/commands/sync#dotfiles-manager-compatibility)。

如果你用 dotfiles 管理工具对 `config.yaml` 做版本控制，可以考虑启用 `preserve_tilde_on_save: true`，让路径以 `~/...` 的形式保存，而不是绝对路径——参见 [Configuration](/docs/reference/targets/configuration#preserve_tilde_on_save)。

---

## Installation

### 我可以把 Skill 同步到自定义或不常见的工具吗？

可以。用 `skillshare target add <name> <path>`，指定该工具的 skills 目录即可。

```bash
mkdir -p ~/.myapp/skills
skillshare target add myapp ~/.myapp/skills
skillshare sync
```

### 我可以在私有 git 仓库中使用 skillshare 吗？

可以。使用 SSH 地址：

```bash
skillshare init --remote git@github.com:you/private-skills.git
```

---

## Sync

### 为什么每次 install/update 之后都要执行 `sync`？

Sync 被有意设计成独立的一步。`install`、`update`、`uninstall` 这类操作只会修改 **Source** 目录——`sync` 才会把这些变更传播到所有 Target。

这样设计能让你：
- **批量处理变更**——一次装 5 个 Skill，只需 sync 一次，而不是五次
- **先预览**——在应用前先执行 `sync --dry-run`
- **掌握主动权**——由你决定何时更新 Target

**注意：** `pull` 是唯一会自动执行 sync 的命令，因为它的用意就是「把一切都更新到最新」。

完整设计理念参见 [Why Sync is a Separate Step](/docs/understand/source-and-targets#why-sync-is-a-separate-step)。

### 如何在多台机器之间同步？

使用基于 git 的跨机器同步：

```bash
# 机器 A：推送变更
skillshare push -m "Add new skill"

# 机器 B：拉取并同步
skillshare pull
```

完整设置方式参见 [Cross-Machine Sync](/docs/how-to/sharing/cross-machine-sync)。

### 如果我不小心通过 symlink 删除了一个 Skill 怎么办？

如果你已经初始化了 git（建议这样做），可以这样恢复：

```bash
cd ~/.config/skillshare/skills
git checkout -- deleted-skill/
```

或从 backup 还原：
```bash
skillshare restore claude
```

### 如果我不小心 uninstall 了一个 Skill 怎么办？

被 uninstall 的 Skill 会被移入 trash，并保留 7 天。用以下方式还原：

```bash
skillshare trash list                  # 查看 trash 中的内容
skillshare trash restore my-skill      # 还原到 Source
skillshare sync                        # 同步回各个 Target
```

如果该 Skill 是从远程 Source 安装的，你也可以重新安装：

```bash
skillshare install github.com/user/repo/my-skill
skillshare sync
```

在 Project mode 下，trash 位于项目目录内的 `.skillshare/trash/`。对 trash 命令使用 `-p` 参数即可。

执行 `skillshare doctor` 可以查看当前 trash 状态（项目数、大小、存放时间）。

### backup 和 trash 有什么区别？

| | backup | trash |
|---|---|---|
| **保护对象** | Target 目录（sync 快照） | Source 中的 Skill（uninstall） |
| **位置** | `~/.local/share/skillshare/backups/` | `~/.local/share/skillshare/trash/` |
| **触发条件** | `sync`、`target remove` | `uninstall` |
| **还原方式** | `skillshare restore <target>` | `skillshare trash restore <name>` |
| **自动清理** | 手动（`backup --cleanup`） | 7 天 |

两者是互补的——backup 保护 Target 免受 sync 变更影响，trash 保护 Source 中的 Skill 不被意外删除。

### 我可以把特定 Skill 只同步到特定 CLI 吗？

可以。例如：Skill A 只同步到 Claude，Skill B 同步到 Antigravity 和 Codex，Skill C 同步到所有 Target。

**选项 1：SKILL.md 中的 `targets` 字段**（由 Skill 作者设置）

```yaml
# skills/skill-a/SKILL.md
---
name: skill-a
targets: [claude]
---
```

```yaml
# skills/skill-b/SKILL.md
---
name: skill-b
targets: [antigravity, codex]
---
```

```yaml
# skills/skill-c/SKILL.md — 没有 targets 字段 = 同步到所有 Target
---
name: skill-c
---
```

**选项 2：配置中的 `include`/`exclude` 过滤器**（由使用者设置）

```yaml
# ~/.config/skillshare/config.yaml
targets:
  claude:
    path: ~/.claude/skills
    include: [skill-a, skill-c]
  codex:
    path: ~/.codex/skills
    include: [skill-b, skill-c]
```

两种方式可以搭配使用——配置中的过滤器会先生效，然后才应用 Skill 层级的 `targets` 字段。参见 [Skill Format — `targets`](/docs/understand/skill-format#targets) 与 [Configuration — filters](/docs/reference/targets/configuration#skill-level-targets)。

---

## Targets

### 与 npx skills 并存使用 universal {#using-universal-alongside-npx-skills}

`universal` Target 指向 `~/.agents/skills`，这也是 [npx skills CLI](https://github.com/vercel-labs/skills) 使用的同一个目录。两个工具可以同时管理这个目录，但有一些注意事项：

**可行的部分：**
- 在 merge 模式（默认）下，skillshare 会在 `~/.agents/skills/` 中创建 **symlink**；npx skills 则会创建**真实目录**。只要 Skill 名称不冲突，两者可以共存。
- skillshare 的清理逻辑只会移除它自己管理的条目——不会删除由 npx skills 安装的文件。
- Agent CLI（Claude Code、Pi 等）会直接读取目录，因此能同时看到两个工具带来的 Skill。

**需要留意的地方：**
- **名称冲突**——如果两个工具都安装了同名 Skill，以最后一次 sync/install 为准。避免用两个工具安装同一个 Skill。
- **copy 模式更激进**——在 copy 模式下（`skillshare target universal --mode copy`），skillshare 每次 sync 都会覆盖受管理的目录。如果 npx skills 在两次 sync 之间修改了同名 Skill，skillshare 会将其替换掉。merge 模式（默认）只创建 symlink，对共存来说更安全。
- **`npx skills list` 不会显示 skillshare 的 Skill**——npx skills CLI 是通过一个锁文件（`~/.agents/.skill-lock.json`）追踪安装内容的，而不是扫描目录。由 skillshare 同步的 Skill 不会出现在 `npx skills list -g` 中，但 Agent CLI **能**看到它们。
- **其他 Agent 专属 Target 依然有用**——`universal` 和 `claude` 指向不同的路径（`~/.agents/skills` 与 `~/.claude/skills`）。同时选用两者是安全的，也不算多余。

**建议的工作方式：**
```bash
# 用 skillshare 作为主要的 Skill 管理工具
skillshare install github.com/user/skills --track
skillshare sync

# npx skills 只用来做不需要同步的一次性社区安装
npx skills add someone/skill -g
```

:::tip
为了与 npx skills 最安全地共存，请让 universal Target 保持在 **merge 模式**（默认）。除非你完全不使用 npx skills，否则不要切换到 copy 模式。
:::

### 我之前用 `claude-code`（或 `gemini-cli` 等）作为 Project Target——这样还有效吗？

有效。旧的 Project Target 名称，例如 `claude-code`、`gemini-cli`、`github-copilot`，仍然可以通过别名解析。例如 `gemini-cli` 会解析为 `gemini`。我们建议把 `.skillshare/config.yaml` 更新为规范名称：

```yaml
# 之前
targets:
  - claude-code

# 之后
targets:
  - claude
```

### `target remove` 是怎么运作的？安全吗？

安全，它的流程是：

1. **Backup**——为该 Target 建立 backup
2. **检测模式**——判断是 symlink 模式还是 merge 模式
3. **Unlink**——移除所有由 skillshare 管理的 symlink，并把 Source 内容以真实文件的形式复制回来。在 merge 模式下，只有指向 Source 目录的 symlink 会被移除；本地（非 symlink）的 Skill 会被保留。
4. **更新配置**——从 config.yaml 中移除该 Target

这就是为什么 `skillshare target remove` 是安全的，而 `rm -rf ~/.claude/skills` 会删掉你的 Source 文件。

### 为什么在 Target 上执行 `rm -rf` 很危险？

在 symlink 模式下，整个 Target 目录就是指向 Source 的一个 symlink。删除它就等于删除 Source。

在 merge 模式下，每个 Skill 各自是一个 symlink。通过这个 symlink 删除某个 Skill，会连带删除 Source 中的文件。

**请始终使用：**
```bash
skillshare target remove <name>   # 安全
skillshare uninstall <skill>      # 安全
```

---

## Tracked Repos

### tracked repo 与普通 Skill 有什么不同？

| 方面 | 普通 Skill | Tracked Repo |
|--------|---------------|--------------|
| Source | 复制到 Source | 连同 `.git` 一起克隆 |
| 更新方式 | `install --update` | `update <name>`（git pull） |
| 前缀 | 无 | `_` 前缀 |
| 嵌套 Skill | 会被拍平 | 用 `__` 拍平 |

### 为什么要加下划线前缀？

`_` 前缀用来标识 tracked repository：
- 帮助你与普通 Skill 区分开
- 防止名称冲突
- 在列表中清楚地显示出来

---

## Skills

### SKILL.md 的格式是什么样的？

```markdown
---
name: skill-name
description: Brief description
---

# Skill Name

Instructions for the AI...
```

完整细节参见 [Skill Format](/docs/understand/skill-format)。

### "unknown target" 警告是什么意思？

在执行 `skillshare check` 或 `skillshare doctor` 时，你可能会看到：

```
! Skill targets: my-skill: unknown target "*"
```

这表示该 Skill 的 `SKILL.md` frontmatter 中的 `targets` 字段包含一个无法识别的名称——通常是 `"*"`（通配符）。skillshare 期望的是**精确的 Target 名称**（例如 `claude`、`cursor`、`codex`），而不是 glob 模式。

**如果你希望某个 Skill 同步到所有 Target**，直接省略 `targets` 字段即可：

```yaml
---
name: my-skill
description: Works everywhere
# 没有 targets 字段 = 同步到所有 Target
---
```

**如果这个警告来自第三方 Skill**，说明该 Skill 作者使用了不受支持的写法。你可以：
1. **忽略这个警告**——该 Skill 仍然能安装，只是不会自动过滤到特定 Target
2. **Fork 并修复**——在该 Skill 的 `SKILL.md` 中移除或修正 `targets` 字段

完整规范参见 [Skill Format — `targets`](/docs/understand/skill-format#targets)。

### 一个 Skill 可以包含多个文件吗？

可以。一个 Skill 目录可以包含：
- `SKILL.md`（必需）
- 任意其他文件（示例、模板等）

在 SKILL.md 的说明中引用它们即可。

---

## Performance

### Sync 好像很慢

检查 skills 目录中是否有大文件。加上忽略规则：

```yaml
# ~/.config/skillshare/config.yaml
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
  - "**/node_modules/**"
  - "**/*.log"
```

### 我最多能有多少个 Skill？

没有硬性上限。性能取决于：
- Skill 数量
- Skill 文件的大小
- Target 数量

数千个小型 Skill 也能正常运作。

---

## Backups

### backup 保存在哪里？

```
~/.local/share/skillshare/backups/<timestamp>/
```

### backup 会保留多久？

默认情况下是无限期保留。用以下方式清理：
```bash
skillshare backup --cleanup
```

---

## Agents

### agent 和 Skill 有什么区别？

Skill 是**目录**，其中包含一个 `SKILL.md` 文件（可以再加上辅助文件、示例、模板）。agent 则是**单一的 `.md` 文件**，带有 frontmatter——没有嵌套结构。两者都支持 install、sync、audit、check、backup 和 trash。

完整比较与 agent 文件格式参见 [Agents](/docs/understand/agents)。

### 哪些 Target 支持 agent？

开箱即用支持的有：`claude`、`cursor`、`augment`、`opencode`（以及 `universal` 别名）。其他 Target 在 agent 同步时会被自动跳过，并显示 `target(s) skipped for agents (no agents path)` 警告。你可以通过编辑 `config.yaml` 手动为它添加 agent 路径：

```yaml
targets:
  myapp:
    path: ~/myapp/skills
    agents:
      path: ~/myapp/agents
```

### 如何在不删除某个 agent 的情况下停用它？

使用 `disable` 命令（或直接编辑 `.agentignore`）：

```bash
skillshare disable my-agent --kind agent     # 在 .agentignore 中添加条目
skillshare enable my-agent --kind agent      # 移除该条目
```

`.agentignore` 位于 agents source 的根目录（Global mode 下是 `~/.config/skillshare/agents/.agentignore`，Project mode 下是 `.skillshare/agents/.agentignore`），使用 [gitignore 语法](https://git-scm.com/docs/gitignore)。也支持 `.agentignore.local` 叠加层，用于仅本地生效的覆写。

### 我可以在 Project mode 下备份 agent 吗？

可以——而且**仅限** agent。`backup` 在 Project mode 下不支持 Skill，但 agent 的流程是明确的例外：

```bash
skillshare backup -p agents     # 备份 Project mode 下的 agent Target
skillshare backup -p --all      # 效果相同；在 Project mode 下 --all 会收窄为 agent
```

如果忘了加 `agents` 过滤条件，会看到 `backup is not supported in project mode (except for agents)`。`restore` 也适用同样的规则。agent 的 backup 存放在常规 Skill backup 旁边的 `<target>-agents/` 目录下。

---

## Security

### 我可以信任第三方 Skill 吗？

Skill 是给你的 AI Agent 的指令——恶意的 Skill 可能会诱使 AI 泄露密钥或执行破坏性命令。skillshare 内置了安全扫描器来缓解这个风险：

- **install 时自动扫描**——每次 `skillshare install` 都会扫描该 Skill
- **CRITICAL 级别的发现会被拦截**——prompt injection、数据外泄、凭证访问默认会被阻止
- **手动扫描**——随时执行 `skillshare audit` 来扫描所有已安装的 Skill

完整的检测模式清单参见 [audit command](/docs/reference/commands/audit)。

### 如果 audit 阻止了我的安装怎么办？

如果某个 Skill 触发了 CRITICAL 级别的发现，安装会被阻止。你有两个选择：

1. **查看该发现**——确认是否为误判（例如只是一段文档示例）
2. **强制安装**——如果信任该来源，用 `--force` 绕过检查

```bash
skillshare install suspicious-skill --force
```

### audit 能捕捉所有问题吗？

没有任何扫描器是完美的。`skillshare audit` 能捕捉常见模式，例如 prompt injection、带密钥的 `curl`/`wget`、凭证文件访问、以及被混淆过的负载。对来自不受信任来源的 Skill，请务必手动检查。

---

## Getting Help

### 我该去哪里报告 bug？

[GitHub Issues](https://github.com/runkids/skillshare/issues)

### 我该去哪里提问？

[GitHub Discussions](https://github.com/runkids/skillshare/discussions)

---

## Related

- [Common Errors](./common-errors.md) — 错误解决方式
- [Windows](./windows.md) — Windows 专属 FAQ
- [Troubleshooting Workflow](./troubleshooting-workflow.md) — 一步步排查问题
