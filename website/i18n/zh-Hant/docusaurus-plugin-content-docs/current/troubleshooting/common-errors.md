---
sidebar_position: 2
---

# 常見錯誤

錯誤訊息與其解決方法。

## 設定錯誤

### `config not found: run 'skillshare init' first`

**原因：** 沒有設定檔存在。

**解決方法：**
```bash
skillshare init
```

如果想要自訂路徑，加上 `--source`：
```bash
skillshare init --source ~/my-skills
```

---

### `failed to load project config: ...`

**原因：** `.skillshare/config.yaml` 存在，但無法解析（YAML 格式錯誤、型別錯誤等）。有變更行為的指令（`uninstall`、`new`、`enable`/`disable`、`check`）在此狀態下會拒絕執行，以避免在你設定了自訂 `sources` 時，意外動到預設的 `.skillshare/skills/` 目錄。

**解決方法：** 修正 YAML 後重新執行指令。常見問題：

```yaml
# 錯誤 — targets 必須是清單
targets: {}

# 正確
targets: []
```

```yaml
# 錯誤 — skills 必須是清單
skills: my-skill

# 正確
skills:
  - name: my-skill
    source: github.com/org/my-skill
```

用任何 YAML linter 驗證檔案，或如果有備份，可暫時從 `.skillshare/backups/` 還原。

---

### `target "<name>": skills target path X overlaps skills source Y`

**原因：** 你的 `sources.skills` 解析出的目錄，與某個 Target 的 Skill 路徑相同（或其中一個包含另一個）。例如，將 `sources.skills: .claude/skills` 與 `claude` Target 一起設定 — 兩者都指向 `.claude/skills/`。若沒有這個防護機制，`sync --force` 會把 Source 當成 Target 目錄，進而刪除其內容。

**解決方法：** 選擇一個不會與任何 Target 重疊的 Source 路徑。常見的安全選擇：

```yaml
# 與專案文件放在一起
sources:
  skills: ./docs/skills

# 保持在 .skillshare/ 下（預設 — 完全移除 sources 這個 key）
```

同樣的檢查也適用於 `sources.agents` 與 Agent Target 路徑之間。

---

## Target 錯誤

### `target add: path does not exist`

**原因：** Skill 目錄尚不存在。

**解決方法：**
```bash
mkdir -p ~/.myapp/skills
skillshare target add myapp ~/.myapp/skills
```

### `target path does not end with 'skills'`

**原因：** 提醒路徑不符合慣例。

**解決方法：** 這只是警告，不是錯誤。如果路徑是故意這樣設定，可以繼續，或修正它：
```bash
skillshare target add myapp ~/.myapp/skills  # 建議做法
```

### `target directory already exists with files`

**原因：** Target 已有可能被覆寫的既有檔案。

**解決方法：**
```bash
skillshare backup
skillshare sync
```

---

## Sync 錯誤

### `deleting a symlinked target removed source files`

**原因：** 你在 symlink 模式下對某個 Target 執行了 `rm -rf`。

**解決方法：**
```bash
# 如果已初始化 git
cd ~/.config/skillshare/skills
git checkout -- .

# 或從備份還原
skillshare restore <target>
```

**預防方法：** 使用 `skillshare target remove` 而非手動刪除。

### `sync` 一直顯示相同的變更 {#sync-keeps-showing-the-same-changes}

**原因：** 兩個 Target 把 Skill 同步到同一個資料夾，但 `include` 或 `exclude` filters 不同。每次同步都會加入其中一個 Target 要的 Skill，同時移除另一個 Target 過濾掉的 Skill，所以資料夾永遠不會穩定下來。`sync` 會指出是哪兩個 Target：

```
! codex and universal sync skills to ~/.agents/skills with different filters, so each sync undoes the other
  keep one: skillshare target codex --skills=false
```

**解決方法：** 讓其中一個 Target 負責寫入這個資料夾，並關閉另一個 Target 的 Skill 同步。它的 agents、MCP servers 與 instructions 仍由 skillshare 管理，而且該工具仍會讀取共用資料夾中的 Skill：

```bash
skillshare target codex --skills=false --dry-run
skillshare target codex --skills=false
```

在 dashboard 中，**Sync** 頁面會顯示相同的警告，並附上一個按鈕，可停止該 Target 的 Skill 同步。讓兩個 Target 使用相同的 filters 也可以解決。

### `sync seems stuck or slow`

**原因：** Skill 目錄中有大型檔案。

**解決方法：** 新增忽略樣式：
```yaml
# ~/.config/skillshare/config.yaml
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
  - "**/node_modules/**"
```

### `no space left on device` / `ENOSPC`（同步期間）

**原因：** 有東西正在佔滿磁碟空間。先檢查備份目錄，再檢查你的 Source。

**解決方法：**
```bash
df -h ~                                     # 確認磁碟已滿
du -sh ~/.local/share/skillshare/backups    # 備份用量
du -sh ~/.config/skillshare/skills          # Source 用量
```

如果備份很大，把它們清掉 — 保留策略會在每次 `sync` 之後自動執行，但在那之前累積出來的目錄可以按需求清除：

```bash
skillshare backup --cleanup --dry-run   # 預覽
skillshare backup --cleanup
```

如果磁碟被固定在 100% 使用率，`rm` 可能會因「Permission denied」而失敗，直到釋放出一點空間為止。先刪除一個大檔案，再進行清理。

如果是 **Source** 很大，代表這些產出物就在你的 Skill 裡面。備份不會複製它們（symlink 的 Skill 會被跳過），但每個以 copy 模式運作的 Target 都會複製。把 runtime 快取、模型權重、瀏覽器 profile 移到 Skill 樹狀結構之外，或用 `ignore:` 排除它們。

備份範圍與 `.gitignore` 及 `ignore:` 的差異，請參閱[備份與磁碟空間](/docs/reference/commands/backup#backups--disk-space)。

---

## Git 錯誤

### `Could not read from remote repository`

**原因：** SSH 金鑰未設定，或 remote URL 錯誤。

**解決方法：**
```bash
# 檢查 SSH 存取
ssh -T git@github.com

# 如果沒有設定 SSH，改用 HTTPS
git -C ~/.config/skillshare/skills remote set-url origin https://github.com/you/my-skills.git

# 或設定 SSH 金鑰
ssh-keygen -t ed25519 -C "you@example.com"
# 然後把公開金鑰加到 GitHub → Settings → SSH keys
```

### `push: remote has changes`

**原因：** Remote 儲存庫領先於本地。

**解決方法：**
```bash
skillshare pull   # 先取得 remote 的變更
skillshare push   # 現在可以 push 了
```

### `pull: local has uncommitted changes`

**原因：** 你有尚未 push 的本地變更。

**解決方法：**
```bash
# 選項 1：先 push 你的變更
skillshare push -m "Local changes"
skillshare pull

# 選項 2：捨棄本地變更
cd ~/.config/skillshare/skills
git checkout -- .
skillshare pull
```

### `pull stopped: this machine and the remote both changed ...`

**原因：** 同一個檔案在兩台機器上被編輯過。`pull` 已復原 merge，所以 repository 沒有變動。只有 `.metadata.json` 衝突時不會出現這個錯誤，那類衝突會自動解決。

**解決方法：**
```bash
cd ~/.config/skillshare/skills
git pull --no-rebase          # 重新 merge 並保留衝突
# 編輯衝突的檔案
git add .
git commit --no-edit
skillshare push
skillshare sync
```

### `Git had no identity`

**原因：** `skillshare init` 建立 source repo 時，git 沒有 `user.name` / `user.email`。skillshare 把備援值（`skillshare@local`）寫進該 repo 自己的設定，讓 commit 能正常進行。repo 的設定優先於 `git config --global`，所以之後設定 global 身分並不會取代它。

你自己建立的 repo 不會被改動：skillshare 只在那一次初始 commit 使用備援值。

**解決方法：** 在該 repo 設定你的身分（路徑以訊息印出的為準，以下是預設值）：
```bash
git -C ~/.config/skillshare/skills config user.name "Your Name"
git -C ~/.config/skillshare/skills config user.email "you@example.com"
```

或移除 repo 內的設定，改用 global 身分：`git -C ~/.config/skillshare/skills config --unset user.name`（`user.email` 同理）。

### `Git root mismatch`

**原因：** `config.yaml` 中的 `git_root` 指向一個沒有 git 儲存庫的 scope 目錄，但另一個 scope 目錄卻有。這通常發生在你變更 `git_root` 卻沒有搬移儲存庫時 — 切換 scope 的意思是「開始為另一個目錄建立版本控制」，而不是「搬移既有的歷史紀錄」。詳見 [`git_root`](/docs/reference/targets/configuration#git-root)。

**解決方法：** 從錯誤訊息印出的三個選項中選一個：
```bash
# 在設定的 scope 上開始一個全新的儲存庫（沒有歷史紀錄）
skillshare init --git-root <scope>

# 搬移既有的儲存庫，保留歷史紀錄
mv <old-scope>/.git <new-scope>/.git

# 或繼續使用既有的儲存庫：把 git_root 改回 config.yaml 中的值
#   git_root: <scope-that-has-the-repo>
```

### `tracked repository clone is missing`

**原因：** `.metadata.json` 中宣告了一個 Tracked repo，但本地的 clone 目錄（例如 `skills/_team-skills/`）不存在。這通常發生在你於新機器上 clone 你的 skillshare Source 儲存庫之後，因為 Tracked repo 目錄有意被列在受管理的 `.gitignore` 區塊中。

**解決方法：** 從中繼資料重新產生遺失的 Tracked repo clone：
```bash
skillshare install
skillshare sync
```

在 Project mode 下：
```bash
skillshare install -p
skillshare sync -p
```

`status`、`check`、`update --all` 和 `doctor` 都會回報這個狀態，並建議執行 `skillshare install`。

### `nested git repositories must be disabled first`

**原因：** 在 `git_root: root` 的情況下，某個子目錄（例如 `skills/_org/` 底下 Tracked 的 Skill 儲存庫）有自己的 `.git`。Git 會把它當成一個**空的 submodule** 上傳，並默默丟棄其中的檔案，因此 `commit`/`push` 會中止，直到每個巢狀儲存庫都被停用為止。

**解決方法：**
```bash
# 停用每個被回報的巢狀儲存庫（可還原 — 改回原名即可重新啟用）
mv ~/.config/skillshare/<dir>/.git ~/.config/skillshare/<dir>/.git.disabled
```
或使用 Web UI 的 Git Sync 頁面上的一鍵停用功能。skillshare 也會自動讓 `config.yaml` 不進入 root-scope 儲存庫（因為它包含機器特有的路徑）。

### `Invalid git_root`

**原因：** `config.yaml` 中的 `git_root` 被設為無法辨識的值（例如拼字錯誤）。

**解決方法：** 使用 `skills`、`agents`、`extras` 或 `root` 其中之一 — 或留空（預設為 `skills`）。

---

## 安裝錯誤

### `skill already exists`

**原因：** 已安裝同名的 Skill。

**解決方法：**
```bash
# 更新既有的 Skill
skillshare install <source> --update

# 或強制覆寫
skillshare install <source> --force
```

### `git failed (exit 128): repository not found or authentication required`

**原因：** 儲存庫 URL 錯誤、儲存庫不存在，或缺少身分驗證。

skillshare 現在會針對常見的 git 失敗提供可行動的錯誤訊息，而非原始的結束碼。錯誤訊息中會包含建議：

```
Error: git failed (exit 128): repository not found or authentication required
```

如果使用了 token 但被拒絕：

```
Error: git failed (exit 128): authentication token was rejected — check permissions and expiry
```

**解決方法：** 請參閱下方的驗證選項。

### `Authentication failed` / `Access denied`

**原因：** HTTPS 憑證遺失、過期，或 token 類型錯誤。

**解決方法 — 選項 1：設定 token 環境變數：**

```bash
# GitHub
export GITHUB_TOKEN=ghp_xxxx

# GitLab（必須是 Personal Access Token，前綴為 glpat-）
export GITLAB_TOKEN=glpat-xxxx

# Bitbucket
export BITBUCKET_TOKEN=your_app_password
```

**Windows（PowerShell）：**
```powershell
$env:GITLAB_TOKEN = "glpat-xxxx"

# 永久設定（重新啟動後仍有效）
[Environment]::SetEnvironmentVariable("GITLAB_TOKEN", "glpat-xxxx", "User")
```

**解決方法 — 選項 2：使用 SSH URL：**
```bash
skillshare install git@github.com:team/private-skills.git
skillshare install git@gitlab.com:team/skills.git
skillshare install git@bitbucket.org:team/skills.git
```

**解決方法 — 選項 3：Git credential helper：**
```bash
gh auth login          # GitHub CLI
git credential approve # 或平台專屬的憑證管理工具
```

**所需的 token 權限：**

| 平台 | Token 類型 | 範圍／權限 |
|----------|-----------|---------------------|
| GitHub | Personal Access Token（`ghp_`） | `repo`（私人儲存庫），無需權限（公開儲存庫） |
| GitLab | Personal Access Token（`glpat-`） | `read_repository` + `write_repository` |
| Bitbucket | Repository Access Token | Read + Write |
| Bitbucket | App Password + `BITBUCKET_USERNAME` | Repositories: Read + Write |

:::warning GitLab token types
只有 **Personal Access Token**（`glpat-`）能用於 git 操作。Feed Tokens（`glft-`）**沒有** git 存取權限。
:::

請參閱[環境變數](/docs/reference/appendix/environment-variables#git-authentication)與[私人儲存庫](/docs/reference/commands/install#private-repositories)。

### `SSL certificate problem` / `certificate verification failed`

**原因：** Git 伺服器使用自簽憑證，或你的系統不信任的內部 CA。常見於自架的 GitLab、Gitea 或 Gogs 實例。

**解決方法 — 選項 1：自訂 CA bundle（建議）：**
```bash
export GIT_SSL_CAINFO=/path/to/company-ca-bundle.crt
skillshare install https://gitlab.internal.company.com/team/skills.git --track
```

**解決方法 — 選項 2：改用 SSH（完全避開 SSL）：**
```bash
skillshare install git@gitlab.internal.company.com:team/skills.git --track
```

**解決方法 — 選項 3：停用 SSL 驗證（不建議）：**
```bash
GIT_SSL_NO_VERIFY=true skillshare install https://gitlab.internal.company.com/team/skills.git --track
```

:::warning
停用 SSL 驗證是一項安全風險。請優先選擇選項 1 或 2。
:::

請參閱[環境變數 — Git SSL / TLS](/docs/reference/appendix/environment-variables#git-ssl--tls)。

### `invalid skill: SKILL.md not found`

**原因：** Source 沒有有效的 SKILL.md 檔案。

**解決方法：** 確認 Source 路徑正確，並指向一個 Skill 目錄。

---

## 更新錯誤

### `pull stopped: this machine and the remote both changed ...`（Tracked repository）

**原因：** Tracked repository 的本地 commits 與 remote 衝突。`update` 會合併已分歧的歷史，但只要有檔案衝突就會停止，並復原 merge。

**解決方法：**
```bash
# 強制更新（以 remote 取代本地）
skillshare update --force

# 或手動解決
cd ~/.config/skillshare/skills/_repo-name
git pull --no-rebase
# 編輯衝突的檔案，然後
git add . && git commit --no-edit
```

:::tip
`skillshare update` 與 `skillshare install` 現在會針對 git 失敗（身分驗證、SSL、分歧分支）提供可行動的錯誤訊息，而非原始的結束碼。
:::

---

## 稽核錯誤

### `security audit failed — critical threats detected`

**原因：** 該 Skill 包含符合重大安全威脅的樣式（Prompt injection、資料外洩、憑證存取）。

**解決方法：**
```bash
# 檢視發現的問題
skillshare audit <skill-name>

# 如果你信任此 Source，強制安裝
skillshare install <source> --force
```

### `audit HIGH: Hidden zero-width Unicode characters detected`

**原因：** 該 Skill 包含隱藏的 Unicode 字元，可能是複製貼上留下的痕跡，也可能是刻意的混淆手法。

**解決方法：** 用能顯示隱藏字元的編輯器開啟檔案並移除它們，或如果信任此 Source，強制安裝。

---

## 升級錯誤

### `GitHub API rate limit exceeded`

**原因：** 未驗證身分的 API 請求過多。

**解決方法：**
```bash
# 選項 1：設定 GitHub token（建議）
export GITHUB_TOKEN=ghp_your_token_here
skillshare upgrade

# 選項 2：強制升級
skillshare upgrade --cli --force
```

在此建立 token：https://github.com/settings/tokens （公開儲存庫不需要任何 scope）

---

## Skill 錯誤

### `skill not appearing in AI CLI`

**原因：**
1. Skill 尚未同步
2. SKILL.md 格式無效
3. AI CLI 快取

**解決方法：**
```bash
# 1. 同步
skillshare sync

# 2. 檢查格式
skillshare doctor

# 3. 重新啟動 AI CLI
```

### Antigravity 無法載入已同步的 Skill {#antigravity-does-not-load-synced-skills}

**原因：** Antigravity app 的 Skill 掃描器只會偵測**真實目錄** — 它會跳過 symlink。skillshare 預設的 `merge` 模式會為每個 Skill 建立一個 symlink（在 Windows 上是 NTFS junction），因此沒有任何一個會被偵測到。在 Windows 上，這會出現 `Incorrect function` 錯誤；在 macOS 與 Linux 上，Skill 則會靜默地消失不見。

這是 Antigravity 端的限制，而非 skillshare 的臭蟲。此限制只影響 `antigravity` target（app，`~/.gemini/config/skills`）；獨立的 `agy` CLI 是另一個 `antigravity-cli` target，讀取 `~/.gemini/antigravity-cli/skills`。有兩種解決方法：

**選項 1 — 將 Target 切換為 `copy` 模式**

```bash
skillshare target antigravity --mode copy
skillshare sync --force
```

會寫入真實目錄取代 symlink。取捨：編輯 Source Skill 後需要重新執行 `skillshare sync`。

**選項 2 — 讓 Antigravity 指向你的 Source 目錄**

在 Antigravity 中：**Settings → Customizations → Skill Custom Paths → 「+ Add」**，然後輸入你的 skillshare Source **絕對路徑**（例如 `/Users/you/.config/skillshare/skills`）。`~` 這種簡寫不會被展開，因此需要完整路徑。

無論用哪種方法，都要重新啟動 Antigravity 以重新載入 Skill。

### `skill name 'X' is defined in multiple places`

**原因：** 多個 Skill 有相同的 `name` 欄位，且會落在同一個 Target 上。

**解決方法：** 在 SKILL.md 中重新命名其中一個，或使用 `include`/`exclude` 篩選條件，將它們導向不同的 Target：
```yaml
# 選項 1：在 SKILL.md 中命名空間化
name: team-a-skill-name

# 選項 2：透過篩選條件路由（Global 設定）
targets:
  codex:
    path: ~/.codex/skills
    include: [_team-a__*]
  claude:
    path: ~/.claude/skills
    include: [_team-b__*]

# 選項 2：透過篩選條件路由（Project 設定）
targets:
  - name: claude
    exclude: [codex-*]
  - name: codex
    include: [codex-*]
```

:::tip
如果篩選條件已經隔離了重複項目，sync 會顯示資訊訊息而非警告 — 無需採取行動。
完整語法請參閱 [Target 篩選條件](/docs/reference/targets/configuration#include--exclude-target-filters)。
:::

---

## Agent 錯誤

### 警告：`No agents folder: <targets>`

**原因：** 你執行了 `skillshare sync`（或 `skillshare sync agents`），而一個或多個已設定的 Target 沒有定義 Agent 目錄。只有 Claude、Cursor、Augment 和 OpenCode 有內建的 Agent 路徑；其他 Target 會被靜默跳過。

**解決方法：**

1. 如果這些 Target 不需要 Agent，可以忽略此警告。
2. 在 `config.yaml` 中為該 Target 新增 `agents:` 子欄位，為它啟用 Agent 同步：

```yaml
targets:
  myapp:
    path: ~/myapp/skills
    agents:
      path: ~/myapp/agents
```

然後重新執行 `skillshare sync agents`。

### `backup is not supported in project mode (except for agents)`

**原因：** 你在沒有加上 `agents` 篩選條件的情況下，執行了 `skillshare backup -p`（或 `skillshare backup -p <target>`）。在 Project mode 下，只支援 Agent 備份 — Skill 備份僅限於 Global mode。

**解決方法：** 加上 `agents` 位置參數，或使用 `--all`：

```bash
skillshare backup -p agents          # Project Agent Targets
skillshare backup -p agents claude   # 指定 Target
skillshare backup -p --all           # 效果相同（會縮小範圍到 Agent）
```

`restore` 也套用相同規則：`restore is not supported in project mode (except for agents)`。

### `agent name 'X' has invalid characters`

**原因：** Agent 的檔名或 `name:` frontmatter 欄位包含允許字元集之外的字元。

**解決方法：** Agent 名稱只能使用 `a-z`、`0-9`、`_`、`-`、`.`。重新命名檔案（並同步更新其 `name:` 欄位），讓兩者共用同一個標準名稱。

### `.agentignore` 樣式沒有生效

**原因：**

1. 檔案位置錯誤。它必須放在 Agent Source 根目錄下：`~/.config/skillshare/agents/.agentignore`（Global）或 `.skillshare/agents/.agentignore`（Project）。
2. 你的樣式比對到了非預期的部分 — 此檔案使用 [gitignore 語法](https://git-scm.com/docs/gitignore)。

**解決方法：** 用 `skillshare doctor` 確認檔案路徑，並重新檢查樣式。Agent 是以基本檔名（不含 `.md`）進行比對的，所以 `draft-*` 會比對到 `draft-experiment.md`。使用 `skillshare disable <agent> --kind agent` 讓 CLI 幫你寫入該項目。

---

## Plugin 錯誤 {#plugin-errors}

### `<agent> CLI is not installed or not on PATH` {#plugin-cli-not-on-path}

**原因：** Plugin 指令會在執行 Skillshare 的機器上呼叫 Agent 的原生 CLI（`claude`、`codex` 等），但找不到這個 CLI。排程工作或由服務啟動的 dashboard，`PATH` 通常比你的終端機短。

**解決方法：**

1. 在那台機器上安裝 Agent 的 CLI。
2. 如果已經安裝，把它所在的目錄加到啟動 Skillshare 的程式的 `PATH`，例如排程工作的環境變數。
3. 若是 [account target](/docs/reference/commands/plugin#accounts)，也可以把 `cli` 設成執行檔的絕對路徑。

### `Codex CLI not found on the machine running Skillshare` {#plugin-codex-not-found}

**原因：** Skillshare 在 `PATH`、Homebrew 的資料夾和 Codex 桌面 app 裡找 Codex CLI，都沒有找到。訊息會列出找過的每個位置。

**解決方法：**

1. 在那台機器上安裝 Codex app 或 Codex CLI。
2. 如果 Codex 裝在其他地方，在啟動 Skillshare 的環境（例如排程工作）把 [`SKILLSHARE_CODEX_CLI`](/docs/reference/appendix/environment-variables#skillshare_codex_cli) 設成它的路徑。這個設定只留在那台機器，不會影響和其他作業系統共用的 config。

### `The native marketplace X is gone` {#plugin-marketplace-gone}

**原因：** 這個 plugin 是匯入的，所以 Skillshare 會從它原本的原生 marketplace 重新安裝，但 Agent 裡沒有註冊這個 marketplace。這在第二台機器上很常見：匯入只記錄在第一台機器上。

**解決方法：**

1. 在 Agent 中重新加入該 marketplace，然後執行 `skillshare sync plugins`。
2. 或從這個 plugin 移除該 Agent，再從 source 重新加入 plugin。請參考 [跨機器 Sync — Plugins](/docs/how-to/sharing/cross-machine-sync#plugins)。

---

## 二進位檔錯誤

### `integration tests cannot find the binary`

**原因：** 二進位檔未建置，或路徑錯誤。

**解決方法：**
```bash
go build -o bin/skillshare ./cmd/skillshare
# 或設定
export SKILLSHARE_TEST_BINARY=/path/to/skillshare
```

---

## 還有問題嗎？

請參閱[疑難排解流程](./troubleshooting-workflow.md)，了解系統性的除錯方法。
