---
sidebar_position: 5.5
---

# 用資料夾組織 Skills

隨著你的 Skill 收藏不斷增加，將它們組織到資料夾中能讓一切保持易於管理 — 而 skillshare 會自動處理其餘的部分。

## 為什麼要組織？

一份超過 20 個 Skills 的平面清單會變得難以瀏覽：

```
~/.config/skillshare/skills/
├── accessibility/
├── ascii-box-check/
├── core-web-vitals/
├── frontend-design/
├── performance/
├── react-best-practices/
├── remotion/
├── seo/
├── skill-creator/
├── ui-skills/
├── vue-best-practices/
├── vue-debug-guides/
├── web-artifacts-builder/
└── ... 20+ more
```

使用資料夾後，你會得到邏輯上的分組，而 skillshare 會在提供給 AI CLI 時自動攤平：

```
SOURCE (organized)                     TARGET (auto-flattened)
───────────────────────────────────    ──────────────────────────────────
~/.config/skillshare/skills/           ~/.claude/skills/
├── frontend/                          ├── frontend__frontend-design
│   ├── frontend-design/               ├── frontend__react__react-best-..
│   ├── react/                         ├── frontend__ui-skills
│   │   └── react-best-practices/      ├── frontend__vue__vue-best-prac..
│   ├── ui-skills/                     ├── frontend__vue__vue-debug-gui..
│   └── vue/                           ├── utils__ascii-box-check
│       ├── vue-best-practices/        ├── utils__remotion
│       ├── vue-debug-guides/          ├── utils__skill-creator
│       └── ...                        ├── web-dev__accessibility
├── utils/                             ├── web-dev__core-web-vitals
│   ├── ascii-box-check/               └── ...
│   ├── remotion/
│   └── skill-creator/
└── web-dev/
    ├── accessibility/
    ├── core-web-vitals/
    └── ...
```

![Source vs Target comparison](/img/organizing-skills-comparison.png)

:::tip 實際範例
完整使用這種模式組織的 Skill 收藏，請參閱 [runkids/my-skills](https://github.com/runkids/my-skills)。
:::

---

## 自動攤平如何運作

skillshare 會使用 `__`（雙底線）作為分隔符，將資料夾路徑轉換為攤平後的名稱：

| Source 路徑 | 同步後的 Target 名稱 |
|---|---|
| `frontend/react/react-best-practices/` | `frontend__react__react-best-practices` |
| `utils/remotion/` | `utils__remotion` |
| `web-dev/accessibility/` | `web-dev__accessibility` |

**重點：**
- 只有包含 `SKILL.md` 的目錄才會被視為 Skill
- 中介資料夾（例如 `frontend/` 本身）只是用來組織用途 — 它們不需要 `SKILL.md`
- `list` 與 `sync` 會探索任意深度的巢狀 Skills
- `check` 與 `update` 也能處理巢狀 Skills

:::note Agents 不支援巢狀結構
本頁討論的是如何組織 **Skills**。Agents 一律是直接放在 `~/.config/skillshare/agents/`（Project mode 中則是 `.skillshare/agents/`）底下的單一 `.md` 檔案 — 它們不支援資料夾巢狀化或自動攤平。若要組織 Agents，請使用命名慣例（例如 `frontend-reviewer.md`、`backend-auditor.md`）以及 `.agentignore` pattern。
:::

---

## 使用巢狀 Skills

### list

同一目錄下的 Skills 會自動被分組顯示：

```bash
$ skillshare list -g

  frontend/vue/
    → vue-best-practices     github.com/vuejs-ai/skills/...

  utils/
    → remotion               github.com/remotion-dev/skills/...

  web-dev/
    → accessibility          github.com/addyosmani/web-quality-...
```

在每個分組中，Skills 會顯示其基礎名稱（而非完整的攤平名稱）。頂層 Skills 會不分組地顯示在最下方。如果所有 Skills 都是頂層的，輸出結果就會是平面清單 — 與舊格式相同。

### check

會偵測巢狀 Skills 並顯示相對路徑：

```bash
$ skillshare check -g
! utils/remotion  update available · github.com/remotion-dev/skills

! Updates available for 1 skill, 14 up to date · 2.1s

Next
  skillshare update --all  pull the updates
```

### update

同時支援**完整路徑**與**短名稱**：

```bash
# 完整相對路徑
skillshare update -g frontend/react/react-best-practices

# 短名稱（basename）— 會自動解析
skillshare update -g react-best-practices

# 更新所有項目
skillshare update -g --all
```

當短名稱符合多個 Skills 時，skillshare 會要求你提供更明確的名稱：

```
'my-skill' matches multiple items:
  - frontend/my-skill
  - backend/my-skill
Please specify the full path
```

### enable / disable

使用資料夾可以輕鬆地一次切換整個類別的開關。`disable`/`enable` 接受 glob pattern，因此可以直接指向該資料夾：

```bash
# 停用 frontend/ 底下所有的 Skills（任意深度）
skillshare disable "frontend/**"

# 用相同的 pattern 重新啟用整個資料夾
skillshare enable "frontend/**"

# 套用到 Targets
skillshare sync
```

這會在 `.skillignore` 中寫入一行 `frontend/**`，並持續涵蓋之後加入該資料夾的任何內容。若要改為切換個別 Skills，請直接傳入它們的名稱（`skillshare disable frontend/react/react-best-practices`）。

:::tip 為 pattern 加上引號
請將資料夾 pattern 用引號包起來（`"frontend/**"`），以避免你的 shell 先展開了 `*`。
:::

詳情請參閱 [enable / disable](/docs/reference/commands/enable) 與 [.skillignore 語法](/docs/reference/filtering#skillignore)。

---

## 直接安裝到資料夾中 {#install-directly-into-folders}

使用 `--into` 可以一步將 Skill 安裝到子目錄中 — 不需要手動 `mv`：

```bash
# 安裝到分類資料夾中
skillshare install anthropics/skills -s pdf --into frontend
# → ~/.config/skillshare/skills/frontend/pdf/

# 多層巢狀
skillshare install ~/my-skill --into frontend/react
# → ~/.config/skillshare/skills/frontend/react/my-skill/

# 也適用於 --track
skillshare install github.com/team/skills --track --into devops
# → ~/.config/skillshare/skills/devops/_team-skills/

# 也適用於 Project mode
skillshare install anthropics/skills -s pdf --into tools -p
# → .skillshare/skills/tools/pdf/
```

執行 `skillshare sync` 後，Targets 會顯示自動攤平後的名稱：
- `frontend/pdf/` → `frontend__pdf`
- `frontend/react/my-skill/` → `frontend__react__my-skill`
- `devops/_team-skills/frontend/ui/` → `devops___team-skills__frontend__ui`

:::tip
`--into` 會自動建立中介目錄，不需要先手動 `mkdir`。
:::

---

## 建議的資料夾結構

### 依領域劃分

```
skills/
├── frontend/
│   ├── react/
│   ├── vue/
│   └── css/
├── backend/
│   ├── api-design/
│   └── database/
├── devops/
│   ├── docker/
│   └── ci-cd/
└── utils/
    ├── git-workflow/
    └── code-review/
```

### 依工具生態系劃分

```
skills/
├── vue/
│   ├── vue-best-practices/
│   ├── vue-debug-guides/
│   ├── vue-pinia-best-practices/
│   └── vue-router-best-practices/
├── react/
│   └── react-best-practices/
└── web/
    ├── accessibility/
    ├── performance/
    └── seo/
```

### 混合：個人 + tracked repos

```
skills/
├── frontend/              # 個人組織的 Skills
│   └── vue/
├── utils/                 # 個人工具
│   └── ascii-box-check/
├── _team-skills/          # Tracked repo（自動更新）
│   ├── code-review/
│   └── deploy/
└── _org-standards/        # 另一個 tracked repo
    └── security/
```

---

## 為你的 Skills 做版本控制

以資料夾組織 Skills，能與 git 自然搭配使用：

```bash
skillshare init --remote git@github.com:yourname/my-skills.git
skillshare push -m "organize skills into categories"
```

這能帶給你：
- **歷史紀錄** — 跨機器追蹤 Skill 的變更
- **備份** — 透過 GitHub/GitLab
- **分享** — 其他人可以瀏覽並 fork 你的收藏
- **跨機器同步** — 透過 `skillshare pull`（參閱 [Cross-Machine Sync](/docs/how-to/sharing/cross-machine-sync)）

---

## 從平面結構遷移到資料夾結構

:::tip 新安裝的 Skills
若是新的 Skills，請直接使用 `--into` 安裝到正確的資料夾中 — 參見上方的 [直接安裝到資料夾中](#install-directly-into-folders)。
:::

你自己建立的 Skill（用 `skillshare new` 或手動建立）沒有安裝紀錄，可以直接用 `mv` 移動：

```bash
cd ~/.config/skillshare/skills
mkdir -p frontend/react
mv my-react-notes frontend/react/
skillshare sync
```

執行 `sync` 後，Targets 會自動更新 — 舊的平面 symlink 會被清除，並建立新的攤平後名稱。

:::warning 透過 `skillshare install` 安裝的 Skill
不要用 `mv` 移動已安裝的 Skill。`.metadata.json` 裡的安裝紀錄以路徑為 key，移動後的副本會失去來源連結，紀錄仍指向舊位置。請改用 [`skillshare move`](/docs/reference/commands/move)。它會把 Skill 和安裝紀錄一起移動，不會重新下載：

```bash
skillshare move react-best-practices frontend/react
skillshare sync
```

`move` 也可以一次移動多個 Skill 或整個資料夾（`skillshare move frontend archive`）。加上 `--dry-run` 可以先預覽。和 `mv` 一樣，執行 `skillshare sync` 之前 Targets 不會更新。
:::

---

## 另請參閱

- [Source & Targets](/docs/understand/source-and-targets) — 攤平機制如何運作
- [Tracked Repositories](/docs/understand/tracked-repositories) — repo 中的巢狀 Skills
- [最佳實務](./best-practices.md) — 命名慣例
- [install](/docs/reference/commands/install) — 使用 `--into` 安裝到子目錄
