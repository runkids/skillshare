---
sidebar_position: 6
---

# 最佳實務

Skills 的命名慣例、組織方式與版本控制。

## 命名

### Skill 名稱

**建議做法：**
- 使用小寫加連字號：`code-review`、`pdf-tools`
- 具描述性：使用 `react-component-generator` 而非 `rcg`
- 為團隊加上命名空間：`acme-code-review`

**避免：**
- 使用空格或特殊字元
- 使用過於通用的名稱：`helper`、`utils`、`tools`
- 與常見的 Skill 名稱衝突

### Repository 名稱

**個人使用：**
```
my-skills
ai-skills
```

**團隊使用：**
```
<team>-skills
<org>-skills
```

---

## 組織方式

### 個人 Skills

```
~/.config/skillshare/skills/
├── code-review/
├── pdf-tools/
├── git-workflow/
└── _team-skills/      # Tracked repo
```

### 團隊 repos

```
team-skills/
├── frontend/
│   ├── react/
│   ├── vue/
│   └── testing/
├── backend/
│   ├── api/
│   └── database/
├── devops/
│   ├── deploy/
│   └── monitoring/
└── README.md
```

### Skill 目錄

```
my-skill/
├── SKILL.md           # 必要
├── README.md          # 選用：給人類看的說明
├── examples/          # 選用：範例檔案
└── templates/         # 選用：程式碼範本
```

---

## 版本控制

### Commit 訊息

遵循 conventional commits：
```
feat(code-review): add security check
fix(pdf-tools): handle empty files
docs(readme): update installation
```

### 分支策略

**個人使用：**
- 單一 `main` 分支即可
- 實驗性內容使用其他分支

**團隊使用：**
- `main` 用於穩定的 Skills
- 開發時使用 feature 分支
- 合併前先進行 PR 審查

### Tags

為穩定版本加上 tag：
```bash
git tag v1.0.0
git push --tags
```

---

## 撰寫 Skill

### 結構

```markdown
---
name: skill-name
description: One-line description
---

# Skill Name

Brief overview.

## When to Use

Clear trigger conditions.

## Instructions

1. Step one
2. Step two

## Examples

Concrete input/output examples.

## When NOT to Use

Explicit exclusions.
```

### License

為發布的 Skills 加上 `license` 欄位 — 這在企業環境中特別重要：

```yaml
---
name: code-review
description: Reviews code for quality
license: MIT
---
```

這會在 `skillshare install` 時顯示，讓使用者能做出符合合規性的判斷。

### 內容

**建議做法：**
- 撰寫清楚、可執行的指示
- 包含範例
- 說明邊界情況
- 保持專注（一個 Skill = 一個目的）

**避免：**
- 撰寫模糊的指示
- 包含過多職責
- 忘記錯誤處理
- 略過測試

---

## 團隊協作

### 對特定 repo 的 Skills 使用 Project mode（`-p`）

當 Skills 與單一程式碼庫緊密耦合（架構、領域規則、部署流程）時，建議使用 Project mode：

```bash
skillshare init -p
skillshare install <source> -p
skillshare sync
```

**這樣做的好處：**
- **可重現的上手流程**：`.skillshare/config.yaml` 對任何 clone 這個 repo 的人而言，都扮演可攜式 Skill 清單的角色。
- **範圍清楚**：Project Skills 留在 `.skillshare/skills/` 中，不會滲透到 Global 的個人 Workflow。
- **更安全的協作**：變更會與專案程式碼一起，經過一般的 git PR 流程審查。
- **降低 commit 雜訊**：在 Project mode 中，`.skillshare/logs/` 預設會被忽略。

跨專案的個人 Skills 請使用 Global mode；特定 repo 的團隊情境則使用 `-p`。

### 對內部工具使用 .skillignore

如果你的團隊 repo 包含內部工具或開發中的 Skills，請加入 `.skillignore` 以避免被意外探索到：

```text title=".skillignore"
# 隱藏，不讓外部探索到
_internal-scripts
test-*
wip-feature
```

這能確保外部貢獻者或執行 `skillshare install <repo> --all` 的自動化流程，不會抓取到內部 Skills。

**使用 `.skillignore.local` 進行本機覆寫**：如果共用 repo 的 `.skillignore` 封鎖了你本機需要的某個 Skill，可以在同一目錄下建立 `.skillignore.local`，在不修改共用檔案的情況下覆寫它：

```text title="_team-skills/.skillignore.local"
# 取消忽略我自己的私有 Skill
!private-mine
```

請將 `.skillignore.local` 加入你的 `.gitignore` — 它本來就是設計為僅限本機使用。

### 擁有權

- 為各個 Skill 分類指派負責人
- 在 README 中記載誰負責維護什麼
- 合併前先審查 PR

### 文件

```
team-skills/
├── README.md           # 設定說明
├── CONTRIBUTING.md     # 如何新增 Skills
├── CHANGELOG.md        # 變更內容
└── skills/
    └── ...
```

### 溝通

- 在團隊聊天群組中公告新的 Skills
- 記錄破壞性變更
- 蒐集使用者的意見回饋

---

## 維護

### 例行任務

```bash
# 每週
skillshare update --all     # 更新 tracked repos
skillshare doctor           # 檢查是否有問題
skillshare backup --cleanup # 移除舊快照和遺留的不完整目錄

# 每月
skillshare list             # 檢視已安裝的 Skills
# 移除不再使用的：skillshare uninstall <name>...
```

### 清理未使用的 Skills

```bash
# 列出所有 Skills
skillshare list

# 移除你不再使用的
skillshare uninstall unused-skill
skillshare sync
```

### 更新依賴項目

```bash
# 更新 CLI
skillshare upgrade --cli

# 更新內建 Skill
skillshare upgrade --skill

# 更新 tracked repos
skillshare update --all
```

---

## 安全性

### 敏感資訊

**絕對不要放進 Skills 中：**
- API keys
- 密碼
- 個人資訊
- 內部 URL

**應改為：**
- 使用環境變數
- 參照外部設定
- 讓 Skills 保持通用性

### 安裝前先審查

安裝第三方 Skills 之前：
- 檢查來源
- 閱讀 SKILL.md
- 先使用 `--dry-run`

完整的安全性 Workflow 請參閱 [Securing Your Skills](/docs/how-to/advanced/security) 指南。

---

## 檢查清單

### 新的 Skill

- [ ] 具描述性的名稱
- [ ] 清楚的說明
- [ ] 可執行的指示
- [ ] 包含範例
- [ ] 已在 AI CLI 中測試過
- [ ] 沒有名稱衝突

### 團隊 repo

- [ ] 清楚的資料夾結構
- [ ] README 含設定說明
- [ ] 具命名空間的 Skill 名稱
- [ ] 為內部工具設定 `.skillignore`
- [ ] PR 審查流程
- [ ] 維護 CHANGELOG

---

## 另請參閱

- [建立 Skills](./creating-skills.md) — Skill 建立指南
- [Skill Design](/docs/understand/philosophy/skill-design) — 複雜度層級、確定性、CLI wrapper pattern
- [Skill Format](/docs/understand/skill-format) — SKILL.md 參考
- [Organization-Wide Skills](/docs/how-to/sharing/organization-sharing) — 團隊分享模式
