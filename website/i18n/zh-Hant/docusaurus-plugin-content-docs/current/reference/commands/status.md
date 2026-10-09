---
sidebar_position: 7
---

# status

顯示 skillshare 目前的狀態：source、tracked 儲存庫、targets 與版本資訊。

```bash
skillshare status
```

啟用 `follow_source_links: true` 時，無法使用的第一層來源連結會顯示包含連結名稱的警告，正常技能仍會列出。使用 `--json` 時，警告寫入 stderr，stdout 保持有效的 JSON。

## 使用時機

- 變更之後檢查所有 targets 是否已同步
- 查看哪些 targets 需要執行 `sync`
- 確認 tracked 儲存庫是否為最新
- 確認目前生效的 audit 政策（profile、threshold、dedupe 模式）
- 檢查 CLI 或 skill 是否有更新

## 輸出範例

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

## 各區段說明

### Source

顯示 skills 資料夾與其中的 skill 數量。若有 agents 資料夾，會在第二行顯示它與 agent 數量。`.skillignore` 生效時，會多一行顯示規則數與被忽略的 skill 數。

### Tracked Repositories

列出以 `--track` 安裝的 git 儲存庫與各自的 skill 數量。`✓` 表示儲存庫沒有未提交的變更；`!` 會加上 `uncommitted changes`，或在無法讀取 git status 時附上錯誤訊息。

### Targets

每個 target 一行：名稱、skills 資料夾，以及 skills 與 agents 的狀態。表格下方的一行列出使用中的 sync 模式。

```
Targets                     skills                agents
  claude  ~/.claude/skills  ✓ 8 linked · 2 local  ✓ 8
  cursor  ~/.cursor/skills  ! 6/8 copied          ! 7/8
  copy: cursor · merge: claude
! 2 skills not synced — run skillshare sync
```

**skills 欄：**

| 顯示 | 意義 |
|------|------|
| `✓ 8 linked` / `✓ 8 copied` | 預期的 skill 都已就位。merge 與 copy 模式以經過 `include`/`exclude` 過濾後的集合計算 |
| `· 2 local` | 該資料夾中你自己的 skill，sync 不會動它們 |
| `! 6/8 linked` | 部分 skill 尚未同步；status 最後會列出數量與 `sync` 指令。`sync` 刻意略過的 skill（`standard` 或 `prefixed` 命名下名稱無效，或名稱衝突）不計入，因為再執行 `sync` 也補不上；`sync` 會列出它們 |
| `✓ symlinked` | symlink 模式：整個資料夾連結到 source |
| `! needs sync` | 模式已變更，執行 `sync` 套用 |
| `! has files` / `! not synced yet` | 這個 target 還沒同步過 |
| `✗ links to …` / `✗ broken link` | 資料夾連到別處，或連到不存在的位置 |
| `skills off` | 這個 target 的 skills 已關閉 |

**agents 欄：** `✓ 8` 是已連結的 agent 數量（最新的副本也算作已連結）。`! 7/8` 表示有部分缺失，請執行 `skillshare sync agents`。只計算該 target 會同步的 agent，也就是經過 `.agentignore`、target 的 agents include/exclude 以及各 agent 的 `targets` frontmatter 之後剩下的。在 copy fallback 中，skillshare 不擁有但內容相同的本機檔案會被保留，並以 `· 1 local` 計算。`—` 表示該 target 沒有 agents 資料夾。沒有 agents source 時，這一欄會省略。

### Extras

當設定了 extras，每個 extra target 各佔一行：

```
Extras
  rules     .cursor/rules     4 files · merge
  commands  .claude/commands  3 files · merge
```

每行顯示 extra 名稱、target 資料夾、檔案數量，以及檔案實際使用的同步模式：在沒有開啟開發人員模式的 Windows 上，連結檔案的 target 會顯示 `copy`。

### Audit

一行顯示目前生效的 audit 政策（由 CLI flags、專案設定或全域設定解析而來）：profile（`default`、`strict` 或 `permissive`），以及會擋下安裝的最低嚴重程度（預設為 `critical`）。dedupe 模式與分析器只有在不同於預設值（`global` 與全部分析器）時才會列出。

### Version

顯示 CLI 與 skill 的版本。有更新的 skill 發布時，會多一行說明如何更新。（僅限 global 模式）

## 選項

| Flag | 說明 |
|------|-------------|
| `--json` | 以 JSON 輸出（供 scripting/CI 使用） |
| `--project, -p` | 使用 project 模式 |
| `--global, -g` | 使用 global 模式 |
| `--help, -h` | 顯示說明 |

## JSON 輸出

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

無法讀取 git status 的 tracked repo 會顯示 `"status": "unknown"`，`message` 中包含錯誤訊息；此時 `dirty` 為 false，且不具意義。

`source.skillignore` 欄位只有在至少存在一個 `.skillignore` 或 `.skillignore.local` 檔案時才會出現。若不存在則為：`"skillignore": { "active": false }`。`files` 陣列在有 `.skillignore.local` 時也會包含其路徑。在文字模式下，若有任何 `.skillignore.local` 生效，`.skillignore` 那一行會顯示 `(.local active)`。

JSON 輸出在 global 與 project 模式下皆支援。

## Project 模式

在專案目錄中，status 會顯示專案的 source、targets 與 extras，路徑以專案根目錄為基準：

```bash
skillshare status        # 若 .skillshare/ 存在則自動偵測
skillshare status -p     # 明確指定 project 模式
```

### 輸出範例

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

Project 模式的 status 不會顯示 Tracked Repositories 或 Version 區段（這些是僅限 global 的功能）。

## 另請參閱

- [sync](/docs/reference/commands/sync) — 將 skills 同步到 targets
- [diff](/docs/reference/commands/diff) — 顯示詳細差異
- [doctor](/docs/reference/commands/doctor) — 診斷問題
- [Project Skills](/docs/understand/project-skills) — Project 模式概念
