---
sidebar_position: 4
---

# trash

管理 trash 目錄中已解除安裝的 skills 與 agents。

```bash
skillshare trash list                    # Interactive TUI (in TTY)
skillshare trash list --no-tui           # Plain text output
skillshare trash restore my-skill        # Restore from trash
skillshare trash restore my-skill -p     # Restore in project mode
skillshare trash delete my-skill         # Permanently delete from trash
skillshare trash empty                   # Empty the trash
skillshare trash agents list             # List trashed agents
skillshare trash agents restore tutor    # Restore an agent from trash
skillshare trash --all list              # List trashed skills + agents
```

## 何時使用

- 復原最近解除安裝的 skill 或 agent（7 天內）
- 永久刪除 trash 中的項目以釋放空間
- 在項目自動到期前查看 trash 內有什麼

## 互動式 TUI

在 TTY 環境下，`trash list` 會啟動一個支援多選、篩選與內嵌 restore/delete 操作的互動式 TUI。每個項目會顯示種類標記：`[S]` 代表 skills，`[A]` 代表 agents。

```
Trash (global) — 5 items

  [ ] [S] my-skill    (512 B, 2d ago)
  [x] [S] old-tool    (1.2 KB, 5d ago)
  [ ] [A] tutor       (2.0 KB, 3d ago)
  [ ] [S] another     (128 B, 1d ago)

  ─────────────────────────────────────────
  Name:         old-tool
  Type:         Skill
  Trashed:      2026-02-27 14:30:05
  Size:         1.2 KB
  Path:         ~/.local/share/skillshare/trash/old-tool_...

  ── SKILL.md ──────────────────────────────
  ---
  name: old-tool
  description: A helpful tool
  ---
  # old-tool
  ...

  ↑↓ navigate  / filter  space select  r restore(1)  d delete(1)  D empty  q quit
```

使用 `--all` 或不加種類篩選時，TUI 會把 skills 與 agents 合併成一份依日期排序（最新在前）的清單。

### 按鍵綁定

| Key | Action |
|-----|--------|
| `↑`/`↓` | Navigate items |
| `←`/`→` | Change page |
| `/` | Enter filter mode (substring match on name) |
| `Space` | Toggle select current item |
| `a` | Toggle select all visible items |
| `r` | Restore selected items (with confirmation) |
| `d` | Permanently delete selected items (with confirmation) |
| `D` | Empty all trash (ignores selection, with confirmation) |
| `Ctrl+d`/`Ctrl+u` | Scroll detail panel down/up |
| `q`/`Ctrl+C` | Quit |

在確認模式下：`y`/`Enter` 確認，`n`/`Esc` 取消。

### 批次操作

當選取多個項目時，`r` 與 `d` 會對所有項目一併執行。如果部分項目失敗（例如要還原的 skill 名稱已存在於 source 中），TUI 會繼續處理其餘項目並顯示合併後的結果：

```
Restored 2 item(s)  Failed: my-skill: already exists
```

使用 `--no-tui` 可跳過 TUI，改印出純文字：

```bash
skillshare trash list --no-tui           # Plain text output
skillshare trash list --no-tui | less    # Pipe to pager manually
```

## 種類篩選

預設情況下，trash 操作的對象是 **skills**。使用位置關鍵字 `agents` 可指定操作 agents，或用 `--all` 同時包含兩者：

```bash
skillshare trash list                    # Skills only (default)
skillshare trash agents list             # Agents only
skillshare trash --all list              # Both skills and agents
skillshare trash agents restore tutor    # Restore a trashed agent
skillshare trash agents empty            # Empty agent trash only
```

## 子指令

### list（別名：`ls`）

顯示目前 trash 中的所有項目。在終端機中會啟動互動式 TUI，或用 `--no-tui` 或非 TTY 環境印出純文字：

```bash
skillshare trash list
skillshare trash agents list
skillshare trash --all list --no-tui
```

純文字輸出：

```
Trash
  my-skill      (1.2 KB, 2d ago)
  old-helper    (800 B, 5d ago)

2 item(s), 2.0 KB total
Items are automatically cleaned up after 7 days
```

### restore

把最近一次 trash 中的版本還原回 source 目錄：

```bash
skillshare trash restore my-skill
skillshare trash agents restore tutor
```

```
✓ Restored: my-skill
ℹ Trashed 2d ago, now back in ~/.config/skillshare/skills
ℹ Run 'skillshare sync' to update targets
```

對於 agents，還原提示會建議改用 `skillshare sync agents`。

如果 source 中已存在同名項目，restore 會失敗。請先解除安裝既有項目，或改用不同名稱。

### delete（別名：`rm`）

永久刪除 trash 中的單一項目：

```bash
skillshare trash delete my-skill
skillshare trash agents delete tutor
```

```
✓ Permanently deleted: my-skill
```

### empty

永久刪除 trash 中的所有項目（會有確認提示）：

```bash
skillshare trash empty
skillshare trash agents empty
```

```
⚠ This will permanently delete 3 item(s) from trash
✓ Emptied trash: 3 item(s) permanently deleted
```

## Backup vs Trash

這兩個安全機制保護的對象不同：

| | backup | trash |
|---|---|---|
| **保護對象** | target 目錄（sync 快照） | source 中的 skills 與 agents（解除安裝） |
| **位置** | `~/.local/share/skillshare/backups/` | `~/.local/share/skillshare/trash/`（skills）、`.../trash/agents/`（agents） |
| **觸發時機** | `sync`、`target remove` | `uninstall` |
| **還原方式** | `skillshare restore <target>` | `skillshare trash restore <name>` |
| **自動清理** | 手動（`backup --cleanup`） | 7 天 |

## 選項

| Flag | Description |
|------|-------------|
| `agents` | Positional keyword — operate on agents instead of skills |
| `--all` | Include both skills and agents |
| `--no-tui` | Disable interactive TUI, use plain text output |
| `--project, -p` | Use project-level trash (`.skillshare/trash/`) |
| `--global, -g` | Use global trash |
| `--help, -h` | Show help |

## 自動清理

過期的 trash 項目（超過 7 天）會在你執行 `uninstall` 或 `sync` 時自動清除。不需要任何 cron 或排程工作。

## 另請參閱

- [uninstall](/docs/reference/commands/uninstall) — 移除 skills（移至 trash）
- [backup](/docs/reference/commands/backup) — 備份 target 目錄
- [restore](/docs/reference/commands/restore) — 從備份還原 targets
