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

在 TTY 中，`trash list` 會開啟互動式的垃圾桶清單，最新的在前面。可以選取一個或多個項目來還原或永久刪除，也可以清空整個垃圾桶；每個動作都會先確認。打開項目會顯示它的檔案，還原前可以先確認內容。按鍵列在畫面底部。使用 `--all` 或沒有指定種類時，skills 與 agents 會列在一起。

如果部分項目失敗（例如要還原的 skill 名稱已存在於 source），其餘項目仍會繼續處理，結果會列出失敗的項目。

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
  my-skill      1.2 KB · 2d ago
  old-helper    800 B · 5d ago

2 items, 2.0 KB
  Each item is removed for good 7 days after it was trashed
```

### restore

把最近一次 trash 中的版本還原回 source 目錄：

```bash
skillshare trash restore my-skill
skillshare trash agents restore tutor
```

```
✓ Restore   my-skill → ~/.config/skillshare/skills · trashed 2d ago

Next
  skillshare sync  link it into your targets again
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
✓ Permanently deleted my-skill
```

### empty

永久刪除 trash 中的所有項目（會有確認提示）：

```bash
skillshare trash empty
skillshare trash agents empty
```

```
! This will permanently delete 3 items from trash
? Continue? [y/N] y
✓ Emptied trash: 3 items permanently deleted · 0.1s
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
