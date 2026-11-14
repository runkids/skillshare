---
sidebar_position: 2
---

# 创建 Skill

从想法到发布 Skill 的完整流程。

:::tip
想控制哪些 Target 会收到你的 Skill？参见 [Filtering Skills](/docs/how-to/daily-tasks/filtering-skills)。
:::

## 概览

```mermaid
flowchart LR
    IDEA["IDEA"] --> CREATE["CREATE"] --> WRITE["WRITE"] --> TEST["TEST"] --> PUBLISH["PUBLISH"]
```

---

## 步骤 1：创建 Skill

```bash
skillshare new my-skill
```

这会创建：
```
~/.config/skillshare/skills/my-skill/
└── SKILL.md  (with template)
```

---

## 步骤 2：编写 Skill

编辑生成的 `SKILL.md`：

```bash
$EDITOR ~/.config/skillshare/skills/my-skill/SKILL.md
```

### 基本结构

```markdown
---
name: my-skill
description: Brief description (shown in skill lists)
---

# My Skill

What this skill does and when to use it.

## Instructions

1. Step one
2. Step two
3. Step three
```

### 编写 Skill 的小技巧

**要具体：**
```markdown
# Good
When the user asks to review code, analyze for:
- Bugs and potential issues
- Style consistency
- Performance concerns

# Bad
Review the code and make it better.
```

**包含示例：**
````markdown
## Example

User: "Review this function"
```python
def add(a, b):
    return a + b
```

Response: Suggest adding type hints...
````

**说明何时不应使用：**
```markdown
## When NOT to Use

- Don't use for simple syntax questions
- Don't use for explaining code (use explain-code skill instead)
```

---

## 步骤 3：部署与测试

### 部署到所有 Target

```bash
skillshare sync
```

### 在你的 AI CLI 中测试

尝试使用这个 Skill：
- 显式调用：`/skill:my-skill`
- 或直接描述任务，看 AI 是否会自动选用

### 迭代

编辑 → sync → 测试，直到效果满意为止。

---

## 步骤 4：发布（可选）

### 与团队共享

推送到你的 git remote：
```bash
skillshare push -m "Add my-skill"
```

团队成员可以拉取：
```bash
skillshare pull
```

### 公开分享

1. 为你的 Skill 创建一个 GitHub 仓库
2. 推送你的 Skill 目录
3. 其他人可以安装：
   ```bash
   skillshare install github.com/you/my-skills/my-skill
   ```

---

## Skill 模板

### 简单 Skill

```markdown
---
name: simple-skill
description: Does one thing well
---

# Simple Skill

When the user asks to do X, follow these steps:

1. First, do Y
2. Then, do Z
3. Finally, confirm completion
```

### 任务导向型 Skill

```markdown
---
name: code-review
description: Reviews code for quality and issues
---

# Code Review

You are a code reviewer. Analyze code for quality issues.

## What to Check

- Bugs and edge cases
- Performance issues
- Security vulnerabilities
- Code style and readability

## Output Format

For each issue found:
1. **Location**: File and line
2. **Severity**: High/Medium/Low
3. **Issue**: What's wrong
4. **Fix**: Suggested solution

## Example

[Include an example input and expected output]
```

### 特定 Target 的 Skill

```markdown
---
name: claude-prompts
description: Prompt patterns specific to Claude Code
targets: [claude]
---

# Claude Prompts

Patterns that work best with Claude Code's capabilities.

## When to Use

Use when crafting prompts for Claude Code specifically.
```

设置了 `targets` 后，该 Skill 只会同步到列出的 Target ——其他 Target 不会收到它。省略 `targets` 则同步到所有地方。

### 流程型 Skill

```markdown
---
name: git-workflow
description: Guides through git commit workflow
---

# Git Workflow

Guide the user through proper git commit practices.

## Steps

1. **Check status**: Run `git status`
2. **Review changes**: Run `git diff`
3. **Stage files**: Add specific files, not `git add .`
4. **Write message**: Follow conventional commits
5. **Commit**: Create the commit
6. **Verify**: Run `git log -1`

## Commit Message Format

```text
type(scope): description

[optional body]
```

Types: feat, fix, docs, style, refactor, test, chore

---

## 进阶主题

### Skill 中的多个文件

一个 Skill 可以包含额外的文件：

```
my-skill/
├── SKILL.md
├── examples/
│   └── sample.py
└── templates/
    └── component.tsx
```

在 SKILL.md 中引用它们：
```markdown
See the example in `examples/sample.py` for reference.
```

### 为团队加上命名空间

用带命名空间的名称避免冲突：

```yaml
name: acme-code-review
```

### 版本追踪

添加版本元数据：

```yaml
---
name: my-skill
description: My skill
metadata:
  version: 1.0.0
  author: Your Name
---
```

### License 元数据

添加 `license` 字段，让用户在安装前就能看到 license 信息：

```yaml
---
name: my-skill
description: My reusable skill
license: MIT
---
```

设置该字段后，`skillshare install` 会在选择提示和确认画面中显示 license。这有助于企业用户做出合规决策。详见 [Skill Format](/docs/understand/skill-format#license)。

### 用 .skillignore 控制发现范围

发布一个包含多个 Skill 的仓库时，你可能有一些内部工具或开发中的 Skill 不想让用户发现。在仓库根目录创建一个 `.skillignore` 文件：

```text title=".skillignore"
# Internal tooling
validation-scripts
scaffold-template

# Exclude an entire group directory
internal-tools/

# Work in progress
prompt-eval-*

# Ignore temp at any depth
**/temp

# Exclude tests but keep test-critical
test-*
!test-critical
```

`.skillignore` 使用 [gitignore 语法](https://git-scm.com/docs/gitignore)——支持 `*`、`**`、`?`、`[abc]`、`!negation`、`/anchored`、`pattern/`（仅目录）以及 `\#`/`\!` 转义。像 `internal-tools` 这样的组名会排除该目录下**所有** Skill。使用更精确的路径如 `internal-tools/helper` 则只排除该组内的特定 Skill。

匹配这些规则的 Skill 不会出现在 `skillshare install <repo>` 的发现列表中。这是在服务端（仓库内）生效的，因此所有用户都会自动受益。真实案例参见 [`runkids/my-skills`](https://github.com/runkids/my-skills)，用户侧的排除方式参见 [install --exclude](/docs/reference/commands/install#excluding-skills)。

### Source root 的 .skillignore（本地）

你也可以在 **Source root**（`~/.config/skillshare/skills/.skillignore`）放置一个 `.skillignore`，从全局隐藏所有命令中的某些 Skill——包括 `doctor`、`status`、`list`、`sync`、`audit`、`diff` 和 `check`：

```text title="~/.config/skillshare/skills/.skillignore"
# Temporarily mute a skill without uninstalling
my-experimental-skill

# Exclude all draft skills
[Dd]raft*

# Hide an entire tracked repo
_archived-team-skills

# Ignore vendored deps at any depth
**/node_modules
*.venv
```

两个层级会同时生效：Source root 的规则影响所有 Skill（tracked 和非 tracked），而仓库层级的规则只影响该仓库自身的 Skill。只要任一层级匹配，该 Skill 就会被排除。

### .skillignore.local（个人覆盖）

如果共享仓库的 `.skillignore` 屏蔽了你本地需要的某个 Skill，可以在同一目录下创建 `.skillignore.local`。它的规则会附加在 `.skillignore` 之后，因此 `!pattern` 这类否定规则会覆盖基础文件：

```text title="_team-skills/.skillignore.local"
# The repo ignores private-*, but I need my own
!private-mine
```

这个文件**不应**被提交——请加入 `.gitignore`。它在 Source root 和仓库层级都可以使用。

---

## 检查清单

发布前：

- [ ] 名称清晰、具体
- [ ] 描述说明了用途
- [ ] 指令可执行
- [ ] 包含示例
- [ ] 已在 AI CLI 中测试
- [ ] 与现有 Skill 无冲突

---

## 另请参阅

- [new](/docs/reference/commands/new) — 使用模板创建 Skill
- [Skill Format](/docs/understand/skill-format) — SKILL.md 结构
- [Skill Design](/docs/understand/philosophy/skill-design) — 复杂度分级、确定性、CLI 封装模式
- [Best Practices](./best-practices.md) — 命名与组织方式
- [Organizing Skills](./organizing-skills.md) — 文件夹结构
