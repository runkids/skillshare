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

## .skillignore

**位置：**
- Source 根目录：`~/.config/skillshare/skills/.skillignore` — 适用于所有 Skill
- 已 track 的仓库根目录：`_team-repo/.skillignore` — 仅适用于该仓库内部

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

### Tracked repo 中的 Agent 专用变体 {#agent-specific-variants-in-tracked-repositories}

```text
_team/internal/skills/claude/review/SKILL.md
_team/internal/skills/codex/review/SKILL.md
```

在 tracked repo 中，`skills/<已知-agent>/<完整技能路径>/SKILL.md` 会自动识别为 Agent 专用变体，例如 `_team/internal/skills/{claude,codex}/review/SKILL.md`。逻辑 Skill 由仓库、Agent 前的完整路径和 Agent 后的完整路径共同确定；即使只剩一个变体也会按 Agent 路由。`--into` 下的仓库也适用。

**merge/copy：** 仅选择匹配 Agent 的文件，支持 `claude-code` 等注册别名以及账户 Target 的 `agent: codex`。共用目录不代表同一 Agent，`universal` 不会自动接收 Codex 变体。找不到匹配项时不选其他变体；同一组中有多个匹配同一 Agent 的别名变体时会跳过并提示，需用过滤器或覆盖设置明确选择。

路由优先级为：Source 的 `.metadata.json` Target 覆盖 > 非空 frontmatter targets > 变体 Agent 推断 > 原有主机路径推断。空的 Source 覆盖表示所有 Target；显式 Target 声明保留原有匹配规则。`.skillignore` 和 include/exclude 仍按物理路径生效。未知 Agent、`shared`/`default` 及 tracked repo 外的布局保留普通发现行为，不推断回退。

`status` 和 dashboard 的 Source/仓库总数按逻辑 Skill 计数：两个 Skill 的四份文件显示为 `2 skills, 4 variants`。JSON 的 `skill_count`/`skillCount` 是逻辑数量；`variant_count`/`variantCount` 及 Target API 的 `sourceVariantCount` 是启用的物理文件总数，也包含普通 Skill。资源列表、操作 ID、Sync 结果和上下文成本仍按物理文件计算。

**symlink：** 整个目录暴露给 Target，无法按变体路由；需要选择 Agent 时请使用 merge/copy。共用目标目录的 Target 也无法隔离不同选择，原有路径冲突提示仍适用。Source 文件保持不变；现有 prefixed copy 命名仍可能改写副本的 `name:`。此功能不执行 sparse checkout。

升级后，merge/copy 会移除已管理但 Agent 不匹配的目标变体，并通过 CLI/服务器警告说明自动路由原因（dry-run 也会提示）。用 Source 的 Target 覆盖可恢复所需副本；空覆盖表示所有 Target。Source 文件保持完整。Target 栏显示有效路由（包括推断），不代表生产者声明了该字段。

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
