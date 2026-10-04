---
sidebar_position: 2
---

# extras

管理與 skill 一起同步的非 skill 資源（rules、commands、prompts 等）。

## Overview

Extras 是 skillshare 管理的額外資源類型——可以把它想成「給非 skill 內容用的 skill」。常見的使用情境包含跨工具同步 AI rules、編輯器 commands，或 prompt 範本。

每個 extra 都有：
- 一個**名稱**（例如 `rules`、`prompts`、`commands`）
- 一個**source 目錄**——可透過 `extras_source` 或個別 extra 的 `source` 設定，預設為 `~/.config/skillshare/extras/<name>/`（global）或 `.skillshare/extras/<name>/`（project）
- 一個或多個檔案同步的 **target**

在 dashboard 中，**Extras → Folders & files** 會列出每個 extra 及其 targets 與模式：

![Extras › Folders & files：rules 與 commands 同步到各自的 targets](/img/extras-folders.png)

## Commands

### `extras memory` {#extras-memory}

管理 `memory` extra 中的共用 Markdown 筆記，可用任意文字編輯器編輯。
[記憶共用教學（英文截圖）](../../how-to/daily-tasks/sharing-memory)示範建立、目錄瀏覽與跨 Agent 使用。

| 子命令 | 行為 |
|---|---|
| `init` | 建立沒有 targets 的 memory extra，補上缺少的 `INDEX.md`、`LEARNED.md`，保留既有檔案與設定 |
| `list` | 列出筆記；`--search <text>` 不分大小寫搜尋路徑與內容，包含子資料夾 |
| `show <note.md>` | 讀取筆記；`--json` 包含 `version` hash |
| `write <note.md> --from <file\|->` | 從檔案或 stdin 讀取內容；新增時不指定 `--version`，更新須使用最近讀取的 version |
| `delete <note.md> --version <hash>` | 備份後刪除指定版本，拒絕過期或缺少的 version |
| `instructions` | 輸出指向實際 source 資料夾的讀取指引；兩種模式都會在每個 task 開始時讀 `INDEX.md`；`--update-mode passive`（預設）讓 Agent 提出值得記的事實、只在使用者要求時更新筆記，`--update-mode active` 讓 Agent 把這類事實存在這裡而不是工具自己的記憶、拿不準時先提議 |

各子命令支援 `--json`、`-g` / `--global`、`-p` / `--project` 及 `--help`。
未指定時自動判斷 scope。預設 global 路徑為 `~/.config/skillshare/extras/memory/`，
project 為 `.skillshare/extras/memory/`；沿用既有 extras source 覆寫設定。

筆記須為相對 `.md` 路徑、UTF-8，最多 1 MiB；排除隱藏檔案、隱藏資料夾及內部符號連結。過大或非 UTF-8 檔案仍列出並標為不支援，其他正常筆記仍可使用。`wiki/architecture.md` 會自動建立資料夾。Dashboard 提供目錄樹、**Preview** / **Source**、**Copy path**、**Edit**、**Delete note** 與 **History**。**Move or rename** 可輸入新的相對 `.md` 路徑，建立缺少的資料夾，保留內容與權限，並拒絕同名目標或過期版本。移動前會在舊路徑備份；Markdown 連結須自行修正。請保留來源根目錄的 `INDEX.md`，供 agent 指引讀取。

儲存檢查最近讀取的 version。衝突會保留草稿，顯示最新儲存內容供比較。**Save my draft** 須確認，使用更新後的 version，備份已儲存內容後再取代。刪除也須確認、檢查版本並備份。**History** 與刪除後的還原連結會開啟 **Backup Files**，以筆記的絕對路徑篩選。CLI 可用 `backup files show <absolute-path>` 與 `backup files restore <absolute-path> <id>`。

**New note** 的 **Link from INDEX.md** 在索引可讀取時顯示並預設勾選，於檔案末尾附加連結，檢查 version 並備份。失敗仍保留新筆記。**Add to INDEX** 可加入未索引的筆記。失效連結會顯示警告，不會自動移除。CLI 寫入不會新增索引連結。

使用 **Connect to agents** 選取工具並為每個工具選擇更新模式（`passive` 或 `active`），再 **Review changes** → **Apply changes**。讀取同一個檔案的工具共用一個區塊，會一起切換模式；已設定工具的模式也能透過同一個預覽變更。此流程將 scope/hash 標記區塊加入或更新至既有 instructions 或共用來源，保留其他內容與指派。可檢查變更、其他讀取工具與已知字元上限。既有檔案會備份，過期預覽會被拒絕。完整但過期的區塊可經檢查後更新；手動修改或格式有誤的區塊會保留。未同步或無法讀取的 instructions 檔案會略過。

**Configured** 僅表示讀取鏈已有目前的指引，不代表已讀取。**Copy verification prompt** 用於新 session，要求 Agent 讀取 `INDEX.md` 與相關筆記、回報完整路徑及使用者加入的臨時驗證值。請手動檢查實際 read tool event；沒有保證可用的讀取 telemetry。

**Copy guidance** 是手動貼上的替代方式，選擇模式後貼上即可；**Open AGENTS.md** 可編輯 instructions。Project 內的來源路徑相對於 **project root**，不依 instructions 檔案位置；外部或 global 來源用絕對路徑，移動後須重新產生。CLI `instructions` 也輸出相同的 scope/hash 區塊。不啟用 native automatic memory、自動學習或 Obsidian 整合。

### `extras init`

建立新的 extra 資源類型。

```bash
# 互動式精靈
skillshare extras init

# CLI 旗標
skillshare extras init <name> --target <path> [--target <path2>] [--mode <mode>]

# 單一檔案 extra
skillshare extras init <name> --file <filename> [--as <filename>] --target <path> [--source <dir>] [--mode <mode>]
```

精靈會在輸入名稱後詢問 **要同步什麼？**：**目錄** 或 **單一檔案**。

**Options:**

| Flag | Description |
|------|-------------|
| `--target <path>` | Target 目錄路徑（可重複指定） |
| `--file <filename>` | 只同步 source 目錄中的這個檔案，建立[單一檔案 extra](#single-file-extras)。必須是單純的檔名，不能含有 `/` 或 `\` |
| `--as <filename>` | 寫入每個 target 時使用的檔名（預設為 `--file` 的名稱）。需要搭配 `--file` |
| `--mode <mode>` | Sync 模式：`merge`（預設）、`copy`，或 `symlink`；`import` 僅限搭配 `--file` |
| `--flatten` | 將子目錄中的檔案直接同步到 target 根目錄（無法與 `symlink` 模式或 `--file` 一起使用） |
| `--source <path>` | 此 extra 的自訂 source 目錄（會覆寫 `extras_source` 與預設值；project mode 中為相對於專案根目錄的路徑） |
| `--force` | 若 extra 已存在則覆寫 |
| `--no-tui` | 略過互動式精靈，只使用 CLI 旗標 |
| `--project, -p` | 在 project 設定中建立（`.skillshare/`） |
| `--global, -g` | 在 global 設定中建立 |

:::note
`--source` 只在 global mode 中支援。Project mode 一律使用 `.skillshare/extras/<name>/` 作為 source 目錄。
:::

**Examples:**

```bash
# 將 rules 同步到 Claude 與 Cursor
skillshare extras init rules --target ~/.claude/rules --target ~/.cursor/rules

# 使用自訂的 source 目錄
skillshare extras init rules --target ~/.claude/rules --source ~/company-shared/rules

# 用新的 target 覆寫既有的 extra
skillshare extras init rules --target ~/.cursor/rules --force

# 使用 copy 模式的 project 範圍 extra
skillshare extras init prompts --target .claude/prompts --mode copy -p

# 以扁平方式同步 agents（像 Claude Code 這類工具只會探索扁平的檔案）
skillshare extras init agents --target ~/.claude/agents --flatten

# 同步單一檔案，並在 target 改名
skillshare extras init pi-prompt --file system.md --as APPEND_SYSTEM.md \
  --source ~/dotfiles/prompts --target ~/.pi/agent
```

`extras init` 只會寫入設定，不會建立 source 檔案，也不會執行 sync。若是單一檔案 extra，它會印出完整的 source 與 target 檔案路徑：

```
  Source    ~/dotfiles/prompts/system.md
  Target    ~/.pi/agent/APPEND_SYSTEM.md · merge

✓ Created extra pi-prompt (single file)

Next
  skillshare sync extras  sync it
```

若 source 檔案還不存在，source 那一行結尾會是 `(not found)`，最後一行則是 `Create the source file, then run 'skillshare sync extras'.`

### `extras list`

列出所有已設定的 extras 及其同步狀態。預設會啟動互動式 TUI。

```bash
skillshare extras list [--json] [--no-tui] [-p|-g]
```

**Options:**

| Flag | Description |
|------|-------------|
| `--json` | JSON output（包含 `source_type`：`per-extra` / `extras_source` / `default`，以及設定時每個 target 的 `extension` 欄位） |
| `--no-tui` | 停用互動式 TUI，改用純文字輸出 |
| `--project, -p` | 使用 project mode 的 extras（`.skillshare/`） |
| `--global, -g` | 使用 global 的 extras（`~/.config/skillshare/`） |

#### Interactive TUI

在 TTY 中，`extras list` 會開啟互動式畫面：左側是 extras，右側是選取 extra 的 targets 與檔案。在這裡可以新增、移除、同步與收回（collect）extras，也可以更改某個 target 的模式或 flatten 設定。按鍵列在畫面底部。

可以用 `skillshare tui off` 永久關閉 TUI。

#### Plain text output

當 TUI 被停用時（透過 `--no-tui`、`skillshare tui off`，或輸出被 pipe）：

```
$ skillshare extras list --no-tui
rules  ~/.config/skillshare/extras/rules · 2 files
✓ ~/.claude/rules  merge
✓ ~/.cursor/rules  copy

codex-agents  ~/.config/skillshare/agents · 3 files
✓ ~/.codex/agents  extension: codex-agents

2 extras
```

若是[單一檔案 extra](#single-file-extras)，source 與每個 target 會顯示完整的檔案路徑，而不是目錄。

已同步的列只會顯示圖示、路徑與模式；未同步的列則會附加狀態文字（`drift`、`modified`、`not synced`、`no source`）。有 transform extension 的 target 會以 `extension: <name>` 取代 sync 模式標示（其底層模式一律是 `copy`）。

### `extras source`

顯示或設定全域的 `extras_source` 目錄。這是儲存 extras source 檔案的預設父目錄。

```bash
skillshare extras source            # 顯示目前的值
skillshare extras source <path>     # 設定新的值
```

不帶參數時，會顯示目前的 `extras_source` 路徑（若為自動偵測則附上 `(default)`）。帶路徑參數時，會更新 global 設定中的 `extras_source`。

:::note
此指令僅限 global。Project mode 一律使用 `.skillshare/extras/`，不支援 `extras_source`。
:::

**Examples:**

```bash
# 顯示目前的 extras_source
skillshare extras source

# 設定為共用目錄
skillshare extras source ~/company-shared/extras
```

### Operating on an existing extra

透過 `extras <name>` 變更 target 的 sync 模式、flatten 設定，或新增／移除 target。變更模式、flatten 或新增 target 後，請執行 `skillshare sync extras` 套用。`--remove-target --prune` 也會立即還原或移除受管理的檔案。

```bash
skillshare extras <name> --mode <mode> [--target <path>] [-p|-g]
skillshare extras <name> --flatten | --no-flatten [--target <path>]
skillshare extras <name> --add-target <path> [--as <filename>] [--mode <mode>] [--flatten] [-p|-g]
skillshare extras <name> --remove-target <path> [--prune] [-p|-g]
skillshare extras <name> --help
```

**Options:**

| Flag | Description |
|------|-------------|
| `--mode <mode>` | 新的 sync 模式：`merge`、`copy`，或 `symlink`；`import` 僅限[單一檔案 extra](#single-file-extras) |
| `--flatten` | 啟用 flatten（將子目錄檔案同步到 target 根目錄） |
| `--no-flatten` | 停用 flatten |
| `--add-target <path>` | 為此 extra 新增一個 target |
| `--as <filename>` | `--add-target` 的 target 檔名（僅限單一檔案 extra；預設為 `file`） |
| `--remove-target <path>` | 從此 extra 移除一個 target（預設僅變更設定） |
| `--prune` | 搭配 `--remove-target` 使用：同時刪除該 target 底下由 skillshare 管理的檔案。若是單一檔案 extra，則會改為還原 target 檔案 |
| `--target <path>` | Target 目錄路徑（多 target 的 extra 使用 `--mode` 時為必填；省略時 `--flatten`/`--no-flatten` 會套用到所有 target） |
| `--project, -p` | 使用 project mode 的 extras（`.skillshare/`） |
| `--global, -g` | 使用 global 的 extras（`~/.config/skillshare/`） |

**Examples:**

```bash
# 變更 rules 模式（單一 target — 自動解析）
skillshare extras rules --mode copy

# 明確指定 target（多 target 的 extra 需要）
skillshare extras rules --mode copy --target ~/.claude/rules

# 一次啟用／停用所有 target 的 flatten
skillshare extras agents --flatten
skillshare extras agents --no-flatten

# 為既有的 extra 新增一個 target（之後再 sync）
skillshare extras rules --add-target ~/.cursor/rules
skillshare extras commands --add-target ~/.config/opencode/commands --mode copy
skillshare extras personal --add-target ~/.claude --as CLAUDE.md --mode import

# 移除一個 target（保留已同步的檔案）
skillshare extras rules --remove-target ~/.cursor/rules

# 移除一個 target 並刪除其已同步的檔案
skillshare extras rules --remove-target ~/.cursor/rules --prune
```

也可以透過 TUI（`e` 鍵）以及 Web UI（每個 target 上的模式下拉選單與 flatten 核取方塊）操作。

### `extras remove`

從設定中移除一個 extra。

```bash
skillshare extras remove <name> [--force] [-p|-g]
```

source 檔案會保留。目錄 extra 會保留已同步的 target。[單一檔案 extra](#single-file-extras) 會先還原 target，再移除設定項目；還原失敗時會保留設定，讓你重試。

### `extras collect`

把 target 中的本機檔案收集回 extras source 目錄。檔案會被複製到 source，並以 symlink 取代。copy 模式的 target 則會保留一般複本形式的檔案。[單一檔案 extra](#single-file-extras) 不支援 collect。

source 中已存在的檔案會被略過。使用 `--force` 可改以 target 版本覆寫它們——例如把直接在 copy 模式 target 中做的修改拉回來。內容已與 source 相同的檔案仍會被略過。

```bash
skillshare extras collect <name> [--from <path>] [--force] [--dry-run] [-p|-g]
```

**Options:**

| Flag | Description |
|------|-------------|
| `--from <path>` | 要從中收集的 target 目錄（若有多個 target 則為必填） |
| `--force`, `-f` | 覆寫 source 中已存在的檔案 |
| `--dry-run` | 顯示會收集哪些內容，但不做任何變更 |

**Example:**

```bash
# 把 rules 從 Claude 收集回 source
skillshare extras collect rules --from ~/.claude/rules

# 預覽會收集哪些內容
skillshare extras collect rules --from ~/.claude/rules --dry-run

# 把 target 的修改拉回來，覆寫既有的 source 檔案
skillshare extras collect rules --force
```

---

## Sync Modes

| Mode | Behavior |
|------|----------|
| `merge` (default) | 從 target 到 source 的逐檔 symlink |
| `copy` | 逐檔複製 |
| `symlink` | 整個目錄的 symlink |
| `import` | 僅限[單一檔案 extra](#single-file-extras)：在 target 檔案中加入一行 `@<source file>` |

在沒有開啟開發人員模式的 Windows 上，`merge` 會改為複製每個檔案，而不是連結它，`sync` 會印出 `file links need Windows Developer Mode; copying instead`。之後 `extras list` 與 `status` 會把該 target 顯示為 `copy`。這些複本會被追蹤，所以之後的 sync 會更新並清理它們、保留你自己的檔案，並在檔案連結可用後換成連結。請參閱 [Windows 疑難排解](/docs/troubleshooting/windows#file-links-need-windows-developer-mode-copying-instead)。

內容相同的本機檔案會顯示為 `local preserved`；`sync extras` 不會為它們建議使用 `--force`。它們仍是本機檔案，不是受管理的連結。

切換模式時（例如從 `merge` 切換到 `copy`），下一次 `sync` 會自動用新模式的格式取代既有的 symlink。不需要 `--force`——symlink 一律可以安全地取代。本機建立的一般檔案則需要 `--force` 才能覆寫。

---

## Flatten

有些 AI 工具（例如 Claude Code 的 `/agents`）只會探索其設定目錄**最上層**的檔案——不會遞迴進入子目錄。如果你的 extras source 使用子目錄來組織檔案，同步後的檔案對該工具來說會是不可見的。

`flatten` 選項可以解決這個問題：無論檔案在 source 中的子目錄深度為何，都會直接同步到 target 根目錄：

```yaml
extras:
  - name: agents
    targets:
      - path: ~/.claude/agents
        flatten: true
```

**Behavior:**
- `flatten: true`: `source/curriculum/tactician.md` → `target/tactician.md`
- `flatten: false` (default): `source/curriculum/tactician.md` → `target/curriculum/tactician.md`

**檔名衝突：** 當兩個不同子目錄中的檔案同名時（例如 `team-a/agent.md` 與 `team-b/agent.md`），先出現的檔案會生效（依路徑字母順序排序）。之後的衝突則會被略過並顯示警告。

**限制：**
- 只適用於 `merge` 與 `copy` 模式——無法與 `symlink` 模式一起使用
- `collect` 會把新收集到的檔案放在 source 根目錄（新檔案沒有子目錄對應）

---

## Extension transforms

有些工具不讀取 markdown。Gemini CLI 需要 TOML 格式的 commands；Codex CLI 需要 TOML 格式的 agents。target 上的 `extension` 欄位會在同步時執行外部腳本，把每個 source 檔案轉換成該 target 的原生格式。

```yaml
extras:
  - name: commands
    targets:
      - path: .claude/commands        # no extension — synced as-is
      - path: .gemini/commands
        extension: gemini-commands           # transform during sync
```

**解析方式** — 單純的名稱會在 extensions 目錄下解析（global 為 `~/.config/skillshare/extensions/<name>`，project 為 `.skillshare/extensions/<name>`）；路徑（`./x.sh`、`/abs/x`）則直接使用。

**複製語意** — `extension` 隱含 `copy` 模式。在帶有 `extension` 的 target 上設定 `mode: merge` 或 `mode: symlink` 會是錯誤。

**單向** — transform 只會由 source 執行到 target。`extras collect` 會略過有 extension 的 target。

**覆寫安全性** — 產生的輸出遵循與 `copy` 模式相同的衝突規則。輸出路徑上殘留的 symlink 會自動被取代；本機建立的既有一般檔案或目錄則會維持原狀並被略過，除非你加上 `--force`（加上 `--force` 時，衝突的目錄會被產生的檔案整個取代）。

### Extension layout

可以是單一可執行檔，或帶有 manifest 的目錄：

```
.skillshare/extensions/gemini-commands/
├── extension.yaml
├── convert.js        # mapping rules you edit
└── md-toml.js        # helper for markdown/frontmatter/TOML
```

`extension.yaml`：

```yaml
run: ["node", "convert.js"]      # explicit command (argv), execed directly
output_ext: toml                  # .md → .toml; omit to keep the source extension
description: "Markdown command → Gemini CLI TOML"
```

單純的單檔可執行檔（沒有 manifest）會直接被 exec（在 Unix 上依賴 shebang），並保留 source 的副檔名。需要重新命名副檔名的 transform 必須使用目錄形式。

### Execution contract

- Source 檔案內容會透過 **stdin** 傳入；腳本則將轉換後的內容寫到 **stdout**。
- 環境變數：`SS_SRC_PATH`、`SS_REL_PATH`（相對於 source 根目錄的路徑——對 Gemini 的 `/namespace:command` 命名方式很有用）、`SS_TARGET_DIR`、`SS_MODE`。
- 非零的結束碼會將該檔案標記為失敗；其他檔案則會繼續處理。

### Cross-platform

這個機制是跨平台的；某個 extension 能否執行取決於它的直譯器（interpreter）。因為 `run` 是明確的指令，用 `node` 或 `python3` 撰寫的 extension 可以在 Windows、macOS、Linux 上運作。純 `bash` 腳本只能在有 shell 可用的環境執行（Unix，或是裝有 Git Bash 的 Windows）。參考用的 extension 偏好使用 Node 作為直譯器，因為它在各平台上都能一致地提供。

### Reference extensions

skillshare repo 在 `extensions/` 底下附上了範例 extension（`gemini-commands`、`codex-agents`、`opencode-agents`）。把其中一個複製到你的 extensions 目錄並自行調整——它們是參考範例，不會自動安裝。每個參考 extension 都會讓 `convert.js` 保持精簡，讓你只需要編輯欄位對應；`md-toml.js` 負責讀取 markdown、解析簡單的 frontmatter，並寫出 TOML。

### Recipe: Codex agents

Codex CLI 需要 TOML 格式的 agents，而非 markdown。因為 `source` 可以指向任何目錄，你可以把你的 agents source 重複用作 extras source，並用 `codex-agents` 轉換它：

```yaml
extras:
  - name: codex-agents
    source: ~/.config/skillshare/agents   # reuse the agents source
    targets:
      - path: ~/.codex/agents
        extension: codex-agents
```

`skillshare sync extras` 會把每個 `<agent>.md` 轉換成 `~/.codex/agents/<agent>.toml`，對應 frontmatter 的 `name`、`description`、`model`，並把 markdown 內文摺進 `developer_instructions`（其他 frontmatter 欄位則會被捨棄）。[Codex custom agent schema](https://developers.openai.com/codex/subagents#custom-agent-file-schema) 要求 `name`、`description`、`developer_instructions`，因此當解析出的 name、description 或 Markdown 內文為空白時，這個參考 transform 會回報清楚的錯誤。不需要另外複製一份 agents。

Agent targets 也可以直接使用 `extension`，不需要透過 extra。詳見 [使用 extension 轉換 agents](/docs/understand/agents#extensions)。

---

## Recipe: shared instructions across agents

:::tip Dashboard
網頁 dashboard 可以幫你完成這些設定，並提供預覽、備份與還原按鈕：請參閱
[讓多個工具共用一份 AGENTS.md](../../how-to/daily-tasks/sharing-instructions.md)。
它使用的是[單一檔案 extra](#single-file-extras)，而不是目錄。
:::

現在大多數 coding agent 都會讀取 `AGENTS.md` 作為常設指示，但每一個都把自己使用者層級的
複本放在不同的目錄。一個帶有多個 target 的 extra，就能把單一的 source 檔案分發給所有工具：

```bash
skillshare extras init instructions \
  --target ~/.codex \
  --target ~/.config/opencode \
  --target ~/.claude \
  --target ~/.gemini \
  --no-tui
```

把你的 `AGENTS.md` 放進解析出的 source 目錄（預設為 `~/.config/skillshare/extras/instructions/`），然後執行 `skillshare sync extras`。

| Agent | Global path | Reads `AGENTS.md` |
|-------|-------------|-------------------|
| Codex CLI | `~/.codex/AGENTS.md` | 直接讀取 |
| opencode | `~/.config/opencode/AGENTS.md` | 直接讀取 |
| Claude Code | `~/.claude/AGENTS.md` | 透過 `CLAUDE.md` 匯入 |
| Antigravity | `~/.gemini/AGENTS.md` | 透過 `GEMINI.md` 匯入 |

有兩個 agent 在使用者層級會讀取自己固定的檔名，因此各自需要一個一行檔案，放在同步的檔案
旁邊。這些只需要寫一次；skillshare 之後不會再碰它們：

```markdown title="~/.claude/CLAUDE.md"
@AGENTS.md
```

```markdown title="~/.gemini/GEMINI.md"
@AGENTS.md
```

Claude Code 讀取的是 `CLAUDE.md` 而不是 `AGENTS.md`，而匯入正是它的 [memory 文件](https://code.claude.com/docs/en/memory) 建議用來與其他 agent 共享單一檔案的做法。Antigravity 把它的全域 rules 放在 `~/.gemini/GEMINI.md`，並會相對於該 rules 檔案自身所在的目錄解析 `@filename`，所以同樣的一行寫法就能抓到同步過來的 `AGENTS.md`。`~/.gemini` 這個 target 也涵蓋了 Antigravity CLI，因為它讀取的是同一個全域檔案。

請保持 source 檔案的名稱為 `AGENTS.md`。像 `memory.md` 這種中性名稱一樣可以同步成功，但會不再被讀取：Codex 是依名稱串接 `AGENTS.md` 檔案，且沒有 import 語法，因此只認得這個名稱的檔案。

因為 target 都是目錄，每個 target 都會以其 source 名稱收到每個檔案。請只在 source 目錄中放你想要到處都有的檔案——多放一個檔案，就會出現在全部四個 target 裡。

:::note
這份 recipe 分享的是你自己寫的指示，不是 agent 自己寫下的 memory。Agent 會以私有格式儲存自己的學習結果——Claude Code 用一個 Markdown 目錄，Codex 用資料庫，Cursor 則是非檔案式的儲存——這些內容無法透過在 target 之間複製檔案來搬移。
:::

---

## 單一檔案 extra {#single-file-extras}

設定了 `file` 的 extra 只會同步 source 目錄中的一個檔案，而不是整個目錄。每個 target 會收到
`<path>/<as>`，其中 `as` 預設為 `file` 的名稱。任何從固定路徑讀取單一檔案的工具都可以使用。例如 Pi
會把 `~/.pi/agent/APPEND_SYSTEM.md` 附加到它的 system prompt；你可以把這段文字以
`system.md` 放在 dotfiles 中，再連結過去：

```yaml
extras:
  - name: pi-prompt
    source: ~/dotfiles/prompts     # project mode：相對於專案根目錄
    file: system.md                # ~/dotfiles/prompts/system.md
    targets:
      - path: ~/.pi/agent
        as: APPEND_SYSTEM.md       # ~/.pi/agent/APPEND_SYSTEM.md 會變成連結
```

用 CLI 建立同一個 extra：

```bash
skillshare extras init pi-prompt --file system.md --as APPEND_SYSTEM.md \
  --source ~/dotfiles/prompts --target ~/.pi/agent
skillshare sync extras
```

`extras init` 的 `--as` 會套用到每個 target。若要在某個 target 使用不同的檔名，請另外新增該 target：

```bash
skillshare extras pi-prompt --add-target ~/Documents/prompts --as pi-system.md
```

Dashboard 的[共用 AGENTS.md](../../how-to/daily-tasks/sharing-instructions.md)
也是單一檔案 extra，可以混用一般連結、改名與 import：

```yaml
extras:
  - name: personal
    file: AGENTS.md                # ~/.config/skillshare/extras/personal/AGENTS.md
    targets:
      - path: ~/.codex             # ~/.codex/AGENTS.md 會變成連結
      - path: ~/.gemini
        as: GEMINI.md              # ~/.gemini/GEMINI.md 會變成連結
      - path: ~/.claude
        as: CLAUDE.md
        mode: import               # ~/.claude/CLAUDE.md 保留原本內容，並匯入該檔案
```

| Mode | Target 檔案 |
|------|-------------|
| `merge`（預設）或 `symlink` | 指向 source 檔案的 symlink（在沒有開啟開發人員模式的 Windows 上為複本） |
| `copy` | source 檔案的複本 |
| `import` | 你自己的檔案，頂端的受管理區塊中有一行 `@<source file>` |

`import` 會把 `@` 那一行放在 `<!-- skillshare:instructions:begin -->` 與
`<!-- skillshare:instructions:end -->` 之間，且絕不更動檔案的其他部分。請只在會展開 `@` 匯入的工具上使用，
例如 Claude Code。

規則：

- 使用連結或 `copy` 的 target 檔案只能屬於一份共用檔案，不能同時 import 另一份。
- `file` 與 `as` 必須是單純的檔名，不能含有 `/` 或 `\`。
- `as` 與 `import` 都需要搭配 `file`。單一檔案 extra 不能使用 `flatten` 與 `extension`。
- 當 target 已經有另一個不同的一般檔案或 symlink 時，sync 會先保存它再取代，不需要 `--force`。
  若擋在路上的是目錄，則會略過。
- 連結之後被改成 `modified` 的 target 也會被取代；修改過的檔案會保存為 drift backup，而不是還原點。
- 當連結被換成內容不同的一般檔案，或受管理的複本被修改時，`extras list` 會顯示 `modified`。
- 把 target 從 `merge`、`symlink` 或 `copy` 切換為 `import` 時，會放回上次在 `import` mode
  的自有內容（包括空內容）；若未用過則使用連接前的內容，再加上 import 區塊。修改過的複本會先保存為 drift backup。
- `extras remove` 與 `--remove-target --prune` 會還原每個 target 檔案：移除連結、複本或 import 那一行，
  並放回第一次 sync 前原本的檔案或 symlink（若原本沒有，就不留檔案）。`modified` 的 target
  會先保存為 drift backup。不加 `--prune` 的 `--remove-target` 會保留單一檔案 target，停止管理並忘掉還原點。之後的 sync 不會清理它；重新連接才會記錄新的還原點。
- 不支援 `extras collect`。若要保留在 target 中做的修改，請把它複製回 source 檔案。若是共用的 `AGENTS.md`，
  dashboard 的 **AGENTS.md** 分頁中的 **收進** 會替你完成這件事。

在 dashboard 中，`file` 為 `AGENTS.md` 的單一檔案 extra 會出現在 **AGENTS.md** 分頁；其他單一檔案 extra
則出現在 **資料夾與檔案**。在那裡，**新增 Extra** 可選擇 **資料夾** 或 **單一檔案**，每個 target 都有 **檔名**，
單一檔案可以使用 `merge`、`copy` 或 `import`。單一檔案的 **名稱** 會跟著檔名去掉副檔名自動填入
（`APPEND_SYSTEM.md` 會填成 `APPEND_SYSTEM`），直到你自己輸入名稱為止。Dashboard 不會編輯檔案內容，請直接編輯 source 檔案。

### 一個資料夾、多個檔案

多個單一檔案 extra 可以共用同一個 `source` 目錄。每個檔案各建立一個 extra；
資料夾中沒有被任何 extra 指定的檔案不會同步：

```yaml
extras:
  - name: pi-system
    source: ~/dotfiles/pi
    file: system.md
    targets:
      - path: ~/.pi/agent
        as: APPEND_SYSTEM.md
  - name: pi-agents
    source: ~/dotfiles/pi
    file: agents.md
    targets:
      - path: ~/.pi/agent
        as: AGENTS.md
```

```bash
skillshare extras init pi-system --source ~/dotfiles/pi --file system.md \
  --as APPEND_SYSTEM.md --target ~/.pi/agent
skillshare extras init pi-agents --source ~/dotfiles/pi --file agents.md \
  --as AGENTS.md --target ~/.pi/agent
```

在 project mode 中，`source` 是相對於專案根目錄的路徑，而且必須在專案內；絕對路徑會被拒絕：

```bash
skillshare extras init review -p --source .skillshare/extras/prompts \
  --file review.md --target .claude/commands
skillshare extras init plan -p --source .skillshare/extras/prompts \
  --file plan.md --target .claude/commands
```

在 Dashboard 中，共用 extras 資料夾裡的單一檔案有一個 **Source folder** 欄位。預設是 extra 的名稱；
填入另一個 extra 的資料夾，就能把兩個檔案放在同一個資料夾。

備份保存在 skillshare 的 state 目錄中（macOS 與 Linux 上為 `~/.local/state/skillshare/extras/backups/`），
每個檔案保留最近 10 份。Drift backup 放在其中的 `extras/backups/<id>/drift/`，`<id>` 由 target 檔案的
路徑推導而來；還原時絕不會使用它們。若要列出或還原任何保存的版本，請使用
[`backup files`](./backup.md#file-history)。

---

## Directory Structure

```
~/.config/skillshare/
├── config.yaml          # extras 設定放在這裡
├── skills/              # skill source
└── extras/              # extras source 根目錄
    ├── rules/           # extras/rules/ 的 source 檔案
    │   ├── coding.md
    │   └── testing.md
    └── prompts/
        └── review.md
```

---

## Configuration

在 `config.yaml` 中：

```yaml
# Optional: set a global default extras source directory
extras_source: ~/my-extras

extras:
  - name: rules
    source: ~/company-shared/rules    # optional per-extra override
    targets:
      - path: ~/.claude/rules
      - path: ~/.cursor/rules
        mode: copy
  - name: agents
    targets:
      - path: ~/.claude/agents
        flatten: true                  # sync subdirectory files flat
  - name: prompts
    targets:
      - path: ~/.claude/prompts
```

### Source Resolution Priority

每個 extra 的 source 目錄會依三層優先順序解析：

1. **個別 extra 的 `source`**（最高）— 直接使用該精確路徑
2. **`extras_source`** — `<extras_source>/<name>/`
3. **預設值** — `~/.config/skillshare/extras/<name>/`（global）或 `.skillshare/extras/<name>/`（project）

`extras list --json` 的輸出包含 `source_type` 欄位（`per-extra`、`extras_source`，或 `default`），表示路徑是由哪一層解析出來的。

:::tip Auto-populated
執行 `skillshare init`，或用 `extras init` 建立你的第一個 extra 時，`extras_source` 會自動被設為預設路徑（`~/.config/skillshare/extras/`）。之後若要變更，請使用 `skillshare extras source <path>`。
:::

---

## Syncing

Extras 透過以下方式同步：

```bash
skillshare sync extras        # 只同步 extras
skillshare sync --all         # 同時同步 skills 與 extras
```

完整的 sync 文件（包含 `--json`、`--dry-run`、`--force` 選項）請參閱 [sync extras](/docs/reference/commands/sync#sync-extras)。

---

## Workflow

```bash
# 1. 建立新的 extra
skillshare extras init rules --target ~/.claude/rules --target ~/.cursor/rules

# 1b. 或使用自訂的 source 目錄
skillshare extras init rules --target ~/.claude/rules --source ~/my-rules

# 1c. 重新設定既有的 extra（覆寫）
skillshare extras init rules --target ~/.cursor/rules --force

# 2. 把檔案加入 source 目錄
# （編輯解析出來的 source 目錄——可用 skillshare extras list --json 檢查）

# 3. 同步到 target
skillshare sync extras

# 4. 列出狀態（source_type 顯示每個 extra 的 source 是從哪裡解析出來的）
skillshare extras list

# 5. 把在 target 中編輯過的檔案收集回 source
skillshare extras collect rules --from ~/.claude/rules

# 6. 變更全域的 extras source 目錄
skillshare extras source ~/company-shared/extras
```

---

## See Also

- [sync](/docs/reference/commands/sync#sync-extras) — 把 extras 同步到 target
- [status](/docs/reference/commands/status) — 顯示 extras 的檔案與 target 數量
- [Configuration](/docs/reference/targets/configuration#extras) — Extras 設定參考
