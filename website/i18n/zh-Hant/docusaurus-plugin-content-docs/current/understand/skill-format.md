---
sidebar_position: 5
---

# Skill Format

skillshare skill 的結構與 metadata。

:::tip 這什麼時候重要？
SKILL.md 格式決定了 AI CLI 如何發現並載入你的 skill。`description` 欄位特別關鍵 — 這是 AI 用來判斷何時啟用你的 skill 的依據。
:::

## 概觀

一個 skill 是包含至少一個 `SKILL.md` 檔案的目錄：

```
my-skill/
└── SKILL.md
```

`SKILL.md` 檔案有兩個部分：
1. **YAML frontmatter** — Metadata
2. **Markdown body** — 給 AI 的指示

---

## 基本結構

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

## 必要欄位

### `name`

Skill 的識別碼。用於：
- 呼叫 skill（例如 `/skill:my-skill`）
- 衝突偵測
- 在 skill 清單中顯示

```yaml
name: my-skill
```

**規則：**
- 小寫字母、數字、連字號、底線
- 必須以字母或數字開頭
- 在所有 skills 中應保持唯一

**範例：**
```yaml
name: code-review
name: pdf-tools
name: acme-frontend-ui  # Namespaced for teams
```

---

## 選用欄位

### `description`

在 skill 清單與搜尋結果中顯示的簡短描述。

```yaml
description: Reviews code for bugs, style issues, and improvements
```

---

## 選用欄位

### `tags`

用於在 hub 索引中篩選與分組的分類標籤。當你執行 `skillshare hub index` 時，SKILL.md frontmatter 中的 tags 會被納入產生的 `skillshare-hub.json`。

```yaml
tags: git, workflow
```

Tags 也是可搜尋的 — `skillshare search workflow --hub ...` 會比對標記為 "workflow" 的 skills。

### `targets`

限制此 skill 同步到哪些 targets。省略時，此 skill 會同步到**所有** targets。

支援兩種擺放方式 — 放在 `metadata:` 之下（建議）或放在頂層：

```yaml
# Recommended: under metadata
metadata:
  targets: [claude, cursor]

# Legacy: top-level (still fully supported)
targets: [claude, cursor]
```

:::info 優先順序規則
若兩者同時存在，`metadata.targets` 的優先權高於頂層的 `targets`。這讓你可以逐步遷移 — 新增 `metadata:` 不會與遺留的頂層欄位衝突。
:::

| 值 | 行為 |
|-------|----------|
| *（省略）* | 同步到所有 targets（預設值） |
| `[claude]` | 只同步到名稱符合 "claude" 的 targets |
| `[claude, cursor]` | 同步到符合任一名稱的 targets |

**跨模式比對：** 一個宣告 `targets: [claude]` 的 skill，也會比對到 project target `claude`，因為兩者指向同一個 AI CLI。比對規則使用 [target registry](/docs/reference/targets/supported-targets)。

**與 config 篩選條件的互動：** Skill 層級的 `targets` 是在 config 層級的 `include`/`exclude` **之後**套用的。兩者都必須通過，skill 才會被同步。詳見 [Configuration](/docs/reference/targets/configuration#skill-level-targets)。

**範例 — 僅限 Claude 的 skill：**

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

這個 skill 只會出現在 Claude Code 的 skill 目錄中，即使你同時設定了 Pi、Codex 等其他 targets 也一樣。

### `pattern`

此 skill 使用的結構化設計模式。由 `skillshare new -P <pattern>` 自動產生。

```yaml
pattern: reviewer
```

可用的模式：`tool-wrapper`、`generator`、`reviewer`、`inversion`、`pipeline`。詳見 [Skill Design Patterns](/docs/understand/philosophy/skill-design-patterns) 了解各模式的說明。

### `category`

此 skill 的用途分類。在 `skillshare new` 執行期間互動式設定，或完全省略。

```yaml
category: quality
```

可用的分類：`library`、`verification`、`data`、`automation`、`scaffold`、`quality`、`cicd`、`runbook`、`infra`。

### `license`

此 skill 的授權識別碼。在安裝過程中顯示，協助合規決策。

```yaml
license: MIT
```

當此欄位存在時，`skillshare install` 會在 skill 選擇提示與確認畫面中顯示授權資訊：

- **單一 skill**：在 skill 資訊框中顯示為 `License: MIT`
- **多 skill repo**：附加在 skill 名稱後方（例如 `my-skill (MIT)`）

這純粹是資訊性質 — 不會阻擋安裝。常見值：`MIT`、`Apache-2.0`、`GPL-3.0`、`BSD-3-Clause`、`ISC`。

---

## `metadata` 區塊

`metadata:` 區塊是一個結構化的 YAML 物件，用於部署與行為相關欄位。這與 30 多個 AI CLI 工具所採用的 [Agent Skills ecosystem convention](https://developers.googleblog.com/en/5-agent-skill-design-patterns-every-adk-developer-should-know/) 一致。

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

目前，`targets` 是 skillshare 唯一會處理的 `metadata` 欄位。其他欄位（如 `pattern`、`domain`、`interaction`）會保留在 frontmatter 中，但 skillshare 不會使用它們 — 它們可能會被生態系中的其他工具消費。

為了向後相容，skillshare 也會讀取頂層的 `targets` 欄位。若兩者同時存在，`metadata.targets` 優先。

## 自訂欄位

你可以新增任何自訂的頂層欄位：

```yaml
---
name: my-skill
description: My custom skill
author: Your Name
version: 1.0.0
---
```

自訂的頂層欄位會儲存在 frontmatter 中，但 skillshare 本身不會使用它們。

---

## Markdown Body

Body 部分包含給 AI 的指示。撰寫時就像在指導一位人類助理。

**良好做法：**
- 清楚、具體的指示
- 輸入與預期輸出的範例
- 邊界情況與錯誤處理
- 何時使用（以及何時不該使用）

**範例：**
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

## 集中化的 Metadata

當你安裝一個 skill 時，skillshare 會將其 metadata 記錄在 `.metadata.json` 中（集中管理所有 skills）：

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

每個 skill 項目包含：

| 欄位 | 說明 |
|-------|-------------|
| `name` | Skill 目錄名稱 |
| `source` | 原始安裝來源輸入值 |
| `type` | 來源類型（`github`、`local` 等） |
| `installed_at` | 安裝時間戳記 |
| `repo_url` | Git clone URL（僅限 git 來源） |
| `subdir` | 子目錄路徑（僅限 monorepo 來源） |
| `version` | 安裝當下的 Git commit hash |

`skillshare update` 與 `skillshare check` 會使用這些資訊來判斷從哪裡取得更新。

**請勿手動編輯此檔案。**

---

## 建立一個 Skill

```bash
skillshare new my-skill
```

這會建立：
```
~/.config/skillshare/skills/my-skill/
└── SKILL.md  (with template)
```

編輯產生出來的 `SKILL.md`，然後執行 `skillshare sync` 來部署。

---

## 驗證 Skills

```bash
skillshare doctor
```

檢查項目：
- 有效的 SKILL.md 格式
- 必要的 `name` 欄位
- 有效的 frontmatter YAML
- 名稱衝突

---

## 另請參閱

- [new](/docs/reference/commands/new) — 使用正確的範本建立一個 skill
- [Creating Skills](/docs/how-to/daily-tasks/creating-skills) — 撰寫 skill 的完整指南
- [Best Practices](/docs/how-to/daily-tasks/best-practices) — 命名與組織的技巧
