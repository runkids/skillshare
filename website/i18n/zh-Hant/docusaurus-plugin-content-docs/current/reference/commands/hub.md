---
sidebar_position: 8
---

# hub

管理 skill hubs——為組織層級的 Skill 探索而儲存的 hub 來源。

## 何時使用

- 為 `search` 查詢設定你組織的 skill 目錄
- 在多個 hub 之間切換（例如公司層級 vs 團隊層級）
- 列出或移除已儲存的 hub 來源

## hub add

儲存一個 hub 來源到 config，供 [`search --hub`](./search.md) 重複使用。

```bash
skillshare hub add <url> [options]
```

| Flag | 說明 |
|------|------|
| `--label`, `-l` | 自訂標籤（預設：從 URL 主機名稱衍生） |
| `--project`, `-p` | 儲存至 project config |
| `--global`, `-g` | 儲存至 global config |

第一個新增的 hub 會自動設為預設值。標籤不區分大小寫。

hub URL 可以是 HTTP(S) URL、本機檔案路徑，或是 SSH URL——scp 風格（`git@host:org/repo.git`）或 scheme 風格（`ssh://git@host/org/repo.git`）。對於 SSH 來源，索引檔案會透過 `//path` 後綴從 repo 中讀取，預設為 repo 根目錄下的 `skillshare-hub.json`。SSH 來源會使用你的 SSH agent/keys 進行 clone，這對私有及 GitHub Enterprise 主機都適用。

當一個 SSH hub 包含同主機的 GitHub 或 GitHub Enterprise domain 前綴項目時，Skillshare 會透過該 hub 的 SSH identity 安裝它們。舉例來說，一個以 `acme@acme.ghe.com:Org/skills.git//hubs/team.json` 新增的 hub，可以包含 `acme.ghe.com/Org/skills/skills/reviewer`，而搜尋結果會以 `acme@acme.ghe.com:Org/skills.git//skills/reviewer` 的形式安裝它。在 SSH hub 之外，domain 前綴的來源仍然代表 HTTPS。

```bash
skillshare hub add https://internal.corp/hub.json --label team
skillshare hub add ./local-hub.json                          # label derived: "local-hub"
skillshare hub add git@ghe.corp.com:team/skills.git --label ghe
skillshare hub add git@ghe.corp.com:team/skills.git//hubs/team.json --label ghe-team
```

## hub list

列出已儲存的 hubs。`*` 標示預設 hub。

```bash
skillshare hub list [options]
```

| Flag | 說明 |
|------|------|
| `--project`, `-p` | 顯示 project hubs |
| `--global`, `-g` | 顯示 global hubs |

```
$ skillshare hub list
  team      https://internal.corp/hub.json · default
  local     ./local-hub.json

2 hubs
```

別名：`hub ls`

## hub remove

依標籤移除已儲存的 hub。

```bash
skillshare hub remove <label> [options]
```

| Flag | 說明 |
|------|------|
| `--project`, `-p` | 從 project config 移除 |
| `--global`, `-g` | 從 global config 移除 |

如果被移除的 hub 是預設 hub，預設值會被清除。

別名：`hub rm`

## hub default

顯示或設定 `search --hub`（不帶參數時）使用的預設 hub。

```bash
skillshare hub default [label] [options]
```

| Flag | 說明 |
|------|------|
| `--reset` | 清除預設值（還原為 community hub） |
| `--project`, `-p` | 使用 project config |
| `--global`, `-g` | 使用 global config |

```bash
skillshare hub default              # 顯示目前的預設值
skillshare hub default team         # 設定預設值為 "team"
skillshare hub default --reset      # 清除預設值 → community hub
```

## hub index

從已安裝的 skills 建立一個 `skillshare-hub.json` 索引檔案。產生的索引可供 [`search --hub`](./search.md#private-index-search) 使用，進行私有、離線的 skill 探索。

### 用法

```bash
skillshare hub index [options]
```

### 選項

| Flag | 說明 |
|------|------|
| `--source`, `-s` | 要掃描的來源目錄（預設：自動偵測） |
| `--output`, `-o` | 輸出檔案路徑（預設：`<source>/skillshare-hub.json`） |
| `--full` | 包含完整 metadata（flatName、type、version 等） |
| `--audit` | 對每個 skill 執行安全稽核並包含風險分數 |
| `--project`, `-p` | 使用 project mode（`.skillshare/`） |
| `--global`, `-g` | 使用 global mode（`~/.config/skillshare`） |
| `--help`, `-h` | 顯示說明 |

### 輸出模式

**Minimal（預設）**——只包含搜尋與安裝所需的必要欄位：

```json
{
  "schemaVersion": 1,
  "generatedAt": "2026-02-12T10:00:00Z",
  "sourcePath": "/home/user/.config/skillshare/skills",
  "skills": [
    {
      "name": "my-skill",
      "description": "A useful skill",
      "source": "owner/repo/.claude/skills/my-skill",
      "tags": ["workflow"]
    }
  ]
}
```

**Full（`--full`）**——包含用於稽核與管理的 metadata：

```json
{
  "name": "my-skill",
  "description": "A useful skill",
  "source": "github.com/owner/repo/.claude/skills/my-skill",
  "tags": ["workflow"],
  "flatName": "my-skill",
  "type": "github-subdir",
  "repoUrl": "https://github.com/owner/repo.git",
  "version": "abc1234",
  "installedAt": "2026-02-10T03:49:06Z",
  "isInRepo": false
}
```

**Audit（`--audit`）**——從 `skillshare audit` 加入安全風險分數：

```json
{
  "name": "my-skill",
  "description": "A useful skill",
  "source": "owner/repo/.claude/skills/my-skill",
  "riskScore": 0,
  "riskLabel": "clean",
  "auditedAt": "2026-02-22T10:00:00Z"
}
```

`--audit` 可以與 `--full` 合併使用，同時包含 metadata 與風險分數。風險標籤：`clean`（0）、`low`（1–25）、`medium`（26–50）、`high`（51–75）、`critical`（76–100）。

Metadata 欄位使用 `omitempty`——多餘的值會被省略：
- `flatName` 在等於 `name` 時省略
- `relPath` 在等於 `source` 時省略
- `isInRepo` 在為 `false` 時省略

### 範例

```bash
# 建立 minimal 索引（預設）
skillshare hub index

# 建立含完整 metadata 的索引
skillshare hub index --full

# 建立含安全風險分數的索引
skillshare hub index --audit

# 完整 metadata + 風險分數
skillshare hub index --full --audit

# 自訂輸出路徑
skillshare hub index -o /shared/team/skillshare-hub.json

# 自訂來源目錄
skillshare hub index -s ~/my-skills

# Project mode
skillshare hub index -p
```

### 工作流程

一個典型的私有 hub 工作流程：

```
1. 安裝 skills               → skillshare install ...
2. 建立索引                  → skillshare hub index
3. 分享索引檔案               → Commit/host skillshare-hub.json (HTTP, file, or Git repo)
4. 隊友搜尋                  → skillshare search --hub [path-url-or-ssh]
```

當索引存放在私有或 GitHub Enterprise repo 中時，隊友可以直接透過 SSH（`git@host:org/repo.git`）將 `--hub` 指向它，而不需要每個人先自行 clone 一份。

更多細節請參閱 [Hub Index 指南](/docs/how-to/sharing/hub-index)。

## Config 格式

已儲存的 hubs 會存放在 `config.yaml` 的 `hub:` 鍵下：

```yaml
hub:
  default: team
  hubs:
    - label: team
      url: https://internal.corp/hub.json
    - label: local
      url: ./local-hub.json
    - label: ghe
      url: git@ghe.corp.com:team/skills.git//hubs/team.json
```

[公開 hub](https://github.com/runkids/skillshare-hub) 是內建的預設值，不需要另外儲存。當沒有設定自訂預設值時，`search --hub` 會自動回退到它。Fork 這個 repo 即可建立你自己組織的 hub。
