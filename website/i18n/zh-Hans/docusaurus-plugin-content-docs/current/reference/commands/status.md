---
sidebar_position: 7
---

# status

显示 skillshare 的当前状态：source、tracked 仓库、targets 和版本。

```bash
skillshare status
```

启用 `follow_source_links: true` 时，不可用的第一层来源链接会显示包含链接名称的警告，正常技能仍会列出。使用 `--json` 时，警告写入 stderr，stdout 保持有效的 JSON。

## 何时使用

- 在做出变更后检查所有 Target 是否已同步
- 查看哪些 Target 需要运行 `sync`
- 验证 tracked 仓库是否为最新
- 验证当前生效的 audit 策略（profile、threshold、dedupe mode）
- 检查 CLI 或 skill 更新

## 示例输出

```
Source
  skills    ~/.config/skillshare/skills  43 skills
  agents    ~/.config/skillshare/agents  2 agents

Tracked repositories
✓ _superpowers  15 skills

Targets                        skills                 agents
  claude     ~/.claude/skills  ✓ 43 linked            ✓ 2
  cursor     ~/.cursor/skills  ✓ 43 linked · 1 local  ✓ 2
  gemini     ~/.gemini/skills  ✓ 43 linked            —
  universal  ~/.agents/skills  ✓ 43 linked            —
  all use merge

Extras
  rules     ~/.claude/rules     2 files · merge
  rules     ~/.cursor/rules     2 files · merge
  commands  ~/.claude/commands  1 file · merge
  team      ~/.codex            1 file · symlink
  team      ~/.claude           1 file · import

Audit    default · blocks critical
Version  CLI 0.24.0 · skill 0.21.12
! Skill 0.21.13 is available — run skillshare upgrade --skill && skillshare sync
```

## 各区块说明

### Source

显示 skills 文件夹及其中的 skill 数量。若存在 agents 文件夹，会在第二行显示它和 agent 数量。`.skillignore` 生效时，会多一行显示规则数和被忽略的 skill 数。

### Tracked Repositories

列出通过 `--track` 安装的 git 仓库及各自的 skill 数量。`✓` 表示仓库没有未提交的更改；`!` 会附上 `uncommitted changes`，或在无法读取 git status 时附上错误信息。

### Targets

每个 Target 一行：名称、skills 文件夹，以及 skills 和 agents 的状态。表格下方的一行列出正在使用的 sync 模式。

```
Targets                     skills                agents
  claude  ~/.claude/skills  ✓ 8 linked · 2 local  ✓ 8
  cursor  ~/.cursor/skills  ! 6/8 copied          ! 7/8
  copy: cursor · merge: claude
! 2 skills not synced — run skillshare sync
```

**skills 列：**

| 显示 | 含义 |
|------|------|
| `✓ 8 linked` / `✓ 8 copied` | 预期的 skill 都已就位。merge 和 copy 模式按经过 `include`/`exclude` 过滤后的集合计算 |
| `· 2 local` | 该文件夹中你自己的 skill，sync 不会改动它们 |
| `! 6/8 linked` | 部分 skill 尚未同步；status 最后会列出数量和 `sync` 命令。`sync` 有意跳过的 skill（`standard` 或 `prefixed` 命名下名称无效，或名称冲突）不计入，因为再次运行 `sync` 也补不上；`sync` 会列出它们 |
| `✓ symlinked` | symlink 模式：整个文件夹链接到 source |
| `! needs sync` | 模式已变更，运行 `sync` 应用 |
| `! has files` / `! not synced yet` | 这个 Target 还没有同步过 |
| `✗ links to …` / `✗ broken link` | 文件夹链接到别处，或链接到不存在的位置 |
| `skills off` | 这个 Target 的 skills 已关闭 |

**agents 列：** `✓ 8` 是已链接的 agent 数量（最新的副本也算作已链接）。`! 7/8` 表示部分缺失，请运行 `skillshare sync agents`。只计算该 Target 会同步的 agent，也就是经过 `.agentignore`、Target 的 agents include/exclude 以及各 agent 的 `targets` frontmatter 之后剩下的。在 copy fallback 中，skillshare 不拥有但内容相同的本地文件会被保留，并计为 `· 1 local`。`—` 表示该 Target 没有 agents 文件夹。没有 agents source 时，此列会省略。

### Extras

配置了 extras 时，每个 extra target 各占一行：

```
Extras
  rules     .cursor/rules     4 files · merge
  commands  .claude/commands  3 files · merge
```

每行显示 extra 名称、target 文件夹、文件数量，以及文件实际使用的同步模式：在未开启开发者模式的 Windows 上，链接文件的 target 会显示 `copy`。

### Audit

一行显示当前生效的 audit 策略（由 CLI flags、项目配置或全局配置解析而来）：profile（`default`、`strict` 或 `permissive`），以及会阻止安装的最低严重级别（默认为 `critical`）。dedupe 模式和分析器只有在不同于默认值（`global` 和全部分析器）时才会列出。

### Version

显示 CLI 和 skill 的版本。有更新的 skill 发布时，会多一行说明如何更新。（仅限 global 模式）

## Options

| Flag | 说明 |
|------|------|
| `--json` | 以 JSON 格式输出（用于脚本/CI） |
| `--project, -p` | 使用 project mode |
| `--global, -g` | 使用 global mode |
| `--help, -h` | 显示帮助信息 |

## JSON 输出

```bash
skillshare status --json
```

```json
{
  "source": {
    "path": "~/.config/skillshare/skills",
    "exists": true,
    "skillignore": {
      "active": true,
      "files": [".skillignore", "_team-skills/.skillignore"],
      "patterns": ["test-*", "vendor/"],
      "ignored_count": 2,
      "ignored_skills": ["test-draft", "vendor/lib"]
    }
  },
  "skill_count": 12,
  "tracked_repos": [
    {"name": "_team-skills", "skill_count": 5, "dirty": false},
    {"name": "_personal-repo", "skill_count": 3, "dirty": true}
  ],
  "targets": [
    {
      "name": "claude",
      "path": "~/.claude/skills",
      "mode": "merge",
      "status": "merged",
      "synced_count": 8,
      "include": [],
      "exclude": []
    }
  ],
  "agents": {
    "source": "~/.config/skillshare/agents",
    "exists": true,
    "count": 8,
    "targets": [
      {"name": "claude", "path": "~/.claude/agents", "expected": 8, "linked": 8, "drift": false}
    ]
  },
  "audit": {
    "profile": "DEFAULT",
    "threshold": "CRITICAL",
    "dedupe": "GLOBAL",
    "analyzers": []
  },
  "version": "0.17.0"
}
```

只有 sync 会拒绝某个 target 时，该 target 才会有 `warning` 字符串，例如在 copy 以外的模式下使用 `prefixed` naming；文字中会说明如何修正。

无法读取 git status 的 tracked repo 会显示 `"status": "unknown"`，`message` 中包含错误信息；此时 `dirty` 为 false，且没有意义。

`source.skillignore` 字段仅在至少存在一个 `.skillignore` 或 `.skillignore.local` 文件时出现。不存在时：`"skillignore": { "active": false }`。`files` 数组在 `.skillignore.local` 存在时会包含其路径。在文本模式下，当任何 `.skillignore.local` 生效时，`.skillignore` 那一行会显示 `(.local active)`。

JSON 输出在 global mode 和 project mode 下都支持。

## Project Mode

在项目目录中，status 会显示项目的 source、targets 和 extras，路径以项目根目录为基准：

```bash
skillshare status        # 如果存在 .skillshare/ 则自动检测
skillshare status -p     # 显式指定 project mode
```

### 示例输出

```
Source
  skills    .skillshare/skills  3 skills
  agents    .skillshare/agents  4 agents
  .skillignore: 3 patterns, 0 skills ignored

Targets                   skills      agents
  claude  .claude/skills  ✓ 3 linked  ✓ 4
  cursor  .cursor/skills  ✓ 3 linked  ✓ 4
  all use merge

Extras
  rules     .cursor/rules     4 files · merge
  commands  .claude/commands  3 files · merge

Audit    default · blocks critical
```

Project status 不显示 Tracked Repositories 和 Version 区块（这些是仅限 global 的功能）。

## 另请参阅

- [sync](/docs/reference/commands/sync) —— 把 skills 同步到 Target
- [diff](/docs/reference/commands/diff) —— 显示详细差异
- [doctor](/docs/reference/commands/doctor) —— 诊断问题
- [Project Skills](/docs/understand/project-skills) —— Project mode 概念
