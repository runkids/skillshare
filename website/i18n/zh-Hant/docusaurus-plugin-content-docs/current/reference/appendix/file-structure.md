---
sidebar_position: 3
---

# 檔案結構

skillshare 的目錄配置與檔案位置。

## 總覽

```
~/.config/skillshare/        # XDG_CONFIG_HOME
├── config.yaml              # 設定檔
├── audit-rules.yaml         # 自訂 audit 規則（選用）
├── mcp.yaml                 # MCP 伺服器，若 sources.mcp 指向此處（選用）
├── skills/                  # Skills source（skills + metadata）
│   ├── .metadata.json       # 已安裝 skill 的 metadata（自動管理）
│   ├── .skillignore         # 選用：將 skills 排除在 sync 之外
│   ├── my-skill/            # 一般 skill
│   │   ├── SKILL.md         # Skill 定義（必要）
│   ├── code-review/         # 另一個 skill
│   │   └── SKILL.md
│   └── _team-skills/        # Tracked repository
│       ├── .git/            # Git 歷史紀錄保留
│       ├── frontend/
│       │   └── ui/
│       │       └── SKILL.md
│       └── backend/
│           └── api/
│               └── SKILL.md
├── agents/                  # Agents source（單一 .md 檔案）
│   ├── .agentignore         # 選用：將 agents 排除在 sync 之外
│   ├── reviewer.md          # Agent 檔案
│   └── auditor.md           # 另一個 agent
├── rules/                   # Extras source（若已設定）
│   ├── coding.md
│   └── testing.md
└── commands/                # Extras source（若已設定）
    └── deploy.md

~/.local/share/skillshare/   # XDG_DATA_HOME
├── backups/                 # 備份目錄
│   ├── 2026-01-20_15-30-00/
│   │   ├── claude/          # claude 的 skills 備份
│   │   ├── claude-agents/   # claude 的 agents 備份
│   │   └── cursor/
│   └── 2026-01-19_10-00-00/
│       └── claude/
└── trash/                   # 已解除安裝的 skills/agents（保留 7 天）
    ├── my-skill_2026-01-20_15-30-00/
    │   └── SKILL.md
    └── old-skill_2026-01-19_10-00-00/
        └── SKILL.md

~/.local/state/skillshare/   # XDG_STATE_HOME
├── logs/                    # 操作記錄（JSONL）
│   ├── operations.log       # install、sync、update 等
│   └── audit.log            # 安全性 audit 掃描
├── mcp/                     # MCP sync 狀態（自動管理）
│   ├── state.json           # Skillshare 擁有哪些原生項目
│   └── backups/             # 每次寫入前的 agent 檔案（每個檔案保留最新 20 份）
└── plugins/                 # Plugin 來源經審查後的本機副本

~/.cache/skillshare/         # XDG_CACHE_HOME      
├── version-check.json       # 版本檢查快取（24 小時 TTL）
└── ui/                      # Web UI dist 快取
    └── 0.13.0/              # 每個版本各自快取的資源
        ├── index.html
        └── assets/
```

---

## 設定檔

### 位置

```
~/.config/skillshare/config.yaml
```

**以 XDG 覆寫：**
```
XDG_CONFIG_HOME=/custom/path → /custom/path/skillshare/config.yaml
```

**Windows 預設值：**
```
%AppData%\skillshare\config.yaml
```

### 內容

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/runkids/skillshare/main/schemas/config.schema.json
source: ~/.config/skillshare/skills
agents_source: ~/.config/skillshare/agents  # 選用；預設為 <source 上層目錄>/agents
mode: merge
targets:
  claude:
    path: ~/.claude/skills
    agents:                                 # 選用；為此 target 啟用 agent sync
      path: ~/.claude/agents
  cursor:
    path: ~/.cursor/skills
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
```

完整參考請見 [Configuration](/docs/reference/targets/configuration)。

---

## Metadata 檔案

### 位置

```
~/.config/skillshare/skills/.metadata.json
```

儲存已安裝與 tracked skills 的 metadata。存放在 source 目錄內，方便透過 git 同步以支援多機器設定。由 `install`、`uninstall` 與 `update` **自動管理** — 請勿手動編輯。

### 內容

```json
{
  "skills": [
    {
      "name": "pdf",
      "source": "anthropics/skills/skills/pdf"
    },
    {
      "name": "_team-skills",
      "source": "github.com/team/skills",
      "tracked": true
    }
  ]
}
```

每個項目記錄 skill 名稱與其安裝來源。Tracked repos（以 `_` 為前綴）會包含完整的儲存庫 URL，供 `update` 與 `check` 操作使用。

---

## Source 目錄

### 位置

```
~/.config/skillshare/skills/
```

**Windows：**
```
%AppData%\skillshare\skills\
```

### 結構

```
skills/
├── .metadata.json                # 集中管理的 skill metadata（自動管理）
├── skill-name/                   # Skill 目錄
│   ├── SKILL.md                  # 必要：skill 定義
│   ├── examples/                 # 選用：範例檔案
│   └── templates/                # 選用：程式碼範本
├── frontend/                     # 類別資料夾（透過 --into 或手動建立）
│   └── react-skill/              # 子目錄中的 skill
│       └── SKILL.md              # 以 frontend__react-skill 同步
└── _tracked-repo/                # Tracked repository
    ├── .git/                     # Git 歷史紀錄
    └── ...                       # Skill 子目錄
```

---

## Skill 檔案

### SKILL.md（必要）

Skill 定義檔案：

```markdown
---
name: skill-name
description: Brief description
---

# Skill Name

Instructions for the AI...
```

詳見 [Skill Format](/docs/understand/skill-format)。

### .skillfollow / .skillfollow.local {#skillfollow-optional}

位於**設定的 skills source 根目錄**，宣告直接子連結名稱（如 `_team-skills`）。Local 檔加入本機名稱，不使用 ignore patterns，也不是巢狀 repo 設定。搭配 source `.gitignore` 的 `/_team-skills`、`/.skillfollow.local`（無尾斜線），每台機器自行建連結；`skillshare follow`/`unfollow` 會一併寫入或移除連結、宣告與 ignore 行。`_` 前綴且含 `.git` 為 tracked repo；根目錄含 `SKILL.md` 尚不支援。見[設定、嚴格名稱與安全](../skillfollow.md)。

### .skillignore（選用） {#skillignore-optional}

將 skills 排除在探索之外。支援兩種位置：

**Repo 層級** — 位於 tracked skill repository 的根目錄。會影響安裝時的探索以及所有安裝後的指令（`doctor`、`status`、`list`、`sync` 等）：

```text title="_team-skills/.skillignore"
# 隱藏 vendored 套件，不讓它們被探索到
.venv
node_modules

# 排除內部工具
validation-scripts
prompt-eval-*
```

**Source 根目錄層級** — 位於你的 source 目錄根目錄（`~/.config/skillshare/skills/.skillignore`）。全域套用於所有 skills（tracked 與非 tracked 皆適用）：

```text title="~/.config/skillshare/skills/.skillignore"
# 暫時停用某個 skill
my-experimental-skill

# 排除所有草稿
draft-*
```

採用 [gitignore 語法](https://git-scm.com/docs/gitignore) — 每行一個規則。支援 `*`（單一段落）、`**`（任意深度）、`?`、`[abc]`（字元類別）、`!pattern`（否定）、`/pattern`（錨定）、`pattern/`（僅限目錄），以及 `\#`/`\!`（跳脫字元）。以 `#` 開頭的行視為註解。像 `internal-tools` 這樣的群組名稱會排除該目錄下的所有 skills；`internal-tools/helper` 則只排除特定的 skill。兩個層級都會生效 — 只要其中一個符合，該 skill 就會被排除。

:::tip
`.skillignore` 是三層篩選機制之一。參見 [篩選 Skills](/docs/how-to/daily-tasks/filtering-skills)，了解所有情境，包括 per-target 篩選器與 SKILL.md 的 `targets`。
:::

### .skillignore.local（選用） {#skillignorelocal-optional}

一個與 `.skillignore` 搭配使用的本機專用覆寫檔案。放在與 `.skillignore` 相同的目錄下（source 根目錄或 tracked repo 根目錄）。`.skillignore.local` 中的規則會附加在 `.skillignore` 之後，因此否定規則（`!pattern`）可以覆寫基礎檔案：

```text title="_team-skills/.skillignore.local"
# repo 的 .skillignore 封鎖了 private-*，但我需要自己的例外
!private-mine
```

此檔案**不應**被 commit 進版本控制 — 請將它加入 `.gitignore`。它的存在是為了讓 repo 的使用者能在本機覆寫 repo 維護者的 `.skillignore`，而不需要修改該檔案。

啟用時，`sync -v`、`status` 與 `doctor` 會顯示 `.local active` 標示。


---

## Agent 檔案

Agents 是與 skills 不同的另一種資源類型。它們存放在一個相鄰的 source 目錄中，並會同步到支援 agent 的 targets（Claude、Cursor、Augment、OpenCode）。

### Agent source 目錄

```
~/.config/skillshare/agents/      # Global mode
.skillshare/agents/               # Project mode
```

Agent source 會由 `skillshare init` 與 `skills/` 一併自動建立。你可以透過 `agents_source` 設定欄位覆寫 global 模式下的位置；project mode 一律使用 `.skillshare/agents/`。

### Agent 檔案格式

每個 agent 都是一個帶有 frontmatter 的單一 Markdown 檔案：

```markdown title="~/.config/skillshare/agents/reviewer.md"
---
name: reviewer
description: Reviews pull requests for security and style issues.
---

# Reviewer

Instructions for the AI agent...
```

Agent 檔名只能使用 `a-z`、`0-9`、`_`、`-`、`.`。與 skills 不同，agents 是**單一檔案** — 它們不包含子目錄。

完整的檔案格式與探索規則請見 [Agents](/docs/understand/agents)。

### .agentignore（選用）

將 agents 排除在 sync 之外。位於 agents source 根目錄：

```text title="~/.config/skillshare/agents/.agentignore"
# 隱藏草稿
draft-*

# 停用特定 agent
experimental-reviewer
```

採用 [gitignore 語法](https://git-scm.com/docs/gitignore)。與 `.skillignore` 相同的規則語法皆適用（`*`、`**`、`!negation`、`#` 註解等）。被停用的 agents 仍會留在 source 目錄中，但不會出現在 sync 結果裡。

`skillshare disable <agent>` 與 `skillshare enable <agent>` 會自動新增/移除對應項目。

### .agentignore.local（選用）

一個本機專用的覆寫檔案（模式與 `.skillignore.local` 相同）。放在 `.agentignore` 旁邊。規則會附加在 `.agentignore` 之後，因此 `!negation` 規則可以重新啟用基礎檔案停用的 agent。不應被 commit 進版本控制。

---

## 備份目錄

### 位置

```
~/.local/share/skillshare/backups/
```

### 結構

```
backups/
└── <timestamp>/             # YYYY-MM-DD_HH-MM-SS
    ├── claude/              # target 的備份
    │   ├── skill-a/
    │   └── skill-b/
    └── cursor/
        └── ...
```

備份會在以下情況自動建立：
- `sync` 與 `target remove` 執行之前自動建立
- 透過 `skillshare backup` 手動建立

---

## Trash 目錄

### 位置

```
~/.local/share/skillshare/trash/
```

**Project mode：**
```
<project>/.skillshare/trash/
```

### 結構

```
trash/
└── <skill-name>_<timestamp>/    # skill-name_YYYY-MM-DD_HH-MM-SS
    ├── SKILL.md
    └── ...                      # 所有原始檔案皆保留
```

被丟進垃圾桶的 skills 會：
- 由 `skillshare uninstall` 建立
- 保留 7 天，之後自動清除
- 以原本的 skill 名稱加上時間戳記命名

---

## Log 目錄

### 位置

```
~/.local/state/skillshare/logs/
```

**Project mode：**
```
<project>/.skillshare/logs/
```

---

## Target 目錄

Targets 是 AI CLI 的 skill 目錄。Sync 之後，它們會包含指向 source 的 symlinks（或副本）。

### Merge mode

每個 skill 各自被 symlink。一份 manifest 會追蹤受管理的 skills，供孤兒清理使用：
```
~/.claude/skills/
├── my-skill -> ~/.config/skillshare/skills/my-skill
├── code-review -> ~/.config/skillshare/skills/code-review
├── local-only/              # 未被 symlink（使用者自建，會被保留）
└── .skillshare-manifest.json  # 追蹤受管理的 skills
```

### Copy mode

每個 skill 都以真實檔案的形式複製。Manifest 會追蹤 checksum 以支援增量 sync：
```
~/.cursor/skills/
├── my-skill/                  # 真實檔案（從 source 複製而來）
├── code-review/               # 真實檔案
├── local-only/                # 使用者自建，會被保留
└── .skillshare-manifest.json  # 追蹤受管理的 skills 與 checksum
```

### Symlink mode

整個目錄都被 symlink：
```
~/.claude/skills -> ~/.config/skillshare/skills/
```

### 以副本取代檔案連結（Windows） {#copies-in-place-of-file-links-windows}

在沒有開啟開發人員模式的 Windows 上，agent targets 以及 `merge` 模式的目錄型 extras 會拿到副本，而不是檔案連結。Target 目錄中會有一個 `.skillshare-manifest.json`，記錄每份副本的 checksum，讓之後的 sync 可以更新或清理它們，並保留你自己的檔案：
```
~/.claude/agents/
├── reviewer.md                # source agent 的副本
├── local-agent.md             # 使用者自建，會被保留
└── .skillshare-manifest.json  # 追蹤複製的 agents 與 checksum
```

單一檔案 extra（例如共用 `AGENTS.md`）則會把它的副本記錄在 skillshare 的 [extras 備份資料夾](../commands/extras.md#single-file-extras)，而不是 target 目錄中。

---

## Tracked Repositories

Tracked repos（以 `--track` 安裝）會保留 git 歷史紀錄：

```
_team-skills/
├── .git/                    # Git 保留
├── frontend/
│   └── ui/
│       └── SKILL.md
└── backend/
    └── api/
        └── SKILL.md
```

### 命名慣例

- `_` 前綴：tracked repository
- 扁平化名稱中的 `__`：路徑分隔符

**在 source 中：**
```
_team-skills/frontend/ui/SKILL.md
```

**在 target 中（扁平化後）：**
```
_team-skills__frontend__ui/SKILL.md
```

---

## 平台差異

:::tip XDG Base Directory
skillshare 遵循 XDG Base Directory Specification。可用 `XDG_CONFIG_HOME`、`XDG_DATA_HOME`、`XDG_STATE_HOME` 與 `XDG_CACHE_HOME` 覆寫基礎目錄。

詳見 [Environment Variables](./environment-variables.md#xdg_config_home)。
:::

### macOS / Linux

| Item | Path |
|------|------|
| Config | `~/.config/skillshare/config.yaml` |
| Metadata | `~/.config/skillshare/skills/.metadata.json` |
| Skills source | `~/.config/skillshare/skills/` |
| Agents source | `~/.config/skillshare/agents/` |
| Backups | `~/.local/share/skillshare/backups/` |
| Trash | `~/.local/share/skillshare/trash/` |
| Logs | `~/.local/state/skillshare/logs/` |
| Version cache | `~/.cache/skillshare/version-check.json` |
| UI cache | `~/.cache/skillshare/ui/{version}/` |
| Link type | Symlinks |

### Windows

| Item | Path |
|------|------|
| Config | `%AppData%\skillshare\config.yaml` |
| Metadata | `%AppData%\skillshare\skills\.metadata.json` |
| Skills source | `%AppData%\skillshare\skills\` |
| Agents source | `%AppData%\skillshare\agents\` |
| Backups | `%AppData%\skillshare\backups\` |
| Trash | `%AppData%\skillshare\trash\` |
| Logs | `%AppData%\skillshare\logs\` |
| Version cache | `%AppData%\skillshare\version-check.json` |
| UI cache | `%AppData%\skillshare\ui\{version}\` |
| Link type | 資料夾用 NTFS Junctions；單一檔案用 symlinks（需要開發人員模式，否則改為複製） |

## XDG Base Directory 配置

skillshare 在 Unix 系統上遵循 [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir/latest/)：

| XDG Variable | Default Path | skillshare Uses For |
|-------------|-------------|---------------------|
| `XDG_CONFIG_HOME` | `~/.config` | `skillshare/config.yaml`、`skillshare/skills/`（包含 `.metadata.json`）、`skillshare/agents/` |
| `XDG_DATA_HOME` | `~/.local/share` | `skillshare/backups/`、`skillshare/trash/` |
| `XDG_STATE_HOME` | `~/.local/state` | `skillshare/logs/` |
| `XDG_CACHE_HOME` | `~/.cache` | `skillshare/ui/`（已下載的 web dashboard） |

### Windows 路徑

| Purpose | Path |
|---------|------|
| Config + Skills | `%AppData%\skillshare\` |
| Data（backups、trash） | `%AppData%\skillshare\` |
| State（logs） | `%AppData%\skillshare\` |
| Cache（UI） | `%AppData%\skillshare\` |

### 遷移說明

如果是從 XDG 拆分之前的版本升級，skillshare 會在第一次執行時自動將資料從舊位置（`~/.config/skillshare/`）遷移到正確的 XDG 目錄。

---

## 相關文件

- [Configuration](/docs/reference/targets/configuration) — 設定檔細節
- [Skill Format](/docs/understand/skill-format) — SKILL.md 格式
- [Agents](/docs/understand/agents) — Agent 檔案格式與探索規則
- [Tracked Repositories](/docs/understand/tracked-repositories) — Tracked repos
