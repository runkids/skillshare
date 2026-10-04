---
sidebar_position: 4
---

# plugin

在支援的工具間管理完整的原生 plugin。Capability 檢查會區分安裝支援與格式探索(format discovery)。從
[跨工具管理 plugin](/docs/how-to/daily-tasks/sharing-plugins) 開始。

```bash
skillshare plugin                         # Interactive manager
skillshare plugin add                     # Source → plugin → targets → review
skillshare plugin discover ./my-plugin --json
skillshare plugin add ./my-plugin --target claude --target codex --no-tui
skillshare plugin add ./my-plugin --no-tui   # 先交給 Skillshare 管理，之後再選 target
skillshare plugin import review@team --from claude --no-tui
skillshare plugin list --json
skillshare plugin inspect review --json
skillshare plugin disable review --target codex --no-tui
skillshare sync plugins --dry-run --json
skillshare sync plugins --no-tui
skillshare plugin enable review --target codex --no-tui
skillshare plugin check review --json
skillshare plugin update review --target claude --no-tui
skillshare plugin remove review --no-tui
```

`enable` 和 `disable` 改變的是 **Skillshare 的同步選擇**，而非 Agent 原生的啟用狀態。取消選取某個
target 會儲存這個選擇。下一次 `sync plugins` 會移除其受管理的安裝，但保留定義。重新選取後，
下一次 sync 就會重新安裝。未受管理的 plugin 不受影響。

`add` 不帶 `--target`（或在互動選單中什麼都不選）會把 plugin 交給 Skillshare 管理，但不安裝到任何地方。之後可以用相同的 source 與 `--name` 加上 target，或在 dashboard 中從該 plugin 的列勾選；安裝的是當下的 source 內容。這類 plugin 在最後一個 target 被移除後仍會保留。不帶 `--target` 的 `remove NAME` 會把它從 Skillshare 移除。

## Commands

| Command | Behavior |
|---|---|
| `list` | 已設定的綁定與原生安裝狀態；在終端機中提供互動式管理介面 |
| `discover SOURCE` | 檢查本機目錄、`owner/repo`，或 HTTPS Git repository |
| `add [SOURCE]` | 選擇並安裝整個 plugin 及其 target adapter，或透過 Pi 安裝 `npm:` 套件 |
| `import [NATIVE-ID]` | 採用既有安裝，不重新安裝也不啟用 |
| `inspect NAME` | 檢查單一受管理的套件 |
| `sync [NAME]` | 協調已選的 target 並重試未完成的原生操作 |
| `check [NAME]` | 比對 source 內容與記錄的 digest，或 Pi npm 套件版本與 npm 最新版；絕不更新 |
| `update [NAME]` | 檢視 source 變更並使用支援的原生更新操作 |
| `enable / disable [NAME]` | 在下一次 sync 中納入/排除某個 target |
| `remove [NAME]` | 解除安裝受管理的綁定並移除其定義 |

在終端機中執行不帶參數的指令會提示輸入缺少的內容。非互動式的異動指令則需要明確的輸入。
`sync` 和 `check` 可以對所有套件操作。`sync --all` **不會**包含 plugin；請明確使用 `sync plugins`。

## Options

| Option | Meaning |
|---|---|
| `--target TARGET` | 可重複指定：`claude`、`codex`、`cursor`、`antigravity`（別名 `agy`）、`antigravity-cli`、`copilot`、`grok`、`pi`、`opencode`，或[某個 Agent 的另一個帳號](#accounts)的名稱；請參閱下方的 capability 表 |
| `--plugin NAME` | 從 source marketplace 選擇一個 plugin |
| `--name NAME` | 新增或匯入時使用的邏輯套件名稱 |
| `--from TARGET` | 從 Claude、Codex、Antigravity CLI、Copilot、Grok、Pi、OpenCode 或[某個 Agent 的另一個帳號](#accounts)匯入 |
| `--dry-run`, `-n` | 預覽而不變更 Skillshare 或 Agent 設定 |
| `--source-ref REF` | 用於 `discover`、`add`、`update` 的 Git branch、tag 或 commit；僅限遠端 source |
| `--entry PATH` | 明確指定已建置的 OpenCode JS/TS 進入點，相對於套件根目錄（`discover` 與 `add`） |
| `--revision ID` | 若自預覽以來 source、設定或原生清單已變更，則拒絕套用 |
| `--json` | 機器可讀輸出；停用 TUI |
| `--no-tui` | 停用互動式選單；同時遵循 `tui: false` |
| `--global`, `-g` | 全域 Skillshare 設定與原生使用者範圍 |
| `--project`, `-p` | 專案設定；Claude、Antigravity、Pi 或 OpenCode（絕不會回退到全域） |

JSON 輸出包含 source 路徑與原生識別碼。請勿在 source URL 中放入憑證。即使某次異動失敗，
仍可能回傳部分 target 的成功結果；只要有任一 target 失敗，CLI 就會以非零狀態碼結束。
重試前請先檢查結果。

## Target support

| Target | Format | Global | Project | Update |
|---|---|:---:|:---:|---|
| Claude Code | `.claude-plugin/plugin.json` | Yes | Yes | 原生更新 |
| Codex | `.codex-plugin/plugin.json` 或 Agent Plugins 根目錄 manifest | Yes | No | 重新整理已審閱的來源後再次 add，僅限在 Codex 中為啟用狀態 |
| Cursor | `.cursor-plugin/plugin.json` 或 Agent Plugins 根目錄 manifest | Yes | No | 取代已檢視的本機複本 |
| Antigravity Desktop | 帶有明確名稱的根目錄 `plugin.json` | Yes | Yes | 取代已檢視的本機複本 |
| Pi | 含 `pi` resources 的 `package.json`，或帶有 `pi-package` 關鍵字與慣用 resource 資料夾 | Yes | Yes，需原生專案信任 | 重新整理受管理的 source 快照 |
| OpenCode | 含 SDK 依賴的 `package.json`、`.opencode/plugins/` 項目，或明確的 `--entry` | Yes | Yes | 重新整理受管理的 source 快照 |

| Antigravity CLI | `agy` 接受的原生根目錄 manifest 或 Claude manifest | Yes | No | 以原生方式更新以保留啟用狀態 |
| GitHub Copilot CLI | `.plugin/plugin.json`、`.github/plugin/plugin.json`、Claude manifest，或 Agent Plugins 根目錄 manifest | Yes | No | 重新整理已檢視的 source，僅在原生啟用狀態已知且為啟用時 |
| Grok Build | `.grok-plugin/plugin.json` 或 Claude manifest | 僅限匯入/移除；安裝需要原生信任 | No | 以原生方式更新 |
| Kimi Code | `kimi.plugin.json` 或 `.kimi-plugin/plugin.json` | 僅限探索 | No | 未自動化 |
| Hermes | `.hermes-plugin/plugin.yaml` | 僅限探索 | No | 未自動化 |
| Devin | `.devin-plugin/plugin.json` | 僅限探索 | No | 未自動化 |

Kimi 的非互動式生命週期、Hermes 的 profile 清單/同意流程，以及 Devin 的本機清單/信任/雲端
區分，這些 adapter 尚未驗證。它們的格式會在探索階段顯示，但安裝功能會附上原因被停用。
source 宣告某個 target 不代表 Skillshare 就能管理它。`list --json` 與 `discover --json` 會包含
`targetDefinitions`，列出允許的操作；探索階段也會針對每種格式揭露 `targetInfo`，內容包含
版本、元件、進入點與驗證問題。損壞的 manifest 只會影響該 target；損壞的 catalog 則以警告方式
回報，不會隱藏有效的格式。

### 某個 Agent 的另一個帳號 {#accounts}

宣告為[某個 Agent 的另一個帳號](/docs/reference/targets/configuration#agent-config-dir)的 target 同時也是 plugin target，適用於 `claude`、`codex` 與 `pi`。Skillshare 會透過 `CLAUDE_CONFIG_DIR`、`CODEX_HOME` 或 `PI_CODING_AGENT_DIR`，對該帳號的 config 目錄執行該 Agent 自己的 CLI：

```yaml
targets:
  claude-work:
    agent: claude
    config_dir: ~/.claude-work
```

```bash
skillshare plugin add owner/repo --target claude-work
skillshare plugin import demo@market --from claude-work
```

設定了 [`cli`](/docs/reference/targets/configuration#agent-config-dir) 的帳號會改用那個相容的 CLI，例如 Pi 帳號用 `omo`，並針對同一個 config 目錄執行。Pi 帳號還會設定 `SENPI_CODING_AGENT_DIR` 與 `OMO_CODING_AGENT_DIR`，因為 Pi 的 fork 會先讀這兩個變數，再讀 `PI_CODING_AGENT_DIR`。找不到 CLI 時操作會失敗；Skillshare 不會改用 Agent 本身的 CLI。

該帳號沿用它所屬 Agent 的操作方式，並以自己的名稱保存自己的綁定，因此同一個 plugin 可以只裝在其中一個帳號。帳號只存在於 global 範圍：專案的 plugin 屬於該專案，而不屬於某個帳號。`--target` 與 `--from` 都接受帳號名稱，終端機選單與 dashboard 的 Plugins 頁面也會把它列在 Agents 旁邊。

### Cursor and Antigravity

這些 adapter 會將整個 plugin 複製到文件記載的本機探索目錄中，不需要 CLI 執行檔，也不會修改
marketplace registry：

- Cursor：`~/.cursor/plugins/local/<name>`；必須允許本機匯入。重新載入 Cursor 並檢查 Customize。
  已從 marketplace 安裝、名稱相同的 plugin 優先於本機複本。
- Antigravity desktop：全域為 `~/.gemini/config/plugins/<name>`；workspace 中為
  `.agents/plugins/<name>`（或既有的 `_agents/plugins/` 目錄）。若兩個 workspace 目錄都存在，
  請先整合它們。
- 獨立的 **agy CLI** 有自己獨立的 plugin 儲存庫。`antigravity` target 管理的是 desktop/workspace
  的探索路徑，不是那個 CLI 的儲存庫。`agy` 只是 Skillshare target 的別名。使用
  `--target antigravity-cli` 代表獨立的 CLI；`--target agy` 則維持其既有的 desktop 意涵。

對這類本機套件請使用 `plugin add`。不支援匯入既有的本機資料夾或 marketplace 安裝。Skillshare
拒絕覆寫非自身擁有的資料夾、symlink，或本機已編輯過的受管理內容。明確的 Antigravity manifest
名稱能讓身分識別在 Git checkout 與快照之間保持穩定。

### Pi and OpenCode

Pi 保留同一套件的第一筆全域登錄與最後一筆 project 登錄。若較早的全域來源或較晚的 project 來源無法確認 identity，Skillshare 無法證明哪筆登錄有優先權，因此可能被覆蓋的項目（包括繼承的 project delta）會保持 Unknown／唯讀，不會藉由刪除 URL query 來猜測 identity。不受這項不確定性影響、已確認的項目仍可編輯。

Pi 使用 `pi install` / `pi remove`；清單讀取的是文件記載的套件設定，不會載入 extension 程式碼。
會遵循 `PI_CODING_AGENT_DIR`。Pi 的專案信任必須在 Pi 中自行建立；Skillshare 不會替你傳遞
`--approve`。

#### pi.dev 的 npm 套件

`plugin add npm:<套件>` 會透過 Pi 本身安裝發布在 npm 上的套件，例如 [pi.dev](https://pi.dev/packages) 列出的套件：

```bash
skillshare plugin add npm:@scope/package --target pi --dry-run --json -g
skillshare plugin add npm:@scope/package@1.2.0 --target pi --no-tui -g
```

Pi 會下載套件並執行它的 install script，Skillshare 無法事先檢查內容；加入前請先在 pi.dev 或 npm 確認套件。`discover` 不接受 npm 來源，npm 來源也不接受 `--source-ref`、`--entry` 或 `--plugin`。只有 Pi target 能接受 npm 來源，包括執行 `pi` 的 Pi 帳號；執行其他執行檔的帳號，請用那個執行檔安裝後再匯入。搭配 `--project` 時，Pi 會把套件裝進專案的設定；專案一旦有 `.pi` 資料夾，要先在 Pi 信任這個專案，Pi 才會修改它的套件。

Pi 每個套件名稱只保留一筆。Pi 已經有相同來源時，`add` 會匯入它；同一套件的其他版本則會安裝，由 Pi 替換那一筆的來源。`update` 會執行 `pi update`，但釘在精確版本的套件 Pi 會維持原版本，請改用新版本重新加入。對於沒有指定版本就加入的套件，`check` 會拿已安裝套件 `package.json` 裡的版本，和公開 registry 上 npm `latest` 標籤指向的版本比較；`update` 遇到已經是該版本的套件時不做任何事。帶版本範圍或標籤加入的套件、被環境變數或 `.npmrc` 指向其他 registry 的套件，或版本不是單純 `X.Y.Z` 的套件，會顯示為需要在 Pi 裡檢查。dashboard 會在 Plugins 頁面和 Pi target 的 Extensions 分頁顯示每個 Pi 套件的已安裝版本。如果你關掉了套件裡的某些 extension，Pi 會把這些規則保留到新版本，Skillshare 也會重新記錄，之後重裝時會還原。已經由另一個 Skillshare 套件管理的 Pi 套件會被拒絕，請改為更新或移除那一個。

在 dashboard 的新增對話框，可以直接貼上 `pi install npm:<套件>` 指令或套件的 pi.dev 網址，兩者都會轉成對應的 `npm:` 來源。

#### 選擇套件的 extension

在 dashboard 中，`pi` 與 Pi 帳號的 target 頁面有一個 **Extensions** 分頁。它列出該 target 的 `settings.json` 中每個套件項目，以及其篩選規則選取的 extension。開關會在該項目的 `extensions` 清單寫入一條精確的 `+path` 或 `-path` 規則；但如果刪除該檔案自己的精確規則就能得到開關要的狀態，開關會改為刪除那條規則，所以把檔案切回其餘規則選取的狀態時不會留下規則。**Remove rule** 只在開關不會刪除規則時顯示，它會刪除該檔案的精確規則（無論寫成相對或絕對路徑），之後該檔案依其餘規則決定；結果會顯示在預覽中。套用前一定會先顯示預覽，而且只修改這些清單：項目的其他鍵、`skills`、`prompts` 與 `themes` 篩選規則、glob 與 `!` 規則，以及檔案的其餘部分都維持原樣。字串項目會變成 `{"source": ...}`，以便放入規則。對字串項目，Pi 只從套件的 `pi` manifest 讀取 skills、prompts 與 themes；物件項目則會在 manifest 沒列出時，從套件的 `skills`、`prompts`、`themes` 資料夾載入它們。有這類資料夾的套件，其字串項目是唯讀的，因為 Skillshare 無法確認轉換後這些資源維持原狀。指向單一檔案的來源也是唯讀的，因為 Pi 會直接載入它並忽略篩選規則。如果預覽後檔案已被修改，或 Pi 正持有設定鎖，就不會寫入任何內容。寫入期間 Skillshare 會以和 Pi 相同的方式持有這個鎖，一旦失去就不寫入。每次套用前都會保存一份變更的 extension 清單及檔案變更前後雜湊的紀錄。成功套用的紀錄會保留，不會自動清理；套用失敗時只移除該次新建的紀錄。這不是 `settings.json` 的副本，無法用來還原整份設定檔。這個分頁會列出設定裡的所有套件，包括直接用 Pi 安裝的，例如 [pi.dev](https://pi.dev/packages) 上的 `npm:` 套件。Skillshare 只透過 `plugin` 安裝與移除套件：`plugin add` 接受本機目錄、Git 來源或 [npm 套件](#pidev-的-npm-套件)，`plugin import --from pi` 則可接管用 Pi 安裝的套件。

Skillshare 讀取套件時不會執行它們，因此這個分頁顯示的是設定選取了哪些檔案（**設定**欄），而不是 Pi 是否已載入它們；套用後請重新載入 Pi。設定指名但套件中不存在的檔案會標示為不存在。Skillshare 無法判斷的選擇會顯示**無法判斷**，並附上原因與修改方式，絕不猜測開或關。編輯需要該 target 自己的 Pi 是 0.99.2 以上（以 Pi 本身驗證過的最舊版本），且設定是嚴格的 JSON；較舊的版本為唯讀，分頁會顯示偵測到的版本。執行其他程式的 Pi 帳號為唯讀，Skillshare 也不會執行它。清單為 `[]`（不載入任何檔案）的項目是唯讀，由 Skillshare 無法評估的模式（例如 `?` 對上 emoji）決定的 extension 也是唯讀。來源為空的項目，或來源、規則中含有未配對的 UTF-16 surrogate 跳脫或無效 UTF-8 的項目，因為 Skillshare 無法和 Pi 一樣準確讀取，會維持原樣並設為唯讀。Pi 只採用套件的第一個全域項目，所以當 Skillshare 無法讀取那個項目時，同一套件後面的項目也是唯讀。

同步到 Pi 的專案在專案頁面上也有這個分頁。它顯示專案設定疊加在全域設定之上後，每個套件選取的內容，並標示是繼承自 `pi (global)` 還是專案覆寫。開關只會把規則存到專案的 `.pi/settings.json`，做法和 `pi config` 相同：全域套件會得到一個專案項目 `{"source": ..., "autoload": false, "extensions": [...]}`，只改變它指名的檔案，全域項目維持原樣。本機來源會寫成相對於 `.pi` 的路徑，npm 或 git 來源則照全域設定的寫法。移除這類項目的最後一條專案規則時，只有不會讓較早登錄的 filters 生效才移除該項目；否則保留空的 winning override。只有明確的 JSON `false` 才表示 delta，`autoload: null` 不是 `false`。沒有對應全域項目、且 `autoload: false` 的專案項目只會載入它用 `+` 指名的檔案。檔案與其 `.pi` 資料夾只會在套用時建立。全域設定與 Pi 的 `trust.json` 永遠不會被寫入，Skillshare 也不會替你信任專案：Pi 只有在信任專案時才會使用專案設定。含有憑證或查詢字串的全域來源不會被複製到專案，所以該套件在專案中是唯讀；專案設定中有 Skillshare 無法讀取的項目時，所有套件都是唯讀。套用時會持有 Pi 對專案檔案的鎖，並在寫入前再次檢查兩個設定檔與套件。Pi 自己的 `extensions` 資料夾中的 extension（包括 [extras](./extras.md) 連結到那裡的檔案）以唯讀方式列出，並說明在哪裡修改；專案會列出自己的資料夾（Pi 只有在信任專案時才會讀取）和全域資料夾。

OpenCode 會在 `opencode.json` 或既有的 `opencode.jsonc` 中，把受管理的進入點註冊為 file URL，
並保留註解與無關的項目。Version 1 使用 `plugin`；version 2 使用 `plugins`。會遵循
`XDG_CONFIG_HOME` 與絕對路徑的全域 `OPENCODE_CONFIG`；模糊或不支援的覆寫會被拒絕。OpenCode
必須位於 PATH 上，Skillshare 才能選擇對應版本的 schema。

本機 OpenCode source 必須已經包含建置好的進入點（`main`、字串形式的 root export，或
`index.js`）以及所需的執行期依賴。Skillshare 不會執行建置腳本，也不會將依賴安裝進 source。
註冊成功不代表模組已成功載入；請在重新載入後檢查 OpenCode。

全域 npm 登錄沒有 managed cache 時顯示 Unknown／唯讀，不代表尚未安裝；Pi 可能使用 Skillshare 不探查的 legacy global npm/pnpm 路徑。

匯入接受一般 Pi source，以及Pi 0.99.2 以上來源與選項格式受支援的 filtered object。預覽只顯示保留的欄位名稱，不顯示 opaque 值。匯入不修改原生設定或已安裝檔案；原始項目保存在 Skillshare 私有狀態，共用設定僅存 digest。sync/update 保留現有項目；解除安裝前保存最新選項，重新安裝時先恢復 object，避免暫時以預設規則啟用其他資源。同一次 Apply 批次恢復時，只接受此次操作自己寫出的精確內容；其他設定變動仍會阻止後續恢復。這些 bindings 必須保留私有狀態：紀錄遺失、被修改或屬於其他 target 時拒絕恢復。Windows 的新登錄目錄以受保護的 owner/SYSTEM ACL 建立。現有目錄與紀錄若允許目前使用者及特權 SYSTEM/Administrators 以外的主體存取，或無法驗證 ACL，便拒絕匯入或恢復。不會修改現有 ACL；請保留紀錄，由擁有者修復存取保護後再重試。不確定的來源或優先序、不支援的編碼，以及 Pi 會正規化的本機參照仍為唯讀。一般 OpenCode 項目可匯入，filtered OpenCode 項目仍拒絕。超過 Pi 10 秒過期門檻的空鎖目錄，只有 inode 與 mtime 未變動時才能回收；新鎖、更新或被替換的鎖、非空目錄、檔案與 symlink 一律保留。過期不代表擁有者已終止，最後檢查與移除不是原子 CAS。已匯入的 Pi 套件在全域模式下以 `pi update SOURCE` 更新，並保留其設定項目；專案中的則要在 Pi 裡更新，因為 `pi update` 也會動到全域套件。已匯入的 OpenCode v1 套件會在其原生工具中更新。OpenCode
v2 的全域匯入可以使用其原生更新指令；專案匯入則必須以原生方式更新，因為 v2 的更新指令是
全域性的。

```bash
skillshare plugin add ./cursor-plugin --target cursor --no-tui
skillshare plugin add ./agy-plugin --target agy --no-tui -p
skillshare plugin add ./pi-package --target pi --no-tui
skillshare plugin add ./opencode-package --target opencode --no-tui
skillshare plugin import npm:my-pi-package --from pi --name my-package --no-tui
```

## Refs and explicit entries

**Add plugin** 中的進階選項可接受選填的 Git ref 與 OpenCode entry。一般的引導流程可以留空。
終端機精靈接受相同的旗標；自動化流程則可以使用：

```bash
skillshare plugin discover obra/superpowers --source-ref v6.3.0 --json
skillshare plugin add owner/repo --source-ref v1.0.0 --target copilot --no-tui
skillshare plugin add ./package --entry dist/plugin.js --target opencode --no-tui
skillshare plugin update review --source-ref v1.1.0 --target claude --dry-run --json
```

綁定會記錄 `source_ref` 以及解析後的 `commit`。安裝使用的是已檢視過的 commit；`check` 與
`update` 會重新解析設定的 ref，因此某個 branch 可以持續前進，而 commit 仍維持固定。`--revision`
是預覽用的 token，不是 Git ref。`--entry` 是相對於各候選項套件根目錄的路徑，且必須已經存在；
它不會觸發 build 或 package manager 安裝。

Copilot 與 Antigravity CLI 的安裝使用已檢視過的本機快照。匯入沒有可供重新安裝的已檢視 source：
移除後，請在原生用戶端中安裝並再次 sync。Grok 也需要原生信任才能安裝/重新安裝。Skillshare
絕不會提供原生信任核准旗標。

## Compatibility and boundaries

- Claude 需要其原生的 `.claude-plugin/plugin.json` 套件。
- Codex 接受 `.codex-plugin/plugin.json` 以及可辨識的可攜式根目錄 `plugin.json` 套件。僅限
  Claude 的套件不會被靜默轉換。
- Source 可能包含帶有本機 plugin 項目的 marketplace。外部 catalog 會依 plugin 名稱/路徑合併。
  衝突的路徑會被拒絕；外部項目會附上說明，指引你直接加入其 repository，或以原生方式安裝後
  再匯入。以指令為基礎的 source 不會自動核准。
- 完整的 source 快照會保留 plugin 腳本、資產，以及安全的相對 symlink（包含
  `AGENTS.md → CLAUDE.md`）。絕對路徑、逃逸路徑、懸空、循環，以及參照 `.git` 的連結與特殊檔案
  都會被拒絕；source 上限為 20,000 個檔案與 100 MiB。
- 原生安裝不代表已在執行期啟用。請重新啟動/重新載入該 Agent，並在該 Agent 中完成驗證或
  hook 信任。
- Codex 會從 `PATH` 執行 `codex`。不在 `PATH` 上時，Skillshare 會依序嘗試 Homebrew 的
  `/opt/homebrew/bin` 和 `/usr/local/bin`，再來是 Codex 桌面 app 內附的 CLI
  （macOS 的 `ChatGPT.app` 或 `Codex.app`、Windows 的 `%LOCALAPPDATA%\OpenAI\Codex\bin`）。Codex 裝在其他位置的機器，
  請設定 [`SKILLSHARE_CODEX_CLI`](/docs/reference/appendix/environment-variables#skillshare_codex_cli)。自己設了 `cli` 的帳號不會進行搜尋。
- 此 adapter 不提供 Codex 原生的專案安裝。全域 Codex 安裝的 sync 選擇仍然可用。
- Codex 沒有 update 指令，因此更新會以重新整理後的快照再次 add 該 plugin。add 一定會啟用它，所以在 Codex 中被停用的 plugin 會被略過。匯入的 Codex plugin 會以 `codex plugin marketplace upgrade NAME` 更新，這會重新安裝 Codex 從該 marketplace 安裝的所有 plugin，Codex 啟動時也會這麼做。
- 更新遇到無法處理的 target 時會略過並說明原因；該 plugin 的其他 Agent 仍會照常更新，被略過的更新會保留為待處理，留待之後的 sync。
- 匯入的 plugin 會保留其原始的 marketplace 身分。對於沒有 source 的匯入 plugin，`check` 無法
  推斷是否有新版本可用，但 Pi 的 npm 套件會向 npm 檢查。若匯入的 Claude 或 Codex plugin 的原生 marketplace 已經不在，
  sync 和 update 會略過該 target 並說明原因。在從未加入該 marketplace 的另一台機器上，
  也會發生同樣的情況。請在 Agent 中恢復該 marketplace，或移除該
  target 後從 source 重新加入 plugin；Skillshare 不會自行把匯入的 plugin 改到其他 source。
- Skillshare 會替每個受管理的 Claude/Codex plugin 註冊一個 marketplace，命名為
  `skillshare-<plugin>-<hash>`（較早的安裝維持 `skillshare-<hash>`）。移除或排除該 plugin 時，
  即使 plugin 已經不在，也會一併移除這個 marketplace；清理失敗會在下次 sync 重試。若
  marketplace 不見了，update 會重新註冊。位於其他路徑的同名註冊與匯入 plugin 的
  marketplace 不會被動到；快照與原生快取會保留。
- 這些註冊指向本機的 Skillshare 狀態目錄，user 與 project 設定都一樣。透過 Git 或 dotfile
  管理工具共享 Agent 設定，會把其他機器上不存在的路徑帶過去；請在每台機器上從 source
  加入 plugin。
- Claude 也會把 skills 目錄裡帶有 plugin manifest 的 skill 資料夾讀成名為
  `<name>@skills-dir` 的 plugin，而且同名的 plugin 只載入一個。新增同名的 Claude plugin 時，
  預覽會說明這點：Claude 會載入該 plugin 並略過那個 skill 資料夾，直到其中一個改名或移除。

原生生命週期已在 Claude Code `2.1.276`、Codex CLI `0.154.0`、Pi `0.85.1`，以及 Copilot CLI
`1.0.86` 上驗證過。Antigravity CLI `1.2.6` 則以獨立的原生安裝/清單/移除操作進行檢查。OpenCode
`1.18.31` 用於驗證版本感知的註冊；v2 schema 則由 fixture 測試涵蓋。Cursor 與 Antigravity 的
檔案系統生命週期已在獨立目錄中測試，但不宣稱涵蓋 GUI 執行期的啟用。已安裝的指令 capability
與清單 schema 會在執行期檢查；不支援的操作會附上說明並被封鎖。

## Official format references

- [Cursor local plugins](https://prod.cursor.com/docs/plugins)
- [Antigravity desktop plugins](https://www.antigravity.google/docs/plugins)
- [Antigravity standalone CLI plugins](https://www.antigravity.google/docs/cli/plugins)
- [Pi packages](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/packages.md)
- [OpenCode v1 plugins](https://opencode.ai/docs/plugins/)
- [OpenCode v2 plugins](https://opencode.ai/v2/docs/plugins)
