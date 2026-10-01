---
sidebar_position: 2
---

# backup

建立、列出並管理 target 目錄的備份。

```bash
skillshare backup              # Backup all skill targets
skillshare backup claude       # Backup specific target
skillshare backup agents       # Backup all agent targets
skillshare backup --all        # Backup skills + agents
skillshare backup --list       # List all backups
skillshare backup --cleanup    # Remove old backups
skillshare backup --delete 2026-01-19_10-00-00  # Delete one backup
skillshare backup files        # Versions of single files skillshare rewrote
```

## 何時使用

- 在進行有風險的變更前手動建立備份
- 列出現有備份以確認可回復的選項
- 清理舊備份，或刪除不再需要的某一份備份
- 取回 `AGENTS.md` 或 `CLAUDE.md` 這類檔案的較早版本

## 自動備份

以下操作前會**自動**建立備份：
- `skillshare sync`（skill targets 與 agent targets）
- `skillshare sync agents`（僅 agent targets）
- `skillshare target remove`

位置：`~/.local/share/skillshare/backups/<timestamp>/`（global），`.skillshare/backups/`（project mode，僅 agents）

每次自動備份後會自動套用保留策略，使用與 `--cleanup` 相同的規則。你不需要手動清理快照。

## 指令

### 建立備份

```bash
skillshare backup              # All targets
skillshare backup claude       # Specific target
skillshare backup --dry-run    # Preview
```

### 列出備份

```bash
skillshare backup --list
```

```
All backups (15.3 MB total)
  2026-01-20_15-30-00  claude, cursor     4.2 MB  ~/.local/share/.../2026-01-20_15-30-00
  2026-01-19_10-00-00  claude             2.1 MB  ~/.local/share/.../2026-01-19_10-00-00
  2026-01-18_09-00-00  claude, cursor     4.0 MB  ~/.local/share/.../2026-01-18_09-00-00
```

### 清理舊備份

```bash
skillshare backup --cleanup           # Remove old backups
skillshare backup --cleanup --dry-run # Preview cleanup
```

預設清理策略：
- 保留最新 10 份備份
- 移除超過 30 天的備份
- 總大小上限為 500 MB

份數和大小上限可以在全域 config 中調整，也可以在 dashboard **目標資料夾** 分頁的保留摘要中修改。`0` 代表不限。30 天的上限是固定的，project 的快照沿用預設值。

```yaml
backup:
  max_count: 20      # 預設 10
  max_size_mb: 1000  # 預設 500
```

最新的快照永遠會被保留，即使它單獨就超過大小上限——你永遠不會失去可回復的還原點。

這項策略會在每次 `sync` 後自動執行，所以 `--cleanup` 只在需要時手動觸發即可。

### 刪除備份

```bash
skillshare backup --delete 2026-01-19_10-00-00            # Delete one snapshot
skillshare backup --delete 2026-01-19_10-00-00 --dry-run  # Show what would be deleted
skillshare backup --delete 2026-01-19_10-00-00 -p         # From the project's .skillshare/backups/
```

時間戳記就是 `--list` 顯示的資料夾名稱。整個快照都會被刪除，包括其中的每一個 target。

### 檔案歷史 {#file-history}

skillshare 在改寫或取代單一檔案之前——例如 `AGENTS.md`、`CLAUDE.md` 這類指令檔，或[共用檔案](/docs/how-to/daily-tasks/sharing-instructions#backups)的某個位置——會先保存舊內容。`backup files` 可以列出並還原這些版本。

```bash
skillshare backup files                                   # Files with saved versions
skillshare backup files show ~/.claude/CLAUDE.md          # Versions of one file, newest first
skillshare backup files restore ~/.claude/CLAUDE.md origin
skillshare backup files restore ./CLAUDE.md 1769000000000000000.shim --dry-run
```

```
Versions of /Users/me/.claude/CLAUDE.md
  1769000000000000000.edit          2026-01-21 12:53:20  history/edit          2.1 KB  # Team rules
  drift:1768900000000000000.mode    2026-01-20 09:06:40  drift/mode            1.9 KB  # Team rules
  origin                            2026-01-10 08:00:00  origin                1.2 KB  # My notes
```

每個版本都有一個 ID：

| ID | Kind | Meaning |
|----|------|---------|
| `<time>[.<reason>]` | `history` | 在 skillshare 寫入檔案之前保存 |
| `drift:<time>[.<reason>]` | `drift` | 你自己做的、被 skillshare 取代的修改 |
| `origin` | `origin` | 第一次接上共用檔案時檔案原本的內容；移除該位置時會自動還原它。若當時沒有檔案，還原它會刪除目前的檔案 |

reason 說明 skillshare 當時正要做什麼：

| Kind | Reason | Saved before |
|------|--------|--------------|
| `history` | `convert` | 將檔案轉換成 `AGENTS.md`，或重新命名為 `AGENTS.md` |
| `history` | `shim` | 在專案檔案中加入 `@AGENTS.md` |
| `history` | `edit` | 在 dashboard 中編輯 |
| `history` | `collect` | 把 target 的改動收進共用檔案 |
| `history` | `attach` | 第一次接上時被共用檔案取代 |
| `history` | `restore` | 還原較舊的版本 |
| `history` | `migrate` | 同步儲存設定時，移除 0.23.0 淘汰的 MCP 設定。請參閱[從 0.22 升級 Pi](/docs/reference/commands/mcp#pi-migration) |
| `drift` | `overwrite` | 你直接修改了檔案，並選擇 **覆蓋** |
| `drift` | `mode` | 切換該位置的模式 |
| `drift` | `restore` | 還原該位置 |

舊版本保存的版本沒有 reason。每個檔案的每一種 kind 保留最近 10 份。

`restore` 會先把目前的內容保存為一個 reason 為 `restore` 的新版本，再寫入選擇的版本。如果路徑是 symlink，除非加上 `--unlink`，否則會拒絕執行；`--unlink` 會把連結換成一般檔案。

`backup files` 會依照模式運作：在專案內（或加上 `-p`）只會列出該專案內的檔案，`show` / `restore` 也會拒絕專案以外的路徑；`-g` 則涵蓋所有檔案。因為 `files` 是子指令，若要備份名稱剛好是 `files` 的 target，請使用 `skillshare backup -t files`。

## Dashboard {#dashboard}

[`skillshare ui`](/docs/reference/commands/ui) 中的 **設定 › 備份** 有三個分頁：

- **目標資料夾** — 上面說明的快照，依日期分組。可依 target 或 **只看 agents** 篩選。展開某個快照可看到每個資料夾的檔案數與大小，並可 **還原** 其中任一個（skill 與 agent 項目都可以）、**複製路徑** 或 **刪除這份備份**。**立即備份** 與 **清理舊備份** 分別對應 `backup` 與 `--cleanup`。旁邊的保留摘要可以開啟面板調整份數和大小上限，**全部刪除** 會在你確認後刪除所有快照，檔案、MCP 和 Hooks 的備份不受影響。
- **檔案** — 上面說明的檔案歷史。選擇一個檔案即可看到它的各個版本與 reason，接著 **預覽並還原** 會顯示與目前檔案的差異或完整版本。有連結的位置只有在你確認 **還原並切斷連結** 之後，才會被換成一般檔案。
- **MCP** — 每次寫入 MCP 設定前建立的備份，依 Agent 設定分組，並列出每份備份新增、變更或移除的 server。**預覽並還原** 會開啟與 **MCP** 頁面相同的還原對話框（命令列上則是 [`mcp restore`](/docs/reference/commands/mcp)）。

![設定 › 備份 › 檔案：還原前預覽較早的 CLAUDE.md 版本](/img/backup-files-preview.png)

在 project mode 下，這個頁面只涵蓋該專案：`.skillshare/backups/` 中的 agent 快照、專案內的檔案，以及專案 MCP 設定的備份。刪除的 skills 與 agents 不在這裡；它們會進入 **Skills** 與 **Agents** 的 **垃圾桶** 分頁。

## 選項

| Flag | Description |
|------|-------------|
| `--all` | Backup both skills and agents |
| `--project, -p` | Use project mode (`.skillshare/backups/`); **agents only** |
| `--global, -g` | Use global mode (default for skills) |
| `--list, -l` | List all backups; with `-p`, the project's |
| `--cleanup, -c` | Remove old backups; with `-p`, the project's |
| `--delete <timestamp>` | Delete one backup; with `-p`, from `.skillshare/backups/` |
| `--target, -t <name>` | Target specific backup (alternative to positional arg) |
| `--dry-run, -n` | Preview without making changes |

`backup files` 有自己的選項：`--project, -p`、`--global, -g`，以及 `restore` 專用的 `--unlink` 與 `--dry-run, -n`。參見[檔案歷史](#file-history)。

`backup` 也接受一個位置參數（positional kind argument）：`skillshare backup agents` 會把備份範圍限定在 agent targets。

## 備份結構

```
~/.local/share/skillshare/backups/
├── 2026-01-20_15-30-00/
│   ├── claude/
│   │   ├── skill-a/
│   │   └── skill-b/
│   └── cursor/
│       ├── skill-a/
│       └── skill-b/
└── 2026-01-19_10-00-00/
    └── claude/
        └── ...
```

實際存在的 skill 目錄取決於 target 的模式——參見[備份內容](#what-gets-backed-up)。

## 備份內容 {#what-gets-backed-up}

備份只保護 `sync` 可能破壞的內容：**存在於 target 中但不存在於 source 中的本機內容**。

- target 中的一般檔案與目錄會被備份
- merge 模式 target 中的 per-skill symlinks 會**被略過**——它們指向你的 source，而 source 本身就是唯一的 source of truth，已經很安全。`skillshare sync` 會重新建立這些 symlinks

也就是說：
- 在 merge 模式下：只有本機（非 symlink）的 skills 會被備份。已同步的 skills 存在於 source 中
- 在 copy 模式下：所有受管理的 skill 目錄都會被備份（它們是真實檔案）
- 在 symlink 模式下：不會備份任何東西（整個目錄是單一 symlink）

如果 target 內只有 symlinks，就不會建立備份，`backup` 會回報沒有需要處理的內容——一個空的還原點沒有意義。

## 備份與磁碟空間 {#backups--disk-space}

備份永遠不會複製你的 source，所以體積很小。有三個容易混淆的獨立機制：

| Mechanism | Scope | What it controls |
|-----------|-------|------------------|
| source 中的 `.gitignore` | 僅 Git | Git 追蹤哪些內容。被忽略的檔案仍然存在於磁碟上 |
| `config.yaml` 中的 `ignore:` | `sync` | `sync` 會把哪些檔案複製進 targets（主要用於 copy 模式）。參見 [sync](/docs/reference/commands/sync) |
| Backup | Snapshot | 僅限本機 target 內容——symlinks（因此也包括 source 中的產物）都會被排除 |

因為 symlink 的 skill 不會被追蹤展開，存在於 source skill 內的大型產物（model weights、`.venv`、瀏覽器 profile、媒體檔案）**永遠不會**被複製進快照，無論 `.gitignore` 或 `ignore:` 是否有提到它們。

保留策略會在每次 `sync` 後自動執行，使用下方的預設策略。若要手動檢查用量：

```bash
du -sh ~/.local/share/skillshare/backups   # Total size on disk
skillshare backup --list                   # Per-snapshot sizes
skillshare backup --cleanup --dry-run      # Preview what retention would remove
```

Copy 模式的 targets 是快照仍可能變大的唯一情況：那些是真實檔案，所以 skill 目錄下的任何內容都會被複製。請把執行期快取與大型產物放在 skill 目錄樹之外，或用 `ignore:` 排除它們，讓它們一開始就不會被送到 target。

## Agent 備份 {#agent-backup}

Agents 有自己的備份流程，與 skill 備份並行執行，有兩點值得留意：

**條目命名**。Agent 備份會存放在每個 timestamp 目錄底下的 `<target>-agents/`，與 skill 備份並列。例如，執行 `skillshare backup --all` 後的目錄結構如下：

```
~/.local/share/skillshare/backups/2026-01-20_15-30-00/
├── claude/          # Skills backup for claude
├── claude-agents/   # Agents backup for claude
└── cursor/
```

**Project mode 與 skills 相反**。在 project mode（`-p`）下，`backup` 會拒絕備份 skill targets，但**會**備份 agent targets。如果你忘了加上 `agents` 篩選，會看到以下錯誤：

```
backup is not supported in project mode (except for agents)
```

因此在 project mode 下，你必須明確指定 `skillshare backup -p agents` 或 `skillshare backup -p --all`。`--list -p` 和 `--cleanup -p` 不需要指定，會直接處理 `.skillshare/backups/`。

```bash
skillshare backup agents                  # All agent targets (global)
skillshare backup agents claude           # Only claude's agents
skillshare backup agents -p               # Project agent targets
skillshare backup --all                   # Skills + agents in one shot
```

參見 [Agents](/docs/understand/agents) 了解 agent 資源模型，以及 [restore](/docs/reference/commands/restore) 了解如何回復。

## 另請參閱

- [restore](/docs/reference/commands/restore) — 從備份還原
- [sync](/docs/reference/commands/sync) — 自動建立備份
- [target remove](/docs/reference/commands/target) — 自動建立備份
