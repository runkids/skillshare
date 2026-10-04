---
sidebar_position: 1
---

# doctor

檢查環境並診斷你的 skillshare 設定問題。

```bash
skillshare doctor
skillshare doctor -p        # Project mode (.skillshare/config.yaml)
skillshare doctor -g        # 強制 global mode
skillshare doctor --json    # 供 CI 使用的結構化 JSON 輸出
```

```text
skillshare doctor

Environment
✓ Config       ~/.config/skillshare/config.yaml
  Config dir   ~/.config/skillshare
  Data         ~/.local/share/skillshare
  State        ~/.local/state/skillshare
✓ Source       ~/.config/skillshare/skills · 43 skills
✓ Agents       ~/.config/skillshare/agents · 2 agents
  Skillignore  not configured
✓ Links        supported
! Git          not initialized (recommended for backup)
✓ Integrity    27/27 skills verified

Targets
✓ claude    skills  merged · merge · 43 shared
✓           agents  synced · merge · 2/2 linked
✓ cursor    skills  merged · merge · 43 shared, 1 local
✓           agents  synced · merge · 2/2 linked
✓ gemini    skills  merged · merge · 43 shared
…
! gemini will see content from: universal
  ~/.agents/skills ← universal
  suggestion: …
…
✗ claude: 1 broken symlink: frontend__css-review
…

Extras
✓ rules     2 files · 2/2 targets OK
✓ commands  1 file · 1/1 targets OK
✓ team      1 file · 4/4 targets OK

Storage
  Backups      last 2026-09-28_12-41-50 · 10m ago
  Trash        1 item, 247 B · oldest under a day

✗ 6 errors, 4 warnings · 1.2s

Next
  skillshare sync  bring the targets up to date
```

## 何時使用

- 有東西無法運作但你不知道原因
- 升級 skillshare 或作業系統之後
- 驗證所有 targets、git 與 symlinks 是否健康
- 提交 bug 回報前的第一個診斷步驟

## 檢查內容

```text
skillshare doctor

Environment
✓ Config       ~/.config/skillshare/config.yaml
  Config dir   ~/.config/skillshare
  Data         ~/.local/share/skillshare
  State        ~/.local/state/skillshare
✓ Source       ~/.config/skillshare/skills · 12 skills
✓ Agents       ~/.config/skillshare/agents · 8 agents
✓ Skillignore  2 patterns, 1 skill ignored
✓ Links        supported
✓ Git          initialized with remote
✓ Integrity    12/12 skills verified

Targets
✓ claude    skills  merged · merge · 8 shared, 2 local
✓           agents  synced · merge · 8/8 linked
✓ codex     skills  merged · merge · 8 shared
✓ cursor    skills  copied · copy · 8 managed
✓           agents  synced · merge · 8/8 linked

Extras
✓ commands  3 files · 1/1 targets OK
✓ rules     4 files · 1/1 targets OK

MCP, hooks and plugins
✓ MCP          all 2 servers OK
✓ Hooks        all 1 hook in sync
  Plugins      none configured

Storage
  Backups      last 2026-01-18_09-00-00 · 3d ago
  Trash        empty

Version
✓ CLI          0.23.5
✓ Skill        0.23.5

✓ All checks passed · 0.4s
```

## 執行的檢查

### Environment

| 檢查項目 | 驗證內容 |
|-------|-----------------|
| Config | Config 檔案存在且有效 |
| Source | Source 目錄存在且可讀取 |
| Agents | Agents source 目錄存在（若有設定） |
| Skillignore | `.skillignore`（及 `.skillignore.local`）目前生效的 patterns 與被忽略的 skill 數量 |
| Source link | skills source 第一層的每個 symlink 或 Windows junction 各一條 info：discovery 不會跟進去，所以其內容對 skillshare 不可見 |
| Links | 系統可以建立 symlinks |
| Git | Repository 狀態與 remote 設定 |

Source link 檢查在 global 與 project mode 都會執行。source 根目錄依 discovery 的方式解析，只檢查第一層項目，不跟隨連結、不讀取其內容。沒有這類連結時不會增加輸出。每個連結在 `doctor --json` 中也會以 status 為 `info` 的 `undeclared_source_links` 檢查出現。

### Targets

每個 target 會顯示 **skills** 與 **agents**（若有設定 agents）的子項目：
- Skills：路徑、同步模式、同步狀態、共用/本機數量
- Agents：同步模式、已連結數量、飄移偵測。在沒有開啟 Developer Mode 的 Windows 上，`merge` 會顯示為 `copy`；最新的受管理副本會算作已連結。skillshare 不擁有、但內容相同的本機檔案會被保留。在 copy fallback 中，agent 計數會以 `local preserved` 分開顯示，例如 `0/1 linked, 1 local preserved`。
- 沒有損壞的 symlinks
- 針對非預期本機衝突的重複 skill 檢查：
  - `merge` 模式：跳過（本機 skills 屬於預期情況）
  - `copy` 模式：由 manifest 管理的副本會被忽略；只有本機衝突的副本會被警告
- 有效的 include/exclude glob patterns
- 適用時提供資訊層級的每個 target 相容性提示（範例 target 優先順序：`cursor` → `antigravity` → `copilot` → `opencode`；若這些 targets 都不存在則不會顯示提示）

### Path Overlap

Doctor 會在兩類重複 skill 風險到達 runtime picker 之前先標示出來：

**`shared_target_paths`**——當兩個或以上已啟用的 targets 解析到同一個主要路徑時觸發。常見原因：同時啟用 `universal` 以及一個會寫入 `~/.agents/skills` 的工具（例如 `warp`、`witsy`）。

```text
! Shared path ~/.agents/skills ← universal, warp
```

解決方式：停用其中一個重疊的 target，或使用 `skillshare target <name> --path <dir>` 設定不同的路徑。

若共用路徑的 targets 其 `include` 或 `exclude` 篩選不同，每次同步都會加入一個 target 要的內容、再刪掉另一個 target 過濾掉的內容，資料夾永遠不會穩定，`sync` 也會一直顯示同樣的待同步變更。Doctor 會標出這種情況，並建議只保留一個 target（若其中有 `universal` 就保留它）、其他的關閉 skills 同步，而不是移除 target：

```text
! Shared path ~/.agents/skills ← codex, universal (different filters, so they undo each other on every sync)
  suggestion: Keep universal syncing skills to ~/.agents/skills and stop the rest with `skillshare target codex --skills=false`.
```

`sync` 也會列出相同的 targets 與要執行的指令，dashboard 的同步頁面則提供按鈕，可直接停止同步該 target 的 skills。設定完全相同的共用路徑 targets 仍適用上方的解決方式。

**`cross_target_discovery`**——當某個已啟用 target 的 runtime 文件說明它也會掃描另一個已啟用 target 寫入的目錄時觸發。例如，從舊設定沿用下來的 config 仍將 `codex` 指向舊的 `~/.codex/skills`，而 `universal` 則寫入 `~/.agents/skills`——這個目錄 Codex 同樣會讀取。兩者都啟用時，Codex 就會在自己的內容之外，額外看到 universal 的內容。

```text
! codex will see content from: universal
  ~/.agents/skills ← universal
```

解決方式：先移除負責掃描的 target（上例中的 `codex`）。它的 runtime 本來就會讀取共用目錄，而且不會影響其他工具。可用 `skillshare target remove codex --dry-run` 預覽。若改為移除寫入者（`universal`），其他讀取 `~/.agents/skills` 的工具也會看不到這些 skill。只有在掃描端 target 帶有被寫入者過濾掉的 skill 時才同時保留兩者，並接受 runtime picker 出現重複列表。

這兩項檢查都是純粹的 metadata 比對——它們讀取已設定的路徑與內建的 `also_scans` 表，不會進行檔案系統探測。

### Version

- CLI 版本
- skillshare skill 版本；有較新的 skill 發布時會提出警告（`skillshare upgrade --skill`）
- 檢查是否有可用更新

### Skill Integrity

對於具有檔案雜湊 metadata 的已安裝 skills，doctor 會驗證自安裝以來沒有檔案被竄改：

- 比對目前的 SHA-256 雜湊值與已儲存的雜湊值
- 回報每個 skill 中被修改、遺失與新增的檔案
- 本機 skills（不在 `.metadata.json` 中）會被靜默略過——這是預期行為
- 有 metadata 但缺少 `file_hashes` 的已安裝 skills 會標示其名稱

```text
! Integrity    5/6 skills verified
!              _team-repo__api-helper: 1 modified, 1 missing
!              1 skill missing file hashes: _old-repo__legacy-skill
```

### Extras

當有設定 extras 時，會驗證：
- 每個 extra 的設定有效（mode、`flatten`、`as`），且沒有兩個 extras 搶同一個檔案
- 每個 extra 的 source 目錄存在
- Target 目錄可以連通
- 目錄型 target 裡的失效符號連結（error）
- 與 source 不一致的檔案，判斷方式同 `skillshare diff`（warning）。設定了 `flatten` 或 `extension` 的 target 不做比對。

```text
✗ rules     → ~/.claude/rules: broken symlink gone.md
!           → ~/.claude/rules: 1 file out of sync (a.md missing in target)
```

### MCP

執行 [`mcp check`](./mcp.md) 的靜態檢查：引用的環境變數有設定、`command` 能在 `PATH` 找到、client 規則接受這個 server，以及每個項目都已同步。Doctor 不解析 host，也不啟動 server；需要時請執行 `skillshare mcp check` 或 `skillshare mcp check --live`。沒有設定任何 server 時顯示 `info`。

```text
✗ MCP          docs: command no-such-mcp-binary was not found on PATH
!              docs → claude: not synced yet; run skillshare sync mcp
```

### Hooks

預覽 `skillshare sync hooks`，不寫入任何檔案。預覽失敗是 error。Sync 還會新增、更新或移除的項目、與原生 hooks 的衝突，以及提示性警告（例如 Agent 未記載的事件名稱）是 warning。沒有設定 hook 時顯示 `info`。

```text
! Hooks        bash-log → claude: not synced (add)
```

### Plugins

預覽 `skillshare sync plugins`，不抓取任何來源。Doctor 只詢問各綁定 Agent 的原生 CLI 目前安裝了什麼，而且只在有 plugin package 時才會這樣做。被阻擋的綁定（例如 Agent 的 CLI 沒有安裝）和仍需 sync 的綁定是 warning。檢查來源是否有新版本仍由 `skillshare plugin check` 負責。沒有設定 package 時顯示 `info`。

### 其他

- 沒有 `SKILL.md` 檔案的 skills
- Skill 層級的 `targets:` 欄位驗證（對未知的 target 名稱發出警告）
- 上次備份時間戳（global mode）
- Trash 狀態（項目數量、總大小、最舊項目的存放時間）
- Targets 中損壞的 symlinks

:::note Project Mode
當一個 project 有 `.skillshare/config.yaml` 時，`skillshare doctor` 會自動以 project mode 執行。

在 project mode 中：
- Config/source 檢查使用 `.skillshare/config.yaml` 與 `.skillshare/skills`
- Trash 狀態使用 `.skillshare/trash`
- Backups 會顯示 `not used in project mode`
:::

## 常見問題

### "Needs sync"

Target 模式已變更但尚未套用：

```bash
skillshare sync
```

### "Not synced"

Target 已連結的 skills 數量少於 source（例如安裝新 skills 之後）：

```bash
skillshare sync
```

### "Has uncommitted changes"

已追蹤的 repo 有本機變更：

```bash
cd ~/.config/skillshare/skills/_team-repo
git status
# Commit or discard changes
```

### "Broken symlink"

某個 skill 已從 source 中移除，但 symlink 仍然存在：

```bash
skillshare sync  # Will prune orphaned symlinks
```

### "Skills without SKILL.md"

Skill 資料夾缺少必要的檔案：

```bash
# Add SKILL.md to each skill, or remove the folder
skillshare new my-skill  # Creates proper structure
```

### "Link not supported"

`doctor` 會在系統暫存目錄（Windows 上是 `%TEMP%`，其他系統是 `$TMPDIR` 或 `/tmp`）裡連結一個測試資料夾。在 Windows 上這個連結是 NTFS junction，不需要系統管理員權限，也不需要開發人員模式，所以開啟開發人員模式無法解決這個錯誤。訊息中的 `junction error:` 那一行會顯示 Windows 拒絕的原因。請確認暫存目錄：

1. 位於本機的 NTFS 磁碟，而不是 FAT32、exFAT 或網路共用資料夾（junction 只能在 NTFS 上使用）
2. 你的帳號有寫入權限，且沒有被防毒或安全軟體封鎖

這項檢查不會測試檔案連結。沒有開發人員模式時，會連結單一檔案的 agents 與 extras 會改為複製；請參閱 [Windows 疑難排解](../../troubleshooting/windows.md#file-links-need-windows-developer-mode-copying-instead)。

## 有問題時的範例輸出

```
Environment
✓ Config       ~/.config/skillshare/config.yaml
✓ Source       ~/.config/skillshare/skills · 12 skills
✓ Agents       ~/.config/skillshare/agents · 8 agents
✓ Links        supported
! Git          3 uncommitted changes
! Integrity    5/6 skills verified
!              _team-repo__api-helper: 1 modified
! Skills without SKILL.md: test-dir, temp

Targets
✓ claude    skills  merged · merge · 8 shared, 2 local
✓           agents  synced · merge · 8/8 linked
! codex     skills  linked · merge · needs sync
✓ cursor    skills  merged · merge · 6 shared
! claude    1 skill not synced · 2/3 linked
✗ cursor: 2 broken symlinks: old-skill, removed-skill

Storage
  Backups      last 2026-01-18_09-00-00 · 3d ago
  Trash        2 items, 45.2 KB · oldest 3 days

Version
✓ CLI          1.2.0
✓ Skill        0.16.0
  Update       v1.2.0 → v1.3.0 available

✗ 1 error, 5 warnings · 0.6s

Next
  skillshare sync          bring the targets up to date
  brew upgrade skillshare  update to v1.3.0
```

## JSON 輸出

在 CI pipelines 與自動化中使用 `--json` 取得機器可讀輸出：

```bash
skillshare doctor --json
```

```json
{
  "checks": [
    { "name": "source", "status": "pass", "message": "Source: ~/.config/skillshare/skills (12 skills)" },
    { "name": "skillignore", "status": "pass", "message": ".skillignore: 3 patterns, 2 skills ignored", "details": ["test-*", "vendor/", "!important", "---", "test-draft", "vendor/lib"] },
    { "name": "sync_drift", "status": "warning", "message": "claude: 1 skill(s) not synced (7/8 linked)", "details": ["new-skill"] },
    { "name": "shared_target_paths", "status": "warning", "message": "1 shared target path(s) — enabled targets writing to the same directory may produce duplicate skills in runtime pickers", "details": ["~/.agents/skills ← universal, warp"], "suggestions": ["Choose one authoritative target for ~/.agents/skills; preview removing duplicate targets with `skillshare target remove <name> --global --dry-run` (currently: universal, warp)."] },
    { "name": "broken_symlinks", "status": "error", "message": "cursor: 1 broken symlink(s)", "details": ["old-skill"] }
  ],
  "summary": { "total": 14, "pass": 12, "warnings": 1, "errors": 1, "info": 0 },
  "version": { "current": "0.17.4", "latest": "0.18.0", "update_available": true }
}
```

檢查狀態：`pass`、`warning`、`error`、`info`。`info` 狀態用於既非通過也非失敗的資訊性檢查（例如找不到 `.skillignore`）。Info 檢查會計入 `total`，但不會計入 `pass`、`warnings` 或 `errors`。

部分警告類檢查（例如 `shared_target_paths`、`cross_target_discovery`）還會包含一個可選的 `suggestions` 陣列，提供可執行的補救步驟。若無建議可提供，此欄位會省略。

### 結束代碼

| 條件 | 結束代碼 |
|-----------|-----------|
| 所有檢查通過（或只有警告） | `0` |
| 任一檢查為 `error` 狀態 | `1` |

### CI 範例

```bash
# Fail pipeline if doctor finds errors
skillshare doctor --json | jq -e '.summary.errors == 0'

# Extract warnings for notification
skillshare doctor --json | jq '[.checks[] | select(.status == "warning")]'
```

:::tip Web Dashboard
Web dashboard 中的 **Health Check** 頁面（`skillshare ui`）提供 `doctor --json` 的視覺化版本，具備篩選開關與可展開的詳細內容。
:::

## 另請參閱

- [status](/docs/reference/commands/status) — 快速狀態檢查
- [sync](/docs/reference/commands/sync) — 修復同步問題
- [upgrade](/docs/reference/commands/upgrade) — 更新 CLI 與 skill
