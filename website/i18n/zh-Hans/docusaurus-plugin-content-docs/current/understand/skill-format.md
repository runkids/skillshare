---
sidebar_position: 5
---

# Skill Format

skillshare Skill 的结构与元数据。

:::tip 什么情况下需要关心这个？
SKILL.md 的格式决定了 AI CLI 如何发现和加载你的 Skill。`description` 字段尤为关键 —— AI 正是依据它来判断何时激活你的 Skill。
:::

## 概览

一个 Skill 是一个至少包含 `SKILL.md` 文件的目录：

```
my-skill/
└── SKILL.md
```

`SKILL.md` 文件由两部分组成：
1. **YAML frontmatter** —— 元数据
2. **Markdown 正文** —— 给 AI 的指令

---

## 基本结构

```markdown
---
name: my-skill
description: Brief description of what this skill does
---

# My Skill

Instructions for the agent when this skill is activated.

## When to Use

Describe when this skill should be used.

## Instructions

1. First step
2. Second step
3. Additional steps as needed
```

---

## 必填字段

### `name`

Skill 标识符，用于：
- 调用该 Skill（例如 `/skill:my-skill`）
- 冲突检测
- 在 Skill 列表中显示

```yaml
name: my-skill
```

**规则：**
- 只能使用小写字母、数字、连字符、下划线
- 必须以字母或数字开头
- 在所有 Skill 中应保持唯一

**示例：**
```yaml
name: code-review
name: pdf-tools
name: acme-frontend-ui  # Namespaced for teams
```

---

## 可选字段

### `description`

在 Skill 列表和搜索结果中显示的简短描述。

```yaml
description: Reviews code for bugs, style issues, and improvements
```

---

## 可选字段

### `tags`

用于在 Hub 索引中进行过滤和分组的分类标签。运行 `skillshare hub index` 时，SKILL.md frontmatter 中的 tags 会被包含进生成的 `skillshare-hub.json` 中。

```yaml
tags: git, workflow
```

Tags 同样可被搜索 —— `skillshare search workflow --hub ...` 会匹配标记为 "workflow" 的 Skill。

### `targets`

限制此 Skill 同步到哪些 Target。省略时，该 Skill 会同步到**所有** Target。

支持两种放置方式 —— 放在 `metadata:` 下（推荐）或放在顶层：

```yaml
# Recommended: under metadata
metadata:
  targets: [claude, cursor]

# Legacy: top-level (still fully supported)
targets: [claude, cursor]
```

:::info 优先级规则
如果两者同时存在，`metadata.targets` 优先于顶层的 `targets`。这样你可以逐步迁移 —— 添加 `metadata:` 不会与遗留的顶层字段产生冲突。
:::

| 取值 | 行为 |
|-------|----------|
| *(省略)* | 同步到所有 Target（默认） |
| `[claude]` | 仅同步到名称匹配 "claude" 的 Target |
| `[claude, cursor]` | 同步到名称匹配任一项的 Target |

**跨模式匹配：** 声明 `targets: [claude]` 的 Skill 也会匹配 Project mode 下的 `claude` Target，因为两者指向同一个 AI CLI。匹配依据 [target registry](/docs/reference/targets/supported-targets)。

**与配置过滤器的交互：** Skill 级别的 `targets` 会在配置级别的 `include`/`exclude` **之后**应用。两者都通过，该 Skill 才会被同步。参见 [Configuration](/docs/reference/targets/configuration#skill-level-targets)。

**示例 —— 仅限 Claude 的 Skill：**

```markdown
---
name: claude-prompts
description: Prompt patterns for Claude Code
metadata:
  targets: [claude]
---

# Claude Prompts
...
```

即使你同时配置了 Pi、Codex 等其他 Target，该 Skill 也只会出现在 Claude Code 的 Skill 目录中。

### `pattern`

该 Skill 所使用的结构设计模式。由 `skillshare new -P <pattern>` 自动生成。

```yaml
pattern: reviewer
```

可用模式：`tool-wrapper`、`generator`、`reviewer`、`inversion`、`pipeline`。各模式详情参见 [Skill Design Patterns](/docs/understand/philosophy/skill-design-patterns)。

### `category`

该 Skill 的用例分类。在 `skillshare new` 交互过程中设置，或完全省略。

```yaml
category: quality
```

可用分类：`library`、`verification`、`data`、`automation`、`scaffold`、`quality`、`cicd`、`runbook`、`infra`。

### `license`

该 Skill 的许可证标识符。安装时会显示，帮助进行合规决策。

```yaml
license: MIT
```

当存在该字段时，`skillshare install` 会在 Skill 选择提示和确认界面中显示许可证：

- **单个 Skill**：在 Skill 信息框中显示为 `License: MIT`
- **多 Skill 仓库**：附加在 Skill 名称后（例如 `my-skill (MIT)`）

此字段仅作信息展示 —— 不会阻止安装。常见取值：`MIT`、`Apache-2.0`、`GPL-3.0`、`BSD-3-Clause`、`ISC`。

---

## `metadata` 区块

`metadata:` 区块是一个用于部署和行为相关字段的结构化 YAML 对象。这与 30 多种 AI CLI 工具所遵循的 [Agent Skills ecosystem convention](https://developers.googleblog.com/en/5-agent-skill-design-patterns-every-adk-developer-should-know/) 保持一致。

```yaml
---
name: my-skill
description: My custom skill
metadata:
  targets: [claude]
  pattern: reviewer
  domain: python
---
```

目前，`targets` 是 skillshare 唯一会处理的 `metadata` 字段。其他字段（如 `pattern`、`domain`、`interaction`）会被保留在 frontmatter 中，但 skillshare 不会使用它们 —— 它们可能会被生态系统中的其他工具消费。

为向后兼容，skillshare 也会读取顶层的 `targets` 字段。如果两者同时存在，`metadata.targets` 优先。

## 自定义字段

你可以添加任意自定义的顶层字段：

```yaml
---
name: my-skill
description: My custom skill
author: Your Name
version: 1.0.0
---
```

自定义的顶层字段会被存储在 frontmatter 中，但 skillshare 本身不会使用它们。

---

## Markdown 正文

正文包含给 AI 的指令。请像指导一位人类助理一样撰写。

**良好实践：**
- 清晰、具体的指令
- 提供输入与预期输出的示例
- 边界情况与错误处理
- 何时使用（以及何时不应使用）

**示例：**
```markdown
# Code Review

You are a code reviewer. Analyze code for:
- Bugs and potential issues
- Style and consistency
- Performance concerns
- Security vulnerabilities

## When to Use

Use this skill when the user asks you to review code, find bugs, or improve code quality.

## Instructions

1. Read the provided code carefully
2. Identify issues in order of severity
3. Suggest specific improvements with code examples
4. Be constructive and explain your reasoning

## Example

User: "Review this function"
```python
def add(a, b):
  return a + b
```

Response: "The function looks correct but could benefit from type hints..."
```

---

## 集中式元数据

当你安装一个 Skill 时，skillshare 会将其元数据记录在 `.metadata.json` 中（所有 Skill 集中管理）：

```json
{
  "skills": [
    {
      "name": "pdf",
      "source": "anthropics/skills/skills/pdf",
      "type": "github",
      "installed_at": "2026-01-20T15:30:00Z",
      "repo_url": "https://github.com/anthropics/skills.git",
      "subdir": "skills/pdf",
      "version": "abc1234"
    }
  ]
}
```

每个 Skill 条目包含：

| 字段 | 说明 |
|-------|------|
| `name` | Skill 目录名 |
| `source` | 原始安装来源输入 |
| `type` | 来源类型（`github`、`local` 等） |
| `installed_at` | 安装时间戳 |
| `repo_url` | Git 克隆 URL（仅 Git 来源） |
| `subdir` | 子目录路径（仅 monorepo 来源） |
| `version` | 安装时的 Git commit hash |

`skillshare update` 和 `skillshare check` 会使用这些信息来确定从何处获取更新。

**请勿手动编辑此文件。**

---

## 创建 Skill

```bash
skillshare new my-skill
```

这会创建：
```
~/.config/skillshare/skills/my-skill/
└── SKILL.md  (with template)
```

编辑生成的 `SKILL.md`，然后运行 `skillshare sync` 进行部署。

---

## 校验 Skill

```bash
skillshare doctor
```

会检查：
- SKILL.md 格式是否有效
- 是否包含必填的 `name` 字段
- frontmatter YAML 是否有效
- 是否存在名称冲突

---

## 参见

- [new](/docs/reference/commands/new) — 使用正确的模板创建 Skill
- [Creating Skills](/docs/how-to/daily-tasks/creating-skills) — 编写 Skill 的完整指南
- [Best Practices](/docs/how-to/daily-tasks/best-practices) — 命名与组织建议
