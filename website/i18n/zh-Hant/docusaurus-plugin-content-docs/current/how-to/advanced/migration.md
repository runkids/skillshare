---
sidebar_position: 5
---

# 遷移

從其他 Skill 管理方式遷移到 skillshare。

## 從手動管理遷移

如果你一直是在各個 AI CLI 之間手動複製 Skills：

### 步驟 1：初始化 skillshare

```bash
skillshare init
```

### 步驟 2：收集既有的 Skills

```bash
# 從各個 AI CLI 收集
skillshare collect claude
skillshare collect pi
skillshare collect codex

# 或一次從所有 CLI 收集
skillshare collect --all
```

### 步驟 3：處理重複項目

如果同一個 Skill 存在於多個地方，`collect` 會提出警告。請選擇要保留哪一個。

### 步驟 4：Sync

```bash
skillshare sync
```

現在所有 Targets 都會透過 symlink 連結到你單一的 Source。

---

## 從其他安裝工具遷移

如果你曾使用 `npx install-skill` 或類似工具：

### 步驟 1：初始化 skillshare

```bash
skillshare init
```

### 步驟 2：備份既有的 Skills

```bash
skillshare backup
```

### 步驟 3：收集或重新安裝

**選項 A：收集既有的**（保留目前版本）
```bash
skillshare collect --all
```

**選項 B：從 Source 重新安裝**（取得最新版本）
```bash
# 檢查 metadata
cat ~/.config/skillshare/skills/.metadata.json

# 重新安裝
skillshare install anthropics/skills/skills/pdf
```

### 步驟 4：Sync

```bash
skillshare sync
```

---

## 從 `npx skills` 遷移 {#from-npx-skills}

你可以繼續用 `npx skills` 管理原有 skills，也可以將選定的 skills 交給 Skillshare。兩者都支援多 agent 與全域／專案安裝；遷移的理由是需要 Skillshare 更廣的資源管理能力，而不是另一個工具只能安裝。見[比較頁](/docs/understand/philosophy/comparison)。

**目前沒有自動匯入 npx lock 或移交管理權的功能。** Collect 保留本地 skill 檔案，不會匯入上游追蹤資訊。從 repository 重新安裝會建立 Skillshare 的追蹤資訊，但取得的內容可能與既有安裝不同。

以下路徑與 lock 欄位依據 **skills v1.8.1**：[全域 lock 原始碼](https://github.com/vercel-labs/skills/blob/e781cb4d07433dfe1d19e924ed45cc61738c55ec/src/skill-lock.ts)、[專案 lock 原始碼](https://github.com/vercel-labs/skills/blob/e781cb4d07433dfe1d19e924ed45cc61738c55ec/src/local-lock.ts)及 [CLI 文件](https://github.com/vercel-labs/skills/blob/e781cb4d07433dfe1d19e924ed45cc61738c55ec/docs/commands.md)。

| 項目 | Global | Project |
|------|--------|---------|
| 共用 skill 位置 | `~/.agents/skills/<name>/` | `.agents/skills/<name>/` |
| Agent 目錄 | 通常是指向共用 skills 的連結；`--copy` 會建立獨立副本 | 同左 |
| Lock 檔案 | 有設定時使用 `$XDG_STATE_HOME/skills/.skill-lock.json`，否則為 `~/.agents/.skill-lock.json` | `skills-lock.json` |

`experimental_sync` 建立的 skills 可能是指向 npm 依賴的連結，而非實體目錄。收集前先檢查實際路徑。

### 步驟 1：決定範圍並保存原始設定

步驟 2–5 的指令明確使用**全域模式**（`-g`）。專案安裝請改看下方的專案說明。

初始化或同步前：

- 決定每個管理工具負責哪些 skills。遷移期間暫停更新、移除與自動同步工作。
- 另外建立原始 skill 目錄、實際內容與 symlink 配置的快照。也要包含可能被取代的既有 Skillshare source。
- 另存適用的 npx lock。npm 專案也要保存 `package.json` 與套件管理器的 lock；這些只記錄來源與依賴，不是本地修改的備份。

驗證交接完成前保留這份快照。不要刪除舊 lock，也不要把 `npx skills remove` 當作清理捷徑。

### 步驟 2：初始化但不複製，然後檢查備份

若尚未初始化 Skillshare，請在 **stdin 與 stdout 都連到終端的互動式環境**執行下列指令。不要透過管線或自動化執行：這些旗標不會停用初次同步，非互動初始化在選定 targets 後會自動同步。

```bash
skillshare init -g --no-copy --no-skill --no-git
```

選擇要管理的 targets，包含對應 `~/.agents/skills` 的 `universal`，並使用本流程採用的 merge mode。若提示是否立即同步，請拒絕。已有設定時，先檢查 source、[targets](/docs/reference/commands/target) 與模式，不要重設。

```bash
skillshare backup -g --dry-run
skillshare backup -g
skillshare backup -g --list
```

檢查輸出中的失敗，確認實際 skill 檔案已涵蓋。Target 備份會**略過 symlink**，不會備份連結指向的內容，也不包含 target 目錄外的 npx lock。它只是步驟 1 快照的補充，不能取代快照。

### 步驟 3：選擇本地副本或上游追蹤

**保留目前檔案：**從存放實體目錄的 target 預覽並收集：

```bash
skillshare collect universal -g --dry-run
skillshare collect universal -g
```

這會收集該 target 中所有符合條件的 skills，不是單一選定的 skill。若副本位於個別 agent 目錄，請換成該設定中的 target 名稱。只有打算收集所有設定 targets 時才用 `collect --all -g`。部分交接時，改為只將選定的實體 skill 目錄複製到 Skillshare source。

Collect 會略過 skill symlink，包括指向 `node_modules` 的連結。這些 skills 可以繼續由 npm 管理，或明確將解析後的實體 skill 目錄複製為本地快照；副本之後不會隨套件更新。除非強制覆寫，collect 也會略過 source 中已存在的名稱。檢查略過／失敗結果，並自行比較重複內容；預覽不是理解 lock 的衝突報告。不要用 `collect --force` 盲目決定保留哪份內容。

收集的 skills 沒有匯入 repository 追蹤資訊，因此 `skillshare update` 無法從原始來源更新它們。

**保留上游更新：**從來源安裝前，先檢查保存的每個 lock 項目。`sourceType` 區分 Git、npm、本地與其他來源；`sourceUrl`、`ref`、`skillPath` 可能提供額外資訊。不是每個項目都有 repository 或 `skillPath`。

例如 GitHub 項目 `source: "anthropics/skills"`、`skillPath: "skills/pdf/SKILL.md"` 對應的是包含該檔案的 skill 目錄：

```bash
skillshare install anthropics/skills/skills/pdf -g --dry-run
skillshare install anthropics/skills/skills/pdf -g
```

若 lock 記錄了 Git ref，請在上述兩個指令都加上 `--branch <recorded-ref>`（見 [install](/docs/reference/commands/install)）；沒有指定時，範例會使用遠端預設分支。這是重新安裝，不是重播 npx 的內容 hash。請與保存的檔案比較並整合本地修改。若已收集的 skill 佔用目的位置，先停止並處理；不要直接強制安裝，因為可能覆寫修改並覆蓋 audit 阻擋。npm 與本地項目需要各自決定來源，不能直接轉成 `owner/repo`。

### 步驟 4：預覽並明確移交管理權

```bash
skillshare sync -g --dry-run
```

檢查**所有設定的 targets**，不只是 `universal`。Merge mode 會保留同名本地目錄，但即使沒有 force，同名的 agent symlink 也可能被改為指向 Skillshare source。確認只影響打算交接的 skills 後才套用：

```bash
skillshare sync -g
```

若也想把保留的實體目錄換成連結，先確認 source 中是要保留的版本，且原始快照完整：

```bash
skillshare sync -g --force --dry-run
# Apply only after reviewing every replacement:
skillshare sync -g --force
```

Force 會作用於設定的 targets，不是單一 skill 的遷移開關。若預覽包含仍由 npx skills 管理的資源，請停止。兩個工具的管理範圍應分開；見[並存說明](/docs/troubleshooting/faq#using-universal-alongside-npx-skills)。

### 步驟 5：驗證並保留單一管理者

檢查結果檔案／連結，並確認各 AI 工具能載入預期的 skills。執行 `skillshare status -g`，檢查保留的本地 skills 或警告；sync 成功不代表每個原始 skill 都已移轉。

Skillshare 不會移除舊 npx lock 項目。交接後不要對已遷移的 skills 使用 `npx skills update`、`remove`、`experimental_install` 或 `experimental_sync`。部分遷移時，避免未指定範圍而包含它們的批次命令。

若要回復，先暫停兩個管理工具，再一起還原保存的原始檔案、連結配置與 lock。Skillshare target 備份可協助復原本地檔案，但無法單獨重建原始 npx symlink 或 lock。

### 專案安裝與 npm 同步

變更前保存專案檔案與 lock，包含尚未提交或被忽略的本地修改。以 `skillshare init -p` 初始化並設定專案 targets，再對 `collect`、`install`、`sync` 使用 `-p`，而非上述全域指令。相同的來源追蹤與 symlink 限制仍適用。Skillshare 不會在專案模式建立 skill-target 備份；請使用已驗證的專案快照，或確實包含相關檔案的版本控制。

若 `package.json` scripts 或 CI 會呼叫 `experimental_sync`，交接重疊的 skills 前先調整工作與 skill 宣告，否則下次安裝依賴可能重新連結它們。繼續讓 npx skills 管理 npm skills 完全合理；不要把連結複製到 `.skillshare/skills/` 就以為已改為獨立管理。

---

## 從 Git Submodules 遷移

如果你一直使用 git submodules：

### 步驟 1：匯出 submodule 內容

```bash
# 在你既有的 Skills repo 中
git submodule foreach 'cp -r $toplevel/$sm_path ~/temp-skills/$name'
```

### 步驟 2：初始化 skillshare

```bash
skillshare init
```

### 步驟 3：匯入 Skills

```bash
# 複製到 Source
cp -r ~/temp-skills/* ~/.config/skillshare/skills/

# 或以 tracked repo 的方式安裝
skillshare install github.com/org/skill-repo --track
```

### 步驟 4：Sync

```bash
skillshare sync
```

---

## 從已 Commit 的 Project Skills 遷移

如果你的 repo 已經在 `.claude/skills/`、`.cursor/skills/` 或類似目錄中 commit 了 Skills：

### 步驟 1：初始化 Project mode

```bash
cd my-project
skillshare init -p
```

### 步驟 2：將 Skills 搬到 `.skillshare/skills/`

```bash
# 將既有的 Skills 複製到 skillshare Source
cp -r .claude/skills/my-skill .skillshare/skills/
cp -r .claude/skills/api-guide .skillshare/skills/

# 移除原始檔案（sync 會重新建立為 symlink）
rm -rf .claude/skills/my-skill .claude/skills/api-guide
```

### 步驟 3：Sync

```bash
skillshare sync
```

現在 `.claude/skills/my-skill` 是指向 `.skillshare/skills/my-skill` 的 symlink — 而所有其他 Targets（Pi、Windsurf 等）也會自動取得相同的 Skills。

### 步驟 4：Commit 這次遷移

```bash
git add .skillshare/ .claude/skills/ .cursor/skills/
git commit -m "Migrate project skills to skillshare"
```

:::tip 多工具的好處
遷移前：Skills 只能在一個 AI CLI 中使用。遷移後：相同的 Skills 會自動出現在每一個已設定的 Target 中。
:::

---

## 從團隊專屬方案遷移

如果你的團隊有自訂的 Skill 分享方式：

### 步驟 1：確認目前的做法

- Skills 儲存在哪裡？
- 如何分享？
- 如何更新？

### 步驟 2：選擇你的遷移路徑

**選項 A：Global mode** — Skills 在每台機器上的所有專案間都可使用。

```bash
# 建立團隊 Skills repo
cp -r /current/team/skills ~/new-team-skills
cd ~/new-team-skills && git init && git add . && git commit -m "Migrate to skillshare"
git push origin main

# 團隊成員以全域方式安裝
skillshare install github.com/org/team-skills --track && skillshare sync
```

**選項 B：Project mode** — Skills 限定在特定 repo 範圍內，透過 git 分享。

```bash
cd my-project
skillshare init -p

# 將團隊 Skills 搬進專案 Source
cp -r /current/team/skills/* .skillshare/skills/

# Sync 並 commit
skillshare sync
git add .skillshare/
git commit -m "Add team skills via skillshare"
```

新加入的團隊成員只需執行：
```bash
git clone github.com/org/my-project
cd my-project
skillshare install -p && skillshare sync
```

**選項 C：兩者並用** — 全組織的標準採用 Global，各專案特有的 Skills 則使用 Project。

```bash
# 組織標準（Global）
skillshare install github.com/org/standards --track && skillshare sync

# 專案特定的 Skills（Project mode）
cd my-project
skillshare init -p
skillshare install github.com/org/project-skills -p && skillshare sync
```

:::tip 該選哪一種？
- **Global**：程式碼規範、安全稽核 — 每個專案都需要的東西
- **Project**：API 慣例、領域規則、部署指南 — 特定於單一 repo 的東西
- **兩者並用**：大多數團隊隨著規模成長，最終都會走到這一步
:::

---

## 從 Global 遷移到 Project

如果你在 Global mode 中有屬於特定專案的 Skills：

### 步驟 1：初始化 Project mode

```bash
cd my-project
skillshare init -p
```

### 步驟 2：從 Global Source 複製 Skills

```bash
# 複製特定的 Skills
cp -r ~/.config/skillshare/skills/api-guide .skillshare/skills/
cp -r ~/.config/skillshare/skills/deploy-rules .skillshare/skills/
```

### 步驟 3：從 Global 中移除（可選）

```bash
skillshare uninstall api-guide
skillshare uninstall deploy-rules
skillshare sync   # 清理 Global symlink
```

### 步驟 4：Sync 並 commit

```bash
skillshare sync   # 自動偵測為 Project mode
git add .skillshare/
git commit -m "Move project-specific skills to project mode"
```

完成後，這些 Skills 會被限定在這個 repo 範圍內，並透過 git 與團隊分享 — 不再佔用你的 Global 設定空間。

---

## 保留歷史紀錄

如果你想保留 git 歷史紀錄：

### 個人 Skills

```bash
# 將既有的 repo clone 到 skillshare 的位置
git clone your-existing-repo ~/.config/skillshare/skills

# 以既有的 Source 初始化 skillshare
skillshare init --source ~/.config/skillshare/skills
```

### 團隊 repos

```bash
# 使用 --track 保留 .git
skillshare install github.com/team/skills --track
```

---

## 回復（Rollback）

如果遷移出了問題：

### 從備份還原

```bash
skillshare restore claude
skillshare restore cursor
```

### 重新開始

```bash
rm ~/.config/skillshare/config.yaml
skillshare init
```

---

## 檢查清單

遷移之前：

- [ ] 列出目前所有 Skill 的位置
- [ ] 找出重複項目
- [ ] 記下任何自訂設定
- [ ] 建立備份

遷移之後：

- [ ] 確認所有 Skills 都出現在 `skillshare list` 中
- [ ] 在每個 AI CLI 中測試 Skills
- [ ] 設定 git remote（若有需要）
- [ ] 向團隊分享新的 Workflow

---

## 另請參閱

- [From Existing Skills](/docs/getting-started/from-existing-skills) — 快速遷移路徑
- [collect](/docs/reference/commands/collect) — 從 Targets 收集
- [Comparison](/docs/understand/philosophy/comparison) — 比較各種方式
