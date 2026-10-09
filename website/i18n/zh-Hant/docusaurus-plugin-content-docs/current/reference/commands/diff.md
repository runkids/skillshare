---
sidebar_position: 2
---

# diff

顯示 source 與 targets 之間的差異。

```bash
skillshare diff              # 所有 targets（互動式 TUI）
skillshare diff claude       # 特定 target
skillshare diff agents       # 僅 agent targets
skillshare diff --stat       # 檔案層級變更
skillshare diff --patch      # 完整 unified diff
```

```text
skillshare diff --no-tui

claude, claude-work, gemini, opencode, universal
  New       remotion-captions

cursor
  Local only  cursor-shortcuts
  New         remotion-captions

Extras
✓ commands  ~/.claude/commands · in sync
✓ rules     ~/.claude/rules · in sync
✓ rules     ~/.cursor/rules · in sync
✓ team      ~/.codex · in sync
✓ team      ~/.claude · in sync
✓ team      ~/.gemini · in sync
✓ team      ~/notes · in sync

! 6 targets: 6 to sync

Next
  skillshare sync     apply the changes
  skillshare collect  copy local-only skills into source
```

## 互動式 TUI

在 TTY 中，`diff` 會開啟互動式畫面：左側是 targets，右側是選取 target 的差異，可以一路看到檔案層級的 diff。按鍵列在畫面底部。使用 `--no-tui` 或把輸出 pipe 出去，就會改印純文字。

## 使用時機

- 在同步之前，查看 source 與某個 target 之間確切的差異
- 找出只存在於 target 中的 skills（本機獨有，尚未收集）
- 找出可以被 symlink 取代的本機複本
- 用 `--stat` 檢視檔案層級變更，或用 `--patch` 檢視完整文字 diff

## 輸出範例

```
claude
  Local override  local-copy
  Local only      my-local-skill
  New             another-skill, missing-skill

✓ cursor    in sync

! 2 targets: 1 to sync, 1 in sync

Next
  skillshare sync          apply the changes
  skillshare sync --force  also replace local copies
  skillshare collect       copy local-only skills into source
```

### 分組後的多 Target 輸出

當多個 targets 有完全相同的 diff 結果時，會合併為單一區塊以減少雜訊：

```
agents, claude
  New       skill-1, skill-2

cursor
  New       skill-1

✓ codex, copilot  in sync
```

結果不同的 targets（例如因為 `include`/`exclude` filters 而不同）仍會分開顯示。

## 標籤

| 標籤 | 意義 | 動作 |
|-------|---------|--------|
| New | 存在於 source，target 中缺失 | `sync` 會新增它 |
| Restore | 曾存在於 target，已被刪除 | `sync` 會還原它 |
| Modified | 內容或 target naming 已變更（copy 模式） | `sync` 會更新它 |
| Renamed | 受管理的項目仍使用先前 `target_naming` 給的名稱 | `sync` 會重新命名它 |
| Local only, skill kept under old name | 本機資料夾佔用了目前 `target_naming` 給 skill 的名稱，skill 留在舊的受管理項目，顯示為 `name (stays at old-name)` | 重新命名或刪除該資料夾後 `sync` |
| Local override | 本機複本而非 symlink | `sync --force` 以取代 |
| Orphan | 存在於 manifest 但不在 source 中 | `sync` 會清除它 |
| Local only | 只存在於 target，不在 source 中 | 用 `collect` 匯入 |

## 檔案層級細節

### `--stat`

顯示每個 skill 中哪些檔案有差異：

```bash
skillshare diff --stat
```

```
claude
  Modified  my-skill
            + new-file.md (120 bytes)
            ~ SKILL.md (840 → 912 bytes)
            - old-file.md (64 bytes)
```

### `--patch`

顯示已修改檔案的完整 unified 文字 diff：

```bash
skillshare diff --patch
```

```
claude
  Modified  my-skill
            ~ SKILL.md (840 → 912 bytes)
            --- SKILL.md
            - old line
            + new line
```

`--stat` 與 `--patch` 兩者皆隱含 `--no-tui`（純文字輸出）。

## Diff 顯示的內容

### Merge 模式的 Targets

對於使用 merge 模式（預設）的 targets：
- 列出 source 中尚未 symlink 到 target 的 skills
- 顯示以本機複本存在（而非 symlink）的 skills
- 找出 target 中僅存在的本機 skills（不在 source 中 — 會被 sync 保留）

### Copy 模式的 Targets

對於使用 copy 模式的 targets：
- 列出 source 中尚未被管理的 skills（manifest 中缺失）
- 透過檢查碼比對顯示內容變更
- 顯示不再存在於 source 中的孤立受管理複本（sync 時會被清除）
- 找出本機獨有的 skills（不在 source 中且未受管理）

### Symlink 模式的 Targets

對於使用 symlink 模式的 targets：
- 只是檢查 symlink 是否指向正確的 source
- 顯示「in sync」或警告 symlink 錯誤

## 使用情境

### Sync 之前

檢查會有什麼變更：

```bash
skillshare diff
# 查看 sync 會做什麼，然後：
skillshare sync
```

### 找出本機 Skills

發現你直接在某個 target 中建立的 skills：

```bash
skillshare diff claude
# 顯示：Local only  my-local-skill

skillshare collect claude  # 匯入到 source
```

### 檢視變更

在同步前確切查看某個 skill 有什麼變更：

```bash
skillshare diff --patch claude   # 完整文字 diff
skillshare diff --stat claude    # 檔案層級摘要
```

### 疑難排解

當 status 顯示有問題時：

```bash
skillshare status          # 顯示「needs sync」
skillshare diff claude     # 確切查看差異之處
skillshare sync            # 修正它
```

## Agent Diff {#agent-diff}

使用 `agents` 關鍵字僅 diff agent targets：

```bash
skillshare diff agents             # 所有支援 agent 的 targets
skillshare diff agents claude      # 特定 target
skillshare diff agents --json      # JSON 輸出
```

Agent diff 會顯示缺失的 agents（需要 sync）、孤立的 symlinks（需要 prune），以及本機獨有的 agent 檔案。只有設有 `agents` 路徑的 targets 會被納入。詳見 [Agents — Supported Targets](/docs/understand/agents#supported-targets) 了解完整清單。

---

## 選項

| Flag | 說明 |
|------|-------------|
| `--project, -p` | 使用 project 模式 |
| `--global, -g` | 使用 global 模式 |
| `--stat` | 顯示檔案層級變更（隱含 `--no-tui`） |
| `--patch` | 顯示完整 unified diff（隱含 `--no-tui`） |
| `--no-tui` | 純文字輸出（略過互動式 TUI） |
| `--json` | 以 JSON 輸出（隱含 `--no-tui`） |

## JSON 輸出

```bash
skillshare diff --json
```

```json
{
  "targets": [
    {
      "name": "claude",
      "mode": "merge",
      "synced": false,
      "items": [
        {"action": "add", "name": "missing-skill", "kind": "skill", "reason": "source only", "is_sync": true},
        {"action": "modify", "name": "local-copy", "kind": "skill", "reason": "local copy (sync --force to replace)", "is_sync": true},
        {"action": "remove", "name": "my-own-skill", "kind": "skill", "reason": "local only", "is_sync": false},
        {"action": "kept", "name": "prototype", "kind": "skill", "reason": "local folder; the skill stays at _emil-design__skills__prototype", "is_sync": false}
      ],
      "include": [],
      "exclude": []
    }
  ],
  "duration": "0.045s"
}
```

當 `sync` 對該 target 無事可做時，`synced` 為 `true`。`"is_sync": false` 的項目（例如只存在於 target 的資料夾）仍會列出，但不會讓它變成 `false`。文字輸出同樣把只有 local-only 資料夾的 target 視為已同步。

`action` 為 `add`、`modify`、`remove` 或 `kept`。`kept` 表示本機資料夾佔用了目前 `target_naming` 給 skill 的名稱，skill 留在舊名稱下，sync 不會變動該資料夾。

## 另請參閱

- [sync](/docs/reference/commands/sync) — 同步到 targets
- [collect](/docs/reference/commands/collect) — 匯入本機 skills
- [status](/docs/reference/commands/status) — 快速總覽
