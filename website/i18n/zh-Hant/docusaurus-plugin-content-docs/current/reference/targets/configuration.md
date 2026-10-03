---
sidebar_position: 4
---

# 設定

skillshare 的設定檔參考文件。

## 總覽

```text
~/.config/skillshare/
├── config.yaml          ← 設定檔
├── skills/              ← Source 目錄（你的 Skill）
│   ├── .metadata.json   ← Skill 中繼資料（自動管理）
│   ├── my-skill/
│   ├── another/
│   └── _team-repo/      ← Tracked 儲存庫
├── extras/              ← Extras Source 根目錄
│   └── rules/           ← 額外資源（例如規則）

~/.local/share/skillshare/
└── backups/             ← 自動備份
    └── 2026-01-20.../
```

---

## IDE 支援（JSON Schema） {#ide-support}

設定檔包含 YAML Language Server 指示詞，可在支援的編輯器中啟用**自動完成**、**驗證**與**懸停說明文件**。

由 `skillshare init` 建立的新設定檔會自動包含這個功能：

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/runkids/skillshare/main/schemas/config.schema.json
source: ~/.config/skillshare/skills
targets:
  claude:
    path: ~/.claude/skills
```

### 新增到既有設定檔

如果你的設定檔是在這個功能推出之前建立的，把這個註解加到**第一行**：

**Global 設定**（`~/.config/skillshare/config.yaml`）：
```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/runkids/skillshare/main/schemas/config.schema.json
```

**Project 設定**（`.skillshare/config.yaml`）：
```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/runkids/skillshare/main/schemas/project-config.schema.json
```

或者直接重新執行 `skillshare init --force`（Global）或 `skillshare init -p --force`（Project），重新產生帶有 schema 註解的設定檔。

### 支援的編輯器

| 編輯器 | 需要的擴充套件 |
|--------|-------------------|
| VS Code | Red Hat 出品的 [YAML](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml) |
| JetBrains IDE | 內建 YAML 支援 |
| Neovim | 透過 LSP 使用 [yaml-language-server](https://github.com/redhat-developer/yaml-language-server) |

---

## 設定檔

**位置：** `~/.config/skillshare/config.yaml`

### 完整範例

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/runkids/skillshare/main/schemas/config.schema.json
# Source 目錄（你編輯 Skill 的地方）
source: ~/.config/skillshare/skills

# 新 Target 的預設 Sync 模式
mode: merge

# 預設 Target 命名方式（flat 或 standard）
# target_naming: flat

# Targets（AI CLI Skill 目錄）
targets:
  claude:
    path: ~/.claude/skills
    # mode: merge（繼承自預設值）

  codex:
    path: ~/.codex/skills
    mode: symlink  # 覆寫預設模式
    include: [codex-*] # 僅適用於 merge/copy 模式

  cursor:
    path: ~/.cursor/skills
    mode: copy  # Cursor 使用真實檔案
    exclude: [experimental-*] # 僅適用於 merge/copy 模式

  # 自訂 Target
  myapp:
    path: ~/apps/myapp/skills

# 遠端 Skill — 由 install/uninstall 自動管理
skills:
  - name: pdf
    source: anthropics/skills/skills/pdf
  - name: _team-skills
    source: github.com/team/skills
    tracked: true

# 儲存時把 $HOME 摺疊回 ~（適合 dotfiles）
# preserve_tilde_on_save: true

# commit/push/pull 使用的目錄（預設 skills，還有 agents、extras、root）
# git_root: skills

# 自訂 Agent Source（選用，覆寫預設位置）
agents_source: ~/my-agents

# 自訂 Extras Source（選用，覆寫預設位置）
extras_source: ~/my-extras

# 要同步的非 Skill 資源
extras:
  - name: rules
    source: ~/company-shared/rules   # 選用的個別 Extra 覆寫
    targets:
      - path: ~/.claude/rules
      - path: ~/.cursor/rules
        mode: copy

# Sync 時要忽略的檔案
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
  - "**/node_modules/**"
  - "**/*.log"
```

---

## 欄位

### `source`

你的 Skill 目錄路徑（單一真實來源）。

```yaml
source: ~/.config/skillshare/skills
```

**預設值：** `~/.config/skillshare/skills`

### `mode`

所有 Target 的預設 Sync 模式。

```yaml
mode: merge
```

| 值 | 行為 |
|-------|----------|
| `merge` | 每個 Skill 各自 symlink。本地 Skill 會被保留。**（預設）** |
| `copy` | 每個 Skill 複製為真實檔案。適用於無法遵循 symlink 的 AI CLI。 |
| `symlink` | 整個 Target 目錄是一個 symlink。 |

### `target_naming`

merge/copy 同步的預設 Target 命名策略。

```yaml
target_naming: flat
```

| 值 | 行為 |
|-------|----------|
| `flat` | 巢狀 Skill 用 `__` 分隔攤平（例如 `frontend__dev`）。**（預設）** |
| `standard` | 直接使用 SKILL.md 的 `name` 欄位（例如 `dev`）。遵循 [Agent Skills 規範](https://agentskills.io/specification)。 |

兩種模式下，`include` / `exclude` 都一律比對扁平化的名稱（`frontend__dev`），所以在 `standard` 下 filter 用的名稱與 `sync` 建立的資料夾名稱不同——見 [include / exclude](#include--exclude-target-filters)。

### `targets`

要同步的 AI CLI Skill 目錄。

```yaml
targets:
  <name>:
    path: <path>
    mode: <mode>  # 選用，覆寫預設值
    include: [<glob>, ...]  # 選用，僅適用於 merge/copy 模式
    exclude: [<glob>, ...]  # 選用，僅適用於 merge/copy 模式
```

**範例：**
```yaml
targets:
  claude:
    path: ~/.claude/skills

  codex:
    path: ~/.codex/skills
    mode: symlink

  custom:
    path: ~/my-app/skills
```

#### 某個 Agent 的另一個帳號 {#agent-config-dir}

Target 可以是某個內建 Agent 的第二個 config 目錄：以 `CLAUDE_CONFIG_DIR` 啟動的 Claude Code、以 `CODEX_HOME` 啟動的 Codex，或以 `PI_CODING_AGENT_DIR` 啟動的 Pi。指定 Agent 與該目錄即可；Skill 與 agents 的路徑會跟著它決定。

```yaml
targets:
  claude-work:
    agent: claude
    config_dir: ~/.claude-work   # Skill 會放到 ~/.claude-work/skills，agents 放到 ~/.claude-work/agents
  codex-work:
    agent: codex
    config_dir: ~/.codex-work    # Skill 會放到 ~/.codex-work/skills
```

Codex 也會讀取共用的 `~/.agents/skills`，但一個帳號只擁有自己的目錄，因此它的 Skill 會放到 `<config_dir>/skills`。Pi 的運作方式相同。只有 Claude 有 agents 目錄。

| 欄位 | 說明 |
|-------|-------------|
| `agent` | 內建的 Agent：`claude`（`CLAUDE_CONFIG_DIR`）、`codex`（`CODEX_HOME`）或 `pi`（`PI_CODING_AGENT_DIR`） |
| `config_dir` | 該帳號的 config 目錄。必須是絕對路徑或以 `~` 開頭，不能是該 Agent 的預設目錄，且只能由一個 Target 使用 |
| `cli` | 選填。用相容的 CLI 取代 Agent 本身來執行這個帳號的 [plugin 指令](/docs/reference/commands/plugin#accounts)，例如 Pi 用 `omo`。填 `PATH` 上的名稱，或絕對路徑（可以用 `~` 開頭）。只能是一個執行檔、不帶參數；shell alias 不會生效 |

沿用 Agent 指令的相容 CLI 可以執行這個帳號的 plugin。例如 omo 是以 Pi 為基礎：

```yaml
targets:
  omo:
    agent: pi
    config_dir: ~/.omo/agent
    cli: omo
```

`cli` 只改變由哪個程式安裝與移除 plugin。Skills、agents 與 MCP servers 仍照舊寫入 `config_dir`。

`mode`、`include`、`exclude` 與其他 Target 設定的運作方式與任何 Target 相同。你自己寫的 `skills.path` 或 `agents.path` 會優先於推導出來的路徑。Target 名稱也可以當作 [MCP target](/docs/reference/commands/mcp#accounts) 與 [plugin target](/docs/reference/commands/plugin#accounts) 使用。 也可以當作 [hooks target](/docs/reference/commands/hooks#accounts) 使用。

#### 指示檔案 {#target-instructions}

skillshare 知道許多內建 Target 的指示檔案（`CLAUDE.md`、`AGENTS.md`、`GEMINI.md`……）。
對於其他工具，`instructions` 會告訴 skillshare 該工具讀哪個檔案，讓 dashboard 能顯示並編輯它，
也能接上[共用 AGENTS.md](../../how-to/daily-tasks/sharing-instructions.md)。在這裡設定的值會取代內建的檔案。

```yaml
targets:
  myagent:
    path: ~/.myagent/skills
    instructions:
      path: ~/.myagent/AGENTS.md
      import: true        # 這個工具會展開 @path 行
```

| 欄位 | 說明 |
|-------|-------------|
| `instructions.path` | 該工具讀取的檔案。在 global config 中，須為絕對路徑或以 `~/` 開頭。在 project config 中，須為相對於專案根目錄的路徑，例如 `.myagent/AGENTS.md`。必須指向檔案，而不是目錄 |
| `instructions.import` | 工具會展開 `@path` 行時設為 `true`。這樣它就能同時使用多份共用檔案，每份各加一行 import。預設為 `false`：工具只使用一份共用檔案，以連結取代它自己的檔案 |

Dashboard 會在新增 Target 時從 **自訂目標** 對話框寫入這個欄位，之後也可以在 Target 的檔案分頁修改。當 Target 正在使用共用檔案時，
它會拒絕變更或移除此欄位。移除它不會刪除該檔案。

#### 其他檔案 {#target-files}

除了指示檔案，工具也可能讀取其他一般檔案，例如 Pi 的 `APPEND_SYSTEM.md`。Dashboard 會在 Target 頁面上把每個檔案顯示成一個分頁。
skillshare 會為 `pi` 與 `omp` 加上 `APPEND_SYSTEM.md`；`files` 列出的是你自己加入的檔案。

```yaml
targets:
  pi:
    files:
      - SYSTEM.md
      - prompts/review.md
```

每個項目都是相對於工具資料夾的路徑：在 global config 中，pi 是 `~/.pi/agent`；在專案中是 `.pi`。項目可以指向子資料夾，
但不能離開該資料夾：絕對路徑、`..`，以及連結到資料夾外的資料夾都會被拒絕。這個資料夾是工具自己的設定資料夾，
例如 codex 的 `~/.codex`，或某個帳號的 [`config_dir`](#agent-config-dir)；skillshare 不知道時，則是 skills 資料夾的上一層。
如果 Target 的資料夾會是你的家目錄或專案根目錄，就不會有 **+** 按鈕。

Dashboard 會在你新增或移除分頁時寫入這個欄位。移除分頁不會刪除檔案。

#### 關閉 Skills {#skills-enabled}

`skills.enabled: false` 會停止同步 Skill 到某個 Target，同時 skillshare 仍繼續管理它的 agents、MCP servers 與指示檔案。適用於已經會讀取另一個 Target 之 Skill 資料夾的工具，避免它找到每個 Skill 兩次。

```yaml
targets:
  pi:
    skills:
      path: ~/.pi/agent/skills
      enabled: false
```

路徑、模式與篩選條件都會保留在設定中，供你之後重新開啟 Skills 時使用。可以用 `skillshare target <name> --skills=false` 設定（同時會移除資料夾中指向 source 的連結），或以 `--no-skills` 新增 Target。詳見 [Skills 開啟或關閉](/docs/reference/commands/target#skills-off)。

### `include` / `exclude`（Target 篩選條件） {#include--exclude-target-filters}

使用每個 Target 各自的篩選條件，控制在 **merge 與 copy 模式**下要同步哪些 Skill。

```yaml
targets:
  codex:
    path: ~/.codex/skills
    include: [codex-*]
  claude:
    path: ~/.claude/skills
    exclude: [codex-*]
```

規則：
- 比對的對象是 Target 的 flat 名稱（例如 `team__frontend__ui`）
- 比對不到任何 skill 的 `include` 模式會被回報，因為這樣的 target 不會同步任何東西，還會移除先前由正確模式連結的條目。即使 `target_naming: standard` 讓 target 目錄顯示 SKILL.md 的裸名稱，filters 用的仍是 flat 名稱
- `include` 會先套用
- `exclude` 會在 include 之後套用
- 樣式語法使用 Go 的 `filepath.Match`（`*`、`?`、`[...]`）
- 在 `symlink` 模式下，include/exclude 會被忽略
- 如果先前已同步的 Source 連結被排除了，`sync` 會移除該 Target 項目
- Target 中既有的本地非 symlink 資料夾會被保留

#### 樣式速查表

| 樣式 | 比對對象 | 典型用途 |
|---------|---------|-------------|
| `codex-*` | `codex-agent`、`codex-rag` | 以前綴分組 |
| `team__*` | `team__frontend__ui` | 儲存庫／群組命名空間 |
| `*-experimental` | `rag-experimental` | 以後綴清理 |
| `core-?` | `core-a`、`core-1` | 單一字元變體 |
| `[ab]-tool` | `a-tool`、`b-tool` | 小型的明確集合 |

#### 情境 A：只有 include

當某個 Target 只該接收一個聚焦的子集時，使用 `include`。

```yaml
targets:
  codex:
    path: ~/.codex/skills
    include: [codex-*, shared-*]
```

使用案例：
- 讓 Codex 只專注在程式碼相關的工作流程
- 避免把只用於寫作／研究的 Skill 送到這個 Target

#### 情境 B：只有 exclude

當某個 Target 該接收幾乎所有東西，只排除一個已知的子集時，使用 `exclude`。

```yaml
targets:
  claude:
    path: ~/.claude/skills
    exclude: [*-experimental, codex-*]
```

使用案例：
- 讓一個主要 Target 保持廣泛涵蓋
- 隱藏不穩定或特定 Target 專用的 Skill

#### 情境 C：include + exclude

當你想先設定一個廣泛的 include，再從中排除少數例外時，兩者並用。

```yaml
targets:
  cursor:
    path: ~/.cursor/skills
    include: [core-*, team__*]
    exclude: [*-deprecated, team__legacy__*]
```

評估順序：
1. 只保留符合 `include` 的名稱
2. 從中移除符合 `exclude` 的項目

假設 Source 中有以下 Skill：
- `core-auth`
- `core-deprecated`
- `team__frontend__ui`
- `team__legacy__docs`
- `misc-tool`

`cursor` 的結果：
- 會同步：`core-auth`、`team__frontend__ui`
- 不會同步：`core-deprecated`、`team__legacy__docs`、`misc-tool`

#### 透過 CLI 管理篩選條件

除了手動編輯 YAML，也可以使用 `target` 指令：

```bash
# Skills
skillshare target claude --add-include "team-*"
skillshare target claude --add-exclude "_legacy*"
skillshare target claude --remove-include "team-*"

# Agents（僅適用於有 Agent 路徑的 Target）
skillshare target claude --add-agent-include "team-*"
skillshare target claude --add-agent-exclude "draft-*"
skillshare target claude --remove-agent-include "team-*"

skillshare sync  # 套用變更
```

重複的樣式會被靜默忽略。無效的 glob 樣式會回傳錯誤。Agent 篩選條件使用與 Skill 篩選條件相同的 glob 語法，但只在 `merge` 與 `copy` 模式下運作。在 `symlink` 模式下，Agent 篩選條件會被忽略，因為整個 Agent 目錄是以單一單位連結的。

完整參考請參閱 [target 指令](/docs/reference/commands/target#target-filters-includeexclude)。

#### Skill 層級的 targets {#skill-level-targets}

Skill 可以在 SKILL.md 中使用 `metadata.targets` 宣告它們相容的 Target。頂層的 `targets` 欄位仍受支援，做為舊版 Skill 的備援方式，但當兩者都存在時，`metadata.targets` 優先：

```yaml
---
name: claude-prompts
metadata:
  targets: [claude]
---
```

這是**第二層**篩選機制，與設定層級的 include/exclude 一起運作：

```
Source Skills
  │
  ├─ 設定的 include/exclude    ← 每個 Target 各自設定，由使用端決定
  │
  └─ Skill 的 targets 欄位     ← 每個 Skill 各自設定，由作者決定
      │
      ▼
  同步到 Target 的 Skill
```

**評估順序：**
1. `include` — 只保留符合的名稱
2. `exclude` — 移除符合的名稱
3. `targets` 欄位 — 移除 targets 清單中不包含此 Target 的 Skill

兩層都必須通過（AND 關係）。設定層級的篩選條件永遠優先 — 即使某個 Skill 宣告了 `targets: [claude]`，設定中的 `exclude: [claude-*]` 仍然會排除它。

**跨模式比對：**`targets: [claude]` 會同時比對 Global Target `claude` 與 Project Target `claude`，因為它們指的是同一個 AI CLI。詳見[支援的 Targets](/docs/reference/targets/supported-targets)。

:::tip
當**使用端**想要控制哪些東西送去哪裡時，使用設定端的篩選條件（`include`/`exclude`）。當**作者**知道某個 Skill 只適用於特定 AI CLI 時，使用 Skill 層級的 `targets`。
:::

#### 篩選條件變更時，既有的 Target 項目會怎樣

當你新增或變更篩選條件，然後執行 `skillshare sync` 時：

| Target 中的既有項目 | 會發生什麼事 |
|-------------------------|--------------|
| 現在被篩選掉的 Source 連結 symlink/junction | 移除（解除連結） |
| 現在被篩選掉的受管理副本（copy 模式） | 移除 |
| Target 中建立的本地非 symlink 目錄 | 保留 |
| 無關的本地內容 | 保留 |

### `skills`

追蹤遠端安裝的 Skill。由 `skillshare install` 與 `skillshare uninstall` 自動管理。

```yaml
skills:
  - name: pdf
    source: anthropics/skills/skills/pdf
  - name: _team-skills
    source: github.com/team/skills
    tracked: true
```

| 欄位 | 必要 | 說明 |
|-------|----------|--------------|
| `name` | 是 | Skill 目錄名稱 |
| `source` | 是 | GitHub URL 或本地路徑 |
| `tracked` | 否 | 若以 `--track` 安裝則為 `true`（預設：`false`） |

當你在沒有任何參數的情況下執行 `skillshare install`，所有已列出但尚未存在的 Skill 都會被安裝。這讓 `config.yaml` 成為一個可攜的 Skill 清單 — 把它複製到另一台機器，執行 `skillshare install && skillshare sync` 即可。

`skills:` 清單會在每次 `install` 與 `uninstall` 操作後自動更新。你不需要手動編輯它。

:::note 已遷移至 .metadata.json
從 v0.16.2 開始，已安裝 Skill 的項目從 `config.yaml` 遷移到一個獨立的檔案。在目前版本中，所有安裝中繼資料都集中儲存在 `skills/` 目錄下的 `.metadata.json` 中。從舊格式（`registry.yaml`、各 Skill 各自的 `.skillshare-meta.json`）遷移會在第一次執行時自動進行。
:::

### `agents_source` {#agents-source}

Agent 的自訂 Source 目錄。覆寫預設的 `~/.config/skillshare/agents/`。

```yaml
agents_source: ~/my-agents
```

設定後，所有 Agent 都會從這個目錄讀取，而非預設位置。支援 `~` 展開。

預設值：`~/.config/skillshare/agents/`（自動偵測，除非你想要自訂位置，否則不需要明確設定）。

:::note 僅限 Global mode
Project mode 一律使用 `.skillshare/agents/`，不支援 `agents_source`。
:::

Agent 檔案格式、同步行為與支援的 Target 詳情，請參閱 [Agents](/docs/understand/agents)。

### `projects` {#projects}

從這份 global config 取得 skills 與 agents 的 project 資料夾。這些資料夾不需要各自擁有 `.skillshare/`，在任何地方執行一次 `skillshare sync` 就會全部寫入。

當 projects 應該拿到不同的 skills 時使用它。Global targets 已經會用同一組觸及每個 project，而 [project mode](/docs/understand/project-skills) 則會把設定保留在 project 的 repo 中給隊友使用。[多個 Projects，一份 Config](/docs/how-to/recipes/many-projects-one-config#scenario) 比較了這三種方式。

```yaml
projects:
  <folder>:                  # 絕對路徑，或以 ~ 開頭
    name: <name>             # 選用，預設為資料夾名稱
    targets: [<target>, ...] # 這個 project 使用的工具
    skills:                  # 存在則同步 skills；留空代表全部同步
      mode: <mode>
      target_naming: <flat|standard>
      include: [<glob>, ...]
      exclude: [<glob>, ...]
    agents:                  # 存在則同步 agents；留空代表全部同步
      mode: <mode>
      include: [<glob>, ...]
      exclude: [<glob>, ...]
```

**範例：**
```yaml
projects:
  ~/work/shop-web:
    targets: [claude, cursor, codex]
    skills:
      mode: copy
      include: ["frontend-*"]
    agents: {}
  ~/work/api-server:
    targets: [claude]
    skills: {}
```

`targets` 中的每一項都是一個[支援的 Target](./supported-targets.md)名稱。Skillshare 會寫入該工具在此資料夾內的 project 路徑，例如 `claude` 的 `.claude/skills` 與 `.claude/agents`。不需要設定 `path`。

- **共用的資料夾只會寫入一次。** 若多個工具讀取同一個 project 資料夾（`cursor` 與 `codex` 都讀取 `.agents/skills`），它們會被合併成同一個同步目標。
- **輸出中的名稱。** `sync`、`status`、`diff`、`doctor` 與 `backup` 會以 `<name>@<target>` 的形式顯示一個 project 的 targets，例如 `shop-web@claude`。`name` 不能包含 `@`、`/` 或 `\`，且兩個 project 不能共用同一個名稱。
- **Agents** 只會寫入擁有 project agents 目錄的工具。只設定 `agents` 而不設定 `skills` 的 project，只會同步 agents。
- **找不到的資料夾會被跳過。** `sync` 會印出 `project <folder>: folder not found, skipped`，且不會重新建立你已經搬移或刪除的 project。
- **`target` 與 `collect` 不會動到 projects。** `skillshare target` 只會列出並編輯 `targets` 區塊，`collect` 也不會把 project 自己的 skills 拉回 source。請在 `config.yaml` 中編輯 `projects`，或在 dashboard 的 **Projects** 頁面編輯。
- 帶有 `targets` frontmatter 欄位的 skill 會依工具比對，因此 `targets: [claude]` 會涵蓋 `shop-web@claude`。

同樣這些資料夾的 MCP servers 列在 [`mcp.projects`](/docs/reference/commands/mcp#manage-several-projects-from-the-global-config) 底下，以相同的資料夾為 key。逐步設定教學請參見[多個 Projects，一份 Config](/docs/how-to/recipes/many-projects-one-config)。

### `extras` {#extras}

非 Skill 資源（規則、指令、Prompt 等），要同步到任意目錄。

```yaml
extras_source: ~/my-extras            # 選用的 Global 預設 Source
extras:
  - name: rules
    source: ~/company-shared/rules    # 選用的個別 Extra 覆寫
    targets:
      - path: ~/.claude/rules
      - path: ~/.cursor/rules
        mode: copy
  - name: agents
    targets:
      - path: ~/.claude/agents
        flatten: true                 # 把子目錄檔案同步成攤平結構
  - name: commands
    targets:
      - path: ~/.claude/commands
```

| 欄位 | 必要 | 說明 |
|-------|----------|--------------|
| `name` | 是 | Extra 識別碼 |
| `source` | 否 | 此 Extra 的自訂 Source 目錄（覆寫 `extras_source` 與預設值） |
| `file` | 否 | 只同步 Source 目錄中的這個檔案：單純的檔名，例如 `system.md` 或 `AGENTS.md`。詳見[單一檔案 Extra](../commands/extras.md#single-file-extras) |
| `targets` | 是 | Target 路徑清單 |
| `targets[].path` | 是 | 目的地目錄 |
| `targets[].mode` | 否 | `merge`（預設）、`copy` 或 `symlink`；`import` 僅限單一檔案 Extra |
| `targets[].as` | 否 | 單一檔案 Extra 在 Target 中的檔名（預設：`file` 的名稱） |
| `targets[].flatten` | 否 | 為 `true` 時，把子目錄檔案直接同步到 Target 根目錄（不能與 `symlink` 或 `file` 併用） |

`extras_source` 會在執行 `skillshare init` 或第一次 `extras init` 時，自動填入預設路徑（`~/.config/skillshare/extras/`）。可覆寫它，讓所有 Extras 使用自訂位置。

**Source 解析順序**（三層優先序）：
1. 個別 Extra 的 `source` → 確切路徑（例如 `~/company-shared/rules`）
2. `extras_source` → `<extras_source>/<name>/`（例如 `~/my-extras/rules/`）
3. 預設值 → `~/.config/skillshare/extras/<name>/`

**Sync 模式：**
- `merge`（預設）— 逐檔 symlink
- `copy` — 逐檔複製
- `symlink` — 整個目錄 symlink

執行 `skillshare sync extras` 進行同步，或執行 `skillshare sync --all` 同時同步 Skill + Extras。

:::info 兩種模式都支援
Extras 在 Global 與 Project mode 下都能運作。在 Project mode 下，Source 是 `.skillshare/extras/<name>/`。
:::

使用細節請參閱 [sync extras](/docs/reference/commands/sync#sync-extras)。

### `ignore`

同步期間要跳過的檔案 glob 樣式。

```yaml
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
  - "**/node_modules/**"
```

**預設樣式：**
- `**/.DS_Store`
- `**/.git/**`

### `gitlab_hosts`

使用巢狀子群組的自架 GitLab 實例主機名稱。名稱中包含 `gitlab` 或 `jihulab` 的主機會自動偵測 — 此欄位只在其他自訂網域時才需要。

```yaml
gitlab_hosts:
  - git.company.com
  - code.internal.io
```

當某個主機名稱列在這裡時，`skillshare install` 會把完整的 URL 路徑當成儲存庫處理（支援最多 20 層的巢狀子群組），而非假設標準的 `owner/repo` 兩段式結構。

**沒有 `gitlab_hosts` 時：**
```bash
# git.company.com/team/frontend/ui → clone「team/frontend」，子目錄「ui」
skillshare install git.company.com/team/frontend/ui
```

**有 `gitlab_hosts: [git.company.com]` 時：**
```bash
# git.company.com/team/frontend/ui → clone「team/frontend/ui」（完整路徑）
skillshare install git.company.com/team/frontend/ui
```

**不用設定的變通做法：** 在路徑結尾加上 `.git`，標示儲存庫路徑的結束位置：
```bash
skillshare install git.company.com/team/frontend/ui.git
```

項目必須是不含 scheme、路徑或連接埠的裸主機名稱。它們會被正規化為小寫。

#### 環境變數

對於沒有設定檔的 CI/CD pipeline，使用 `SKILLSHARE_GITLAB_HOSTS`（以逗號分隔）：

```bash
SKILLSHARE_GITLAB_HOSTS=git.company.com,code.internal.io skillshare install git.company.com/team/frontend/ui
```

當設定檔與環境變數都有設定時，它們的值會被**合併**（去除重複）。環境變數中的無效項目會被靜默略過。

### `azure_hosts`

自架的 Azure DevOps Server 實例主機名稱。針對 `dev.azure.com` 與 `*.visualstudio.com` 的內建樣式一律有效 — 此欄位只在使用自訂網域的地端部署 Azure DevOps Server 時才需要。

```yaml
azure_hosts:
  - azuredevops.mycompany.com
```

當某個主機名稱列在這裡時，包含 `/_git/` 的 URL 會透過 Azure DevOps 的解析邏輯處理，正確擷取出 org、project 和 repo，且不會在 clone URL 後面附加 `.git`。

**沒有 `azure_hosts` 時：**

```bash
# 會落到通用的 HTTPS 解析邏輯 — clone URL 會變成
# https://azuredevops.mycompany.com/Org/Project.git（錯誤）
skillshare install https://azuredevops.mycompany.com/Org/Project/_git/Repo
```

**有 `azure_hosts: [azuredevops.mycompany.com]` 時：**

```bash
# 正確解析 — clone URL 為
# https://azuredevops.mycompany.com/Org/Project/_git/Repo
skillshare install https://azuredevops.mycompany.com/Org/Project/_git/Repo
```

項目必須是不含 scheme、路徑或連接埠的裸主機名稱。它們會被正規化為小寫。

#### 環境變數

對於 CI/CD pipeline，使用 `SKILLSHARE_AZURE_HOSTS`（以逗號分隔）：

```bash
SKILLSHARE_AZURE_HOSTS=azuredevops.mycompany.com skillshare install \
  https://azuredevops.mycompany.com/Org/Project/_git/Repo
```

### `gitea_hosts`

自架的 Gitea 實例主機名稱。名稱中包含 `gitea` 的主機，例如 `gitea.com` 或 `gitea.company.com`，會自動被偵測。此欄位只在其他自訂網域時才需要。

```yaml
gitea_hosts:
  - git.company.com
```

當某個主機名稱列在這裡時：

- `install` 與 `update` 會針對該主機使用 [`GITEA_TOKEN`](/docs/reference/appendix/environment-variables#gitea_token) 進行 HTTPS 身分驗證
- 當無法使用 sparse checkout 或失敗時，`install` 會透過 Gitea Contents API 下載子目錄，而不是 clone 整個儲存庫。如果該 API 呼叫也失敗，則會回退為完整 clone。

項目必須是不含 scheme、路徑或連接埠的裸主機名稱。它們會被正規化為小寫。

#### 環境變數

對於 CI/CD pipeline，使用 `SKILLSHARE_GITEA_HOSTS`（以逗號分隔）：

```bash
SKILLSHARE_GITEA_HOSTS=git.company.com skillshare install https://git.company.com/team/skills/review
```

當設定檔與環境變數都有設定時，它們的值會被**合併**（去除重複）。

### `cnb_hosts`

自架的 [CNB](https://cnb.cool) 實例主機名稱。`cnb.cool` 會自動被偵測。此欄位只在私有部署使用其他網域時才需要。

```yaml
cnb_hosts:
  - cnb.company.com
```

列出的主機會使用 [`CNB_TOKEN`](/docs/reference/appendix/environment-variables#cnb_token) 進行 HTTPS 身分驗證，子目錄安裝也可以透過 CNB contents API 處理，並有相同的完整 clone 回退機制。

項目必須是不含 scheme、路徑或連接埠的裸主機名稱。它們會被正規化為小寫。

#### 環境變數

```bash
SKILLSHARE_CNB_HOSTS=cnb.company.com skillshare install https://cnb.company.com/team/skills/review
```

### `audit`

安全稽核設定。

```yaml
audit:
  block_threshold: CRITICAL
  profile: default
  dedupe_mode: global
  enabled_analyzers: [static, dataflow, tier, integrity]
```

| 欄位 | 值 | 預設值 | 說明 |
|-------|--------|---------|--------------|
| `block_threshold` | `CRITICAL`、`HIGH`、`MEDIUM`、`LOW`、`INFO` | `CRITICAL` | 阻擋 `skillshare install` 的最低嚴重程度 |
| `profile` | `default`、`strict`、`permissive` | `default` | 稽核設定檔預設值（設定 threshold 與 dedupe 的預設值） |
| `dedupe_mode` | `legacy`、`global` | `global` | 發現項目的去重複模式 |
| `enabled_analyzers` | Analyzer ID 陣列 | *（全部）* | 要執行的 Analyzer 允許清單（省略代表全部） |

**Profile** 會設定合理的預設值，可被明確的欄位值覆寫：

| Profile | Threshold | Dedupe | 說明 |
|---------|-----------|--------|--------------|
| `default` | `CRITICAL` | `global` | 與目前行為相同 |
| `strict` | `HIGH` | `global` | 對重視安全的團隊更嚴格的阻擋 |
| `permissive` | `CRITICAL` | `legacy` | 僅供參考，最小程度的阻擋 |

**Analyzer ID：** `static`、`dataflow`、`tier`、`integrity`、`structure`、`cross-skill`

**優先順序：** CLI 旗標 → Project 設定 → Global 設定 → Profile 預設值。

- `block_threshold` 只控制安裝**何時被阻擋** — 掃描一律都會執行
- 使用 `--skip-audit` 可讓單次安裝略過掃描
- 使用 `--force` 可覆寫阻擋（發現項目仍會顯示）

### `context_budget`

Token 預算警告門檻。當 Token 數超過預算時，`sync` 與 `analyze` 之後會出現警告。

```yaml
context_budget:
  warn_always_loaded_tokens: 10000
  warn_on_demand_tokens: 100000
```

| 欄位 | 型別 | 預設值 | 說明 |
|-------|------|---------|--------------|
| `warn_always_loaded_tokens` | 整數 | `10000` | 當一律載入的 Token 數超過此值時發出警告。`0` 代表停用 |
| `warn_on_demand_tokens` | 整數 | `100000` | 當按需載入的 Token 數超過此值時發出警告。`0` 代表停用 |

省略時套用預設值（10K / 100K）。使用 `skillshare sync --quiet` 可抑制警告。輸出格式請參閱 [sync — Context 成本](/docs/reference/commands/sync#context-cost)。

### `preserve_tilde_on_save`

為 `true` 時，會在寫入 `config.yaml` 前，把 `$HOME` 前綴摺疊回 `~`。這能讓磁碟上的設定檔跨機器保持可攜性 — 適用於透過 dotfiles（stow、chezmoi、yadm、bare git repo）共享設定的情境。

```yaml
preserve_tilde_on_save: true
```

**預設值：** `false`（既有行為不變 — 路徑會以絕對路徑儲存）。

若沒有這個旗標，每次儲存都會把 `~/...` 路徑重寫成 `/home/alice/...`（展開後的形式）。當設定檔受版本控制並跨機器共享時，這會造成雜亂的 diff 並破壞可攜性。

啟用此旗標後，序列化的 YAML 會為任何位於 `$HOME` 之下的路徑使用 `~`：

```yaml
# 之前（預設）：絕對路徑，機器特定
source: /home/alice/.config/skillshare/skills
targets:
  claude:
    skills:
      path: /home/alice/.claude/skills

# 之後（preserve_tilde_on_save: true）：可攜
source: ~/.config/skillshare/skills
targets:
  claude:
    skills:
      path: ~/.claude/skills
```

記憶體中的設定不受影響 — `Load()` 仍然會照常展開 `~`。非 home 目錄下的絕對路徑（例如 `/opt/shared/skills`）則會原封不動地傳遞。

:::note 僅限 Global mode
此選項只適用於 Global 的 `config.yaml`。Project 設定（`.skillshare/config.yaml`）通常使用相對路徑，不需要摺疊 tilde。
:::

### `git_root` {#git-root}

選擇 `skillshare commit`、`push` 和 `pull` 操作的目錄。

```yaml
git_root: skills
```

| 值 | 受版本控制的目錄 |
|-------|---------------------|
| `skills`（預設） | Skill Source（`~/.config/skillshare/skills/`） |
| `agents` | Agent Source（`~/.config/skillshare/agents/`） |
| `extras` | Extras Source（`~/.config/skillshare/extras/`） |
| `root` | 設定根目錄（`~/.config/skillshare/`）— Skill + Agent + Extras 合併在同一個儲存庫中；`config.yaml` 會自動被忽略 |

**預設值：** `skills`

可在 init 時透過 `skillshare init --git-root <scope>` 設定，或在 init 摘要中選 **Change settings** 修改。

#### init 之後變更 scope

在已經初始化完成的環境上，以非互動方式切換 scope：

```bash
skillshare init --git-root <scope>   # Global mode；如果你的 cwd 是一個專案，加上 -g
```

這會在新的 scope 目錄上初始化一個 git 儲存庫（如果那裡已經有一個則重複使用），把 `git_root` 寫入設定，且不會提示或要求 `--remote`。不過，它**不會**搬移既有的儲存庫 — 切換 scope 的意思是「開始為另一個目錄建立版本控制」，而不是「搬移歷史紀錄」：

- **全新歷史紀錄** — `skillshare init --git-root <scope>` 會在新的 scope 目錄上初始化一個空的儲存庫。
- **保留歷史紀錄** — 先執行 `mv <old-scope>/.git <new-scope>/.git`，再執行 `skillshare init --git-root <scope>` 來記錄這個 scope。

你也可以直接編輯 `config.yaml` 中的 `git_root`。如果 `git_root` 指向一個沒有儲存庫的目錄，而另一個 scope 目錄卻有，`commit`/`push`/`pull` 會印出「Git root mismatch」錯誤，並附上確切的 `skillshare init` / `mv` 指令來解決問題。

:::note 僅限 Global mode
`git_root` 只適用於 Global mode。Project mode 使用 `.skillshare/` 目錄，不支援此欄位。
:::

---

## Project 設定

**位置：** `.skillshare/config.yaml`（在專案根目錄下）

Project 設定使用與 Global 設定不同的格式。

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/runkids/skillshare/main/schemas/project-config.schema.json
# Targets — 字串或物件形式
targets:
  - claude                    # 字串：已知的 Target，使用預設值
  - cursor
  - name: custom-ide               # 物件：自訂路徑與模式
    path: ./tools/ide/skills
    mode: symlink
  - name: codex                    # 帶有篩選條件的物件
    include: [codex-*]
    exclude: [codex-experimental-*]

# 遠端 Skill — 由 install/uninstall 自動管理
skills:
  - name: pdf
    source: anthropic/skills/pdf
  - name: _team-skills
    source: github.com/team/skills
    tracked: true                  # 連同 git 歷史紀錄一起 clone

# Audit — 欄位與 Global 相同
audit:
  block_threshold: HIGH
  profile: strict
```

### `targets`（Project）

支援兩種 YAML 形式：

| 形式 | 範例 | 使用時機 |
|------|---------|--------------|
| **字串** | `- claude` | 已知的 Target，使用預設路徑與 merge 模式 |
| **物件** | `- name: x, path: ..., mode: ..., include: [...], exclude: [...]` | 自訂路徑、覆寫模式，或個別 Target 的篩選條件 |

物件項目也可以設定 [`instructions`](#target-instructions)，路徑要相對於專案根目錄。

### `skills`（Project）

與 [Global 的 `skills` 欄位](#skills) 使用相同的結構。由 `skillshare install -p` 與 `skillshare uninstall -p` 自動管理。

:::tip 可攜清單
`config.yaml` 在 Global 與 Project mode 下都是可攜的 Skill 清單 — 在新機器上執行 `skillshare install && skillshare sync`（或在專案中執行 `skillshare install -p`），即可重現相同的設定。
:::

---

## 管理設定

### 檢視目前的設定

```bash
skillshare status
# 顯示 source、targets、模式
```

### 直接編輯設定

```bash
# 在編輯器中開啟
$EDITOR ~/.config/skillshare/config.yaml

# 然後同步以套用變更
skillshare sync
```

### 重設設定

```bash
rm ~/.config/skillshare/config.yaml
skillshare init
```

---

## 自訂稽核規則

**位置：**

| 模式 | 路徑 |
|------|------|
| Global | `~/.config/skillshare/audit-rules.yaml` |
| Project | `.skillshare/audit-rules.yaml` |

規則會依序合併：**內建 → Global → Project**。你可以新增規則、停用內建規則，或覆寫嚴重程度。

```yaml
rules:
  # 新增自訂規則
  - id: flag-todo
    severity: MEDIUM
    pattern: todo-comment
    message: "TODO comment found"
    regex: '(?i)\bTODO\b'

  # 停用內建規則
  - id: insecure-http-0
    enabled: false
```

| 欄位 | 必要 | 說明 |
|-------|----------|--------------|
| `id` | 是 | 唯一的規則識別碼 |
| `severity` | 是 | `CRITICAL`、`HIGH`、`MEDIUM`、`LOW`、`INFO` |
| `pattern` | 是 | 樣式分類名稱 |
| `message` | 是 | 給人閱讀的發現說明 |
| `regex` | 是 | 用於比對的正規表示式 |
| `exclude` | 否 | 當該行也符合此正規表示式時，抑制此比對結果 |
| `enabled` | 否 | 設為 `false` 可停用一個內建規則 |

要建立起始範本檔案：

```bash
skillshare audit --init-rules       # Global
skillshare audit --init-rules -p    # Project
```

完整細節請參閱 [audit 指令](/docs/reference/commands/audit)。

---

## 環境變數

| 變數 | 說明 |
|----------|-------------|
| `SKILLSHARE_CONFIG` | 覆寫設定檔路徑 |
| `GITHUB_TOKEN` | 用於解決 API 速率限制問題 |

**範例：**
```bash
SKILLSHARE_CONFIG=~/custom-config.yaml skillshare status
```

---

## Skill 中繼資料

當你安裝一個 Skill 時，skillshare 會把它的中繼資料記錄在集中式的 `.metadata.json` 檔案中：

```json
{
  "skills": [
    {
      "name": "pdf",
      "source": "anthropics/skills/skills/pdf",
      "type": "github",
      "installed_at": "2026-01-20T15:30:00Z",
      "repo_url": "https://github.com/anthropics/skills.git",
      "subdir": "skills/pdf",
      "version": "abc1234"
    }
  ]
}
```

每個 Skill 項目包含：

| 欄位 | 說明 |
|-------|-------------|
| `name` | Skill 目錄名稱 |
| `source` | 原始的安裝來源輸入值 |
| `type` | Source 類型（`github`、`local` 等） |
| `installed_at` | 安裝時間戳記 |
| `repo_url` | Git clone URL（僅限 git Source） |
| `subdir` | 子目錄路徑（僅限單一儲存庫內含多個套件的 Source） |
| `version` | 安裝時的 git commit hash |

`skillshare update` 與 `skillshare check` 會用它來得知該從哪裡取得更新。

**請勿手動編輯這個檔案。**

---

## 平台差異

### macOS / Linux

```yaml
source: ~/.config/skillshare/skills
targets:
  claude:
    path: ~/.claude/skills
```

使用 symlink。

### Windows

```yaml
source: %AppData%\skillshare\skills
targets:
  claude:
    path: %USERPROFILE%\.claude\skills
```

資料夾以 NTFS junction 連結（不需要系統管理員權限）。單一檔案（`merge` 模式的 agents 與目錄型 extras，以及單一檔案 extras）需要檔案 symlink，而這需要開發人員模式；沒有開啟時會改為複製。請參閱 [Windows 疑難排解](/docs/troubleshooting/windows#file-links-need-windows-developer-mode-copying-instead)。

---

## 相關文件

- [Source 與 Targets](/docs/understand/source-and-targets) — 核心概念
- [Sync 模式](/docs/understand/sync-modes) — Merge vs copy vs symlink
