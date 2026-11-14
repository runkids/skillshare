---
sidebar_position: 2
---

# 建立 Skills

從想法到發布 Skill 的完整流程。

:::tip
想控制哪些 Targets 會收到你的 Skill？請參閱 [篩選 Skills](/docs/how-to/daily-tasks/filtering-skills)。
:::

## 總覽

```mermaid
flowchart LR
    IDEA["構想"] --> CREATE["建立"] --> WRITE["撰寫"] --> TEST["測試"] --> PUBLISH["發布"]
```

---

## 步驟 1：建立 Skill

```bash
skillshare new my-skill
```

這會建立：
```
~/.config/skillshare/skills/my-skill/
└── SKILL.md  (含範本)
```

---

## 步驟 2：撰寫 Skill

編輯產生的 `SKILL.md`：

```bash
$EDITOR ~/.config/skillshare/skills/my-skill/SKILL.md
```

### 基本結構

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

### 撰寫優質 Skill 的技巧

**具體明確：**
```markdown
# Good
When the user asks to review code, analyze for:
- Bugs and potential issues
- Style consistency
- Performance concerns

# Bad
Review the code and make it better.
```

**包含範例：**
````markdown
## Example

User: "Review this function"
```python
def add(a, b):
    return a + b
```

Response: Suggest adding type hints...
````

**說明何時不該使用：**
```markdown
## When NOT to Use

- Don't use for simple syntax questions
- Don't use for explaining code (use explain-code skill instead)
```

---

## 步驟 3：部署與測試

### 部署到所有 Targets

```bash
skillshare sync
```

### 在你的 AI CLI 中測試

嘗試使用該 Skill：
- 明確呼叫：`/skill:my-skill`
- 或描述任務，看看 AI 是否會自動採用它

### 反覆調整

編輯 → 同步 → 測試，直到效果良好為止。

---

## 步驟 4：發布（選用）

### 與團隊分享

推送到你的 git remote：
```bash
skillshare push -m "Add my-skill"
```

團隊成員可以拉取：
```bash
skillshare pull
```

### 公開分享

1. 建立一個 GitHub repo 存放你的 Skills
2. 推送你的 Skills 目錄
3. 其他人可以安裝：
   ```bash
   skillshare install github.com/you/my-skills/my-skill
   ```

---

## Skill 範本

### 簡單的 Skill

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

### 任務導向的 Skill

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

當設定了 `targets` 時，該 Skill 只會同步到符合的 Targets — 其他 Targets 不會收到它。省略 `targets` 則會同步到所有地方。

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

## 進階主題

### Skill 中的多個檔案

一個 Skill 可以包含額外的檔案：

```
my-skill/
├── SKILL.md
├── examples/
│   └── sample.py
└── templates/
    └── component.tsx
```

在你的 SKILL.md 中參照它們：
```markdown
See the example in `examples/sample.py` for reference.
```

### 為團隊建立命名空間

使用具命名空間的名稱以避免衝突：

```yaml
name: acme-code-review
```

### 版本追蹤

加入版本 metadata：

```yaml
---
name: my-skill
description: My skill
metadata:
  version: 1.0.0
  author: Your Name
---
```

### License metadata

加入 `license` 欄位，讓使用者在安裝前就能看到授權資訊：

```yaml
---
name: my-skill
description: My reusable skill
license: MIT
---
```

設定後，`skillshare install` 會在選擇提示與確認畫面中顯示授權資訊。這有助於企業使用者做出符合合規性的判斷。詳情請參閱 [Skill Format](/docs/understand/skill-format#license)。

### 使用 .skillignore 控制探索範圍

當你發布一個包含多個 Skills 的 repository 時，可能會有內部工具或開發中的 Skills，不希望使用者探索到。請在 repo 根目錄建立一個 `.skillignore` 檔案：

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

`.skillignore` 使用 [gitignore 語法](https://git-scm.com/docs/gitignore) — 支援 `*`、`**`、`?`、`[abc]`、`!negation`、`/anchored`、`pattern/`（僅限目錄），以及 `\#`／`\!` 跳脫字元。像 `internal-tools` 這樣的群組名稱，會排除該目錄底下**所有**的 Skills。若要只排除群組內特定的某個 Skill，請使用精確路徑，例如 `internal-tools/helper`。

符合這些 pattern 的 Skills 不會出現在 `skillshare install <repo>` 的探索結果中。這是在伺服器端（repo 內）套用的，因此所有使用者都能自動受益。實際範例請參閱 [`runkids/my-skills`](https://github.com/runkids/my-skills)，使用者端的排除方式請參閱 [install --exclude](/docs/reference/commands/install#excluding-skills)。

### Source 根目錄的 .skillignore（本機）

你也可以在你的 **Source 根目錄**（`~/.config/skillshare/skills/.skillignore`）放置一個 `.skillignore`，以全域方式從所有指令中隱藏 Skills — 包含 `doctor`、`status`、`list`、`sync`、`audit`、`diff` 與 `check`：

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

兩層規則都會生效：Source 根目錄的 pattern 會影響所有 Skills（tracked 與非 tracked），而 repo 層級的 pattern 只會影響該 repo 的 Skills。只要任一層符合，該 Skill 就會被排除。

### .skillignore.local（個人覆寫）

如果共用 repo 的 `.skillignore` 封鎖了你本機需要的某個 Skill，可以在同一目錄下建立一個 `.skillignore.local`。它的 pattern 會附加在 `.skillignore` 之後，因此 `!pattern` 的否定規則能覆寫基礎檔案：

```text title="_team-skills/.skillignore.local"
# The repo ignores private-*, but I need my own
!private-mine
```

這個檔案**不應該**被 commit — 請將它加入 `.gitignore`。它在 Source 根目錄與 repo 層級都適用。

---

## 檢查清單

發布之前：

- [ ] 清楚、具體的名稱
- [ ] 說明解釋了用途
- [ ] 指示是可執行的
- [ ] 包含範例
- [ ] 已在 AI CLI 中測試過
- [ ] 與既有 Skills 沒有衝突

---

## 另請參閱

- [new](/docs/reference/commands/new) — 使用範本建立 Skill
- [Skill Format](/docs/understand/skill-format) — SKILL.md 結構
- [Skill Design](/docs/understand/philosophy/skill-design) — 複雜度層級、確定性、CLI wrapper pattern
- [最佳實務](./best-practices.md) — 命名與組織方式
- [Organizing Skills](./organizing-skills.md) — 資料夾結構
