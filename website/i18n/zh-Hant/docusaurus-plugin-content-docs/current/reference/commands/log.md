---
sidebar_position: 4
---

# log

檢視持久化的操作與稽核日誌，用於除錯與合規。

```bash
skillshare log                    # 互動式 TUI（TTY 上預設啟用）
skillshare log --audit            # 只顯示稽核日誌
skillshare log --tail 50          # 顯示最後 50 筆
skillshare log --cmd sync         # 只顯示 sync 相關項目
skillshare log --status error     # 只顯示錯誤
skillshare log --since 2d         # 最近 2 天的項目
skillshare log --stats            # 顯示摘要統計
skillshare log --json             # 以 JSONL 輸出
skillshare log --no-tui           # 純文字輸出
skillshare log --clear            # 清除操作日誌
skillshare log -p                 # Project 的操作 + 稽核日誌
```

## 何時使用

- 互動式瀏覽並篩選日誌項目
- 除錯失敗操作的發生原因
- 檢視稽核軌跡以進行合規或疑難排解
- 依指令、狀態或時間範圍篩選日誌以進行調查

## 互動式 TUI

在 TTY 中，`skillshare log` 會開啟互動式的操作清單，旁邊顯示選取項目的詳細資訊，並提供篩選與依指令分類的統計畫面。操作紀錄與 audit 紀錄會合併並依時間排序。按鍵列在畫面底部。

使用 `--no-tui` 跳過 TUI，改印出純文字：

```bash
skillshare log --no-tui           # 純文字輸出
skillshare log --no-tui | less    # 手動接到 pager
```

## 記錄了什麼

每個會變更狀態的 CLI 與 Web UI 操作，都會被記錄為一筆 JSONL 項目，包含時間戳、指令、狀態、持續時間及情境參數。

| 指令 | 日誌檔案 |
|---------|----------|
| `install`, `uninstall`, `sync`, `push`, `pull`, `collect`, `backup`, `restore`, `update`, `target`, `trash`, `config`, `check`, `diff`, `init`, `upgrade` | `operations.log` |
| `audit` | `audit.log` |

呼叫這些 API 的 Web UI 動作，記錄方式與 CLI 操作相同。

## 日誌類型

### 預設檢視
在一次輸出中顯示**兩個區段**：
- Operations 日誌
- Audit 日誌

```bash
skillshare log
```

### 僅稽核檢視

將安全稽核掃描與一般操作分開記錄。

```bash
skillshare log --audit
```

### 篩選

依指令、狀態或時間範圍縮小結果範圍。當 `--cmd` 指定的指令只出現在特定日誌中時（例如 `--cmd audit` 只會出現在 audit.log 中），不相關的區段會自動跳過。

```bash
skillshare log --cmd install              # 只顯示 install 項目
skillshare log --status error             # 只顯示錯誤
skillshare log --since 1h                 # 最近一小時（也支援：30m, 2d, 1w）
skillshare log --since 2026-01-15         # 自特定日期起
skillshare log --cmd sync --status error  # 合併多個篩選條件
```

### JSON 輸出

輸出原始 JSONL，供腳本與自動化使用：

```bash
skillshare log --json                     # 所有項目以 JSONL 輸出
skillshare log --json --cmd sync          # 篩選後的 JSONL
```

## 範例輸出（純文字）

在 terminal 使用 `--no-tui` 時（輸出被 pipe 時，每筆的細節會合併成一行）：

```
Operations · last 2
✓ sync      2026-02-10 14:35 · 0.3s
            targets: 3
            scope: global

✗ sync      2026-02-10 14:31 · 0.8s · error
            targets: 3
            failed: 1
            scope: global
  ~/.local/state/skillshare/logs/operations.log

Audit · last 1
✗ audit     2026-02-10 14:36 · 1.1s · blocked
            scope: all-skills
            scanned: 12
            passed: 11
            failed: 1
            failed skills:
              - prompt-injection-skill
              - data-exfil-skill
  ~/.local/state/skillshare/logs/audit.log
```

## 日誌格式

項目以 JSONL 格式儲存（每行一個 JSON 物件）：

```json
{"ts":"2026-02-10T14:30:00Z","cmd":"install","args":{"source":"anthropics/skills/pdf"},"status":"ok","ms":1200}
```

| 欄位 | 說明 |
|-------|------|
| `ts` | ISO 8601 時間戳 |
| `cmd` | 指令名稱 |
| `args` | 指令特定情境（source、name、target 等） |
| `status` | `ok`、`error`、`partial` 或 `blocked` |
| `msg` | 錯誤訊息（當 status 不是 ok 時） |
| `ms` | 持續時間（毫秒） |

## 日誌位置

```
~/.local/state/skillshare/logs/operations.log    # Global operations
~/.local/state/skillshare/logs/audit.log         # Global audit
<project>/.skillshare/logs/operations.log   # Project operations
<project>/.skillshare/logs/audit.log        # Project audit
```

## 在 Git 中追蹤日誌（Project Mode）

Project mode 預設會忽略 `.skillshare/logs/`，以避免產生雜訊過多的 commits。

如果你的團隊想要對日誌檔案進行版本控制，請在 `.skillshare/.gitignore` 的受管理區塊之後，加入這些**使用者覆寫**規則：

```gitignore
# User override: track logs
!logs/
!logs/*.log
```

如果你的 repository 根目錄的 `.gitignore` 也忽略了 `.skillshare/`，請在那裡一併加入對應的取消忽略規則。

## 選項

| Flag | 說明 |
|------|------|
| `-a`, `--audit` | 只顯示稽核日誌 |
| `-t`, `--tail <N>` | 顯示最後 N 筆項目（預設：20） |
| `--cmd <name>` | 依指令名稱篩選（例如 `sync`、`install`、`audit`） |
| `--status <status>` | 依狀態篩選（`ok`、`error`、`partial`、`blocked`） |
| `--since <dur\|date>` | 依時間篩選（`30m`、`2h`、`2d`、`1w`，或 `2006-01-02`） |
| `--stats` | 顯示摘要統計（總數、成功率、依指令細分） |
| `--json` | 輸出原始 JSONL（每行一個 JSON 物件） |
| `--no-tui` | 停用互動式 TUI，改用純文字輸出 |
| `-c`, `--clear` | 清除選定的日誌檔案（預設為 operations，加 `--audit` 則為 audit） |
| `-p`, `--project` | 使用 project 層級日誌 |
| `-g`, `--global` | 使用 global 層級日誌 |
| `-h`, `--help` | 顯示說明 |

## Web UI

日誌也可以在 Web dashboard 的 `/log` 頁面中檢視：

```bash
skillshare ui
# 導覽至 Log 頁面
```

Log 頁面提供：
- `All`、`Operations` 與 `Audit` **分頁**
- 依指令、狀態與時間範圍（1h、24h、7d、30d）**篩選**
- 含時間、指令、詳情、狀態與持續時間的**表格檢視**
- 有失敗/警告 skill 名稱時顯示的**稽核詳細列**
- **清除**與**重新整理**控制項

## 統計檢視

### CLI

```bash
skillshare log --stats                # 所有操作的摘要
skillshare log --stats --cmd sync     # 只顯示 sync 的統計
skillshare log --stats --since 7d     # 最近 7 天的統計
```

## 日誌保留

日誌會自動截斷以避免無限成長。預設限制為**每個日誌檔案 1000 筆項目**（每次 CLI 或 Web UI 操作 = 1 筆項目）。`operations.log` 與 `audit.log` 分別追蹤。

若要覆寫預設值，請在 `config.yaml` 中加入：

```yaml
log:
  max_entries: 500  # entries per file; 0 = unlimited (default: 1000)
```

## 另請參閱

- [audit](/docs/reference/commands/audit) — 安全掃描（記錄至 audit.log）
- [status](/docs/reference/commands/status) — 顯示目前同步狀態
- [doctor](/docs/reference/commands/doctor) — 診斷設定問題
