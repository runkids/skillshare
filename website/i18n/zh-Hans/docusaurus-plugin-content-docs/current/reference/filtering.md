---
sidebar_position: 3
---

# 过滤参考

三层过滤机制的完整规范，用于控制哪些 Skill 到达哪些 Target。

:::tip 需要快速指南？
参见 [Filtering Skills](/docs/how-to/daily-tasks/filtering-skills) 获取场景驱动的指南。
:::

## 概览

| 层级 | 范围 | 设置位置 | 语法 | 评估时机 |
|-------|-------|-------------|--------|-------------|
| `.skillignore` | 对所有 Target 隐藏 | Source 目录或已 track 的仓库根目录 | [gitignore](https://git-scm.com/docs/gitignore) | Discovery |
| SKILL.md `metadata.targets` | 将 Skill 限制到列出的 Target | 各 Skill frontmatter | YAML 列表 | Sync（在 discovery 时解析） |
| Agent `targets` | 将 Agent 限制到列出的 Target | 各 Agent frontmatter | YAML 列表 | Sync（在 discovery 时解析） |
| Target include/exclude | 按 Target、按资源 | `config.yaml` 或 CLI flag | Go [`filepath.Match`](https://pkg.go.dev/path/filepath#Match) glob | Sync |

:::note Sync mode 注意事项
以上三层过滤只适用于 **merge** 和 **copy** Sync 模式。
在 **symlink** 模式下，整个 Source 目录作为一个整体被链接——逐个 Skill 的过滤不会生效。
:::

## 评估顺序与优先级

一个 Skill 必须通过**所有**层级才能到达某个 Target：

1. **`.skillignore`** — 在 discovery 时评估。匹配的 Skill 永远不会进入 sync 流程。
2. **Target include/exclude** — 在 sync 时评估（`FilterSkills`）。Skill 会被发现，但对不匹配的 Target 会被跳过。
3. **SKILL.md `metadata.targets`** — 在 sync 时评估（`FilterSkillsByTarget`）。Skill 被限制到其声明的 Target。

## .skillfollow

[`.skillfollow`](./skillfollow.md) 是发现 opt-in，不是另一层 filter。文件放在 skills source 根目录，声明第一层外部组/tracked repo 链接；local 文件为名称并集，不接受 gitignore patterns。加入链接的根目录锚定无末尾斜杠 ignore 与 `/.skillfollow.local`；root/repo `.skillignore` 仍过滤 followed skills。见[完整设置与状态](./skillfollow.md#states)。

## .skillignore

**位置：**
- Source 根目录：`~/.config/skillshare/skills/.skillignore` — 适用于所有 Skill
- 已 track 的仓库根目录：`_team-repo/.skillignore` — 仅适用于该仓库内部，不限 source 中的深度（例如以 `--into group/sub` 安装的仓库，或 followed 组内的仓库）。仓库嵌套时以最内层仓库的规则为准

**语法：** 完整的 [gitignore](https://git-scm.com/docs/gitignore) 语法 —— `*`（单段）、`**`（任意深度）、`?`、`[abc]`、`!pattern`（取反）、`/pattern`（锚定）、`pattern/`（仅目录）。

**`.skillignore.local`：** 放在 `.skillignore` 旁边。规则会追加在基础文件之后——以最后匹配的规则为准。使用 `!pattern` 来取消忽略。不要提交此文件。不含 `/` 的规则会匹配任意深度的同名路径，所以 `!feature-radar` 也会重新启用 `feature-radar/feature-radar`；只想匹配顶层文件夹时写 `!/feature-radar`。Dashboard 会在 **Settings → Files** 显示此文件的规则，被它覆盖的开关会报告为失败。

**CLI 可见性：**

| 命令 | 输出 |
|---------|--------|
| `skillshare sync` | 数量 + Skill 名称 |
| `skillshare status --json` | `source.skillignore` 对象，包含 pattern 和被忽略的列表 |
| `skillshare doctor` | Pattern 数量与被忽略数量 |

📖 [File structure 参考](/docs/reference/appendix/file-structure#skillignore-optional)

## SKILL.md targets 字段 {#skillmd-targets-field}

**格式：** 顶层或嵌套于 `metadata` 之下：

```yaml
# 推荐
metadata:
  targets: [claude, cursor]

# 旧版兼容写法
targets: [claude, cursor]
```

**行为：** 白名单——该 Skill 只会 sync 到列出的 Target。省略此字段表示 sync 到所有 Target。如果 `metadata.targets` 与顶层 `targets` 同时存在，`metadata.targets` 优先。

**Tracked repos：** 在 dashboard 中为 tracked repo 内的 skill 设置的 targets 存储在 source 的 `.metadata.json` 中，而不是它的 SKILL.md，因此 clone 保持干净，`update` 也能照常运行。该设置优先于 skill 的 `metadata.targets`。

**别名：** Target 名称支持别名。`claude` 会匹配配置为 `claude-code` 的 Target。参见 [Supported Targets](/docs/reference/targets/supported-targets)。

📖 [Skill format — targets 字段](/docs/understand/skill-format#targets)

**Agent** 也通过 Agent frontmatter 中顶层的 `targets` 列表支持相同的白名单机制。没有该字段的 Agent 会 sync 到所有支持 Agent 的 Target。参见 [Agents — Agent File Format](/docs/understand/agents#agent-file-format)。

## Target include/exclude 过滤器 {#target-includeexclude-filters}

**通过 CLI 设置：**

```bash
# Skill
skillshare target claude --add-include "team-*"
skillshare target cursor --add-exclude "legacy-*"
skillshare target claude --remove-include "team-*"

# Agent
skillshare target claude --add-agent-include "team-*"
skillshare target claude --add-agent-exclude "draft-*"
skillshare target claude --remove-agent-include "team-*"
```

**存储位置：** `config.yaml` 中，Skill 对应 `targets.<name>.include` / `targets.<name>.exclude`，Agent 对应 `targets.<name>.agents.include` / `targets.<name>.agents.exclude`。

**语法：** Go [`filepath.Match`](https://pkg.go.dev/path/filepath#Match) glob 模式，匹配扁平化的资源名称。Skill 使用扁平化的 Skill 名称（例如 `_team__frontend__ui`）；Agent 使用扁平化的 `.md` 文件名。

| 支持 | 不支持 |
|-----------|--------------|
| `*`（任意字符） | `**`（递归） |
| `?`（单个字符） | `{a,b}`（花括号展开） |
| `[abc]`（字符类） | |

**优先级：** 当 `include` 和 `exclude` 同时设置时，先应用 `include`，再应用 `exclude`。同时命中两者的资源会被排除。

**可视化编辑器：** `skillshare ui` → Targets 页面 → "Customize filters" 按钮。

📖 [Target 命令](/docs/reference/commands/target#target-filters-includeexclude) · [过滤行为示例](/docs/reference/commands/sync#filter-behavior-examples) · [Configuration](/docs/reference/targets/configuration#include--exclude-target-filters)

## 另请参阅

- [Filtering Skills](/docs/how-to/daily-tasks/filtering-skills) — 场景驱动的操作指南
