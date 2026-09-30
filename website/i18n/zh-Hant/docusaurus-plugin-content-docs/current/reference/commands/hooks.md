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
skillshare hooks restore BACKUP_ID --dry-run
skillshare hooks restore BACKUP_ID
```

沒有子指令時列出 entries。`add`／`edit` 從 `--file` 讀取 Entry JSON 或 YAML；匯入不指定名稱時列出候選項目（每個 Agent event 或檔案各一項），指定名稱才儲存。儲存匯入即原地接管讀到的註冊：下次同步直接 adopt，不需 `--replace`；沒匯入的 event 維持不受管理。匯入只讀取設定或程式碼，不會執行。新增、編輯、匯入、啟用、停用與移除預設只儲存來源，加入 `--sync` 才同步。同步與還原會重新預覽。

| Option | Meaning |
|---|---|
| `--file PATH` | Entry JSON/YAML；匯入時為原生設定或程式碼 |
| `--from AGENT` | 原生 Agent 格式或要讀取的 Agent |
| `--sync` | 儲存並同步 |
| `--replace` | 明確取代既有來源 entry 或該 entry 衝突的原生輸出 |
| `--dry-run, -n` | 只預覽，不儲存或寫入 |
| `--json` | 結構化輸出 |
| `--revision ID` | 要求符合指定預覽 revision |
| `--global, -g` | 使用 global 設定 |
| `--project, -p` | 使用 project 設定 |

`hooks list --json` 顯示備份 ID 與完整目的地路徑。設定變動後，舊預覽失效；儲存或同步前須重新預覽。

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

| Option | Meaning |
|---|---|
| `description` | 可選描述 |
| `enabled` | 預設 true；false 保留來源，下次同步移除未被修改的自有輸出 |
| `bindings` | Agent ID 到原生 binding 的對應 |
| `bindings.AGENT.events` | command／設定型 Agent 的原生 event map |
| `bindings.AGENT.code` | Pi、Amp、OpenCode 的原生 extension/plugin 程式碼 |
| `bindings.AGENT.files` | 可選 UTF-8 腳本檔，以相對檔名為 key |

Agent ID 為 `claude`、`codex`、`gemini`、`copilot`、`cursor`、`droid`、`qwen`、`antigravity`、`pi`、`amp`、`opencode`；`factory` 是 `droid` 的別名，`antigravity-cli` 與 `agy` 是 `antigravity` 的別名。event、matcher、handler type、command、timeout 單位與 payload 均保留原生格式，不自動跨 Agent 轉換。event 名稱會對照各 command Agent 文件列出的事件檢查：未知名稱（例如拼錯的 `Stopp`）在預覽與 plan 的 `warnings` 中顯示警告，但不阻擋同步，因為 Agent 會陸續新增事件。Pi、Amp、OpenCode 的程式碼不檢查。Pi、Amp、OpenCode 的程式碼與 imports 須符合已安裝版本；發布至獨立的 `skillshare-NAME.ts`，不產生共用執行引擎。command binding 的腳本位於 Agent 設定目錄的 `hooks/skillshare/NAME/`，command 保留你提供的原生 macro 或明確路徑。請在預覽確認完整路徑。

## 原生目的地

| Agent | Global | Project | Format |
|---|---|---|---|
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

global scope 使用原生設定目錄的環境變數 override；project scope 只寫入專案，不退回 global 路徑。Codex inline TOML 等其他來源仍獨立存在。Antigravity 與其 CLI（`agy`）讀取同一份 `hooks.json`；每個 hook 是一個以其名稱命名的 block，匯入時保留原名。CLI 的 `~/.gemini/antigravity-cli/settings.json` 中的 hooks 保持獨立。Droid 發布獨立 hooks 檔會影響原生載入來源，請先檢查既有 inline hooks。


Droid 有有效 inline hooks 時，同步會拒絕建立獨立檔案。先匯入並檢查，移除原 inline hooks 後再同步。

## 專案、衝突與復原

global 設定的 `hooks.projects` 將絕對專案路徑對應到相同 Entry 格式的 `entries`，可在「專案 → Hooks」管理。已有 `.skillshare/config.yaml` 的專案須使用 project scope；單一專案同步只處理該專案。

同步保留無關設定與非 Skillshare 管理的 hooks。內容相同不代表擁有權。自有輸出若被外部修改，停用、移除與還原也會回報衝突。明確取代僅作用於選定 entry；預覽會列出完整動作與路徑。共用檔案的每一列 plan 會列出該 entry 新增（`+`）、更新（`~`）、移除（`−`）的 event，JSON plan 的 `events` 也有相同資訊。`update` 表示該 entry 在檔案中仍有註冊，`remove` 表示完全離開該檔案。編輯會保留檔案原本的格式（精簡或縮排）；Skillshare 新增的 `hooks` key 在最後一個 hook 移除時一併移除。

備份還原原生輸出並保留之後新增的無關內容，不改寫來源定義。使用「設定 → 備份 → Hooks」或 `hooks restore` 先預覽再還原。

**已同步**只表示 Skillshare 已寫入設定。請依 Agent 的原生流程重啟／重新載入；信任、啟用 hooks 及程式碼相容性由 Agent 控制。管理操作不執行 hook command，也不自動改變原生信任。
