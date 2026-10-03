---
sidebar_position: 5
---

# search

從 GitHub repositories 探索並安裝 skills。

## 何時使用

- 從 hub indexes 探索社群 skills
- 依名稱、tag 或描述尋找 skills
- 在安裝前瀏覽可用的 skills

## 快速開始

```bash
skillshare search vercel       # 依關鍵字搜尋
skillshare search              # 瀏覽熱門 skills
```

這會在 GitHub 上搜尋包含符合查詢內容的 `SKILL.md` 檔案的 repositories。

## 瀏覽模式

未提供查詢時，`search` 會瀏覽 GitHub 上熱門的 skills：

```bash
skillshare search              # 瀏覽熱門 skills
skillshare search --list       # 列出熱門 skills
```

這會使用 `filename:SKILL.md` 作為 GitHub 查詢，並依星數排序結果，優先顯示最熱門的 skill repositories。

## 運作方式

```
skillshare search [query]
        │
        ▼
GitHub Code Search API (filename:SKILL.md + query)
        │
        ▼
Fetch star counts for each repository
        │
        ▼
Sort by stars (most popular first)
        │
        ▼
Interactive selector → Install selected skill
```

## 選擇要安裝的項目

在終端機中，結果會開在選擇器裡，目前所在 skill 的詳情顯示在列表旁。可以選取多個一起安裝；沒有選取時，`Enter` 會安裝目前所在的那一個。按鍵列在畫面底部。

## 選項

| 旗標 | 說明 |
|------|-------------|
| `--project`, `-p` | 安裝到專案層級設定（`.skillshare/`） |
| `--global`, `-g` | 安裝到全域設定（`~/.config/skillshare`） |
| `--hub [URL]` | 從 hub index 搜尋（預設：[skillshare-hub](https://github.com/runkids/skillshare-hub)；或自訂 URL/路徑） |
| `--list`, `-l` | 僅列出結果，不提示安裝 |
| `--json` | 以 JSON 輸出（供腳本使用） |
| `--limit N`, `-n N` | 最大結果數（預設：20，最大：100） |
| `--help`, `-h` | 顯示說明 |

:::tip 自動偵測
若未指定 `--project` 或 `--global`，skillshare 會自動偵測：如果目前目錄存在 `.skillshare/config.yaml`，預設為 project mode；否則為 global mode。
:::

## 範例

### 瀏覽熱門項目

```bash
skillshare search              # 瀏覽熱門 skills（無查詢字串）
```

### 基本搜尋

```bash
skillshare search pdf           # 互動式搜尋並安裝
skillshare search "code review" # 多字詞搜尋
```

### 列表模式

```bash
skillshare search commit --list
```

輸出：
```
  fix            facebook/react/.claude/skills/fix         ★ 242.7k
                 Use when you have lint errors, formatting issues...
  verify         facebook/react/.claude/skills/verify      ★ 242.7k
                 Use when you want to validate changes before committing...
  commit-helper  ...ockroach/.claude/skills/commit-helper  ★ 31.8k
                 Help create git commits and PRs with properly formatted messages...

Found 3 skills

Next
  skillshare install <source>  install one
```

### JSON 輸出

```bash
skillshare search react --json --limit 5
```

```json
[
  {
    "Name": "react-patterns",
    "Description": "React and Next.js performance optimization...",
    "Source": "facebook/react/.claude/skills/react-patterns",
    "Stars": 242700,
    "Owner": "facebook",
    "Repo": "react",
    "Path": ".claude/skills/react-patterns"
  }
]
```

### Project Mode

```bash
skillshare search pdf -p           # 搜尋並安裝到專案
skillshare search react --project  # 相同意思，長格式旗標
```

已安裝的 skills 會放到 `.skillshare/skills/`，專案設定也會自動更新。如果專案尚未初始化，skillshare 會先執行 `init -p`。

### 限制結果數

```bash
skillshare search frontend -n 5   # 只顯示前 5 筆結果
```

## 驗證 {#authentication}

GitHub Code Search API 需要驗證。skillshare 會自動偵測你的憑證：

1. **GitHub CLI**（建議）— 如果你已用 `gh` 登入：
   ```bash
   gh auth login
   ```

2. **環境變數** — 設定 `GITHUB_TOKEN` 或 `GH_TOKEN`：
   ```bash
   export GITHUB_TOKEN=ghp_your_token_here
   ```

### 建立 Token

如果你不使用 `gh` CLI：

1. 前往 [GitHub Settings → Tokens](https://github.com/settings/tokens)
2. 產生新 token（classic）
3. 公開 repos 不需要任何 scopes
4. 設定 token：
   ```bash
   export GITHUB_TOKEN=ghp_your_token_here
   ```

## 結果如何排序

1. **搜尋** — GitHub Code Search 找出符合查詢的 `SKILL.md` 檔案
2. **篩選** — 移除 forked repositories（重複項目）
3. **取得星數** — 取得每個唯一 repository 的星數
4. **排序** — 依星數排序（最熱門優先）
5. **限制** — 回傳前 N 筆結果

這確保高品質、熱門的 skills 會優先出現。

## 社群 Hub

從 [skillshare-hub](https://github.com/runkids/skillshare-hub) 瀏覽並安裝社群精選的 skills：

```bash
skillshare search --hub                # 瀏覽 skillshare-hub 中所有 skills
skillshare search react --hub          # 在 skillshare-hub 中搜尋「react」
```

當 `--hub` 未帶 URL 使用時，預設會使用社群 [skillshare-hub](https://github.com/runkids/skillshare-hub) index。

想與社群分享你的 skill？[開一個 PR](https://github.com/runkids/skillshare-hub) 來新增你的 skill — CI 會對每個提交執行 `skillshare audit`。

## 已儲存的 Hub 標籤

用 [`hub add`](./hub.md#hub-add) 儲存 hubs，之後可用 label 搜尋，而不用每次都輸入完整 URL：

```bash
# 儲存一次 hub
skillshare hub add https://internal.corp/hub.json --label team

# 依 label 搜尋
skillshare search react --hub team

# 設為空的 --hub 的預設值
skillshare hub default team
skillshare search --hub              # 使用「team」hub
```

`--hub <value>` 的解析順序：
1. URL 或路徑（以 `http`、`/`、`.`、`~`、`file://` 開頭，或是 SSH URL 如 `git@…`/`ssh://…`）→ 直接使用
2. 否則 → 從已儲存的 hubs 中查找 label
3. 空的 `--hub`（無值）→ 設定檔預設值 → 社群 hub 後備

管理已儲存 hubs 請見 [`hub`](./hub.md)。

## 私有索引搜尋 {#private-index-search}

從私有 hub index（而非 GitHub）搜尋：

```bash
# 本地檔案
skillshare search react --hub ./skillshare-hub.json

# HTTP URL
skillshare search react --hub https://internal.corp/skills/skillshare-hub.json

# SSH URL — clone 該 repo 並讀取 index（適用於私有／GHE 主機）
skillshare search react --hub git@github.com:org/skills.git
skillshare search react --hub git@ghe.corp.com:team/skills.git//hubs/team.json

# 瀏覽所有 skills（空查詢）
skillshare search --hub ./skillshare-hub.json --json

# 也支援等號語法
skillshare search react --hub=./skillshare-hub.json
```

:::note SSH hub 來源
SSH 形式的 `--hub` 值，會透過淺層 clone 該 repo（使用你的 SSH agent/keys）並讀取其中的 index 檔案來解析。repo 內的檔案路徑來自 `//path` 後綴 — `git@host:org/repo.git//hubs/team.json` — 若省略則預設為 repo 根目錄下的 `skillshare-hub.json`。scp 風格（`git@host:org/repo.git`）與 scheme 風格（`ssh://git@host/org/repo.git`）的 URL 皆可接受。

在[網頁儀表板](./ui.md)中，SSH hub 來源必須先[儲存](./hub.md#hub-add)；伺服器只會 clone 已儲存的 hubs。
:::

用 [`hub index`](./hub.md) 建立 index：

```bash
skillshare hub index                           # 產生 skillshare-hub.json
skillshare search --hub ./skillshare-hub.json  # 搜尋它
```

:::tip 預設 hub
`skillshare search --hub`（不帶 URL）預設會使用社群 [skillshare-hub](https://github.com/runkids/skillshare-hub) index，所以你不需要每次都輸入完整 URL。也可以用 `skillshare hub default <label>` 設定自己的預設值。
:::

更多細節見 [Hub Index Guide](/docs/how-to/sharing/hub-index)。

## 小技巧

### 尋找官方 Skills

搜尋知名組織：
```bash
skillshare search anthropic    # Anthropic 的 skills
skillshare search facebook     # Meta/Facebook 的 skills
skillshare search vercel       # Vercel 的 skills
```

### 尋找特定功能

依你想做的事情搜尋：
```bash
skillshare search "pull request"
skillshare search deployment
skillshare search testing
skillshare search database
```

### 連續搜尋

找不到結果，或在選擇器中按 `Esc` 時，會再詢問新的關鍵字：

```
? Search again (or press Enter to quit): react
```

## 疑難排解

### 「GitHub Code Search API requires authentication」

執行 `gh auth login` 或設定 `GITHUB_TOKEN`。見 [Authentication](#authentication)。

### 「GitHub API rate limit exceeded」

- 已驗證的使用者：Code Search 每分鐘 30 次請求
- 稍等一分鐘後再試
- 用 `--limit` 減少 API 呼叫次數

### 找不到新的 Repository

GitHub 為新 repositories 建立索引會有延遲（數小時到數天）。如果找不到你的 repo：
- 直接安裝：`skillshare install owner/repo/path/to/skill`
- 等待 GitHub 為該 repository 建立索引

### 結果與查詢不符

GitHub Code Search 會比對 `SKILL.md` 檔案內的內容。即使某個 skill 本身不是專門討論 Vercel 的，只要它的描述中提到「vercel」，就會出現在 vercel 的搜尋結果中。

安裝前用 `--list` 檢視結果。
