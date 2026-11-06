---
sidebar_position: 21
---

# project

管理 global config 會同步 skills 與 agents 的專案資料夾，不必在每個資料夾建立 `.skillshare/`。除非選擇只保存在本機，專案都儲存在 `config.yaml` 中。

`project` 一律使用 global config，即使在 project 內執行也一樣。這和 `-p` / `--project` 不同：後者會選擇 repo 自己的設定，而且會被這個指令拒絕。如果隊友應該隨著 repo 一起拿到這套設定，請使用 [project mode](/docs/understand/project-skills)。

## 用法 {#usage}

```bash
skillshare project                          # Same as project list
skillshare project list --json
skillshare project show <project> --json
skillshare project add <root> --target T [options]
skillshare project edit <project> [options]
skillshare project sync <project> [--dry-run] [--json]
skillshare project localize <project> [--dry-run] [--json]
skillshare project share <project> [--yes] [--dry-run] [--json]
skillshare project remove <project> [--shared] [--yes] [--dry-run] [--json]
```

`add` 需要一個已存在的資料夾，以及至少一個 target。其他指令接受 project 的有效名稱（自訂名稱或資料夾 basename）或它的根路徑。要明確指定根路徑，請在相對路徑前加上 `./` 或 `../`。如果一個沒有前綴的參數同時符合某個 project 的名稱和另一個 project 的根路徑，指令會回報這兩個候選，而不會替你挑一個。Symlink 別名會維持為各自獨立的註冊。

## 旗標 {#flags}

| 旗標 | 適用於 | 說明 |
|------|------------|-------------|
| `--target T` | add, edit | Target 工具；可重複指定或使用逗號清單。add 時為必填；edit 時會取代整個清單 |
| `--name N` | add, edit | 自訂名稱；`--name=` 會還原為資料夾 basename |
| `--skills`, `--no-skills` | add, edit | 啟用或停用 skills；add 時預設啟用 |
| `--agents`, `--no-agents` | add, edit | 啟用或停用 agents；add 時預設停用 |
| `--skills-mode MODE`, `--agents-mode MODE` | add, edit | `merge`、`symlink` 或 `copy`；空值會清除覆寫設定 |
| `--skills-naming NAMING` | add, edit | `flat`、`standard` 或 `prefixed`；空值代表沿用繼承值 |
| `--skills-include P`, `--skills-exclude P` | add, edit | 重複指定以提供多個 glob pattern；會取代對應的清單；空值會清空清單 |
| `--agents-include P`, `--agents-exclude P` | add, edit | Agents 套用相同的取代規則；filters 需要 merge 或 copy 模式 |
| `--machine-local` | add | 只儲存在本機，不寫入 `config.yaml` |
| `--shared` | add, remove | add：即使 [`projects_storage`](#local-registrations) 為 `machine-local`，也儲存到 `config.yaml`。remove：同時移除本機所覆寫專案在 `config.yaml` 中的副本 |
| `--sync` | add, edit | 儲存後同步這個 project 的 skills 與 agents；不啟用 force |
| `--dry-run`, `-n` | add, edit, sync, remove, localize, share | 預覽，不寫入也不提示確認 |
| `--yes` | remove, share | 略過確認；非預覽情況下以 JSON 輸出移除時必須提供。share 只在 `config.yaml` 中已有該專案的不同設定時才會詢問 |
| `--json` | all | 結構化輸出；也請一併檢查程序的結束碼 |
| `--global`, `-g` | all | 明確指定 global 範圍（也是這個指令的預設值） |
| `--help`, `-h` | all | 顯示說明 |

指定資源設定會啟用該資源，包括清除既有的 `enabled: false`。省略設定時會保留其停用狀態。Edit 只會變更你提供的設定；啟用既有資源時會保留它的設定。互相衝突的開關，以及停用與設定的組合（例如 `--no-skills --skills-mode copy`）都會被拒絕。沒有 agents 命名方式的旗標。和 [`target`](/docs/reference/commands/target) 不同，這些 include/exclude 旗標會取代整個清單，而不是逐一新增或移除 pattern。

## 範例 {#examples}

### 註冊、預覽與同步 {#register-and-sync}

```bash
skillshare project add ~/work/shop-web --target claude,codex \
  --skills-mode copy --skills-include 'frontend-*' --agents --dry-run
skillshare project add ~/work/shop-web --target claude,codex \
  --skills-mode copy --skills-include 'frontend-*' --agents --sync
skillshare project show shop-web --json
```

專案儲存在 `config.yaml` 的 [`projects`](/docs/reference/targets/configuration#projects) 下，因此使用該檔案的每台機器都會取得它。對於只存在於本機的資料夾，請加上 `--machine-local`；請參閱[專案的儲存位置](#local-registrations)。共用的工具資料夾會合併為一個 sync target。一般的 `skillshare sync` 也會同步所有專案。

停用 skills 的 project 永遠不會執行 skills 同步流程，在 global symlink 模式下也一樣。請求或啟用的資源選擇為空時，會直接成功，不進行探索、備份或 target 寫入。

### 取代 filters 或重設覆寫設定 {#edit-settings}

```bash
skillshare project edit shop-web --skills-include 'frontend-*' --skills-include 'core-*'
skillshare project edit shop-web --skills-exclude= --skills-mode= --name=
skillshare project sync shop-web --dry-run --json
skillshare project sync shop-web --json
```

Add 與 edit 會在寫入前驗證完整的有效設定。編輯會儲存在專案所在的位置：`config.yaml`，或僅限本機專案所在的本機。在 `config.yaml` 中，編輯透過 YAML anchor 宣告的專案，也會改變該 anchor 的別名。變更結果會列出受影響的根路徑。沒有任何變化的 add 或 edit 會回傳 `applied: false` 與 `unchanged: true`，但仍會執行 `--sync`。以不同設定重複執行 add 會失敗，並提示改用 `project edit`。自訂名稱不能看起來像根路徑（例如 `~name`、`./name` 或 `C:name`）。

### 移除專案 {#remove}

```bash
skillshare project remove shop-web --dry-run --json
skillshare project remove shop-web --yes --json
```

移除會從專案所在的位置刪除它。從 `config.yaml` 移除專案，會讓所有使用該檔案的機器都失去該專案，確認提示中也會說明這一點。如果本機覆寫了 `config.yaml` 中的專案，移除只會刪除本機的副本：`config.yaml` 中的版本會重新生效，JSON 結果會在 `remaining` 中回報。加上 `--shared` 則兩者都移除。

移除時會先分離 skills，再移除專案。整個資料夾的連結會被移除；merge 模式的連結在 source 內容仍存在時會變成本機副本；copy 模式的檔案會保留，只移除其 manifest。仍被某個存續且啟用的 target 擁有的路徑會被保留。

CLI edit 會依相同規則分離被移出的 skill 路徑。只要專案的 skills 仍停用，任何 CLI 編輯（包括改名或僅修改 Agent 設定）也會分離其殘留的 skill 路徑，即使先前是在 Dashboard 中停用也一樣。複製回本機及保留存續且啟用的 target 所擁有路徑的規則仍然適用。Dashboard 儲存不會分離 skills。Agent 檔案會原地保留。預覽與結果會列出被移除、複製回來以及被保留的路徑。

## JSON 與失敗處理 {#json-and-failures}

| 指令 | 主要欄位 |
|---------|-------------|
| list | `projects` 陣列（沒有註冊時為空） |
| show | `project` 物件 |
| add, edit, remove, localize, share | `action`、`project`、`applied`、`affected`、`warnings`；可選的 `sync`、`detach` 或 `remaining`（remove） |
| sync | `action`、`project`、`sync`；預覽另外包含 `applied: false` |

Project 物件包含 `root`（儲存時的寫法）、`path`（展開後的路徑）、`name`、`targets`、`skills`、`agents`、`groups`、`missing`、`hasOwnConfig`、`origin`（`shared` 或 `local`）、`originPath`（所屬檔案）與 `sharedCopy`（`config.yaml` 中也宣告了的本機專案）。省略或停用的資源為 `null`。啟用的資源會列出其設定。文字格式的 `project list` 與 `project show` 會回報開/關狀態。Sync 結果包含 `results`、`failed`、`warnings`，以及探索與 filter 的診斷資訊。

某個 target 失敗時，sync 會回報已完成的 targets，並以結束碼 1 結束。Sync 預覽會在 `failed` 中回報衝突並以結束碼 0 結束；無效的輸入仍然會失敗。如果 add/edit 儲存成功但 sync 失敗，註冊會維持已儲存的狀態；修正 target 後再用 `project sync` 重試。找不到的 project 資料夾仍可以 edit，但無法 sync。

Edit 與移除並非交易式操作：後續的分離或儲存失敗，可能會留下先前已完成的檔案系統變更。JSON 結果會回報已完成的 `detach`、`registration`（`kept`、`updated` 或 `removed`）以及 `error`。Edit 儲存失敗時為 `applied: false`，並描述已持久化的 project；儲存成功但之後 sync 失敗時仍為 `applied: true`。重試前請先檢查這些欄位。在任何變更發生前的失敗會使用錯誤 payload。所有 project 命令都略過啟動遷移、清理與版本檢查。

這個指令只管理 skills 與 agents。MCP 的 project 設定與 hooks 使用各自的指令；它不會轉換 targets，也不會編輯 project 自己的設定。

## 專案的儲存位置 {#local-registrations}

專案儲存在以下兩處之一：

- **`config.yaml`**（`origin: shared`，預設）。使用該設定檔的每台機器都會取得該專案，例如透過 dotfiles 或 `skillshare push`。
- **僅限本機**（`origin: local`）。位於設定目錄之外的 registry，適用於只存在於一台機器上的資料夾。

`project add` 預設儲存到 `config.yaml`，除非傳入 `--machine-local`。若要將僅限本機設為預設，請在 `config.yaml` 中設定 [`projects_storage: machine-local`](/docs/reference/targets/configuration#projects-storage)；此時 `--shared` 可把單一專案儲存到 `config.yaml`。儀表板的 **新增專案** 對話框提供同樣的選擇，並以該設定作為初始值。編輯與移除都在專案所儲存的位置進行。

在兩處之間移動已儲存的專案：

```bash
skillshare project localize shop-web --dry-run --json   # keep a copy on this machine only
skillshare project share shop-web --dry-run --json      # move it into config.yaml
```

`localize` 會把 `config.yaml` 中的專案複製到本機，不會變更 `config.yaml`、解除連結或同步。其他機器仍使用 `config.yaml` 的版本；本機則使用自己的副本。`share` 會把僅限本機的專案寫入 `config.yaml`，並刪除本機副本。當 `config.yaml` 中已有同一專案但設定不同時，`share` 會先詢問再取代；`--yes` 則不詢問直接取代。如果專案已經在目標位置，兩者都不會做任何事，也都不會同步。

本機專案會完整取代相同正規化根目錄（展開 `~`、清理路徑；專案符號連結根目錄各自獨立）的 `config.yaml` 項目。v0.29.0 儲存的移除操作會在該機器上隱藏 `config.yaml` 中的專案，在重新新增該專案之前會一直保持隱藏。不同根目錄的名稱必須唯一，同一層的重複正規化根目錄會被拒絕。無論專案本身儲存在哪裡，專案的 MCP servers 與 hooks 一律位於 `config.yaml` 的 `mcp.projects` 與 `hooks.projects` 下。

儲存位置為 `$XDG_STATE_HOME/skillshare/projects/<identity>.yaml`；未指定時，Unix 使用 `~/.local/state/skillshare/projects`，Windows 使用 `%LOCALAPPDATA%\skillshare\state\projects`。請保持狀態目錄為本機位置。識別碼依解析後的設定檔目的地決定；設定檔符號連結別名共用狀態，搬移目的地會使用新註冊。讀取不建立檔案，無效版本或內容會拒絕載入。

儀表板的 **Convert** 動作會在一次寫入中，把一般 targets 取代為 `config.yaml` 中的 `projects` 宣告。已經宣告、被本機覆寫或在本機被隱藏的根目錄無法轉換。原始設定編輯器會在寫入前，把提交的 YAML 與本機專案合併後再驗證。

## 並行寫入 projects {#concurrency}

CLI 與儀表板共用本機狀態鎖，最多等候 10 秒並在取得後重新載入。候選資料會在解除連結與寫入前檢查共用與本機快照；其他共用設定寫入者不受此保護。儀表板 Convert 是明確的共用設定變更，會拒絕已宣告、隱藏或本機覆蓋的根目錄。原始設定編輯器會先驗證提交內容與本機項目的合併結果。 像 `share` 這樣同時變更兩處的操作會先寫入 `config.yaml`，再寫入本機 registry，因此寫入被中斷時也不會遺失專案。

## 相關 {#related}

- [多個 Projects，一份 Config](/docs/how-to/recipes/many-projects-one-config)
- [Project 設定欄位](/docs/reference/targets/configuration#projects)
- [MCP project 設定](/docs/reference/commands/mcp#manage-several-projects-from-the-global-config)
