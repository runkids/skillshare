---
sidebar_position: 3
---

# check

檢查 tracked 儲存庫與已安裝 skills 是否有可用更新，但不套用任何變更。

```bash
skillshare check                      # 檢查所有儲存庫與 skills
skillshare check my-skill             # 檢查單一 skill
skillshare check a b c                # 檢查多個 skills
skillshare check --group frontend     # 檢查 frontend/ 下所有 skills
skillshare check x -G backend         # 混用名稱與群組
skillshare check --json               # 機器可讀輸出
```

## 使用時機

### 更新前

在執行 `update` 之前先預覽會有什麼變更：

```bash
skillshare check         # 查看有哪些更新
skillshare update --all  # 套用更新
skillshare sync          # 分發變更
```

### CI/CD Pipeline

在 CI 中檢查過期的 skills：

```bash
result=$(skillshare check --json)
# 解析 JSON 以偵測過期的 skills
```

## 執行內容

`check` 會檢查你的 source 目錄，並回報以下項目的更新狀態：

1. **Tracked 儲存庫** — 從 origin fetch，顯示落後多少個 commits
2. **已安裝的 skills（含 metadata）** — 比較已安裝版本與遠端 HEAD
3. **過期的 skills** — 偵測上游儲存庫中已刪除的子目錄
4. **從本機路徑安裝的 skills** — 將安裝來源路徑中的檔案與安裝時記錄的檔案比對（參見[從本機路徑安裝](#local-path-installs)）
5. **本機 skills** — 將沒有安裝 metadata 的 skill 標記為「local source」（沒有可比對的對象）
6. **Skill 層級的 `targets` 驗證** — 對 SKILL.md `targets` frontmatter 欄位中未知的 target 名稱發出警告

與 `update` 不同，`check` 絕不會修改任何檔案。

## 從本機路徑安裝 {#local-path-installs}

從磁碟目錄安裝的 skill（`skillshare install /path/to/skill`）會記錄該路徑，以及所複製每個檔案的 hash。`check` 會重新計算該路徑下檔案的 hash，並回報：

- **up to date** — 檔案與安裝時一致
- **update available** — 來源路徑中有檔案被修改、新增或刪除
- **error** — 來源路徑已不存在（`local source not found: <path>`）

這適用於由其他應用程式發布和更新的 skill，例如 App bundle 裡的 skill：

```bash
skillshare install /Applications/Surge.app/Contents/Resources/Skills/surge

# App 更新後：
skillshare check surge     # → Update available
skillshare update surge    # 從該路徑重新複製，並執行安全稽核
skillshare sync
```

在記錄檔案 hash 之前安裝的 skill，會維持「local source」狀態，直到執行 update 或重新安裝。在 project mode 中，透過相對路徑（如 `./vendor/my-skill`）安裝的 skill 不會比對。

## 輸出範例

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

## 檢查特定 Skills

你可以指定一個或多個 skill 名稱來檢查，而不用掃描全部：

```bash
skillshare check my-skill                # 單一 skill
skillshare check skill-a skill-b         # 多個 skills
```

使用 `--group` / `-G` 檢查群組目錄下所有可更新的 skills：

```bash
skillshare check --group frontend        # frontend/ 下所有 skills
skillshare check -G frontend -G backend  # 多個群組
skillshare check my-skill -G frontend    # 混用名稱與群組
```

若某個位置參數對應到一個群組目錄（而非儲存庫或 skill 本身），會自動展開：

```bash
skillshare check frontend               # 自動偵測為群組
```

沒有 metadata 的本機 skills 在展開群組時會被跳過。

## 選項

| Flag | 說明 |
|------|-------------|
| `--group`, `-G` `<name>` | 檢查群組中所有可更新的 skills（可重複指定） |
| `--project`, `-p` | 檢查 project 層級的 skills（`.skillshare/`） |
| `--global`, `-g` | 檢查 global skills（`~/.config/skillshare`） |
| `--json` | 以 JSON 輸出（供 scripting/CI 使用） |
| `--help`, `-h` | 顯示說明 |

:::tip 自動偵測
若未指定 `--project` 或 `--global`，skillshare 會自動偵測：若目前目錄存在 `.skillshare/config.yaml`，預設使用 project 模式；否則使用 global 模式。
:::

## JSON 輸出

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

`"status": "error"` 的 skill 在已知原因時會包含 `message` 欄位，例如 `"message": "local source not found: /path/to/skill"`。

## 狀態圖示

| 圖示 | 意義 |
|------|---------|
| `✓` | 已是最新 |
| `⬇` | 有可用更新（tracked 儲存庫：落後的 commits 數；skill：有新版本） |
| `⚠` | 過期 — 上游子目錄已刪除或重新命名 |
| `!` | 有未提交的變更 |
| `•` | 本機來源（沒有安裝 metadata 可比對） |

:::info 過期的 skills
當某個 skill 的子目錄在上游被重新命名或刪除，`check` 會回報為 **stale（過期）**。使用 `update --prune` 清除過期的 skills。
:::

:::tip Monorepo 感知
對於從子目錄安裝的 skills，`check` 只有在該特定目錄有變更時才回報「update available」— 而不是儲存庫其他不相關部分有新 commits 時。
:::

## Project 模式

```bash
skillshare check -p                    # 檢查所有 project skills
skillshare check -p my-skill           # 檢查特定 project skill
skillshare check -p --group frontend   # 檢查 project 群組
skillshare check -p --json             # project 的 JSON 輸出
```

## Agent 支援

`skillshare check agents` 會將檢查範圍限定於 agents，回報 agents source 目錄中 `.md` 檔案的漂移與更新狀態：

```bash
skillshare check agents              # 檢查所有 agents
skillshare check agents --json       # agents 的 JSON 輸出
skillshare check agents -p           # 檢查 project agents
```

若不加 `agents` 參數，`check` 僅針對 skills 運作（預設行為）。詳見 [Agents](/docs/understand/agents) 了解背景概念。

## 另請參閱

- [update](/docs/reference/commands/update) — 套用更新
- [list](/docs/reference/commands/list) — 檢視已安裝的 skills
- [status](/docs/reference/commands/status) — 顯示同步狀態
- [Agents](/docs/understand/agents) — Agent 概念
