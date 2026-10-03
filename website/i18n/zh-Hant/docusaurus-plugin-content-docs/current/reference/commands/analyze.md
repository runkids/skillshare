---
sidebar_position: 4
---

# analyze

分析每個 target 的 skill 在 context window 用量與 skill 品質。

```bash
skillshare analyze                    # 互動式 TUI（預設）
skillshare analyze claude             # 單一 target 的詳細資訊
skillshare analyze --verbose          # 最大的前 10 個 description
skillshare analyze --json             # 機器可讀輸出
skillshare analyze -p                 # Project mode
```

## When to Use

### Optimize Context Budget

找出哪些 skill 消耗最多 context window token：

```bash
skillshare analyze           # 互動式瀏覽所有 target
```

### Compare Across Targets

查看不同 target 之間 context 用量的差異（例如 Claude vs Cursor）：

```bash
skillshare analyze           # 在 TUI 中按 Tab 切換 target
```

### Check Skill Quality

找出欄位缺漏、description 過短，或沒有觸發詞句的 skill：

```bash
skillshare analyze           # Lint 圖示（✗/⚠）會顯示在 TUI 中
```

### CI/Scripting

取得機器可讀的 context 指標與 lint 結果：

```bash
skillshare analyze --json | jq '.targets[].always_loaded.estimated_tokens'
skillshare analyze --json | jq '.targets[].skills[] | select(.lint_issues | length > 0)'
```

## What It Does

`analyze` 會為每個 skill 計算兩層 context 成本：

1. **一律載入（Always loaded）** — 來自 SKILL.md frontmatter 的 `name + description`（每次請求都會載入 context，用於 skill 比對）
2. **依需求載入（On-demand）** — frontmatter 之後的 skill 內文（只有在該 skill 被觸發時才會載入）

token 估算採用 `chars / 4` 作為近似值。

### Skill Quality Lint

除了 token 分析之外，`analyze` 還會對每個 skill 執行內建的 lint 引擎。Lint 規則會檢查 SKILL.md 的結構與 description 品質，並直接在 TUI 與 JSON 輸出中呈現問題。

| Rule | Severity | What it checks |
|------|----------|----------------|
| `missing-name` | error | `name` 欄位為空或缺漏 |
| `missing-description` | error | `description` 欄位為空或缺漏 |
| `empty-body` | error | Skill 內文（frontmatter 之後）為空 |
| `description-too-short` | warning | Description 少於 50 個字元 |
| `description-too-long` | warning | Description 超過 1024 個字元的目標上限 |
| `description-near-limit` | warning | Description 介於 900–1024 個字元之間 |
| `no-trigger-phrase` | warning | Description 缺少觸發詞句（例如「Use when…」） |

在 TUI 中，有 lint 問題的 skill 名稱旁會顯示 ✗（error）或 ⚠（warning）圖示。詳細資訊面板包含一個 **Quality** 區塊，列出所有發現的問題。

## Interactive TUI

在 TTY 中，`analyze` 會開啟互動式畫面，一次看一個 target：左側是依 token 用量排序的 skills，右側是選取 skill 的 token 明細、lint 問題與描述。打開 skill 會顯示檔案原文（含 front matter），可以直接對照 lint 問題。按鍵列在畫面底部。使用 `--no-tui` 改印純文字。

### Color Coding

token 消耗程度依每個 target 動態計算的百分位門檻分類：

| Color | Meaning |
|-------|---------|
| 🔴 Red | P75 以上（消耗量前 25%） |
| 🟡 Yellow | P25–P75（中間 50%） |
| 🟢 Green | P25 以下（最低 25%） |

## Example Output

### Default (--no-tui)

```
claude · 7 skills
  Always loaded  ~362 tokens
  On-demand max  ~22 tokens

  Add -v for the 10 largest descriptions
```

### Verbose

```
skillshare analyze --verbose

claude · 7 skills
  Always loaded  ~362 tokens
  On-demand max  ~22 tokens

  Largest descriptions
    my-big-skill   ~180 tokens
    another-skill  ~120 tokens
    …
```

### Single Target

傳入 target 名稱會自動啟用 verbose 輸出：

```bash
skillshare analyze claude
```

### Filter by Group

```bash
# 查看所有 frontend skill 的總 token 成本
skillshare analyze claude --json --filter frontend

# 預先填入 TUI 中的搜尋框
skillshare analyze --filter marketing
```

## Options

| Flag | Description |
|------|-------------|
| `[target]` | 顯示單一 target 的詳細資訊（會自動啟用 verbose） |
| `--verbose`, `-v` | 顯示每個 target 最大的前 10 個 description |
| `--no-tui` | 停用互動式 TUI，輸出純文字 |
| `--project`, `-p` | 分析 project 層級的 skill（`.skillshare/`） |
| `--global`, `-g` | 分析 global 的 skill（`~/.config/skillshare`） |
| `--filter <text>` | 依名稱／路徑的子字串篩選 skill |
| `--json` | 以 JSON 輸出（用於腳本／CI） |
| `--help`, `-h` | 顯示說明 |

:::tip Auto-detection
若未指定 `--project` 或 `--global`，skillshare 會自動偵測：如果目前目錄存在 `.skillshare/config.yaml`，就預設為 project mode；否則為 global mode。
:::

## JSON Output

```bash
skillshare analyze --json
```

```json
{
  "targets": [
    {
      "name": "claude",
      "skill_count": 7,
      "always_loaded": {
        "chars": 1448,
        "estimated_tokens": 362
      },
      "on_demand_max": {
        "chars": 88,
        "estimated_tokens": 22
      },
      "skills": [
        {
          "name": "my-skill",
          "description_chars": 180,
          "description_tokens": 45,
          "body_chars": 400,
          "body_tokens": 100,
          "lint_issues": [
            {
              "rule": "no-trigger-phrase",
              "severity": "warning",
              "category": "format",
              "message": "Description lacks trigger phrases (e.g. 'Use when...'); agents may not know when to invoke this skill"
            }
          ]
        }
      ]
    }
  ]
}
```

沒有 lint 問題的 skill 會省略 `lint_issues` 欄位。

## Project Mode

```bash
skillshare analyze -p                  # project skill 的互動式 TUI
skillshare analyze -p --verbose        # Verbose 文字輸出
skillshare analyze -p claude           # 單一 target 的詳細資訊
skillshare analyze -p --json           # JSON 輸出
```

## Filtering

使用 `--filter` 可以把結果縮小到一部分的 skill。篩選會對 skill 的相對路徑（包含群組目錄）執行不區分大小寫的子字串比對。

舉例來說，如果你用 `--into frontend` 安裝了 skill：
- `--filter frontend` 會比對到 `frontend/` 群組中的所有 skill
- `--filter react` 會比對到路徑中含有「react」的任何 skill

在 TUI 模式中，`--filter` 會預先填入篩選輸入框。你也可以使用 `/` 鍵開始互動式篩選。

在 JSON 模式中，輸出會包含帶有匯總 token 計數的 `filtered_summary`：

```json
{
  "filter": "frontend",
  "matched_count": 5,
  "total_count": 50,
  "filtered_summary": {
    "always_loaded": { "chars": 2400, "tokens": 600 },
    "on_demand": { "chars": 8000, "tokens": 2000 },
    "total": { "chars": 10400, "tokens": 2600 }
  },
  "skills": [...]
}
```

當搜尋或篩選啟用時，Web UI 也會顯示動態的 token 摘要列。

## Budget Warnings

當設定了 `context_budget` 門檻時，只要任何 target 超過該預算，`analyze` 就會顯示警告。設定細節請參閱 [sync — Context Cost](/docs/reference/commands/sync#context-cost)。

## See Also

- [list](/docs/reference/commands/list) — 查看已安裝的 skill
- [audit](/docs/reference/commands/audit) — 掃描 skill 的安全威脅
- [tui](/docs/reference/commands/tui) — 開關互動式 TUI
