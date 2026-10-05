---
sidebar_position: 1
---

# ui

啟動用於視覺化 skill 管理的 Web dashboard。

```bash
skillshare ui                  # 在前景執行
skillshare ui start            # 啟動（或重用）背景伺服器
skillshare ui stop             # 停止背景伺服器
```

在你的預設瀏覽器中開啟 `http://127.0.0.1:19420`。

## 模式

| 模式 | 行為 |
|------|----------|
| `skillshare ui`（預設） | 在前景執行 UI 伺服器；`Ctrl+C` 可停止它 |
| `skillshare ui start` | 以背景行程啟動 UI 伺服器並將控制權交還給 shell。重新執行 `start` 時，若既有行程仍健康則會重用它 |
| `skillshare ui stop` | 停止由 `skillshare ui start` 啟動的背景 UI 伺服器 |

## 何時使用

- 透過視覺化 Web 介面管理 skills、targets 與 sync
- 瀏覽並安裝 skills，不必死記 CLI flags
- 執行安全稽核並取得視覺化的發現報告
- 與較不熟悉 CLI 的團隊成員分享 dashboard 檢視畫面

## Flags

| Flag | 預設值 | 說明 |
|------|---------|-------------|
| `-p`, `--project` | | 以 project mode 執行（使用 `.skillshare/`） |
| `-g`, `--global` | | 以 global mode 執行（使用 `~/.config/skillshare/`） |
| `--port <port>` | `19420` | HTTP 伺服器連接埠 |
| `--host <host>` | `127.0.0.1` | 綁定位址（Docker 請使用 `0.0.0.0`） |
| `-b`, `--base-path <path>` | | 供 reverse proxy 使用的子路徑（例如 `/skillshare`） |
| `--no-open` | `false` | 不自動開啟瀏覽器 |
| `--app` | `false` | 在可用時以桌面應用程式風格的 Chromium app 視窗開啟 dashboard（僅 `start` 模式） |
| `--clear-cache` | | 在前景模式中：清除快取的 UI 資源後結束。搭配 `start`：先清除快取，再於背景啟動 |

:::tip 自動偵測
若目前目錄存在 `.skillshare/config.yaml`，dashboard 會自動以 project mode 啟動。使用 `-g` 可強制以 global mode 啟動。
:::

## 範例

```bash
# 預設：在前景以 localhost:19420 開啟瀏覽器
skillshare ui

# Project mode（管理 .skillshare/ 中的 skills）
skillshare ui -p

# 自訂連接埠
skillshare ui --port 8080

# Docker / 遠端存取
skillshare ui --host 0.0.0.0 --no-open

# 在背景啟動並返回 shell
skillshare ui start

# 以無瀏覽器外框的桌面應用程式風格視窗啟動
skillshare ui start --app

# 停止背景伺服器（使用記住的 host/port）
skillshare ui stop

# 清除快取的 UI 資源，然後在背景重新啟動
skillshare ui start --clear-cache
```

## Dashboard 頁面

側邊欄依任務分組頁面：同步、你管理的內容、內容去向，以及維護作業。名稱下方的一行會顯示模式與其資料夾，例如 `Global · ~/.config/skillshare`。

部分頁面在需要注意時會在側邊欄顯示數字提示。這些數字在 dashboard 分頁開啟時每 15 秒重新整理一次：

- **Sync**：一次同步會套用的變更
- **Git Sync**：未 commit 的檔案，或在工作目錄乾淨時尚未 push 的 commits
- **Audit**：上次掃描時被阻擋的 skills 與 agents（執行過一次掃描後才會顯示）

| 頁面 | 說明 |
|------|------|
| **Dashboard** | Skills、agents、extras、MCP servers、hooks、plugins 與 targets 的數量，以及需要注意的項目 |
| **Sync** | 在寫入前預覽每個 target 的每項變更。選擇要包含的部分（Skills、Agents、Extras、MCP）。在 target 內編輯過的檔案，除非開啟 **Force**，否則會保留。只存在於 target 中的項目可以從這裡收集回 source。每次同步都會先備份 target 資料夾。某個 target 失敗時，其他 target 仍會同步：頁面會在其他警告上方列出每個失敗的 target，以及它的部分（Skills、Agents、Extras 或 Config）和錯誤，常見原因（symlink 指向別處、權限不足、唯讀檔案系統、檔案或資料夾不存在、target 設定無效）還會附上一句易懂的說明；若 Skills 的 symlink 指向別處，會提供 **開啟 Force**；變更清單中也會標出該 target。即使所有 target 都失敗，也會以同樣方式列出。**上次同步** 卡片會列出最近一次同步中失敗的 target |
| **Git Sync** | Commit 並 push source repo、push 尚未在 remote 上的 commits，以及 pull。開啟頁面時會先從 remote fetch，所以 **Pull** 按鈕會顯示 remote 有幾個新的 commits。Pull 會同步 repo scope 所涵蓋的內容（`skills`、`agents`、`extras` 或 `root`），如同 [`pull`](/docs/reference/commands/pull)。**Sync both ways** 會 commit 本機變更、pull 並合併、sync targets，然後 push，如同 [`push --pull`](/docs/reference/commands/push#push-and-pull-together)；遇到衝突時會在 push 前停止。當 remote 因為有較新的 commits 而拒絕 push 時，錯誤訊息會提供 **Pull**。當第一次 pull 無法與 remote 合併時，會提供強制 pull 以本機檔案取代 remote 分支 |
| **Hubs** | 從 Skills 頁面進入。列出內建 hub、已儲存的 hubs 與你自己的 Hub（**我的**）；選擇一個即可篩選並安裝其中的 skills。**加入或建立 Hub** 可加入現有的 hub、建立新的 Hub，或匯入 `skillshare-hub.json`。自己的 Hub 以 **編輯** 修改；**分享** 會下載索引並產生 `hub add` 指令。參見 [`hub`](/docs/reference/commands/hub) |
| **Skills** / **Agents** | 已安裝的項目、**Updates** 分頁與 **Trash** 分頁。Skills 還有一個 **Analyze** 分頁，用來估算每個 skill 為 target 的 context 增加多少 tokens。**Install** 可搜尋 GitHub，或從 URL 或路徑安裝。**+ New Skill** 開啟建立精靈。列表與卡片檢視可依 **Folder**（tracked repo、`frontend/react` 這類資料夾，或 **Root**）篩選，也能依 **Folder** 分組，Root 排最前，其餘資料夾依名稱 A→Z 排序。**tree** 檢視左邊顯示 source 資料夾，右邊顯示詳細資訊：點資料夾或 skill 來選取，Cmd/Ctrl 點擊加入選取，Shift 點擊選取範圍，雙擊 skill 開啟它。右邊可用一個開關啟用或停用所有選取的項目（被 `.skillignore.local` 的 `!` 規則保持啟用的項目會回報為失敗並附上原因）、設定它們的 targets（包括 tracked repo 與其子資料夾），並列出每個 skill 各自的開關；tracked repo 另外提供 **Update repo** 與 **Uninstall repo**。帶有 `disable-model-invocation: true` 的 skill 會在列表、其磚塊（tile）與詳細頁面上帶有 **manual only** 標籤，這與 [`list`](/docs/reference/commands/list) 中用 `m` 切換的狀態相同。在 skill 編輯器中，**Add field** 說明每個 frontmatter 欄位的作用。**Sync skills** / **Sync agents** 會先預覽，再只把該類型同步到所有 targets；更新、解除安裝或 collect 之後，從 **Sync Now** 也會開啟同一個對話框 |
| **Extras** | **目錄**：與 skills 一同同步的 rules、commands 及其他資料夾。**AGENTS.md**：在 global mode 中是共用 `AGENTS.md` 與使用它們的 targets；在專案中則是專案的 `./AGENTS.md`，以及每個 target 是否讀得到它。詳見[讓多個工具共用一份 AGENTS.md](../../how-to/daily-tasks/sharing-instructions.md)  **Memory**: [記憶共用教學](../../how-to/daily-tasks/sharing-memory.md)（英文截圖）。搜尋、預覽、編輯、刪除筆記、INDEX 連結、衝突時保留草稿、Backup Files 歷史與還原，以及 **Connect to agents** → **Review changes** → **Apply changes**；設定狀態不代表已讀取 |
| **MCP** | 每個 server 一列，並以 chips 顯示它同步至哪些 Agents；列上的計數按鈕會開啟它們的切換開關。**新增伺服器** 接受 URL、指令、貼上的片段或檔案；**從目標匯入** 讀取已安裝 Agent 目前的設定。每個 server 的選單都有 **檢視各 Agent 會寫入的設定**，會列出來源與每個 Agent 的檔案，並顯示你選取的那一個（含尚未儲存的編輯內容）。衝突時提供 **從該 Agent 匯入** 或 **以來源覆寫**。server 對話框的 **工具** 區塊用來設定[工具政策](./mcp.md#tool-policy)：**載入工具**，它會用對話框目前的設定（不論是否已儲存）啟動 server 一次並列出工具供勾選；用於 `*` 萬用字元的 **排除規則** 列；區塊會列出每個不套用部分政策的已選 Agent，設定政策後該列也會顯示標籤。對於 Pi server，對話框另有 [Pi 內建 MCP](./mcp.md#pi) 的設定：工具曝光模式與其他 Pi 設定，後者以 JSON 填入 [`piOptions`](./mcp.md#pi-options)，說明則放在資訊提示中。**預設值** 用來編輯 `mcp.targets`。Sync 框中的 **Sync MCP** 會列出待寫入的變更，只寫入 MCP 設定檔，並為每個檔案保留備份；它下方的 [**檢查**](./mcp.md#check-servers-before-an-agent-starts-them) 會檢查 servers，**備份與還原** 則可預覽並還原這些備份。 |
| **Plugins** | 每個 plugin 一列，並以其 Agents 作為切換開關。展開一列還會列出來源支援的其他 Agents；勾選其中之一即可預覽安裝內容。列選單可以同步、更新、移除，或開啟 **View files**，也就是唯讀瀏覽 Skillshare 已檢視過的本機副本。參見 [跨工具管理 plugins](/docs/how-to/daily-tasks/sharing-plugins) |
| **Targets** | 附帶狀態的 target 列表。**新增目標** 也接受 **另一個帳號**：你已在使用的某個 Agent 的第二個 config 資料夾，並會預覽它寫入的位置。每個 target 的頁面可編輯 include/exclude 篩選條件，並將僅存於本機的 skills 收集回 source。**同步內容** 欄以連結列出每個 target 拿到的東西：**Skills**、**Agents**、**MCP**、**Hooks** 與數量、Pi 的 **Extensions**，以及接上共用 `AGENTS.md` 的 target 指示檔；點哪個就開 target 頁的那個分頁。有 MCP 設定檔的 target 會多一個 **MCP** 分頁，每個 server 一列；點一下就會儲存，**Sync all targets** 會寫入所有 target 的 MCP 設定檔。每個 target 還有一個以它所讀檔案命名的分頁（**CLAUDE.md**、**GEMINI.md**、**AGENTS.md**……），會顯示讀取順序、編輯該檔案，並能將它轉換成 `AGENTS.md` |
| **Projects** | 僅限 global mode。global config 會同步進去的 project 資料夾，來自 [`projects`](/docs/reference/targets/configuration#projects) 與 [`mcp.projects`](./mcp.md#projects-in-the-dashboard)。**新增專案** 會要求填入資料夾、它的 targets，以及要同步的內容。每個 project 都有 **Skills** 與 **Agents** 分頁，附帶篩選條件、預覽畫面與會被寫入的資料夾，還有一個 **MCP** 分頁可在該資料夾中關閉 global servers 或給它專屬的 servers，並有 **檢查** 可檢查這些 servers。**Sync project** 會先預覽，再只同步該 project 的 skills、agents 與 MCP。已經指向某個 project 資料夾的 target 可以被轉換 |
| **Audit** | 對 skills 與 agents 進行安全掃描，依嚴重程度列出發現項目。**Rules** 分頁可依分類瀏覽每一項規則：關閉某項、變更其嚴重程度、對整個分類套用嚴重程度、選擇掃描設定檔（`default`、`strict`、`permissive`），或開啟編輯器自訂 `audit-rules.yaml` |
| **Settings** | 分頁式：**General**（source 路徑、同步模式、外觀）、**Backup**（target 資料夾快照、`AGENTS.md` 這類檔案的較早版本，以及 MCP 設定備份；參見 [`backup`](./backup.md#dashboard)）、**Log**（操作歷史）、**Health**（與 [`doctor`](/docs/reference/commands/doctor) 相同的檢查）、**Extensions**（同步時的檔案轉換）、**Files**（直接編輯 `config.yaml`、`.skillignore` 與 `.agentignore`；有 `.skillignore.local` 或 `.agentignore.local` 時，因為它在該檔案之後套用、可以覆蓋它，編輯器會在上方顯示它的規則） |

變更清單旁的 **捨棄變更** 會在確認後，將所選 Git 範圍內所有已追蹤的檔案與暫存區還原至最後一次提交，並刪除未追蹤的檔案與資料夾。被 Git 忽略的檔案、巢狀 Git 儲存庫，以及 `root` 範圍的 `config.yaml` 都會保留。此操作不會更動提交紀錄，也不會推送至遠端，且無法復原。**試跑** 只會預覽，不會變更檔案。儲存庫必須已有第一次提交，才能捨棄變更。

在 **Updates** 分頁中，進度列會顯示更新進度，正在更新的列也會標示出來。被阻擋或失敗的更新會顯示在獨立區塊。

舊連結如 `/collect`、`/install`、`/search`、`/trash`、`/analyze`、`/backup`、`/log` 與 `/doctor` 會重新導向至新的位置。

**Files** 分頁會在編輯器旁顯示一個面板。對於 `config.yaml`，它會顯示游標所在欄位的作用、檔案結構與尚未儲存的變更；對於 ignore 檔案，它會列出目前 patterns 隱藏了哪些內容。`Cmd+S` / `Ctrl+S` 可儲存。**Audit -> Rules -> Edit YAML** 下的 rules 編輯器有相同的面板，外加一個 **Test** 分頁，可用你貼上的行來測試規則的正規表示式。

### 主題系統

Dashboard 支援兩種視覺風格與三種色彩模式，可透過側邊欄的 **Theme** 按鈕切換：

| 設定 | 選項 | 預設值 |
|---------|---------|---------|
| **Style** | `Clean`（專業風格）、`Playful`（粗體外框、硬陰影、手寫標題） | Playful |
| **Mode** | `Light`、`Dark`、`System`（跟隨作業系統偏好設定） | Light |

主題偏好設定會持久保存在 localStorage 中，跨 session 保留。

### Project Mode 的差異

在 project mode（`-p`）中執行時，dashboard 會做以下調整：

- **側邊欄**會在名稱下方顯示 `Project · <project path>`
- **Git Sync 頁面**被隱藏（project skills 使用該 project 自己的 git）
- **Sync** 只會備份 agent target 資料夾，如同 `skillshare sync -p`
- Settings 中的 **Backup 分頁**被隱藏（請改用版本控制）
- Dashboard 中的 **Tracked Repos 區塊**被隱藏（不適用）
- **Settings -> Files** 會顯示 `.skillshare/config.yaml` 與 project 層級的 `.skillignore`，而非 global 版本
- **Available targets** 會列出 project 層級的 targets（例如相對於 project 根目錄的 `.claude/skills/`）
- **Targets** 會統計並切換 project 的 MCP servers，寫入 project 自己的檔案（例如 `.mcp.json`）。Claude Code、OpenCode、Kilo Code 與 Pi 的 **MCP** 分頁也會列出在這個 project 關閉 global server 的開關
- **Install** 會自動調和 project config 中的 `skills:` 項目
- **Extras -> AGENTS.md** 會編輯專案的 `./AGENTS.md`，而不是共用檔案，並為只讀自己檔案的 targets 提供一個小修正

## UI 預覽

<div style={{display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))', gap: '1rem'}}>
  <img src="/img/web-install-demo.png" alt="安裝流程" />
  <img src="/img/web-dashboard-demo.png" alt="儀表板總覽" />
  <img src="/img/web-skills-demo.png" alt="技能瀏覽頁面" />
  <img src="/img/web-skill-detail-demo.png" alt="技能詳細資訊頁面" />
  <img src="/img/web-sync-demo.png" alt="同步控制項" />
  <img src="/img/web-search-skills-demo.png" alt="GitHub 搜尋畫面" />
  <img src="/img/web-projects-demo.png" alt="列出專案資料夾的 Projects 頁面" />
</div>

## REST API

Web dashboard 在 `/api/` 上提供 REST API。所有端點皆回傳 JSON。

| Method | Path | 說明 |
|--------|------|------|
| GET | `/api/overview` | Skill/target 數量、模式、版本、config 資料夾（`configDir`） |
| GET | `/api/skills` | 列出所有 skills 及其 metadata |
| GET | `/api/skills/{name}` | Skill 詳細內容 + SKILL.md 內容 |
| GET | `/api/skills/templates` | 取得可用於建立 skill 的 patterns 與分類 |
| POST | `/api/skills` | 建立新 skill（name、pattern、category、scaffoldDirs） |
| DELETE | `/api/skills/{name}` | 解除安裝一個 skill |
| GET | `/api/targets` | 列出 targets 及其狀態、include/exclude 篩選條件，以及每個 target 的預期數量 |
| POST | `/api/targets` | 新增一個 target |
| DELETE | `/api/targets/{name}` | 移除一個 target |
| POST | `/api/sync` | 執行同步（支援 `dryRun`、`force`、`kind`，以及 `project`：一個已宣告的 project 根目錄，會把同步範圍限定在該 project 的 targets）。除非設定 `dryRun`，否則會先備份 targets |
| POST | `/api/git/commit` | 從 source repo 建立本機 git commit，但不 push |
| POST | `/api/git/discard` | 捨棄設定的 Git 範圍內尚未提交的變更（僅限全域模式，儲存庫必須已有第一次提交）。支援 `dryRun`；保留被 Git 忽略的檔案、巢狀 Git 儲存庫，以及 `root` 範圍的 `config.yaml` |
| GET | `/api/git/status` | Source repo 狀態，包含尚未 push 的 commits（`ahead`），以及截至上次 fetch 尚未 pull 的 upstream commits（`behind`）。不會執行 fetch |
| POST | `/api/push` | Commit 所有變更後再 push。首次 push 時會設定 upstream。當 remote 有這個 repo 沒有的 commits 時，會以 `409` 與錯誤代碼 `push_rejected` 失敗；先 pull 再 push 即可 |
| POST | `/api/pull` | Pull 之後同步 repo scope 所涵蓋的內容。已分歧的歷史會被合併；`.metadata.json` 的衝突會自動解決，其他衝突則會失敗並復原 merge。當第一次 pull 無法合併時，會以錯誤代碼 `merge_failed` 失敗；帶 `force: true` 重試可以本機檔案取代 remote 分支。帶 `alwaysSync: true` 時，即使沒有拉到新內容也會同步 targets。remote 沒有任何分支時回傳 `400 remote_empty` |
| GET | `/api/diff` | Source 與 targets 之間的差異 |
| GET | `/api/search?q=` | 在 GitHub 上搜尋 skills |
| POST | `/api/install` | 從來源安裝一個 skill |
| GET | `/api/audit` | 掃描所有 skills 是否存在安全威脅 |
| GET | `/api/audit/rules` | 取得自訂稽核規則 YAML |
| PUT | `/api/audit/rules` | 儲存自訂稽核規則（驗證正規表示式） |
| POST | `/api/audit/rules` | 建立起始 audit-rules.yaml |
| GET | `/api/audit/rules/compiled` | 合併內建規則與自訂規則後的每一條規則，以及目前生效的 profile |
| POST | `/api/audit/rules/toggle` | 啟用、停用或重新調整某個規則或整個 pattern 的等級 |
| POST | `/api/audit/rules/reset` | 刪除自訂規則並還原內建預設值 |
| PATCH | `/api/audit/policy` | 設定 `blockThreshold`、`profile`，或兩者皆設 |
| GET | `/api/log` | 列出帶有可選篩選條件的日誌項目 |
| GET | `/api/config` | 取得 YAML 格式的 config |
| PUT | `/api/config` | 更新 config YAML |
| GET | `/api/skillignore` | 取得 `.skillignore` 內容、存在時的 `.skillignore.local` 內容，以及忽略統計 |
| PUT | `/api/skillignore` | 更新 `.skillignore` 內容 |
| GET | `/api/doctor` | 執行所有健康檢查（JSON） |
| GET | `/api/health` | 存活探測；伺服器就緒後回傳 `200` |
| GET | `/api/version` | 目前/最新版本 + 是否有可用升級 |
| POST | `/api/upgrade` | 就地執行 `skillshare upgrade`（若執行檔為開發版本則回傳 `devMode: true`） |
| POST | `/api/restart` | 重啟本機 UI 伺服器；可選的 `{ "clearCache": true }` body 會先清除快取的 UI 資源 |

## 就地升級

當 dashboard 偵測到有更新的 CLI 版本可用時，**Update** 對話框與 **Doctor** 頁面的 *Version* 卡片都會顯示 **Update now** 按鈕：

1. UI 呼叫 `POST /api/upgrade`，在主機上執行 `skillshare upgrade`。
2. 新執行檔就緒後，UI 呼叫 `POST /api/restart` 重啟本機伺服器。
3. 瀏覽器輪詢 `GET /api/health`，並在新伺服器就緒後自動重新載入。

如果正在執行的是開發版本（`version == "dev"`），upgrade 端點會回傳 `devMode: true`，UI 會模擬重啟而不修改磁碟上的任何內容。

如果自動重新載入沒有完成，對話框會提示你執行 `skillshare ui start` 以重新啟動背景伺服器。

## Reverse Proxy {#reverse-proxy}

如果你在共用伺服器上、透過 reverse proxy 執行 dashboard（例如 homelab、內部工具平台），可使用 `--base-path` 讓它在子路徑下與其他服務並存：

```bash
skillshare ui --base-path /skillshare --host 0.0.0.0 --no-open
```

或透過環境變數：

```bash
SKILLSHARE_UI_BASE_PATH=/skillshare skillshare ui --host 0.0.0.0 --no-open
```

### Nginx

```nginx
location /skillshare/ {
    proxy_pass http://127.0.0.1:19420;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
}
```

### Caddy

```
handle_path /skillshare/* {
    reverse_proxy 127.0.0.1:19420
}
```

:::tip
不使用 `--base-path` 時，dashboard 的行為與以往完全相同——直接在 `localhost:19420` 存取不需要任何額外設定。
:::

:::note MCP 設定
MCP 頁面只有在瀏覽器以 `localhost` 或 IP 位址（例如 `http://192.168.1.20:19420`）開啟 dashboard 時才能運作。透過網域名稱存取時（包括 reverse proxy），MCP 請求會回傳 403：DNS rebinding 攻擊一律使用網域名稱。若要在遠端機器上管理 MCP 設定，請用 `ssh -L 19420:127.0.0.1:19420 HOST` 轉發連接埠，再開啟 `http://localhost:19420`。
:::

## Docker 使用方式

若要在 Docker 中使用 Web UI（首次下載 UI 需要網路連線）：

```bash
make playground

# 在容器內：
skillshare ui --host 0.0.0.0 --no-open
```

然後在主機上開啟 `http://localhost:19420`（連接埠 19420 會自動對應）。

## Project Mode

Web dashboard 完整支援 project 層級的 skills：

```bash
cd my-project
skillshare ui -p
```

或者，如果 `.skillshare/config.yaml` 存在（自動偵測），直接執行 `skillshare ui` 即可。

Dashboard 會讀寫 `.skillshare/config.yaml`、同步至 project 本機的 targets，並在安裝後調和遠端 skill 項目——就跟 CLI 一樣。

## 執行時 UI 下載

`skillshare ui` 會在首次啟動時，自動從對應的 GitHub Release 下載預先建置的 UI 資源。這些資源會快取於 `~/.cache/skillshare/ui/<version>/`（遵循 `XDG_CACHE_HOME`），因此後續啟動會是即時且離線的。

- **首次執行**需要網際網路連線來下載 UI 資源（約 2 MB）
- **後續執行**使用快取的資源——不需要網路
- **升級時**，舊的快取版本會自動清除；新 UI 會在 `skillshare upgrade` 期間預先下載
- **手動清除快取**，執行 `skillshare ui --clear-cache`

## Homebrew 說明

所有安裝方式（Homebrew、安裝腳本、手動下載執行檔）都使用執行時 UI 下載。當你執行 `skillshare ui` 時，它會在首次啟動時自動從 GitHub 下載 UI 資源。之後，會使用快取的資源離線運作。

若要清除已下載的 UI 快取：

```bash
skillshare ui --clear-cache
```

## 架構

Web UI 是一個單頁 React 應用程式，於執行時從對應的 GitHub Release 下載，並從磁碟快取（`~/.cache/skillshare/ui/<version>/`）提供服務。

```
skillshare ui
  ├── Go HTTP server (net/http)
  │   ├── /api/*    → REST API handlers
  │   └── /*        → Cached React SPA (runtime download)
  └── Browser opens http://127.0.0.1:19420
```

## 另請參閱

- [status](/docs/reference/commands/status) — CLI 狀態檢查
- [sync](/docs/reference/commands/sync) — CLI 同步指令
- [Project Setup](/docs/how-to/sharing/project-setup) — Project mode 設定指南
- [Docker Sandbox](/docs/how-to/advanced/docker-sandbox) — 在 Docker 中執行 UI
