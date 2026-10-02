---
sidebar_position: 3
---

# check

检查已跟踪仓库和已安装 skills 是否有可用更新，但不应用任何更改。

```bash
skillshare check                      # 检查所有仓库和 skills
skillshare check my-skill             # 检查单个 skill
skillshare check a b c                # 检查多个 skills
skillshare check --group frontend     # 检查 frontend/ 中的所有 skill
skillshare check x -G backend         # 混合使用名称和 group
skillshare check --json               # 机器可读输出
```

## 何时使用

### 更新之前

在运行 `update` 之前预览会有哪些变化：

```bash
skillshare check         # 查看有哪些更新
skillshare update --all  # 应用更新
skillshare sync          # 分发更改
```

### CI/CD 流水线

在 CI 中检查过时的 skills：

```bash
result=$(skillshare check --json)
# 解析 JSON 以检测过时的 skills
```

## 它做了什么

`check` 会检查你的 source 目录，并报告以下内容的更新状态：

1. **已跟踪仓库**——从 origin 拉取，显示落后多少个 commit
2. **已安装 skills（带 metadata）**——将已安装版本与 remote HEAD 进行比较
3. **过时的 skills**——检测哪些 skill 的子目录已在 upstream 仓库中被删除
4. **从本地路径安装的 skills**——将安装来源路径中的文件与安装时记录的文件进行比较（参见[从本地路径安装](#local-path-installs)）
5. **本地 skills**——将没有安装 metadata 的 skill 标记为 "local source"（没有可比较的对象）
6. **Skill 级别的 `targets` 校验**——对 SKILL.md `targets` frontmatter 字段中的未知 target 名称发出警告

与 `update` 不同，`check` 绝不会修改任何文件。

## 从本地路径安装 {#local-path-installs}

从磁盘目录安装的 skill（`skillshare install /path/to/skill`）会记录该路径以及所复制每个文件的哈希值。`check` 会重新计算该路径下文件的哈希值，并报告：

- **up to date**——文件与安装时一致
- **update available**——来源路径中有文件被修改、新增或删除
- **error**——来源路径已不存在（`local source not found: <path>`）

这适用于由其他应用程序分发和更新的 skill，例如 App bundle 中的 skill：

```bash
skillshare install /Applications/Surge.app/Contents/Resources/Skills/surge

# App 更新后：
skillshare check surge     # → Update available
skillshare update surge    # 从该路径重新复制，并执行安全审计
skillshare sync
```

在记录文件哈希值之前安装的 skill，会保持 "local source" 状态，直到执行 update 或重新安装。

## 示例输出

```
skillshare check

  Tracked Repos
  ─────────────────────────────────────────
  ✓ _team-skills       up to date
  ⬇ _shared-rules      3 commits behind
  ! _design-system     has uncommitted changes

  Installed Skills (remote)
  ─────────────────────────────────────────
  ✓ pdf                up to date          anthropics/skills
  ⬇ commit             update available    anthropics/skills
  ⚠ old-helper         stale (deleted upstream)
  • local-skill        local source

  ⚠ 1 skill(s) stale (deleted upstream) — run 'skillshare update --all --prune' to remove
  Summary: 1 repo + 1 skill have updates available
  Run 'skillshare update <name>' or 'skillshare update --all'
```

## 检查指定的 Skills

你可以按名称检查一个或多个 skill，而不是扫描所有内容：

```bash
skillshare check my-skill                # 单个 skill
skillshare check skill-a skill-b         # 多个 skills
```

使用 `--group` / `-G` 检查某个 group 目录中所有可更新的 skills：

```bash
skillshare check --group frontend        # frontend/ 下的所有 skill
skillshare check -G frontend -G backend  # 多个 groups
skillshare check my-skill -G frontend    # 混合使用名称和 group
```

如果某个位置参数匹配的是 group 目录（而非仓库或 skill 本身），会自动展开：

```bash
skillshare check frontend               # 自动识别为 group
```

在展开 group 时，没有 metadata 的 skills（仅本地）会被跳过。

## 选项

| 标志 | 描述 |
|------|-------------|
| `--group`, `-G` `<name>` | 检查某个 group 中所有可更新的 skills（可重复使用） |
| `--project`, `-p` | 检查 project 级别的 skills（`.skillshare/`） |
| `--global`, `-g` | 检查 global skills（`~/.config/skillshare`） |
| `--json` | 以 JSON 输出（用于脚本/CI） |
| `--help`, `-h` | 显示帮助 |

:::tip 自动检测
如果既未指定 `--project` 也未指定 `--global`，skillshare 会自动检测：如果当前目录存在 `.skillshare/config.yaml`，则默认使用 project mode；否则使用 global mode。
:::

## JSON 输出

```bash
skillshare check --json
```

```json
{
  "tracked_repos": [
    {"name": "_team-skills", "status": "up_to_date", "behind": 0, "branch": "main"},
    {"name": "_shared-rules", "status": "behind", "behind": 3, "branch": "develop"}
  ],
  "skills": [
    {"name": "pdf", "source": "anthropics/skills", "version": "a1b2c3d",
     "status": "up_to_date", "installed_at": "2024-06-01T10:00:00Z"},
    {"name": "commit", "source": "anthropics/skills", "version": "x9y8z7w",
     "status": "update_available", "installed_at": "2024-05-15T08:30:00Z"},
    {"name": "old-helper", "source": "anthropics/skills", "version": "d4e5f6g",
     "status": "stale", "installed_at": "2024-03-10T09:00:00Z"},
    {"name": "local-skill", "source": "", "version": "",
     "status": "local", "installed_at": "2024-04-20T12:00:00Z"}
  ]
}
```

`"status": "error"` 的 skill 在已知原因时会包含 `message` 字段，例如 `"message": "local source not found: /path/to/skill"`。

## 状态图标

| 图标 | 含义 |
|------|---------|
| `✓` | 已是最新 |
| `⬇` | 有可用更新（已跟踪仓库：落后的 commit 数；skill：有更新版本） |
| `⚠` | 过时——子目录已在 upstream 被删除或重命名 |
| `!` | 有未提交的更改 |
| `•` | 本地 source（没有安装 metadata 可比较） |

:::info 过时的 skills
当某个 skill 的子目录在 upstream 被重命名或删除时，`check` 会将其报告为**过时（stale）**。使用 `update --prune` 清理过时的 skills。
:::

:::tip Monorepo 感知
对于从子目录安装的 skills，`check` 只有在该特定目录发生变化时才会报告 "update available"——而不是仓库中其他不相关部分有新 commit 时。
:::

## Project Mode

```bash
skillshare check -p                    # 检查所有 project skills
skillshare check -p my-skill           # 检查指定的 project skill
skillshare check -p --group frontend   # 检查 project group
skillshare check -p --json             # project 的 JSON 输出
```

## Agent 支持

`skillshare check agents` 将检查范围限定为仅 agents，报告 agents source 目录中 `.md` 文件的漂移和更新状态：

```bash
skillshare check agents              # 检查所有 agents
skillshare check agents --json       # agents 的 JSON 输出
skillshare check agents -p           # 检查 project agents
```

不带 `agents` 参数时，`check` 仅对 skills 起作用（默认行为）。关于背景信息，参见 [Agents](/docs/understand/agents)。

## 另请参阅

- [update](/docs/reference/commands/update) — 应用更新
- [list](/docs/reference/commands/list) — 查看已安装的 skills
- [status](/docs/reference/commands/status) — 显示同步状态
- [Agents](/docs/understand/agents) — Agent 概念
