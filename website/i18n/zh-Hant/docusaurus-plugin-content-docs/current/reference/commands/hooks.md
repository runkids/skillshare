---
sidebar_position: 4
---

# hooks

管理具名 hooks，將各 Agent 的原生定義同步至設定檔。Dashboard 的 **Hooks** 提供新增、編輯、匯入、啟用／停用、預覽與同步；「目標」、「專案」、「同步」及「設定 → 備份」也有對應入口。

從 hook 每列的選單可檢視各目標的原生設定或程式碼，包括腳本檔案與目的地路徑。這個唯讀預覽只顯示該 hook 的內容，共用檔案會保留其他設定。停用的 hook 仍可檢視，但不會發布。

## 指令

```bash
skillshare hooks
skillshare hooks list --json
skillshare hooks add check --file ./check.yaml
skillshare hooks edit check --file ./updated-check.yaml --sync
skillshare hooks import --from claude --json
skillshare hooks import imported --from claude --file ./settings.json --dry-run
skillshare hooks disable check --sync
skillshare hooks enable check --sync
skillshare hooks sync --dry-run --json
skillshare hooks sync
skillshare hooks sync check --replace --dry-run
skillshare sync hooks --dry-run --json
skillshare sync hooks
skillshare hooks remove check --sync
skillshare hooks remove check --keep-files
skillshare hooks restore BACKUP_ID --dry-run
skillshare hooks restore BACKUP_ID
```

沒有子指令時列出 entries。`add`／`edit` 從 `--file` 讀取 Entry JSON 或 YAML；匯入不指定名稱時列出候選項目（每個 Agent event 或檔案各一項），指定名稱才儲存。儲存匯入即原地接管讀到的註冊：下次同步直接 adopt，不需 `--replace`；沒匯入的 event 維持不受管理。匯入只讀取設定或程式碼，不會執行。新增、編輯、匯入、啟用、停用與移除預設只儲存來源，加入 `--sync` 才同步。同步與還原會重新預覽。

| Option | Meaning |
|---|---|
| `--file PATH` | Entry JSON/YAML；匯入時為原生設定或程式碼 |
| `--from AGENT` | `codex-2` 等帳號 target 也可使用。  原生 Agent 格式或要讀取的 Agent |
| `--sync` | 儲存並同步 |
| `--keep-files` | 搭配 `remove`：停止管理該 hook，並讓它的原生項目維持原樣。不可與 `--sync` 併用。參見[下方說明](#stop-managing-a-hook) |
| `--replace` | 明確取代既有來源 entry 或該 entry 衝突的原生輸出 |
| `--dry-run, -n` | 只預覽，不儲存或寫入 |
| `--json` | 結構化輸出 |
| `--revision ID` | 要求符合指定預覽 revision |
| `--global, -g` | 使用 global 設定 |
| `--project, -p` | 使用 project 設定 |

`hooks list --json` 顯示備份 ID 與完整目的地路徑。設定變動後，舊預覽失效；儲存或同步前須重新預覽。

## 停止管理某個 hook {#stop-managing-a-hook}

```bash
skillshare hooks remove check --keep-files
```

這會從來源移除 `check`，並忘記 Skillshare 為它寫入的原生註冊與檔案。Agent 檔案不會變動。之後這些項目就屬於你：同步不會再移除或更新它們，`hooks import` 也會再次列出它們。`--keep-files` 不可與 `--sync` 併用。Dashboard 的移除對話框在 Hooks 頁面與專案的 **Hooks** 分頁都提供 **停止管理**。**只從來源移除** 則不同：下次同步時，會從 Agent 檔案刪掉該 hook 的項目。

只會影響你移除它的那個範圍。停止管理 global hook 時，專案中同名的 hook 仍受管理，反之亦然。若要重新管理這些項目，請匯入它們。

## 來源欄位

宣告放在目前 Skillshare 設定的 `hooks.entries`。名稱識別一個 entry；`bindings` 指定接收的 Agent 及其原生定義。空的 bindings 保留來源，但不發布。

```yaml
hooks:
  entries:
    check:
      description: Run the project's check after Claude finishes
      enabled: true
      bindings:
        claude:
          events:
            Stop:
              - hooks:
                  - type: command
                    command: "make check"
                    timeout: 120
```

傳給 `hooks add check --file check.yaml` 的檔案只包含 Entry 的 `description`、`enabled`、`bindings`，不包含 `hooks.entries` 外層。

Skillshare 以縮排的 block 格式寫入 `hooks` 區段；每次儲存也會把先前被擠成一行的 entry 展開。Hooks 頁的 **config.yaml** 按鈕會開啟「設定 → 檔案」並定位到 `hooks:` 區段；點擊 `hooks` 底下的欄位，右側面板會顯示說明。**美化** 會展開巢狀的單行區段，`targets: [claude, codex]` 這類短清單則保持一行。

| Option | Meaning |
|---|---|
| `description` | 可選描述 |
| `enabled` | 預設 true；false 保留來源，下次同步移除未被修改的自有輸出 |
| `bindings` | Agent ID 到原生 binding 的對應 |
| `bindings.AGENT.events` | command／設定型 Agent 的原生 event map |
| `bindings.AGENT.code` | Pi、Amp、OpenCode 的原生 extension/plugin 程式碼 |
| `bindings.AGENT.files` | 可選 UTF-8 腳本檔，以相對檔名為 key |

Agent ID 為 `claude`、`codex`、`gemini`、`copilot`、`cursor`、`droid`、`qwen`、`antigravity`、`pi`、`amp`、`opencode`、`git`；`factory` 是 `droid` 的別名，`antigravity-cli` 與 `agy` 是 `antigravity` 的別名。event、matcher、handler type、command、timeout 單位與 payload 均保留原生格式，不自動跨 Agent 轉換。event 名稱會對照各 command Agent 文件列出的事件檢查：未知名稱（例如拼錯的 `Stopp`）在預覽與 plan 的 `warnings` 中顯示警告，但不阻擋同步，因為 Agent 會陸續新增事件。Pi、Amp、OpenCode 的程式碼不檢查。Pi、Amp、OpenCode 的程式碼與 imports 須符合已安裝版本；發布至獨立的 `skillshare-NAME.ts`，不產生共用執行引擎。command binding 的腳本位於 Agent 設定目錄的 `hooks/skillshare/NAME/`，command 保留你提供的原生 macro 或明確路徑。請在預覽確認完整路徑。 global binding 也可以使用在 `targets` 下宣告的帳號 target 名稱。

## 原生目的地

| Agent | Global | Project | Format |
|---|---|---|---|
| Git | `$XDG_CONFIG_HOME/git/skillshare/hooks.gitconfig` | `<git-common-dir>/skillshare/hooks.gitconfig` | Git 2.54+ `hook.<name>` |
| [Claude Code](https://code.claude.com/docs/en/hooks) | `~/.claude/settings.json` | `.claude/settings.json` | `hooks` event map with matcher groups |
| [Codex](https://learn.chatgpt.com/docs/hooks) | `~/.codex/hooks.json` | `.codex/hooks.json` | Wrapped `hooks` event map |
| [Gemini CLI](https://geminicli.com/docs/hooks/reference/) | `~/.gemini/settings.json` | `.gemini/settings.json` | `hooks` event map |
| [Copilot CLI](https://docs.github.com/en/copilot/reference/hooks-reference) | `~/.copilot/hooks/skillshare-NAME.json` | `.github/hooks/skillshare-NAME.json` | Version 1, `hooks` event map |
| [Cursor](https://cursor.com/docs/hooks) | `~/.cursor/hooks.json` | `.cursor/hooks.json` | Version 1, native lowerCamelCase events |
| [Factory Droid](https://docs.factory.com/harness/hooks) | `~/.factory/hooks.json` | `.factory/hooks.json` | Unwrapped event map |
| [Qwen Code](https://qwenlm.github.io/qwen-code-docs/en/users/features/hooks/) | `~/.qwen/settings.json` | `.qwen/settings.json` | `hooks` event map |
| [Antigravity](https://antigravity.google/docs/hooks) | `~/.gemini/config/hooks.json` | `.agents/hooks.json` | 具名 hook block，每個 hook 一個 |
| [Pi](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/extensions.md) | `~/.pi/agent/extensions/skillshare-NAME.ts` | `.pi/extensions/skillshare-NAME.ts` | Native extension code |
| [Amp](https://ampcode.com/docs/plugin-api) | `~/.config/amp/plugins/skillshare-NAME.ts` | `.amp/plugins/skillshare-NAME.ts` | Native plugin code |
| [OpenCode](https://opencode.ai/docs/plugins/) | `~/.config/opencode/plugins/skillshare-NAME.ts` | `.opencode/plugins/skillshare-NAME.ts` | Supplied v1/v2 plugin code |

global scope 使用原生設定目錄的環境變數 override，但指向帳號 `config_dir` 時除外；project scope 只寫入專案，不退回 global 路徑。Codex inline TOML 等其他來源仍獨立存在。Antigravity 與其 CLI（`agy`）讀取同一份 `hooks.json`；每個 hook 是一個以其名稱命名的 block，匯入時保留原名。CLI 的 `~/.gemini/antigravity-cli/settings.json` 中的 hooks 保持獨立。Droid 發布獨立 hooks 檔會影響原生載入來源，請先檢查既有 inline hooks。Copilot 只在受信任的資料夾中載入 `.github/hooks` 的專案 hooks。


Droid 有有效 inline hooks 時，同步會拒絕建立獨立檔案。先匯入並檢查，移除原 inline hooks 後再同步。

## 某個 Agent 的另一個帳號 {#accounts}

宣告了 `agent` 與 `config_dir` 的 target 可以用自己的名稱接收 hooks。Claude、Codex 與 Pi 帳號使用其 Agent 的原生 binding 格式。`git` 這個 key 永遠代表 Git config hooks，名為 `git` 的帳號不會收到 hooks；只要有 entry 使用 `git` binding，計畫就會顯示警告；請改用其他名稱。

```yaml
targets:
  codex-2:
    agent: codex
    config_dir: ~/.codex-2
    skills: {enabled: false}
hooks:
  entries:
    check:
      bindings:
        codex-2:
          events:
            Stop:
              - hooks:
                  - type: command
                    command: "echo checked"
```

| 帳號的 Agent | 原生目的地 |
|---|---|
| `claude` | `<config_dir>/settings.json` |
| `codex` | `<config_dir>/hooks.json` |
| `pi` | `<config_dir>/extensions/skillshare-NAME.ts` |

command 腳本寫入 `<config_dir>/hooks/skillshare/NAME/`。帳號僅用於 global 設定；project 設定與 `hooks.projects` 應綁定 Agent 本身，因為所有帳號都會讀取相同的專案檔案。`config_dir` 不存在時會警告並跳過，不會建立帳號目錄。

當 `CODEX_HOME`、`CLAUDE_CONFIG_DIR` 或 `PI_CODING_AGENT_DIR` 指向已宣告帳號的目錄時，一般 Agent binding 使用內建預設目錄，plan 會顯示警告。各帳號仍使用自己的 `config_dir`，其他 override 保留現有行為。包括 symlink 在內，解析為同一目錄的兩個 binding key 不能在一次同步中寫入。

如果 target 已移除、目錄移動或帳號目錄不存在，已管理的輸出及其所有權記錄會保留，並顯示警告。恢復 target 或目錄即可繼續管理。若要釋放某個項目暫存的所有權而不修改檔案，請先預覽再執行 `skillshare hooks sync NAME --replace -g`。移除 target 時若 binding 仍使用該名稱會顯示警告；下次 hooks 同步前也要移除這些 binding。

用 `skillshare hooks import --from codex-2 -g --json` 從帳號匯入。Dashboard 顯示 **codex-2 (Codex)**，並使用 Codex 的事件與編輯器。每個 Codex 目錄都需要在 `/hooks` 中個別審核並信任變更的 hooks；同步不會修改信任或執行 hooks。

## Git hooks {#git-hooks}

`bindings.git.commands` 宣告 Git 2.54+ 的具名 config hook。每個名稱包含
`events: [pre-commit]`、單行 `command` 與選用 `parallel`（Git 2.55+）。
名稱不能是 Git event 名，以字母或數字開頭，只含字母、數字、`_`、`-`、`.`，
最多 128 字元，不含 `..`，也不能以 `.` 結尾。同一目標的已啟用 entry 名稱重複會衝突。
未知 event 只警告。Dashboard 使用 YAML 編輯整個 Git binding；順序按 entry 名再按 hook 名排序。

Git 會把 event 參數附加到 command，並向每個 hook 提供完整 stdin。
複合 shell 邏輯放在 UTF-8 `files`，用 `command: "{files}/check.sh"` 引用。
`{files}` 展開為各機器上帶引號的絕對 helper 路徑；檔案以 LF 與可執行權限產生。
以 `fi`、`done`、`esac`、`}`、`)` 結尾的 command 會遭拒。

global 輸出為 `$XDG_CONFIG_HOME/git/skillshare/hooks.gitconfig`（預設 `~/.config/git/…`）。
include 寫入目標依序為絕對路徑 `GIT_CONFIG_GLOBAL`、既有 `~/.gitconfig`、
既有 XDG `git/config`、新建 `~/.gitconfig`。project 輸出為
`<git-common-dir>/skillshare/hooks.gitconfig`，由 common `config` 引入。
linked worktree 共用目標，每個 common directory 只宣告一個非 bare 儲存庫頂層 root。

只透過原生 lock 修改可寫的普通 config，不穿透 symlink 寫入。缺少 include 時顯示
**inactive** 與手動加入的確切行。舊 Git 同樣 inactive，不產生 fallback dispatcher。
缺少 Git 或 root 時略過輸出並保留所有權記錄。手動 include 與 includeIf 仍屬使用者所有，
不會擴大條件。conditional include 存在不代表已啟用。用 `hooks list -g --json` 的
`git`／`projectGit` 查看能力、include 與 `core.hooksPath`。

產生的檔案由 Skillshare 整體擁有。`--replace` 備份後重新產生整個檔案，覆蓋外部編輯。
同名外部 hook 只能在可寫普通目標 config 內取代；system／include 檔案衝突與其他 config
的有效所有權不能取代。`enabled=false` 只警告。preview 與 section backup 不含無關私密設定。
刪除或停用後的同步移除擁有的輸出與 include，保留手動 include 與無關設定。
restore 保留後來無關的編輯，不更改 source。Git binding 因共用檔案而拒絕 `--keep-files`。

config hook 與 hookdir script 可能都執行；遷移前手動檢查重複。Git import、hookdir 辨識、
script mode、結構化 editor 與 doctor 留待後續 phase。產生 Windows 格式路徑，但 Windows 執行未驗證。

全域與專案命令也應使用不同的名稱，Git 會合併兩個作用域。位於未啟用父條件下的巢狀 include 同樣保留條件。無法讀取或巢狀過深的 include 宣告會阻止同步。

plan 保留實際的 `add`、`update`、`remove` 或 `unchanged` 動作，並透過 `inactiveReason` 另行回報無法執行的原因。已同步的 inactive 輸出不算待寫入的變更。

## 專案、衝突與復原

global 設定的 `hooks.projects` 將絕對專案路徑對應到相同 Entry 格式的 `entries`，可在「專案 → Hooks」管理。已有 `.skillshare/config.yaml` 的專案須使用 project scope；單一專案同步只處理該專案。

同步保留無關設定與非 Skillshare 管理的 hooks。內容相同不代表擁有權。自有輸出若被外部修改，停用、移除與還原也會回報衝突。明確取代僅作用於選定 entry；預覽會列出完整動作與路徑。共用檔案的每一列 plan 會列出該 entry 新增（`+`）、更新（`~`）、移除（`−`）的 event，JSON plan 的 `events` 也有相同資訊。`update` 表示該 entry 在檔案中仍有註冊，`remove` 表示完全離開該檔案。編輯會保留檔案原本的格式（精簡或縮排）；Skillshare 新增的 `hooks` key 在最後一個 hook 移除時一併移除。Skillshare 建立的檔案若已沒有其他內容就會刪除，連同它建立且已清空的資料夾；預覽會把這顯示為刪除檔案。原本就存在的檔案與資料夾，以及你加入的任何內容都會保留。

備份還原原生輸出並保留之後新增的無關內容，不改寫來源定義。使用「設定 → 備份 → Hooks」或 `hooks restore` 先預覽再還原。

**已同步**只表示 Skillshare 已寫入設定。請依 Agent 的原生流程重啟／重新載入；信任、啟用 hooks 及程式碼相容性由 Agent 控制。管理操作不執行 hook command，也不自動改變原生信任。
