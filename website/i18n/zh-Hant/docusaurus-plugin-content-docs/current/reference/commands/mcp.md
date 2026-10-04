---
sidebar_position: 3
---

# mcp

管理可攜式 MCP 連線定義，並同步原生 Agent 設定。
先從 [設定一次 MCP](/docs/how-to/daily-tasks/sharing-mcp) 開始。

## Commands

```bash
skillshare mcp
skillshare mcp add
skillshare mcp edit
skillshare mcp edit docs --url https://updated.example/mcp --no-tui
skillshare mcp add docs --url https://example.com/mcp --target claude --sync
skillshare mcp add local --target codex -- company-mcp --workspace /path/to/workspace
skillshare mcp import docs --from claude --target claude --target cursor --sync
skillshare mcp import docs --file ./provider.json --target claude
skillshare mcp list --json
skillshare mcp check
skillshare mcp check docs --json --no-dns
skillshare mcp check --live --timeout 30s
skillshare mcp remove docs --sync
skillshare mcp remove docs --keep-files
skillshare mcp restore BACKUP_ID --dry-run
skillshare sync mcp --dry-run --json
skillshare sync mcp
skillshare sync --all
```

| Option | 意義 |
|---|---|
| `--target CLIENT` | 接收端 client；重複指定可選擇多個 clients。`--target none` 會把 server 保留在 Skillshare 中，不寫入任何 client。參見[下方說明](#keep-a-server-without-syncing-it) |
| `--url URL` | `add` 用的 Streamable HTTP 端點 |
| `-- command args...` | `add` 用的本機執行檔與字面參數 |
| `--disabled` | Project mode，搭配 `add`：關閉一個由 Agent 的 global config 定義的 server。參見[下方說明](#turn-off-a-global-server-in-one-project) |
| `--tools-allow TOOLS` | 只保留這些工具，以逗號分隔；`*` 代表任意字元；`""` 會清除。參見[工具政策](#tool-policy) |
| `--tools-deny TOOLS` | 永遠排除這些工具，以逗號分隔；優先於 allow；`""` 會清除。參見[工具政策](#tool-policy) |
| `--pi-options JSON` | Pi 內建 MCP 的其他單一 server 欄位，以 JSON 物件表示。參見 [Pi](#pi-options) |
| `--from CLIENT` | 要匯入的既有 client，或 `--file` 的格式 |
| `--file PATH` | 原生 JSON/JSONC、TOML 或 Goose YAML；`.toml` 預設為 Codex，其他格式會從其 MCP 區段偵測；使用 `--from` 可明確指定格式 |
| `--sync` | 儲存並同步；非互動式的 add/import/remove 預設只會儲存 |
| `--keep-files` | 搭配 `remove`：停止管理該 server，並讓它在各 Agent 中的項目維持原樣。不可與 `--sync` 併用。參見[下方說明](#stop-managing-a-server) |
| `--replace` | 在 add/import 期間明確取代既有的 source 定義；在 import 時，若匯入的 client 項目不同也會一併改寫 |
| `--dry-run`, `-n` | 只預覽，不儲存或寫入原生設定 |
| `--json` | 結構化輸出；sync/preview 報告只包含名稱、路徑與動作，不含 server 的值 |
| `--no-dns` | 搭配 `check` 使用：略過遠端 server 的主機名稱解析。參見[下方說明](#check-servers-before-an-agent-starts-them) |
| `--live` | 搭配 `check` 使用：另外啟動每個本機 server，並呼叫每個遠端 server。參見[下方說明](#probe-servers-live) |
| `--timeout DURATION` | 搭配 `check --live` 使用：每個 server 探測的時間上限，例如 `30s`；預設為 `10s` |
| `--no-tui` | 停用互動選單；`tui: false`、`--json` 或非終端機輸入/輸出時也會停用 |
| `--revision ID` | 要求 add、import、remove 或 `sync mcp` 使用相符的 preview |
| `--global`, `-g` | 使用 global Skillshare 設定 |
| `--project`, `-p` | 使用 project Skillshare 設定 |

不帶任何 subcommand 時，`mcp` 會在互動式終端機中開啟可搜尋的管理介面，或在非互動模式下印出狀態。不帶名稱的非互動式匯入，會列出解析出的候選項供選擇，且不會儲存。候選項包含可攜式定義，可識別的機密資料會轉換為參照。Agent 專屬欄位會列為警告並被省略；已停用的 servers 與不支援的傳輸方式會擋下該候選項。`restore` 一律會先重新預覽再套用；使用 `--dry-run` 可只檢視而不套用。

`--pi-extension`、`--pi-options-prune` 與 `--direct-tools` 已在 0.23.0 移除，現在使用時會失敗，並以訊息說明應改用什麼。參見[從 0.22 升級 Pi](#pi-migration)。

`sync mcp` 接受 scope flags、`--dry-run`、`--json`、`--no-tui` 與 `--revision`。`sync --all` 包含 skills、agents、extras 與 MCP + hooks；單獨的 `sync` 則維持既有的資源行為。MCP + hooks 衝突會在 `--all` 變更其他資源之前先檢查。資源類型與原生檔案是各自獨立的操作，而非單一交易。

## 互動式管理

執行 `skillshare mcp` 或 `skillshare mcp list` 即可新增、匯入、編輯、移除、同步與還原連線；所選連線的詳情顯示在列表旁。連線列表會隱藏參數、標頭與環境變數的值，並省略 URL 查詢字串。按鍵列在畫面底部。

當省略名稱或 backup ID 時，`mcp edit`、`mcp remove` 與 `mcp restore` 會提供選單。編輯器涵蓋 command/URL、參數、環境變數、HTTP headers、bearer-token 環境參照、接收端 targets 與[工具政策](#tool-policy)（**工具**）。參數接受一行一個字面參數，或一個 JSON 陣列。切換傳輸方式會清除不適用於新連線類型的欄位。

Add、edit、remove 與 import 在 **Save and sync** 或 **Save only** 之前會顯示預覽。Remove 另外提供 **Stop managing**，效果與 `--keep-files` 相同。Escape 可取消待處理的草稿。Restore 會預覽並確認對 Agent 項目的變更；它不會改寫 source 定義。

不帶 server 名稱的 import 支援多重選取。無效的候選項會被跳過；除非指定 `--replace`，否則既有的 source 名稱會被跳過。此批次要選擇一組相容的接收端 clients。整個批次會先驗證完畢，source 才會一次儲存；後續原生檔案 I/O 失敗仍維持既有的復原行為。

對於腳本，請提供名稱與 flags。`mcp edit NAME --url URL`、`mcp edit NAME --target CLIENT` 與 `mcp edit NAME -- command args...` 會更新指定欄位，同時保留其他適用的設定。除非加上 `--sync`，否則只會儲存。搭配 `--no-tui` 時，remove 需要名稱，restore 需要 backup ID。`--dry-run` 永遠不會儲存或同步變更。

## Source 欄位

選擇內嵌的 `mcp.servers`，或是由 `sources.mcp` 指定的外部檔案。外部檔案要有頂層的 `servers` 映射。`mcp.targets` 與 [`mcp.projects`](#manage-several-projects-from-the-global-config) 仍留在 Skillshare config 中。Schema 位於 repository 中的 `schemas/mcp.schema.json`。

| Server 欄位 | 意義 |
|---|---|
| `command` | 本機執行檔；與 `url` 互斥 |
| `args` | 本機執行檔的字面參數列表 |
| `env` | 本機環境變數值：字串或 `{fromEnv: VARIABLE}` |
| `url` | HTTP(S) MCP 端點；不可含內嵌憑證或 fragment |
| `headers` | HTTP headers：字串或 `{fromEnv: VARIABLE}` |
| `bearerToken` | `{fromEnv: VARIABLE}`；不可與 Authorization header 並存 |
| `transport` | 選填的 `stdio` 或 `streamable-http`；省略時自動推斷 |
| `targets` | 選填的接收端 clients；覆寫 `mcp.targets`。空清單會讓 server 只保留在 Skillshare 中。參見[下方說明](#keep-a-server-without-syncing-it) |
| `tools` | 哪些工具會提供給模型：`allow`、`deny`。只需寫一次，會依各 Agent 轉換。參見[工具政策](#tool-policy) |
| `piOptions` | Pi 內建 MCP 的其他單一 server 欄位。參見 [Pi](#pi-options) |
| `disabled` | 只能是 `true`，不能有其他連線欄位，且必須有 project 在作用範圍內：project mode，或 `mcp.projects` 下的某個 root。參見[下方說明](#turn-off-a-global-server-in-one-project) |

Client ID 有 `claude`、`codex`、`cursor`、`vscode`、`opencode`、`kilocode`、
`grok`、`antigravity`、`amp`、`claude-desktop`、`cline`、`copilot`、`factory`、`gemini`、
`goose`、`junie`、`kiro`、`lmstudio`、`warp`、`windsurf` 與 `pi`。
`grok` 指的是官方的 xAI Grok CLI。Server 名稱使用字母、
數字、點、底線與連字號。一個 server 必須直接或透過 `mcp.targets`
選擇至少一個 client 才能同步，除非它自己的 `targets` 是空清單。

### 保留 server 但不同步 {#keep-a-server-without-syncing-it}

帶有 `targets: []` 的 server 會留在 Skillshare source 中，不會寫入任何 client。
可以用它把某個 server 從所有 clients 移除，同時保留定義以便日後使用。
如果它先前同步過，下一次同步會從那些 clients 移除它的項目。

```yaml
mcp:
  targets: [claude, codex]
  servers:
    docs:
      url: https://example.com/mcp
      targets: []
```

```bash
skillshare mcp add docs --url https://example.com/mcp --target none
skillshare mcp edit docs --target none
skillshare mcp edit docs --target claude   # 恢復同步
```

- **省略 `targets` 是另一回事。** 這時 server 會繼承 `mcp.targets`，
  而該清單也是空的時候，它會被拒絕。
- `none` 不能與其他 client 一起使用。
- 在終端機的選單中，不選任何 client 直接確認。在 dashboard 中，取消勾選
  每個 client；該 server 會被標示為 **尚未選擇 Agent**。
- Project 的 servers 與 `mcp.projects` 底下的 servers 也是同樣的運作方式。
- `disabled` 項目仍然需要至少一個 client，因為它必須在某個地方把 server
  關閉。
- `mcp list` 會把這樣的 server 顯示為 `kept no targets`。

對於 Grok，名稱必須以字母或底線開頭，只能包含字母、
數字、連字號與單一底線，且不能以底線結尾。
像 `company-docs` 這樣的名稱適用於所有支援的 clients。

## Native destinations {#native-destinations}

| Client | Global | Project | Section |
|---|---|---|---|
| Claude Code | `~/.claude.json` | `.mcp.json` | `mcpServers` |
| Codex | `~/.codex/config.toml` | `.codex/config.toml` | `mcp_servers` |
| Cursor | `~/.cursor/mcp.json` | `.cursor/mcp.json` | `mcpServers` |
| VS Code | User `mcp.json`（如下） | `.vscode/mcp.json` | `servers` |
| OpenCode | `~/.config/opencode/opencode.json` | `opencode.json` | `mcp` |
| Kilo Code | `~/.config/kilo/kilo.jsonc` | `kilo.jsonc` | `mcp` |
| Grok CLI | `~/.grok/config.toml` | `.grok/config.toml` | `mcp_servers` |
| Antigravity (AGY) | `~/.gemini/config/mcp_config.json` | `.agents/mcp_config.json` | `mcpServers` |
| [Amp](https://ampcode.com/docs/customize/mcp) | `~/.config/amp/settings.json` | `.amp/settings.json` | `amp.mcpServers`（字面鍵值） |
| [Claude Desktop](https://modelcontextprotocol.io/docs/develop/connect-local-servers) | Claude 應用程式資料目錄下的 `claude_desktop_config.json` | 僅限 Global | `mcpServers` |
| [Cline](https://github.com/cline/cline/tree/main/apps/vscode/src/services/mcp) | `~/.cline/data/settings/cline_mcp_settings.json` | 僅限 Global | `mcpServers` |
| [Copilot CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-mcp-servers) | `~/.copilot/mcp-config.json` | `.github/mcp.json` | `mcpServers` |
| [Factory Droid](https://docs.factory.ai/harness/mcp) | `~/.factory/mcp.json` | `.factory/mcp.json` | `mcpServers` |
| [Gemini CLI](https://geminicli.com/docs/tools/mcp-server/) | `~/.gemini/settings.json` | `.gemini/settings.json` | `mcpServers` |
| [Goose](https://block.github.io/goose/docs/guides/config-files/) | `~/.config/goose/config.yaml` | 僅限 Global | `extensions`（YAML） |
| [Junie](https://junie.jetbrains.com/docs/junie-cli-mcp-configuration.html) | `~/.junie/mcp/mcp.json` | `.junie/mcp/mcp.json` | `mcpServers` |
| [Kiro](https://kiro.dev/docs/mcp/configuration/) | `~/.kiro/settings/mcp.json` | `.kiro/settings/mcp.json` | `mcpServers` |
| [LM Studio](https://lmstudio.ai/docs/app/mcp) | `~/.lmstudio/mcp.json` | 僅限 Global | `mcpServers` |
| [Warp](https://docs.warp.dev/agents/capabilities/mcp/) | `~/.warp/.mcp.json` | `.warp/.mcp.json` | `mcpServers` |
| [Windsurf (Cascade)](https://docs.devin.ai/desktop/cascade/mcp) | `~/.codeium/windsurf/mcp_config.json` | 僅限 Global | `mcpServers` |

Dashboard 的 server 表單編輯 HTTP headers 的方式與環境變數相同，
包括 `fromEnv` 參照。Server 選單中的 **View what each Agent gets**，以及其表單中檔案數量旁的同名選項，
會以唯讀方式顯示 Sync 對所選 client 會寫入的原生文字內容；在表單中，它會反映尚未儲存的編輯內容。機密資料仍以參照形式呈現。

JSON 項目會依照檔案本身的縮排，一行寫入一個欄位。若 Skillshare 擁有的某個項目
仍然寫在同一行，會回報為 `update` 並重新排版寫入。它不擁有的項目，
以及有人手動格式化過的項目，會保留原有排版。

Dashboard 只會提供目前 scope 與主機平台可用的目的地。每個 server 各佔一列，名稱下方以 chips
顯示它會寫入的 clients；右側的計數按鈕會開啟該 server 的完整 client 清單。僅限 Global 的 clients 在 project mode 中無法選擇。
右側的 **Sync** 框會列出尚未寫入的變更：勾選某個 client 只會編輯 source。
**Sync MCP** 會列出這些變更，確認後只寫入 MCP 設定檔，並為每個檔案保留備份。
同一個框中分隔線下方，有 server 時會出現 **檢查**，可[檢查這些 servers](#check-servers-before-an-agent-starts-them)；
**備份與還原** 則可瀏覽這些備份。
下方的 **Agents** 會列出這台機器上偵測到的 clients。
當某個 client 的 MCP 檔案存在，或該 client 用來存放設定的資料夾存在時，就算做偵測到，
所以剛安裝、還沒有 MCP 檔案的 client 也會顯示出來。在 project mode 中，
當 project 有自己的 MCP 檔案，或該 client 在 global 層級被偵測到時，就會列出該 client。

其他 client 細節：

- `codex` 目的地是單一份 `config.toml`，由 Codex CLI、Codex IDE
  擴充功能與 ChatGPT 桌面應用程式共用，所以同步到 `codex` 的 server 會出現在
  這三者中。ChatGPT 桌面應用程式會在 **Settings → MCP servers** 下列出它們。
  Codex 只會在受信任的 project 中讀取 `.codex/config.toml`；在不受信任的
  project 中，已同步的 servers 不會載入，且不會顯示錯誤。`cwd`、
  `http_headers_helper`、核准模式、逾時，以及 `oauth` 表格
  都沒有可攜式對應形式：import 會將它們省略並顯示警告，sync 則會將它們保留在
  既有項目中。`enabled_tools` 與 `disabled_tools` 來自[工具政策](#tool-policy)，
  匯入時也會讀回政策中。由 Codex plugin 包裝的 MCP servers，會設定在
  `plugins.<plugin>.mcp_servers` 下，不受此處管理。
- Claude Desktop 的檔案同步僅支援 **stdio**，僅限 macOS 與 Windows。
  其目錄在 macOS 上為 `~/Library/Application Support/Claude`，在 Windows 上為
  `%APPDATA%/Claude`。遠端連接器請在應用程式內設定。
- Cline 只作用於預設的 VS Code Stable profile，不含 Cline CLI 或其他 IDE。
- Copilot CLI 項目會寫入 `tools`：[工具政策](#tool-policy)允許的完整工具名稱，
  否則為 `["*"]`。匯入時會把 `tools` 讀回政策中。若存在 project 層級的 `.mcp.json`，sync 會停止，因為 Copilot 會優先讀取該
  檔案而非 `.github/mcp.json`；請先整合這些檔案。
  在 project mode 中同時選擇 Claude Code 與 Copilot CLI 也會在寫入任一檔案前被擋下。
  其中一個 client 請改用 global mode。
- Gemini 使用 `httpUrl` 表示 Streamable HTTP。其 `url` 欄位代表舊版 SSE，
  在 import 時會被拒絕。Cline 使用 `type: streamableHttp`；Goose 使用
  `type: streamable_http` 與 `uri`。Skillshare 會自動轉換這些格式。
- Goose 在 Windows 上使用 `%APPDATA%/Block/goose/config/config.yaml`。YAML 編輯
  會保留不相關的設定、註解與內建 extensions，但可能會改變格式。
  Aliases、merges、重複的鍵與多份文件會擋下編輯。
  內建 extensions 與 keychain 的 `env_keys` 無法作為可攜式 MCP
  連線匯入。
- Claude Code 會跳過名為 `workspace`、`claude-in-chrome` 或 `computer-use` 的 server，
  這些名稱由它保留給內建 servers 使用。它也絕不會把自己的憑證送給
  遠端 server：`ANTHROPIC_API_KEY`、`ANTHROPIC_AUTH_TOKEN`、`AWS_BEARER_TOKEN_BEDROCK`、
  `HTTPS_PROXY` 與 `NPM_TOKEN` 在 `url` 與 `headers` 中會讀取為空值。Skillshare 對 Claude
  的這兩者都會拒絕。請把憑證複製到一個你自訂名稱的變數中。
- Claude Code 也有一個本機 scope：使用 `claude mcp add` 且未指定
  `--scope` 新增的 servers，會依 project 各自存放在 `~/.claude.json` 中。本機 server
  會整體覆蓋 `.mcp.json` 或 user scope 中同名的 server。在 project mode 中，Skillshare
  會在被隱藏的項目旁回報這類 server 的存在，但不會阻擋同步。可從 project 資料夾中
  用 `claude mcp remove NAME -s local` 移除它。
- Cline 的 VS Code 擴充功能、CLI 與 SDK 共用 `~/.cline/data/settings/`。該
  擴充功能會把較舊的 VS Code `globalStorage` 檔案搬到那裡一次，之後就不再
  讀取它，所以 Skillshare 只有在 `~/.cline/data` 尚不存在時才會寫入舊檔案。
  `CLINE_MCP_SETTINGS_PATH`、`CLINE_DATA_DIR` 與 `CLINE_DIR` 會依此順序
  被採用。
- Windsurf 支援的是文件記載的 Cascade 設定。Windsurf 較新的
  Devin Local agent 會讀取自己的 `~/.config/devin/mcp_config.json`，Skillshare
  不管理它。Warp 的 project 連線每個 session 仍需要在 Warp 內部核准。
- Amp 只有在執行過 `amp mcp approve <name>` 後，才會從 project 的
  `.amp/settings.json` 執行 server。Global servers 不需要核准。
- Kiro 只會展開其「Mcp Approved Env Vars」設定中列出的名稱所對應的
  `${VARIABLE}`，且只接受 localhost 的 `http://` URL。
- VS Code 會為 `User/profiles/` 下每個非預設 profile 各自保留一份
  `mcp.json`。Skillshare 管理的是預設 profile 的檔案。

環境參照的匯出格式，對 Amp、Copilot CLI、
Factory、Gemini CLI 與 Kiro 為 `${VARIABLE}`，對 Cline 與 Windsurf 為 `${env:VARIABLE}`。
Claude Desktop、Goose、Junie、LM Studio 與 Warp 目前拒絕 `fromEnv` 與
`bearerToken` 的匯出，因為它們的原生插值行為尚未驗證。
請使用不含自訂憑證的連線，或在支援的接收端 client 中自行驗證。
Skillshare 絕不會將參照解析為明文。

Antigravity 使用目前的[官方 MCP 設定格式](https://antigravity.google/docs/mcp)，
包括遠端連線用的 `serverUrl`。Skillshare 會自動轉換可攜式 `url`。
較舊的 `.gemini/antigravity/` 與 `.gemini/antigravity-cli/` 設定
位置不受管理。Antigravity 的 `fromEnv` 與 `bearerToken` 匯出
會被封鎖，因為其文件記載的設定格式並未指定環境
插值方式。請使用不需要自訂機密 headers 的連線，並在 Antigravity 內完成
支援的 OAuth 登入。Skillshare 絕不會將參照展開為
明文憑證。

OpenCode 的 global 目錄遵循 `XDG_CONFIG_HOME`。若既有的
`opencode.jsonc` 存在，會優先使用它而不是建立 `opencode.json`。在 project 中，
OpenCode 也會讀取 `.opencode/` 中的這兩個檔名，因此若檔案放在那裡，Skillshare 就會
寫入那個檔案；新檔案則會建立在 project 根目錄。若有多個檔案
存在，請先整合它們再進行同步。自訂的 OpenCode config
路徑、目錄覆寫、內嵌 config 與繼承的上層檔案不受
管理。它們可能會覆蓋 OpenCode 中所選的目的地。

Kilo Code 使用與 OpenCode 相同的格式。它會讀取 project 根目錄與
`.kilo/` 中的 `kilo.jsonc` 與 `kilo.json`，並將兩者合併，因此 Skillshare 會寫入
既有的那一個檔案，只有在都不存在時才會建立 `kilo.jsonc`。若
兩者都存在，請先整合它們再進行同步。`KILO_CONFIG`、
`KILO_CONFIG_DIR` 以及舊版 VS Code 擴充功能的 `mcp_settings.json`
不受管理。

Kilo Code 將 project config 視為不受信任。它不允許在其中使用 `{env:VARIABLE}`
參照，一旦發現 project 檔案就會忽略整個檔案。因此在 project
mode 中，Skillshare 會拒絕使用 `fromEnv` 或 `bearerToken` 的 Kilo Code server。
請在允許使用參照的 global mode 中定義該 server。

OpenCode 與 Kilo Code 使用 `local`/`remote` 類型與 `{env:VARIABLE}` 參照；Grok 使用
`${VARIABLE}` 參照。Skillshare 會自動轉換這些格式。Claude 的
`"type": "streamable-http"` 會匯入為 HTTP。已停用的連線會擋下 import。
其他沒有可攜式對應形式的原生選項，例如 Codex 的
`startup_timeout_sec` 或 `envFile`，會在 import 時被省略並顯示警告；
sync 會將它們保留在 Agent 既有的項目中。Pi 使用其內建 MCP；詳見
[下方說明](#pi)。

VS Code Stable 的預設 user 檔案為：

- macOS：`~/Library/Application Support/Code/User/mcp.json`
- Linux：`${XDG_CONFIG_HOME:-~/.config}/Code/User/mcp.json`
- Windows：`%APPDATA%/Code/User/mcp.json`

Global 的 Claude、Codex、Grok 與 Copilot 路徑遵循 `CLAUDE_CONFIG_DIR`、`CODEX_HOME`、
`GROK_HOME` 與 `COPILOT_HOME`。`OPENCODE_CONFIG` 與 `OPENCODE_CONFIG_DIR` 不受管理。Amp 與 Goose 在
使用 `.config` 路徑的平台上會遵循 `XDG_CONFIG_HOME`。
Project 目的地是相對於所選 project 根目錄。Project 信任、
server 核准與驗證仍屬於接收端 Agent 的責任。

### 某個 Agent 的另一個帳號 {#accounts}

宣告為[某個 Agent 的另一個帳號](/docs/reference/targets/configuration#agent-config-dir)的 target 同時也是 MCP target，適用於 `claude`（`CLAUDE_CONFIG_DIR`）、`codex`（`CODEX_HOME`）與 `pi`（`PI_CODING_AGENT_DIR`）。它的 servers 會以該 Agent 的格式寫入該帳號自己的檔案：Claude 是 `<config_dir>/.claude.json`，Codex 是 `<config_dir>/config.toml`，Pi 是 `<config_dir>/mcp.json`。

```yaml
targets:
  claude-work:
    agent: claude
    config_dir: ~/.claude-work

mcp:
  targets: [claude, claude-work]      # 兩個帳號都會拿到每個 server
  servers:
    docs:
      url: https://example.com/mcp
    jira:
      command: jira-mcp
      targets: [claude-work]          # 只給工作帳號
```

在這個例子中，`docs` 會寫入 `~/.claude.json` 與 `~/.claude-work/.claude.json`，`jira` 只會寫入第二個檔案。`--target claude-work` 可搭配 `mcp add` 與 `mcp edit` 使用，dashboard 也會把這個帳號列在 Agents 旁邊。

每個帳號讀取的是同一份 project 檔案，因此在 `mcp.projects` 內以及 project mode 中，請使用 Agent 本身的名稱。Claude Code 會把 project 的關閉清單存在每個帳號的檔案中：[在某個 project 中關閉 server](#turn-off-a-global-server-in-one-project) 會把這個開關寫入每個擁有該 server 的帳號。`mcp import --from claude-work` 以及 dashboard 的 Import from target 讀取的是該帳號自己的檔案。`mcp import --file <path> --from claude-work` 讀取的則是你自己匯出的檔案，格式是該帳號所屬 Agent 的格式。

## Turn off a global server in one project {#turn-off-a-global-server-in-one-project}

Agent 會同時讀取自己的 global MCP 檔案與 project 的檔案。因此定義在
global 檔案中的 server 會在每個 project 中載入。若要讓它在某個
project 中不要載入，請新增一個**使用該 Agent 的 global 檔案中相同名稱**的項目，
並標記為 `disabled`。

這只適用於四種 clients：

| Client | 是否支援 | Skillshare 會寫入什麼 |
|---|---|---|
| Claude Code | 是 | `~/.claude.json`：名稱會加入這個 project 的 `disabledMcpServers` 清單 |
| OpenCode | 是 | `opencode.json`：`"NAME": {"enabled": false}` |
| Kilo Code | 是 | `kilo.jsonc`：`"NAME": {"enabled": false}` |
| Pi | 是，Pi 1.0.1 起 | `.pi/mcp.json`: `"NAME": {"enabled": false}`，見下方 |
| Codex | 否 | 見下方說明 |
| 其他所有 client | 否 | 選擇它會是錯誤；不會寫入任何內容 |

只有開關會被寫入。Agent 仍會沿用其 global 項目中的 command 或 URL。
其他 clients 之所以被拒絕，是因為它們會用 project 項目整個取代 global
項目，或是沒有 project 檔案，因此單獨寫入一個開關反而會弄壞
server，而不是把它關閉。

Codex 被拒絕的原因不同。它確實會把 `.codex/config.toml` 逐欄位合併到
global 檔案之上，所以在 global config 有定義該 server 的機器上，單獨的
`enabled = false` 是可行的。但在沒有定義的機器上，合併後的項目會缺少
`command` 或 `url`，導致 Codex 因 `invalid transport` 而整個設定載入失敗。
`.codex/config.toml` 通常會被 commit，所以一個隊友的開關
可能導致另一個隊友的 Codex 無法啟動。請改為逐機器關閉該 server，
在 `~/.codex/config.toml` 中設定 `enabled = false`。

Pi 會用 project 中的同名項目整筆取代 global 項目，但 Pi 1.0.1 起，沒有 `command`、`url` 或
`type` 的項目改為覆寫：只改 global server 的 `enabled`、`exposure` 和 `toolExposure`，args、env
和憑證都沿用 global server。Pi 的 `/mcp` 寫的也是同樣的項目。Pi 1.0.1 以前會把它當成無效項目回報。
在 global Pi config 沒有該 server 的機器上，Pi 啟動時會回報沒有可覆寫的 server，其餘設定照常載入。
這個開關不需要 global server 的任何內容，所以 project mode 也能用。舊版寫入的、帶有 global server
`command` 或 `url` 的項目，會在下次同步時改寫成覆寫項目。

### OpenCode and Kilo Code

```bash
cd my-project
skillshare mcp add company-docs --disabled --target opencode --target kilocode
skillshare sync mcp
```

```yaml
# .skillshare/config.yaml
mcp:
  servers:
    company-docs:
      disabled: true
      targets: [opencode, kilocode]
```

### Claude Code

Claude Code 會從單一 scope 整個取用一個 server 項目，絕不會合併欄位，所以
在 `.mcp.json` 中設一個開關會取代該 server，而不是把它關閉。它把
自己每個 project 的關閉清單存放在 `~/.claude.json` 中，也就是 `/mcp` 面板編輯的那份。
Skillshare 會把名稱加到那裡，放在這個 project 的絕對路徑下，
不會寫入 `.mcp.json`。

```bash
skillshare mcp add company-docs --disabled --target claude
skillshare sync mcp
```

- 這份清單存放在你的機器上，而不是 repository 中。每個隊友都要在自己的
  checkout 中執行一次 `skillshare sync mcp`。
- 你自己在 `/mcp` 中關閉的名稱，永遠不會被認領或移除。
- 若你在 `/mcp` 中把 server 重新開啟，下一次同步會回報衝突。
  請從 `.skillshare/config.yaml` 中移除該項目，或用 replace 再次關閉它。
- 此清單以 project 的路徑為鍵值，所以搬移 project 需要重新同步。

### Rules

- **必須有 project 在作用範圍內。** 請在有 `.skillshare/config.yaml` 的 project 中執行
  （由 `skillshare init -p` 建立）、加上 `-p`，或把該項目放在
  [`mcp.projects`](#manage-several-projects-from-the-global-config) 下的某個 project root。
  在沒有任何 project 在作用範圍內的 global `mcp.servers` 中，它會被拒絕。
- **`disabled` 必須單獨存在。** 該項目只能帶 `targets`。加入 `command`、`url`、
  `env`、`headers`、`piOptions` 或 `tools` 會是錯誤。
- **`targets` 可以省略。** 該項目會跟著 project 的 targets：每次同步時，它會寫到
  project 所使用、且支援個別 project 開關的 clients。在 `mcp.projects` 底下，
  Skillshare 也知道同名的 global server，因此範圍會再縮小到該 server 實際寫入的
  clients。之後變更 project 的
  targets 時，不需要修改這個項目。若要自行決定，請列出 `targets`；該清單中任何
  不支援的 client 都會是錯誤。
- **名稱必須相符。** Skillshare 不會讀取 Agent 的 global 檔案，所以它
  無法確認該名稱的 server 是否真的存在。名稱不符任何 server 也無妨：
  Agent 會直接忽略它。
- **要重新開啟時**，移除該項目（`skillshare mcp remove company-docs`）
  並同步。開關會從當初寫入它的檔案中移除：project 自己的檔案，
  或 Claude Code 的 `~/.claude.json`。
- **Skillshare 自己定義的 server 不需要這麼做。** 改為在該 server 上取消選擇
  該 Agent，下一次同步就會移除它的項目。

在 dashboard 中，這是 **關閉全域伺服器** 按鈕。在 project mode 中它位於 **伺服器** 標題旁；
在 project 的 MCP 分頁中，則位於 **新增伺服器** 旁邊。

## Manage several projects from the global config {#manage-several-projects-from-the-global-config}

Project mode 會把每個 project 的 MCP 設定放在該 project 的
`.skillshare/config.yaml` 中，並在該資料夾內執行同步。如果你比較想把所有 project
集中在一處管理，請在 **global** config 的 `mcp.projects` 下列出這些 project 資料夾。
之後在任何位置執行一次 `skillshare sync mcp`，就會在同一份計畫中寫入 global
檔案與每個 project 的檔案。

```yaml
# ~/.config/skillshare/config.yaml
mcp:
  servers:
    context7:
      command: npx
      args: ["-y", "@upstash/context7-mcp"]
      targets: [claude, opencode]
  projects:
    ~/work/project01:
      targets: [claude, opencode]
      servers:
        context7:                  # 只在這個 project 中關閉
          disabled: true
    ~/work/project02:
      servers:
        internal-docs:             # 只存在於這個 project
          url: https://example.com/mcp
          targets: [opencode]
```

Project 只需要列出與 global config 不同的部分。像 `context7` 這樣的 global server
不需要在這裡新增項目：Agent 會同時讀取自己的 global 檔案與 project 的檔案，
所以它已經會在每個 project 中載入。`disabled` 項目會
[在該資料夾中把它關閉](#turn-off-a-global-server-in-one-project)，
適用於該處列出的任何 client。

每個 key 都是一個 project 資料夾：絕對路徑，或以 `~` 開頭的路徑。其下放的是
該 project 自己的 `config.yaml` 會放在 `mcp` 下的同一組 `targets` 與 `servers`，
而且它們會寫入相同的 [project 檔案](#native-destinations)。沒有 `targets` 的
project 會繼承 global 的 `mcp.targets`。

當同一個 server 出現在不只一個位置時，預覽會標出檔案：

```text
context7     add          opencode (~/.config/opencode/opencode.json)
context7     add          opencode (~/work/project01/opencode.json)
```

從清單中移除某個 project，下一次同步時就會移除 Skillshare 寫在那裡的項目，
與移除一個 server 相同。

若要讓多個 projects 使用同一個 server，請用 YAML anchor 定義一次，再重複使用：

```yaml
mcp:
  projects:
    ~/work/project01:
      servers:
        internal-docs: &internal-docs
          url: https://example.com/mcp
          targets: [opencode]
    ~/work/project02:
      servers:
        internal-docs: *internal-docs
```

請把 anchor 放在 `mcp.projects` 之內。指向 `mcp.servers` 上某個 anchor 的 alias
也能運作，但 `skillshare mcp add` 與 dashboard 會改寫 `mcp.servers`；它們儲存時
會把這類 alias 完整展開寫出，讓檔案保持有效，之後它就不會再跟著 global server
的後續編輯而變動。

### Projects in the dashboard {#projects-in-the-dashboard}

在 global mode 下，dashboard 有一個 **專案** 頁面。它會列出
[`projects`](/docs/reference/targets/configuration#projects) 與 `mcp.projects`
底下的每個資料夾，每個 project 都有一個 **MCP** 分頁。

![專案的 MCP 分頁：依專案開關的全域 server，以及專案專用的 server](/img/projects-mcp-tab.png)

- **新增專案** 會要求填入資料夾與它的 targets。勾選 **MCP** 可以讓該資料夾
  同時列在 `mcp.projects` 底下。
- **MCP** 分頁會列出每個 global server，並各附一個開關。關閉其中一個會儲存一筆
  不含 `targets` 的 `disabled` 項目，因此它會如
  [上方說明](#turn-off-a-global-server-in-one-project)所述跟著 project 的 targets；
  重新開啟則會移除該項目。下方則是只存在於該 project 的 servers。
- 已關閉的 server 會顯示它在哪些 Agents 中被關閉的 logo。當 project 的某個 Agent
  不支援個別 project 開關時，該列會說明這個 server 在那裡仍會載入。如果項目列出了
  自己的 `targets`，且與 project 的不同，就會出現 **改成跟專案一致**：它會把該項目
  重新儲存為不含 `targets` 的版本。
- 分頁 Sync 框中的 **Sync MCP** 會寫入整份 MCP 計畫，並說明其中有多少變更不屬於
  這個 project。專案頁面頂端的 **Sync project** 只會寫入這個 project 的 skills、
  agents 與 MCP。
- **預設值** 位於 MCP 頁面底部，用來編輯 `mcp.targets`。
- 當 project 自己的 Agent 檔案中有 Skillshare 未管理的 servers 時，分頁會在清單上方
  說明，並附上 **Import**。參見[下方說明](#unmanaged-servers)。

儲存時只會改寫你變更的那個 project。其他 project 的 YAML 會維持原樣，包含
anchor 與 alias，而寫成 `~/work/app` 的資料夾也會保留它的 `~`。與本頁其他地方
一樣，儲存只會變更 `config.yaml`；寫入檔案的是 Sync。

限制：

- `mcp.projects` 只會從 global config 讀取。包含它的 project config 會被拒絕。
- 沒有指令可以編輯它：`skillshare mcp add` 管理的是 `mcp.servers`，
  `mcp.projects` 會維持原樣。請在 `config.yaml` 中編輯它，或使用
  [dashboard](#projects-in-the-dashboard)。
- 以 Claude Code 為 target 的 `disabled` 項目會寫入 `~/.claude.json`，也就是
  global servers 寫入的同一個檔案，因為 Claude Code 的個別 project 關閉清單就放在那裡。
  servers 本身則維持原樣。
- 如果某個資料夾也有自己的 `.skillshare/config.yaml` 在管理同一個項目，
  計畫會回報衝突，而不是覆寫它。

## 在 Agent 啟動 server 之前先檢查 {#check-servers-before-an-agent-starts-them}

```bash
skillshare mcp check
skillshare mcp check docs github --json
skillshare mcp check --no-dns
```

`mcp check` 會針對 source 中的每個 server（或只針對指定的 server）回答「照同步後的樣子能不能正常運作？」。
在 global config 中，它也會檢查 [`mcp.projects`](#manage-several-projects-from-the-global-config)
底下每個根目錄的 servers，並依該根目錄讀取每個 Agent 的規則與同步狀態。它是唯讀的：除非加上 [`--live`](#probe-servers-live)，否則不會啟動 server、
送出 HTTP 請求、執行指令或寫入檔案。

| 檢查項目 | 等級 |
|---|---|
| `env`、`headers` 或 `bearerToken` 中的 `fromEnv` 變數未設定或為空 | error |
| 在 `PATH` 上找不到本機 server 的 `command`（開頭的 `~/` 會被展開） | error |
| 遠端 server 的主機無法透過 DNS 解析（限時 3 秒；可用 `--no-dns` 略過） | warning |
| 某個 Agent 的規則拒絕該 server，例如 Claude Code 保留的名稱 | error |
| 某個 Agent 的項目與 source 衝突，與 `sync mcp --dry-run` 相同 | error |
| 某個 Agent 的項目尚未寫入或尚未更新 | warning |
| 該 server 設定了 `targets: []`，只保留在 Skillshare 中 | info |
| 所選的某個 Agent 無法容納該 server [工具政策](#tool-policy)的一部分 | warning |

變數的值永遠不會被印出。只要發現任何 error，指令就會以 1 結束，否則以 0 結束；warning 永遠不會
造成失敗。未知的 server 名稱是一個 error，並會列出已知的名稱。一個名稱會選取 global 與每個 project 中
所有同名的 servers，已知名稱也包含 project servers。

在終端機中，project server 的標題會標示它所屬的 project：

```text
✓ docs
  · claude: in sync
✗ docs  (project ~/work/app)
  ✗ command no-such-mcp-binary was not found on PATH
  ! claude: not synced yet; run skillshare sync mcp
```

使用 `--json` 時，報告的結構如下：

```json
{
  "servers": [
    {
      "name": "docs",
      "ok": false,
      "findings": [
        { "level": "error", "check": "env", "target": "", "message": "bearerToken reads DOCS_TOKEN, which is not set", "subject": "DOCS_TOKEN" },
        { "level": "warning", "check": "sync", "target": "claude", "message": "not synced yet; run skillshare sync mcp" }
      ]
    },
    {
      "name": "docs",
      "project": "/home/me/work/app",
      "ok": true,
      "findings": [
        { "level": "info", "check": "sync", "target": "claude", "message": "in sync" }
      ]
    }
  ],
  "summary": { "errors": 1, "warnings": 1 }
}
```

`check` 是 `env`、`command`、`url`、`dns`、`client-rule`、`sync`、`targets`、`tools` 或 `live` 其中之一。
`target` 表示 Agent 或帳號；當該項發現是針對 server 本身時則為空。
`subject` 在 `env`、`command` 與 `dns` 發現中表示變數、指令或主機；在成功的 `live` 探測中表示 server
回報的名稱；在 `live` 登入 warning 中表示 resource metadata URL；其他情況下省略。
`project` 是該 server 所屬的 `mcp.projects` 根目錄，以絕對路徑表示（開頭的 `~` 會被展開）；
global server 則省略此欄位。`summary` 會計算報告中的每個 server，包含 project servers。

在 dashboard 中，MCP 頁面 Sync 框中的 **檢查** 按鈕會執行同樣的檢查。它在有 servers 可檢查時才會出現，
只在點擊時執行，會在 server 清單上方顯示摘要，並在每個 server 下方顯示它的 error 或 warning；
重新載入頁面後不會保留任何內容。MCP 頁面只列出 global servers，因此它的摘要與各列都不包含
project servers，即使與某個 global server 同名也一樣。project 的 MCP 分頁在它的 Sync 框中
有自己的 **檢查**，會回報該 project 自己的 servers。變數是從啟動 `skillshare ui` 的終端機讀取。

### 即時探測 servers {#probe-servers-live}

```bash
skillshare mcp check --live
skillshare mcp check docs --live --timeout 30s --json
```

`--live` 會先執行靜態檢查，再連線到每個選取且沒有 error 的 server。有 error 的 server 或已停用的項目
不會被連線；會有一則 `info` 發現說明原因。

- **本機（stdio）servers。** Skillshare 會在你目前的環境中以 `args` 啟動 `command`，並加上該 server 的
  `env`，其中每個 `fromEnv` 的值都從你的 shell 讀取。project server 會在其 project 資料夾中啟動，global
  server 則在目前目錄中啟動。這會像 Agent 一樣在你的電腦上執行該 server 的程式碼，所以只對你信任的
  servers 使用 `--live`。Skillshare 會送出 `server/discover`。若 server 回應的 error 不是 MCP protocol
  error，或未在逾時時間的三分之一內回應，就會被視為早於 MCP 2026-07-28 的版本，改用 `initialize`
  handshake。接著 Skillshare 會呼叫 `tools/list` 計算工具數量，然後停止該 server：先關閉 stdin，再對
  server 的 process group 送出 SIGTERM，然後是 SIGKILL。在 Windows 上則會終止該 process。
- **遠端（Streamable HTTP）servers。** Skillshare 會帶著該 server 的 `headers` 與 `bearerToken` 以 POST
  送出 `server/discover`，並讀取 JSON 或 SSE 回應。沒有 MCP error 的 `400`、`404` 或 `405` 會退回使用
  `initialize`。`401` 是一則 warning，「sign-in required」，並附上 `WWW-Authenticate` header 中的
  resource metadata URL。Skillshare 永遠不會登入或啟動 OAuth。

每個 server 的整個探測共用一個時間上限：10 秒，或 `--timeout` 指定的值（例如 `30s` 或 `1m`）。最多同時
探測四個 servers。未搭配 `--live` 使用 `--timeout` 會是一個 error。

| 結果 | 等級 |
|---|---|
| server 有回應：它的名稱與版本、protocol 版本以及工具數量 | info |
| 遠端 server 需要登入（HTTP 401） | warning |
| 指令無法啟動、提早結束，或未在時限內回應 | error |
| protocol error、不支援的 protocol 版本，或任何其他 HTTP 狀態 | error |

本機 server 失敗時，訊息結尾會附上最多五行它的 stderr。`env`、`headers` 與 `bearerToken` 的值會從每則
訊息中移除；短於四個字元的值則維持原樣。值會照原樣傳遞：Skillshare 永遠不會執行 Pi 的 `!command` 值，
也不會讀取 `piOptions`。結束代碼遵循相同規則，只要發現任何 error 就是 1。`--live` 不會寫入任何檔案，
也不會留下操作紀錄項目。

使用 `--json` 時，有回應的 server 還會多一個 `live` 物件：

```json
{
  "name": "docs",
  "ok": true,
  "findings": [
    { "level": "info", "check": "live", "target": "", "message": "responds: docs-server 1.4.0, protocol 2026-07-28, 12 tool(s)", "subject": "docs-server" }
  ],
  "live": { "protocolVersion": "2026-07-28", "serverInfo": { "name": "docs-server", "version": "1.4.0" }, "tools": 12 }
}
```

`serverInfo` 是 server 對自己的描述，沒有任何驗證。server 未被探測或探測失敗時，會省略 `live`。

dashboard 的 **檢查** 按鈕只執行靜態檢查。dashboard 只在一個地方探測 server：server 對話框
[工具區塊](#tool-policy-dashboard)中的 **載入工具**，它會依對話框目前的欄位啟動 server 一次，
列出它的工具。搭配 `--json` 時，`live` 也會包含 `toolNames`，即 `tools/list` 回傳的名稱。

## 停止管理某個 server {#stop-managing-a-server}

```bash
skillshare mcp remove docs --keep-files
```

這會把 `docs` 從 source 移除，並忘記 Skillshare 曾為它寫入哪些 Agent 項目。
Agent 檔案不會有任何變動。從此之後，這些項目就歸你管理：sync 既不會移除，也不會更新
它們。`--keep-files` 不可與 `--sync` 併用。終端機的 remove 精靈以 **Stop managing**
提供這個選項，dashboard 的移除對話框也有，MCP 頁面與 project 的 **MCP** 分頁中都能使用。

只有你移除它的那個範圍會變動。停止管理某個 global server，不影響 project 中同名的
server，反之亦然。若要重新管理某個項目，請匯入它。

## Skillshare 未管理的 servers {#unmanaged-servers}

dashboard 會讀取目前範圍的 Agent 設定檔，以及 `mcp.projects` 底下每個資料夾的設定檔，
找出這份 source 沒有定義、也沒有任何 Skillshare 設定在管理的 servers。找到時，server
清單上方會有一則說明，告訴你有幾個、在哪些 Agents 中。**Import** 會開啟匯入，並預先
選好其中第一個 Agent。project 的 **MCP** 分頁會針對該 project 自己的檔案顯示同樣的
說明；它的匯入會讀取該 project 的檔案，並把 servers 儲存到該 project。沒有可連線對象
的項目，例如 Goose 的內建 extensions，不會被計入。

### 接手某個 Agent 已有的項目

當你新增的 server 名稱已被某個 Agent 檔案使用時，sync 不會覆寫該項目。計畫會回報
衝突 `existing entry is not managed`，並在你為該項目做出選擇前不寫入任何檔案：

- 從該 Agent 匯入：`skillshare mcp import NAME --from CLIENT`，或 dashboard 中該衝突的
  **Import from** 按鈕，例如 **Import from Cursor**。與 source 相符的項目會直接被採用。若 conflict 位於
  `mcp.projects` 底下的資料夾，按鈕會讀取該資料夾的檔案，並匯入到那個 project。
- 以 source 定義取代它：dashboard 中的 **Replace with source**，或匯入時加上
  `--replace`。

## Safety and limitations

- JSONC 註解與不相關的設定會被保留。已變更的、由 Skillshare 擁有的
  項目會整體取代，所以那些項目內的註解可能因此改變。只有
  Skillshare 寫入的欄位會被比對與取代；Agent 專屬欄位（例如逾時設定）
  會被保留。
  Agent 自行填入的預設值，例如 `"type": "stdio"`、空的 `env` 或
  header 名稱大小寫，不算變更。用 `enabled: false` 或 `disabled: true`
  關閉一個受管理的 server，會回報為衝突。
  Pi 是例外：只修改 `enabled` 不會造成所有權衝突；同步時仍以 source 的 `piOptions.enabled` 為準。
- 當 Agent 重寫同一份檔案中不相關的設定時（如 Claude Code 對
  `~/.claude.json` 所做的那樣），預覽仍然有效。只有該檔案的
  MCP 項目發生變更時才需要重新預覽。
- Codex 與 Grok 的編輯支援一般的 `[mcp_servers.NAME]` 表格及其子表格。
  已更新的項目會保持原位，CRLF 換行符號也會保留。
  內嵌／點記法的 MCP 定義必須先轉換為表格才能寫入；
  否則會被拒絕，且不會修改檔案。
- 原生檔案的 symlinks、格式錯誤的檔案與重複的 JSON 屬性都會擋下
  寫入。被 symlink 的 Skillshare `config.yaml` 會直接寫入其目標檔案。檔案權限會被保留；新的原生
  檔案、擁有權紀錄與備份都使用私有權限。
- 若某項目已經與 source 相符，會回報為未變更且不會寫入，例如在
  拉取隊友的變更之後。如果這份設定先前並未管理它，例如從某個 Agent
  匯入後才勾選該 Agent，計畫會顯示 `adopt`：sync 會把它記為受管理，但不改動檔案；
  之後移除該 server 或取消勾選該 Agent 就會移除它。你在 Agent 裡自己關閉的
  server 仍屬於你。不同的未受管理項目需要匯入或
  明確的逐項目取代；只要另一份 Skillshare 設定檔仍然存在，
  就不能覆蓋它的擁有權。如果該設定檔已不存在，就永遠無法釋放該項目，
  因此衝突訊息會說明這是殘留項目並指出是哪個檔案，並在明確的匯入或取代時
  接手它；終端機與 dashboard 上的那則衝突都可以這麼做。若該檔案只是讀不到，
  例如位於未掛載的磁碟上，仍視為擁有者還在。
- Dashboard 的 MCP 設定只有在瀏覽器以 `localhost` 或 IP 位址開啟
  dashboard 時才能運作。透過網域名稱存取時（包括 reverse
  proxy），MCP 請求會回傳 403，因為 DNS rebinding 攻擊一律使用
  網域名稱。
- 憑證使用環境參照；沒有機密儲存庫、OAuth session 同步、
  持續性健康監控、套件安裝、gateway、registry 或 plugin 同步功能。
  `mcp check --live` 是唯一會啟動或呼叫 server 的指令。
- 此版本不支援 VS Code Insiders、自訂 profiles、遠端 workspaces 與舊版 SSE。
- VS Code 目前不會在 `headers` 內代換 `${env:VARIABLE}`
  （[microsoft/vscode#336232](https://github.com/microsoft/vscode/issues/336232)），
  所以同步到 VS Code 的 header 與 `bearerToken` 參照，在此問題修復前
  會以未解析的原始值送達 server。
- 本機操作紀錄存放在 Skillshare state 目錄的 `mcp/` 下：
  `state.json`、寫入期間的 `pending.json`，以及 `backups/`（每個
  Agent 檔案保留最新 20 份）。請勿把這個
  目錄當作可攜式清單分享出去。


## 工具政策 {#tool-policy}

`tools` 決定 server 的哪些工具會提供給模型。只要在 server 上寫一次；同步時
Skillshare 會把它轉換成各 Agent 自己的欄位。

```yaml
mcp:
  servers:
    github:
      command: github-mcp
      targets: [pi, codex, copilot, opencode]
      tools:
        allow: [get_*, search_code, list_issues]
        deny: [get_secret]
```

```bash
skillshare mcp add github --target pi --target codex --tools-allow 'get_*,search_code' --tools-deny get_secret -- github-mcp
skillshare mcp edit github --tools-allow ''          # clear the allow list
skillshare mcp import github --from claude --target pi --tools-deny get_secret
```

| 欄位 | 意義 |
|---|---|
| `allow` | 設定後，只保留符合的工具 |
| `deny` | 移除符合的工具，即使 `allow` 也符合它們 |

`allow` 與 `deny` 中的項目是工具名稱，`*` 代表任意字元。其他萬用字元（`? [ ] { }`）、
空白與逗號都會被拒絕，重複列出的名稱也一樣。若 `deny` 清單移除了 `allow` 保留的每個工具，
會是錯誤。`disabled` 項目不能設定 `tools`。這兩個旗標可搭配 `mcp add`、`mcp edit` 與
`mcp import` 使用；清單以逗號分隔，空值會清除該部分。
Pi 如何提供工具不屬於政策：那是 Pi 的 `exposure`，在 [`piOptions`](#pi-options) 中設定。

### 各 Agent 會收到什麼 {#tool-policy-agents}

並非每個 Agent 都能容納政策的每個部分。Skillshare 會寫入該 Agent 文件化格式所支援的部分，
並指出其餘部分；它絕不會靜默丟棄任何部分。

| Agent | 寫入內容 | 不套用 |
|---|---|---|
| [Pi](https://github.com/earendil-works/pi/blob/v0.99.0/packages/coding-agent/docs/mcp.md) | `toolExposure` 依序為被拒絕的工具設為 `hidden`、允許的工具，以及設定 `allow` 時的 `"*": "hidden"` | 無 |
| [Codex](https://developers.openai.com/codex/config-reference) | `enabled_tools` 與 `disabled_tools`，僅限完整名稱。Codex 會在 `enabled_tools` 之後套用 `disabled_tools` | `allow` 中的 `*` 萬用字元；`deny` 中無法併入完整 `allow` 清單的 `*` 萬用字元 |
| [Copilot CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-mcp-servers) | `tools`：允許的完整名稱扣除被拒絕的名稱，否則為 `["*"]` | `allow` 中的 `*` 萬用字元；`allow` 未列出完整名稱時的 `deny`，因為 Copilot 沒有拒絕清單 |
| [OpenCode](https://opencode.ai/docs/permissions/)、[Kilo Code](https://kilo.ai/docs/code-with-ai/platforms/cli#permissions) | 無 | 全部。兩者都只在 server 項目之外、以 `<server>_<tool>` 為鍵的頂層 `permission` map 中篩選工具 |
| 其他所有 Agent | 無 | 全部 |

在 Pi 中，完整的工具名稱優先於任何萬用字元，因此若某個允許的完整名稱符合被拒絕的萬用字元，
它就不會寫入 `toolExposure`。允許的工具會取得 server 的 `piOptions.exposure`；當它未設定或為
`hidden` 時，則使用 Pi 預設的 `codemode`：因此 `hidden` 搭配 `allow` 代表只有允許的工具可見。

未套用的部分會出現在三個地方：

- 同步計畫中，每個 Agent 一行 warning，並列出相關 servers：

  ```text
  ! tool policy not applied for opencode: allow, deny (github)
  ```

  搭配 `--json` 時，同樣的文字會出現在計畫的 `notices` 中。
- [`mcp check`](#check-servers-before-an-agent-starts-them)，以每個 Agent 一則 `tools`
  warning 呈現。
- Dashboard 中 server 對話框的工具區塊，以及 **檢視各 Agent 會寫入的設定**。Dashboard
  不會為這些情況，或為下方已淘汰的 Pi 設定，顯示頁面層級的提示。

Codex 的 `enabled_tools` 與 `disabled_tools` 是受管理的欄位：清除政策會移除它們，而在
Skillshare 擁有的項目中手動編輯它們會顯示為衝突。匯入時會把 Codex 的
`enabled_tools`/`disabled_tools` 與 Copilot 的 `tools` 讀回 `tools`。Pi 的
`toolExposure` 只有在搭配 server 的 `exposure` 寫入該政策會產生完全相同的 `toolExposure`
時才會變成 `tools`；否則會保留在 `piOptions` 中，並顯示 warning。`exposure` 一律保留在
`piOptions`。

### Dashboard 中的工具 {#tool-policy-dashboard}

Server 對話框在 targets 之後有一個 **工具** 區塊，除了 `disabled` 項目以外的每個 server 都有。
這個區塊一律顯示。標題旁的資訊圖示說明這個區塊，摘要則顯示 `全部工具`、政策內容
（例如 `只允許 1 個，排除 2 個`），或載入工具後的 `已選 9 / 14`。

- 標題下方的方框放工具清單。尚未載入時提供 **載入工具**，它會用對話框目前的設定（不論是否
  已儲存）啟動 server 一次，與 [`mcp check --live`](#probe-servers-live) 使用相同的探測。只在
  點擊時執行，也不會儲存任何東西，所以新的 server 也能使用。失敗時會用白話說明原因，原始錯誤
  放在旁邊資訊圖示的提示裡，按鈕則變成 **重試**。之後修改指令、網址或相關設定，已載入的清單會被清除。
- 載入後每個工具都有一個核取方塊，勾選的工具才會給模型使用。取消勾選會把該工具的完整名稱加入
  `deny`；重新勾選會把它從 `deny` 移除，若非空的 `allow` 仍排除它，則把名稱加入 `allow`。
  被 `deny` 萬用字元排除的工具無法勾選，提示會指出是哪條規則。搜尋框可篩選清單，**全選** 與
  **全不選** 只作用在目前顯示的列，重新整理按鈕會再載入一次清單。
- 方框底部的 **排除規則** 列用來輸入 `*` 萬用字元，以及 server 沒有列出的名稱：輸入一個後按
  Enter。`allow` 有項目時，上方另有一列 **只允許**，用法相同。載入工具前，所有已儲存的項目都
  顯示在這裡。無效的名稱，或移除所有允許工具的拒絕清單，會顯示在對話框中並擋下 **儲存**。
- 再往下，對話框會說明每個已選 Agent 實際會拿到什麼：哪些會照這份清單提供工具、只套用一部分的
  Agent 會怎麼做（例如 Copilot CLI 沒有拒絕清單，仍會提供取消勾選的工具），以及哪些不支援篩選。

Server 列會以白話顯示政策標籤（例如 `工具：已排除 2 個工具`），而 **檢視各 Agent 會寫入的設定** 會針對每個 Agent
警告它不套用的部分。

## Pi {#pi}

Pi ≥ 0.99.0 已[內建 MCP](https://github.com/earendil-works/pi/blob/v0.99.0/packages/coding-agent/docs/mcp.md)，
這也是 Skillshare 為 Pi 寫入 MCP servers 的唯一方式。第三方的
`pi-mcp-adapter` 與 `pi-mcp-extension` 已不再支援作為同步目的地。

| 範圍 | 檔案 |
|---|---|
| Global | `~/.pi/agent/mcp.json`（會遵循 `PI_CODING_AGENT_DIR`） |
| Project | `.pi/mcp.json` |

個人及含憑證的 server 請放入 `~/.pi/agent/mcp.json`。僅在受信任的專案，將專案需要的 server 放入 `.pi/mcp.json`。同名 project entry 會完整取代 global entry。Skillshare 直接編輯檔案，提供預覽與備份；不會信任專案、啟動 server、安裝套件或核准 OAuth。

```bash
skillshare mcp add docs --url https://example.com/mcp --target pi --tools-deny 'delete_*' --pi-options '{"exposure":"deferred","timeout":120}' --no-tui
skillshare sync mcp --dry-run
skillshare sync mcp
```

```yaml
mcp:
  servers:
    docs:
      url: https://example.com/mcp
      targets: [pi]
      tools:
        deny: [delete_*]
      piOptions:
        exposure: deferred
        timeout: 120
```

原生輸出使用 `command`/`args` 或 `url`，搭配 `${NAME}` 環境變數參照。同步後，在 Pi 執行
`/reload` 或開啟新 session，並用 `/mcp` 檢查連線及核准 OAuth。只設定 Pi 的簡單 server，
可用 `pi mcp add` 編輯全域檔案；加 `-l` 寫入 project 檔案。`pi mcp list` 會啟動所有啟用的
server 檢查連線；`pi mcp login NAME` 需要使用者授權。

Pi 的 server 名稱只接受字母、數字、`_` 和 `-`；只差在 `-` 和 `_` 的名稱會被 Pi
視為同一個 server，因此同步會拒絕第二個。Pi 的 project 項目會整筆取代 global
中的同名項目；要在單一 project 中關閉 global server，請見
[Turn off a global server in one project](#turn-off-a-global-server-in-one-project)。

Pi 1.0.1 起，Pi 的 `/mcp` 可以在專案中新增只有 `enabled`、`exposure` 或 `toolExposure` 的項目，
用來覆寫同名的 global server。它不是 server，所以匯入會略過它。`disabled` 項目寫入的也是這種覆寫，
所以剛好是 `{"enabled": false}` 的覆寫不算衝突。同步寫入開關後，你在 Pi 中替它加上的 Pi 設定（例如
`exposure`）會像其他由同步管理的 Pi 項目一樣保留；在 Pi 中把 server 重新開啟則算衝突。如果專案定義了
同名的 server，或不是同步寫入的覆寫和 `disabled` 項目不同，同步會回報衝突，直到你取代該項目，或在 Pi
中移除這個覆寫。

### 其他 Pi 設定 {#pi-options}

`piOptions` 存放 Pi 內建 MCP 的其他單一 server 欄位，只有 Pi 會收到。

- `exposure` 支援 `codemode`（Pi 預設）、`codemode-deferred`（`codemode` 的舊名稱）、
  `deferred`、`direct` 或 `hidden`。`toolExposure` 把工具名稱或萬用字元對應到上述其中一個值：完整名稱優先，
  其次是第一個符合的萬用字元。匯入與 JSON／YAML 轉換會保留規則順序。`exposure` 也決定
  [`tools`](#tool-policy) 允許清單保留的工具如何提供。建議用 `tools` 取代 `toolExposure`，
  因為它也會套用到其他 Agents；同一個 server 不能同時設定 `tools` 與 `toolExposure`。
- `timeout`（正數秒）、`cwd`、`enabled`、`oauth` 和 `auth` 會經過驗證。`description`
  等未知欄位會原樣傳遞。
- `auth: {provider: NAME}` 會把該 provider 的 `/login` token 當成 bearer token 送出。
  它需要 https 的 `url`（localhost 可用 http），而且只能在 global 模式使用，因為 Pi 只從
  global 檔案讀取它。
- `oauth.authServerMetadataUrl`（Pi 1.0 以上）必須使用 https（localhost 可用 http），因為 Pi
  會直接信任這份文件，不再自動探索。Pi 1.0 依 server 名稱與 URL 保存 OAuth 登入，所以
  重新命名 server 或修改它的 `url` 後，需要在 Pi 重新登入。
- 連線欄位請使用主要表單。`directTools`、`includeTools`、`excludeTools` 與其他
  `pi-mcp-adapter` 設定會被拒絕，因為 Pi 內建 MCP 不會讀取它們；請改用 `tools`。
- `settings` 與 `autoEnableCodemode` 是頂層設定，不是 server 選項：請直接在 Pi
  編輯；同步會保留它們。
- 憑證請使用環境變數參照。可攜式 env／headers 中的 `!command` 字面值會被拒絕，
  `piOptions` 中任何位置的命令值也一樣，包括 `oauth.clientId` 這類非 secret 欄位。

清空 JSON 或從中移除某個欄位時，若該欄位是 Skillshare 寫入且未被修改，下一次同步就會把它
從 Pi 的檔案中移除。你自己在 Pi 加入的欄位會保留。Skillshare 寫入後又在 Pi 中被修改的欄位，
會擋下同步，直到你匯入它為止。

```bash
skillshare mcp edit docs --pi-options '{"timeout":60}' --no-tui
skillshare mcp edit docs --pi-options '{}' --no-tui
```

在 dashboard 中，server 對話框的 Pi 區塊有 **工具曝光模式** 與 **其他 Pi 設定**。
**Pi 設定** 與 **工具曝光模式** 旁的資訊圖示會說明它們，**Pi 設定** 旁的連結則會開啟
Pi 的 MCP 文件。對話框會在你儲存前標出 **其他 Pi 設定** 中的 `pi-mcp-adapter` 欄位。
工具區塊有設定時，**工具曝光模式** 仍可編輯；此時只有 **其他 Pi 設定** 中的 `toolExposure`
會被拒絕，因為它由 `tools` 寫入。Server 列上的 Pi 標籤會以簡短文字顯示曝光模式，例如
`codemode` 顯示為 `透過程式碼`。

### 從 0.22 升級 Pi {#pi-migration}

升級後第一次同步之前，請先在 Pi 確認兩件事：

- **Pi 0.99.0 或更新版本。** Skillshare 現在只把 Pi 的 servers 寫入 `mcp.json`，由 Pi 在
  0.99.0 加入的內建 MCP 讀取。較舊的 Pi 不會讀這個檔案，所以在更新 Pi 之前，這些 servers
  都不會載入。Skillshare 不會檢查 Pi 的版本。
- **如果 Pi 還裝著 `pi-mcp-adapter` 或 `pi-mcp-extension`，請將它移除。** Pi 的
  [MCP 文件](https://github.com/earendil-works/pi/blob/v0.99.0/packages/coding-agent/docs/mcp.md)
  說明，已安裝且註冊了 `/mcp` 的 extension 會取代內建 MCP。`pi-mcp-extension` 本身也會讀
  `mcp.json`；`pi-mcp-adapter` 從 3.0.0 起不再讀它，所以 Skillshare 搬過去的 servers 不會
  再透過 adapter 載入。

當同步把 servers 從這兩個 extension 搬走時，`sync mcp --dry-run`、`sync mcp` 與 `--json`
會提示一次：

```text
! Pi's built-in MCP needs Pi 0.99.0 or later; on older Pi these servers stop loading until Pi is updated. If pi-mcp-adapter or pi-mcp-extension is still installed in Pi, remove it, because it can take the place of Pi's built-in MCP
```

以下情況會出現這則提示：同步移除 Skillshare 寫在 `mcp-adapter.json` 的項目、改寫它為
`pi-mcp-extension` 寫的項目，或找到只有這兩個 extension 會讀的設定（`piExtension:
pi-mcp-adapter` 或 `pi-mcp-extension`、`directTools`，以及下方列出的 `piOptions` 欄位）。
那次同步之後就不會再出現。

0.23.0 移除了 Pi 模式選擇（`piExtension`：`builtin`、`pi-mcp-adapter`、
`pi-mcp-extension`）、`piOptionsPrune` 開關與 `directTools`。舊的設定仍可載入。
`sync mcp --dry-run` 與 `sync mcp` 會為找到的每一種已淘汰設定印出一則 warning，並列出相關
servers，例如：

```text
! Pi now uses its built-in MCP; the next sync updates the config: context7, local (shop)
```

只出現在 `mcp.projects` 下某個 project 中的 server，會在括號中顯示該 project 資料夾。

下一次同步會做的事：

| 0.23.0 之前 | 同步之後 |
|---|---|
| `piExtension: builtin` | 移除該鍵；其他不變 |
| `piExtension: pi-mcp-extension` | 移除該鍵。該項目原本就在 `mcp.json` 中，因此會在原處以內建格式重寫 |
| `piExtension: pi-mcp-adapter` | 移除該鍵。server 會寫入 `mcp.json`，並移除 Skillshare 在 `mcp-adapter.json` 中寫入的項目。你自己加到 `mcp-adapter.json` 的項目維持原樣 |
| `piOptionsPrune` | 移除該鍵。同步一律會移除 Skillshare 寫入且未被修改的已清除欄位（[見上方](#pi-options)） |
| server 上的 `directTools` | `true` → `piOptions.exposure: direct`；`"search"` → `deferred`；名稱清單 → `piOptions.toolExposure`，並把這些工具設為 `direct` |
| `mcp.directTools`，或 `mcp.projects` 下某個 project 的 `directTools` | 預設值會依上述方式，寫入每個送往 Pi 且沒有自己值的 server。project 的 `false` 會覆寫 global 的值 |
| `piOptions.includeTools` / `excludeTools` | `tools.allow` / `tools.deny`；同時設定的 `directTools` 仍會轉為 `piOptions.exposure` |
| `piOptions` 中其他 `pi-mcp-adapter` 欄位：`approveTools`、字串形式的 `auth`（Pi 自己的 `auth` 物件會保留）、`bearerToken`、`bearerTokenEnv`、`bearerTokenStore`、`caFile`、`debug`、`exposeResources`、`idleTimeout`、`inheritEnv`、`lifecycle`、`protocolVersion`、`requestHeadersCommand`、`requestTimeoutMs`、`searchKeywords`、`socket`、`tasks`、`toolPrefix`、`trace` | 移除，因為 Pi 內建 MCP 不會讀取它們 |

若 `directTools`、`includeTools` 或 `excludeTools` 會覆寫 server 已設定的曝光模式，或不是
工具名稱清單，就會被捨棄，並顯示各自的 warning。

第一次套用這些變更的同步，也會儲存不含已淘汰設定的 Skillshare 設定：`config.yaml`，或
`sources.mcp` 指定的檔案。寫入前，它會把舊檔案保留在[檔案歷史](/docs/reference/commands/backup#file-history)
中，原因為 `migrate`，並為每個檔案印出一行：

```text
→ Updated config.yaml for 0.23.0 (backup: <path of the saved version>)
```

這會發生在 `skillshare sync mcp`、`skillshare sync --all`（即使沒有任何 Agent 檔案變更）
與 dashboard 的同步中。`--dry-run` 與預覽不會寫入任何內容。若儲存設定失敗，Agent 檔案已經
寫入，而設定維持原樣；錯誤訊息會說明這點，下一次同步會再試一次。儲存成功後，這些 warning
就會消失。

已移除的旗標現在會失敗並顯示訊息：

| 旗標 | 改用什麼 |
|---|---|
| `--pi-extension` | 直接拿掉。Pi 一律使用其內建 MCP |
| `--pi-options-prune` | 直接拿掉。同步一律會移除 Skillshare 先前寫入且未被修改的欄位 |
| `--direct-tools` | 所有工具用 `--pi-options '{"exposure":"direct"}'`，Pi 中的個別工具則用 `--pi-options '{"toolExposure":{"TOOL":"direct"}}'` |

`skillshare mcp import --from pi` 仍會讀取 Pi `mcp.json` 旁邊的 `pi-mcp-adapter`
`mcp-adapter.json`，讓你把 servers 搬過來。兩個檔案都定義同一個 server 時，以 `mcp.json`
為準。同步會把 server 寫入 Pi 的 `mcp.json`；在 `mcp-adapter.json` 中只會移除它在
0.23.0 之前寫入的項目。它的 `directTools`、`includeTools` 與 `excludeTools` 會依上述方式轉換，
其他 adapter 專屬欄位則會被省略並顯示 warning。在 dashboard 中，**從目標匯入** 會把這兩個
Pi 檔案列為各自獨立的來源。
