---
sidebar_position: 9
---

# 比較 Skill 管理方式

**Vercel 的 `npx skills`** 和 **Skillshare** 都能跨多個 AI 工具管理 skills。差別不是「一個只能安裝，另一個才能同步」，而是你需要管理 skills，還是更完整的 AI 工作環境。

本頁依據已發布的 **skills v1.8.1** [套件資訊](https://registry.npmjs.org/skills/1.8.1)及其[官方 CLI 文件](https://github.com/vercel-labs/skills/blob/e781cb4d07433dfe1d19e924ed45cc61738c55ec/docs/commands.md)。實驗性命令可能變更或移除；這是指定版本的比較，不是對所有安裝工具的概括。

## 架構概覽

### npx skills：發現與管理 skills

[skills.sh](https://skills.sh) 提供發現入口。CLI 支援多 agent、專案與全域安裝、指向共用副本的 symlink 或獨立複本，以及更新檢查與更新。Agent 選擇及確認可透過旗標提供，不一定需要互動提示。

它也會以 lock 檔案記錄已安裝的 skills。實驗性命令支援專案還原與 npm 同步，因此不能將它描述為沒有持久狀態或只能安裝。

### Skillshare：跨工具管理工作環境

Skillshare 將來源資源與 target 設定分開，再同步到各工具。除了 skills，也管理 agent 定義、rules 等 extras、MCP 連線設定，以及支援用戶端的原生 hooks。本地 Web dashboard 提供視覺化管理介面。

真正的差別是將這些資源一起納入跨工具、跨機器與團隊工作流程，而不是獨有的 symlink 或多 agent 安裝能力。資源格式與用戶端能力仍有差異，並非每種資源都適用於每個工具。

## 功能比較

| 能力 | npx skills v1.8.1 | Skillshare |
|------|------------------|------------|
| **發現** | skills.sh 與 `skills find` | CLI 搜尋與本地 Web dashboard |
| **多 agent 與範圍** | 多 agent；專案與全域安裝 | 設定中的 targets；專案與全域模式 |
| **共用 skill 檔案** | 指向共用副本的 symlink；可選 copy | 來源目錄搭配 merge、copy 或整個目錄的 symlink 模式 |
| **更新** | `skills check`（有更新或本地修改時以結束代碼 1 結束）與 `skills update` | `skillshare check` 與 `skillshare update` |
| **專案還原** | 實驗性 `skills experimental_install`（或不帶來源的 `skills install`）從 `skills-lock.json` 還原支援的來源 | 宣告式 skill 設定與來源資源；見 [manifest](/docs/understand/declarative-manifest) |
| **npm 專案同步** | 實驗性 `skills experimental_sync` 讀取已安裝依賴與 `package.json` 的 skill 宣告 | 來源到 target 的同步；不是相同的 npm 依賴工作流程 |
| **跨機器工作流程** | 專案 lock 還原與 npm 同步可協助重建專案 skills | Git `push` / `pull`，範圍由 `git_root` 決定 |
| **Skills 以外** | 所引用的 CLI 文件以 skills 為主 | Agent 定義、rules 等 extras、MCP 設定與支援用戶端的 hooks |
| **本地管理與復原** | 所引用的 CLI 文件未列出本地 dashboard 或 target 備份命令 | `skillshare ui`、target 備份與還原，以及受管理檔案復原 |
| **執行環境** | Node.js 與 npm/npx | Go 執行檔；Git 工作流程需要 Git |

### 實驗性命令涵蓋什麼

- **`experimental_install`**（也可用不帶來源的 `skills install` 執行）將支援的 lock 項目還原到 `.agents/skills/`，並連結到 agent 目錄（以 copy 模式記錄的 skills 則為複製）：優先使用 `--agent` 指定的 agent，其次是上次選擇的 agent，再其次是偵測到的 agent。它不會安裝 npm 依賴。已安裝且檔案與 lock 內容 hash 一致的 skills 不會重新取得，只補上缺少的 agent 連結；內容不一致的 skills 會從來源還原，`--force` 則重新安裝所有 skills。Frozen-lock 模式會檢查內容 hash，且為 CI 預設。
- **`experimental_sync`** 連結已安裝 npm 依賴附帶的 skills，並解析 `package.json` 的 skill 宣告。它以 `skills-lock.json` 追蹤這些項目、移除不再提供的 skills，並保留另外安裝的 skills。使用 copy 時，依賴更新後需要再次同步。

這些確實是同步與還原能力，但不是整套 AI 工具設定的備份。

## 選擇合適的工具

**繼續使用 npx skills**：如果主要需求是發現、安裝與更新公開或私有 skills，即使同時使用多個 agent 也很合理。若希望 skills 跟隨 npm 專案依賴，且能接受此工作流程仍屬實驗性，也適合考慮。

**選擇 Skillshare**：當問題不只是安裝 skills，例如：「我同時用 Claude Code 和 Codex，希望把受管理的 skills、rules、MCP 設定與支援的 hooks 帶到另一台電腦。」它的來源／target 設定、Git 工作流程、本地 dashboard 與復原工具服務於這個較廣的管理需求。

可攜性仍需要設定：選擇 Git 範圍、在新機器設定 targets，並另外準備憑證與外部依賴。`push` 預設只處理 skills 來源，不會自動發布全部設定或備份整台機器。見 [Git root 範圍](/docs/reference/commands/push)與 [sync](/docs/reference/commands/sync)。

## 遷移至 Skillshare

不必立刻取代運作良好的安裝工具。只移交希望由 Skillshare 管理的資源，並避免兩個工具同時更新或移除同一個受管理的 skill。

目前遷移並非無損的 lock 匯入：收集的 skills 是本地副本，不保留原始 repository 的追蹤資訊。若需要後續透過 Skillshare 更新，請從記錄的來源重新安裝。移交管理權前先備份與預覽；請遵循 [npx skills 遷移指南](/docs/how-to/advanced/migration)，不要將 collect 本身視為完整遷移。

## 延伸閱讀

- [遷移](/docs/how-to/advanced/migration)
- [核心概念](/docs/understand)
- [備份](/docs/reference/commands/backup)
