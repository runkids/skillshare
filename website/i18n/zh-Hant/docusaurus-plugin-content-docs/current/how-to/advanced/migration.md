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

## 從 `npx skills` 遷移

如果你是用 [npx skills CLI](https://github.com/vercel-labs/skills)（`npx skills add ...`）安裝 Skills，skillshare 可以接管相同的目錄。你也可以[讓兩個工具並存](/docs/troubleshooting/faq#using-universal-alongside-npx-skills)，只把想交給 skillshare 管理的 Skills 搬過來。

`npx skills` 存放各項內容的位置：

| 項目 | Global | Project |
|------|--------|---------|
| Skill 檔案 | `~/.agents/skills/<name>/`（實體目錄） | `.agents/skills/<name>/` |
| Agent 目錄 | 指向上述檔案的 symlink（例如 `~/.claude/skills/<name>`） | 同左 |
| Lock file | `~/.agents/.skill-lock.json`（或 `$XDG_STATE_HOME/skills/.skill-lock.json`） | `skills-lock.json` |

以 `--copy` 安裝的 Skills 在每個 Agent 目錄中都是實體目錄，而不是 symlink。

### 步驟 1：初始化 skillshare

```bash
skillshare init
```

### 步驟 2：備份既有的 Skills

```bash
skillshare backup
```

### 步驟 3：收集 Skills

`collect` 會複製實體目錄並略過 symlink，因此 Agent 目錄中的 symlink 不會被重複收集。

```bash
skillshare collect universal --dry-run   # 預覽
skillshare collect universal             # ~/.agents/skills
```

如果你用過 `--copy`，或其他 Agent 目錄中也有 Skills，請改為執行 `skillshare collect --all`，並依照[從手動管理遷移](#從手動管理遷移)中的說明處理重複項目。

收集來的 Skills 是單純的本機副本。它們不會記得來自哪個 repository，因此 `skillshare update` 無法更新它們。

### 步驟 4：重新安裝想持續更新的 Skills（可選）

Lock file 記錄了每個 Skill 的來源。`source` 是 repository，`skillPath` 是它在 repository 中的位置。

```bash
cat ~/.agents/.skill-lock.json

# "source": "anthropics/skills", "skillPath": "skills/pdf/SKILL.md"
skillshare install anthropics/skills/skills/pdf
```

如果該 Skill 已在步驟 3 中收集到你的 Source，請加上 `--force`。

### 步驟 5：Sync

```bash
skillshare sync
```

在 merge mode 下，sync 會保留 Target（`~/.agents/skills`）中同名的實體目錄，並將其回報為保留的本機 Skill。其他 Agent 目錄（例如 `~/.claude/skills`）中由 `npx skills` 建立的 symlink，會改為指向你的 skillshare Source。若也要把這些實體目錄換成 symlink，請在步驟 3 收集完成後執行：

```bash
skillshare sync --force
```

`sync --force` 會在取代任何內容之前先備份 Target。之後請只用 skillshare 管理這些 Skills。`npx skills` 會在自己的 lock file 中追蹤安裝紀錄，因此請避免對它們執行 `npx skills update` 或 `npx skills remove`。

若是使用 `npx skills` 的專案，請依照[從已 Commit 的 Project Skills 遷移](#從已-commit-的-project-skills-遷移)操作，並以 `.agents/skills/` 作為要搬移的目錄。

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
