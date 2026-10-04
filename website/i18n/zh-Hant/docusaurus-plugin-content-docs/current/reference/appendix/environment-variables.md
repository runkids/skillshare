---
sidebar_position: 2
---

# 環境變數

skillshare 所辨識的所有環境變數。

## 設定

### SKILLSHARE_CONFIG

覆寫設定檔路徑。

```bash
SKILLSHARE_CONFIG=~/custom-config.yaml skillshare status
```

**預設值：** `~/.config/skillshare/config.yaml`

---

### XDG_CONFIG_HOME

依照 [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir/latest/) 覆寫基礎設定目錄。

```bash
export XDG_CONFIG_HOME=~/my-config
# skillshare 將會使用 ~/my-config/skillshare/
```

**預設行為：**

| Platform | Default |
|----------|---------|
| Linux | `~/.config/skillshare/` |
| macOS | `~/.config/skillshare/` |
| Windows | `%AppData%\skillshare\` |

**優先順序：** `SKILLSHARE_CONFIG` > `XDG_CONFIG_HOME` > 平台預設值。

:::note
如果你是在初次設定完成後才設定 `XDG_CONFIG_HOME`，請手動將現有的 `~/.config/skillshare/` 目錄搬移到新位置。
:::

---

### XDG_DATA_HOME

覆寫資料目錄（backups、trash）。

```bash
export XDG_DATA_HOME=~/my-data
# skillshare 將會使用 ~/my-data/skillshare/backups/ 與 ~/my-data/skillshare/trash/
```

**預設值：** `~/.local/share/skillshare/`

---

### XDG_STATE_HOME

覆寫 state 目錄（操作記錄）。

```bash
export XDG_STATE_HOME=~/my-state
# skillshare 將會使用 ~/my-state/skillshare/logs/
```

**預設值：** `~/.local/state/skillshare/`

---

### XDG_CACHE_HOME

覆寫快取目錄（版本檢查快取、UI dist 快取）。

```bash
export XDG_CACHE_HOME=~/my-cache
# skillshare 將會使用 ~/my-cache/skillshare/
```

**預設值：** `~/.cache/skillshare/`

:::tip 自動遷移
從 v0.13.0 開始，skillshare 針對 backups、trash 與 logs 遵循 XDG Base Directory Specification。如果你是從舊版本升級，這些目錄會在第一次執行時自動從 `~/.config/skillshare/` 遷移到正確的 XDG 位置。
:::

---

### SKILLSHARE_GITLAB_HOSTS

以逗號分隔的自管 GitLab 主機名稱清單，這些主機會以支援巢狀 subgroup 的方式處理。適合用在沒有設定檔的 CI/CD 環境。

```bash
SKILLSHARE_GITLAB_HOSTS=git.company.com,code.internal.io skillshare install git.company.com/team/frontend/ui
```

當環境變數與設定檔中的 [`gitlab_hosts`](/docs/reference/targets/configuration#gitlab_hosts) 同時設定時，兩者的值會被**合併**（並去除重複）。無效的項目（包含 scheme、路徑、port，或空白）會被靜默略過。

**預設值：** 無（只會使用設定檔中的值）

---

### SKILLSHARE_AZURE_HOSTS

以逗號分隔的自架 Azure DevOps Server 主機名稱清單。

```bash
SKILLSHARE_AZURE_HOSTS=azuredevops.mycompany.com,tfs.internal.io
```

當環境變數與設定檔中的 [`azure_hosts`](/docs/reference/targets/configuration#azure_hosts) 同時設定時，兩者的值會被**合併**（並去除重複）。環境變數中的無效項目會被靜默略過。

**預設值：** _（無）_

### SKILLSHARE_GITEA_HOSTS

以逗號分隔的自架 Gitea 主機名稱清單。名稱中含有 `gitea` 的主機不需要額外設定即可被偵測到。

```bash
SKILLSHARE_GITEA_HOSTS=git.company.com,code.internal.io
```

會與設定檔中的 [`gitea_hosts`](/docs/reference/targets/configuration#gitea_hosts) 合併。

**預設值：** _（無）_

### SKILLSHARE_CNB_HOSTS

以逗號分隔的自架 CNB 主機名稱清單。`cnb.cool` 不需要額外設定即可被偵測到。

```bash
SKILLSHARE_CNB_HOSTS=cnb.company.com
```

會與設定檔中的 [`cnb_hosts`](/docs/reference/targets/configuration#cnb_hosts) 合併。

**預設值：** _（無）_

---

## Web UI

### SKILLSHARE_UI_BASE_PATH

設定在 reverse proxy 後方提供 web dashboard 服務時所使用的 URL 子路徑。

```bash
SKILLSHARE_UI_BASE_PATH=/skillshare skillshare ui --host 0.0.0.0 --no-open
```

等同於 `--base-path /skillshare`。若兩者同時設定，以 flag 為準。

**預設值：** 無（dashboard 會在根路徑 `/` 提供服務）

參見 [Reverse Proxy](/docs/reference/commands/ui#reverse-proxy)，取得 Nginx 與 Caddy 的範例。

---

## GitHub API

### GITHUB_TOKEN

GitHub 個人存取權杖（personal access token）。

**用途：**
- GitHub API 請求（`skillshare search`、`skillshare upgrade`、版本檢查）
- **Git clone 驗證** — 透過 HTTPS 安裝私有 repo 時會自動注入

**建立權杖：**
1. 前往 https://github.com/settings/tokens
2. 產生新的 token（classic）
3. 範圍：私有 repo 選 `repo`，公開 repo 不需要
4. 複製該 token

官方文件：[Managing your personal access tokens](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)

**用法：**
```bash
export GITHUB_TOKEN=ghp_your_token_here
skillshare install https://github.com/org/private-skills.git --track
```

**Windows：**
```powershell
# 目前的 session
$env:GITHUB_TOKEN = "ghp_your_token"

# 永久設定
[Environment]::SetEnvironmentVariable("GITHUB_TOKEN", "ghp_your_token", "User")
```

---

### GH_TOKEN

另一個 GitHub token 變數。由 [GitHub CLI（`gh`）](https://cli.github.com/) 使用，skillshare 也會將它視為備援來源辨識。

**Token 解析順序：** `GITHUB_TOKEN` → `GH_TOKEN` → `gh auth token`

如果你已經在使用 `gh`，並透過 `gh auth login` 完成驗證，skillshare 會自動取得該 token — 不需要額外設定環境變數。

```bash
export GH_TOKEN=ghp_your_token_here
skillshare search "react patterns"
```

---

## Git 驗證 {#git-authentication}

這些變數可讓私有 repo 啟用 HTTPS 驗證。設定後，skillshare 會在 `install` 與 `update` 期間自動注入 token — 不需要修改 URL。

參見 [Private Repositories](/docs/reference/commands/install#private-repositories)，取得細節與 CI/CD 範例。


### GITLAB_TOKEN

GitLab 個人存取權杖或 CI job token。用於以 HTTPS 方式 clone GitLab 上的私有 repo。

**建立權杖：**
1. 前往 https://gitlab.com/-/user_settings/personal_access_tokens
2. 新增權杖
3. 範圍：`read_repository`（僅 pull）或 `read_repository` + `write_repository`（push 與 pull）
4. 複製該 token（前綴 `glpat-`）

官方文件：[Token overview](https://docs.gitlab.com/security/tokens/)

:::warning Token 類型
只有 **Personal Access Token**（`glpat-`）與 Project/Group Access Token 能用於 git 操作。Feed Token（`glft-`）**沒有** git 存取權限。
:::

```bash
export GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx
skillshare install https://gitlab.com/org/skills.git --track
```

**Windows：**
```powershell
$env:GITLAB_TOKEN = "glpat-xxxxxxxxxxxxxxxxxxxx"
```

### BITBUCKET_TOKEN

Bitbucket repository access token，或 app password。用於以 HTTPS 方式 clone Bitbucket 上的私有 repo。

**建立權杖（repository access token）：**
1. 前往 Repository → Settings → Access tokens
2. 建立權杖，權限選 Read（僅 pull）或 Read + Write（push 與 pull）
3. 複製該 token — 會自動使用 `x-token-auth`（不需要使用者名稱）

**建立權杖（app password）：**
1. 前往 https://bitbucket.org/account/settings/app-passwords/
2. 建立 app password，權限選 Repositories: Read（僅 pull）或 Repositories: Read + Write（push 與 pull）
3. 同時設定 `BITBUCKET_USERNAME`（或將它包含在 URL 中，例如 `https://<username>@bitbucket.org/...`）

官方文件：[Access tokens](https://support.atlassian.com/bitbucket-cloud/docs/access-tokens/)

```bash
export BITBUCKET_USERNAME=your_bitbucket_username
export BITBUCKET_TOKEN=your_app_password
skillshare install https://bitbucket.org/team/skills.git --track
```

**Windows：**
```powershell
$env:BITBUCKET_USERNAME = "your_bitbucket_username"
$env:BITBUCKET_TOKEN = "your_app_password"
```

### BITBUCKET_USERNAME

當 `BITBUCKET_TOKEN` 是 app password 時，搭配使用的 Bitbucket 使用者名稱。

```bash
export BITBUCKET_USERNAME=your_bitbucket_username
export BITBUCKET_TOKEN=your_app_password
skillshare install https://bitbucket.org/team/skills.git --track
```

### AZURE_DEVOPS_TOKEN

Azure DevOps Personal Access Token（PAT）。用於以 HTTPS 方式 clone Azure DevOps 上的私有 repo。

**建立權杖：**
1. 前往 `https://dev.azure.com/{org}/_usersSettings/tokens`
2. 選擇「+ New Token」
3. 範圍：Code → Read（僅 pull）或 Code → Read & Write（push 與 pull）
4. 複製該 token（84 字元、帶有 `AZDO` 簽章的字串）

官方文件：[Use Personal Access Tokens](https://learn.microsoft.com/en-us/azure/devops/organizations/accounts/use-personal-access-tokens-to-authenticate?view=azure-devops)

```bash
export AZURE_DEVOPS_TOKEN=your_pat_here
skillshare install https://dev.azure.com/org/project/_git/repo --track
```

**Windows：**
```powershell
$env:AZURE_DEVOPS_TOKEN = "your_pat_here"
```

### GITEA_TOKEN

Gitea 存取權杖。用於以 HTTPS 方式 clone `gitea.com` 與自架 Gitea 上的私有 repo，以及在安裝子目錄而不進行完整 clone 時使用 Gitea Contents API。

**建立權杖：**
1. 在 Gitea 中，前往 Settings → Applications
2. 產生新的 token
3. 權限：`repository: Read`（僅 pull）或 `repository: Read and Write`（push 與 pull）

```bash
export GITEA_TOKEN=your_token
skillshare install https://gitea.com/org/skills.git --track
```

**Windows：**
```powershell
$env:GITEA_TOKEN = "your_token"
```

此 token 只會傳送給你安裝來源的主機。若自架的執行個體位於名稱中不含 `gitea` 的網域，需要設定 [`gitea_hosts`](/docs/reference/targets/configuration#gitea_hosts)，否則會改用 `SKILLSHARE_GIT_TOKEN`。

### CNB_TOKEN

對 repository 具有讀取權限的 [CNB](https://cnb.cool) 存取權杖。用於以 HTTPS 方式 clone `cnb.cool` 與自架 CNB 上的私有 repo。

```bash
export CNB_TOKEN=your_token
skillshare install https://cnb.cool/org/skills --track
```

**Windows：**
```powershell
$env:CNB_TOKEN = "your_token"
```

部署在其他網域的私有實例需要設定 [`cnb_hosts`](/docs/reference/targets/configuration#cnb_hosts)。

### SKILLSHARE_GIT_TOKEN

適用於任何 HTTPS git 主機的通用備援 token。當沒有設定特定平台的 token 時使用。

```bash
export SKILLSHARE_GIT_TOKEN=your_token
skillshare install https://git.example.com/org/skills.git --track
```

**Windows：**
```powershell
$env:SKILLSHARE_GIT_TOKEN = "your_token"
```

**Token 優先順序：** 特定平台的 token（`GITHUB_TOKEN`、`GITLAB_TOKEN`、`BITBUCKET_TOKEN`、`AZURE_DEVOPS_TOKEN`、`GITEA_TOKEN`、`CNB_TOKEN`）> `SKILLSHARE_GIT_TOKEN`。

---

## Plugin {#plugins}

### SKILLSHARE_CODEX_CLI

[Plugin 指令](/docs/reference/commands/plugin)要執行的 Codex CLI，只對這台機器有效。Skillshare 自己找不到 Codex 時再設定。它從環境變數讀取，所以 macOS 和 Windows 共用同一份 `config.yaml` 時，不需要寫一個在另一台機器上錯誤的路徑。

沒有設定時，Skillshare 先從 `PATH` 執行 `codex`，再依序嘗試這些位置：

| 作業系統 | 位置 |
|---|---|
| macOS | `/opt/homebrew/bin/codex`、`/usr/local/bin/codex`，以及 Codex app 內附的 CLI（`/Applications` 或 `~/Applications` 裡的 `ChatGPT.app` 或 `Codex.app`） |
| Windows | 官方 PowerShell 安裝程式的 `%LOCALAPPDATA%\Programs\OpenAI\Codex\bin\codex.exe`，再來是 Codex app 安裝的 CLI：`%LOCALAPPDATA%\OpenAI\Codex\bin\<version>\codex.exe`（取最新的一份） |

```bash
export SKILLSHARE_CODEX_CLI=/Applications/ChatGPT.app/Contents/Resources/codex-cli/bin/codex
```

**Windows:**
```powershell
$env:SKILLSHARE_CODEX_CLI = "C:\Tools\codex.exe"
```

自己設了 `cli` 的 [account target](/docs/reference/commands/plugin#accounts) 仍使用那個 `cli`。

---

## Git SSL / TLS {#git-ssl--tls}

這些標準 Git 環境變數會被傳遞給所有 git 操作。對於使用自簽憑證或內部 CA 的自架 Git 伺服器（GitLab、Gitea 等）非常實用。

### GIT_SSL_CAINFO

自訂 CA 憑證組合檔的路徑。當你的自架 Git 伺服器使用由內部 CA 簽署的憑證時使用此變數。

```bash
export GIT_SSL_CAINFO=/path/to/ca-bundle.crt
skillshare install https://gitlab.internal.company.com/team/skills.git --track
```

### GIT_SSL_NO_VERIFY

完全停用 SSL 憑證驗證。僅在無法設定 CA 憑證時作為最後手段使用。

:::warning 安全風險
停用 SSL 驗證會讓連線容易受到中間人攻擊（man-in-the-middle）。建議改用搭配正確 CA 憑證組合檔的 `GIT_SSL_CAINFO`，或改用 SSH。
:::

```bash
GIT_SSL_NO_VERIFY=true skillshare install https://gitlab.internal.company.com/team/skills.git --track
```

**替代方案：改用 SSH 以完全避開 SSL：**
```bash
skillshare install git@gitlab.internal.company.com:team/skills.git --track
```

---

## 測試

### SKILLSHARE_TEST_BINARY

覆寫 integration tests 使用的 CLI 執行檔路徑。

```bash
SKILLSHARE_TEST_BINARY=/path/to/skillshare go test ./tests/integration
```

**預設值：** 專案根目錄下的 `bin/skillshare`

---

## 使用範例

### 暫時覆寫

```bash
# 單一指令
SKILLSHARE_CONFIG=/tmp/test-config.yaml skillshare status

# 多個指令
export SKILLSHARE_CONFIG=/tmp/test-config.yaml
skillshare status
skillshare list
unset SKILLSHARE_CONFIG
```

### 永久設定（macOS/Linux）

加入 `~/.bashrc` 或 `~/.zshrc`：
```bash
export GITHUB_TOKEN="ghp_your_token_here"
```

### 永久設定（Windows）

```powershell
[Environment]::SetEnvironmentVariable("GITHUB_TOKEN", "ghp_your_token", "User")
```

---

## 摘要

| Variable | Purpose | Default |
|----------|---------|---------|
| `SKILLSHARE_CONFIG` | 設定檔路徑 | `~/.config/skillshare/config.yaml` |
| `SKILLSHARE_UI_BASE_PATH` | Reverse proxy 用的 Web UI 子路徑 | 無 |
| `SKILLSHARE_GITLAB_HOSTS` | 自訂 GitLab 主機名稱（逗號分隔） | 無 |
| `SKILLSHARE_AZURE_HOSTS` | 自訂 Azure DevOps Server 主機名稱（逗號分隔） | 無 |
| `SKILLSHARE_GITEA_HOSTS` | 自訂 Gitea 主機名稱（逗號分隔） | 無 |
| `SKILLSHARE_CNB_HOSTS` | 自訂 CNB 主機名稱（逗號分隔） | 無 |
| `XDG_CONFIG_HOME` | 基礎設定目錄 | `~/.config`（Linux/macOS）、`%AppData%`（Windows） |
| `XDG_DATA_HOME` | 資料目錄（backups、trash） | `~/.local/share` |
| `XDG_STATE_HOME` | State 目錄（logs） | `~/.local/state` |
| `XDG_CACHE_HOME` | 快取目錄（版本檢查、UI） | `~/.cache` |
| `GITHUB_TOKEN` | GitHub API + git clone 驗證 | 無 |
| `GH_TOKEN` | GitHub API（`GITHUB_TOKEN` 的備援） | 無 |
| `GITLAB_TOKEN` | GitLab git clone 驗證 | 無 |
| `BITBUCKET_TOKEN` | Bitbucket git clone 驗證 | 無 |
| `BITBUCKET_USERNAME` | app password 驗證用的 Bitbucket 使用者名稱 | 無 |
| `AZURE_DEVOPS_TOKEN` | Azure DevOps git clone 驗證 | 無 |
| `GITEA_TOKEN` | Gitea git clone + Contents API 驗證 | 無 |
| `CNB_TOKEN` | CNB git clone + contents API 驗證 | 無 |
| `SKILLSHARE_GIT_TOKEN` | 通用 git clone 驗證（備援） | 無 |
| `SKILLSHARE_CODEX_CLI` | 這台機器上 plugin 指令使用的 Codex CLI | `PATH` 上的 `codex`，再找已知的安裝位置 |
| `GIT_SSL_CAINFO` | 自訂 CA 憑證組合檔路徑 | 系統預設值 |
| `GIT_SSL_NO_VERIFY` | 停用 SSL 憑證驗證 | `false` |
| `SKILLSHARE_TEST_BINARY` | 測試執行檔路徑 | `bin/skillshare` |

---

## 相關文件

- [Configuration](/docs/reference/targets/configuration) — 設定檔參考
- [Windows Issues](/docs/troubleshooting/windows) — Windows 環境設定
