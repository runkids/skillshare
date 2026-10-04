---
sidebar_position: 2
---

# Environment Variables

skillshare 识别的所有环境变量。

## Configuration

### SKILLSHARE_CONFIG

覆盖配置文件路径。

```bash
SKILLSHARE_CONFIG=~/custom-config.yaml skillshare status
```

**默认值：** `~/.config/skillshare/config.yaml`

---

### XDG_CONFIG_HOME

按照 [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir/latest/) 覆盖基础配置目录。

```bash
export XDG_CONFIG_HOME=~/my-config
# skillshare 将使用 ~/my-config/skillshare/
```

**默认行为：**

| Platform | Default |
|----------|---------|
| Linux | `~/.config/skillshare/` |
| macOS | `~/.config/skillshare/` |
| Windows | `%AppData%\skillshare\` |

**优先级：** `SKILLSHARE_CONFIG` > `XDG_CONFIG_HOME` > 平台默认值。

:::note
如果你在初始设置之后才设置 `XDG_CONFIG_HOME`，需要手动将现有的 `~/.config/skillshare/` 目录移动到新位置。
:::

---

### XDG_DATA_HOME

覆盖数据目录（backups、trash）。

```bash
export XDG_DATA_HOME=~/my-data
# skillshare 将使用 ~/my-data/skillshare/backups/ 和 ~/my-data/skillshare/trash/
```

**默认值：** `~/.local/share/skillshare/`

---

### XDG_STATE_HOME

覆盖状态目录（操作日志）。

```bash
export XDG_STATE_HOME=~/my-state
# skillshare 将使用 ~/my-state/skillshare/logs/
```

**默认值：** `~/.local/state/skillshare/`

---

### XDG_CACHE_HOME

覆盖缓存目录（版本检查缓存、UI dist 缓存）。

```bash
export XDG_CACHE_HOME=~/my-cache
# skillshare 将使用 ~/my-cache/skillshare/
```

**默认值：** `~/.cache/skillshare/`

:::tip Automatic migration
从 v0.13.0 开始，skillshare 遵循 XDG Base Directory Specification 存放 backups、trash 和 logs。如果你从旧版本升级，这些目录会在首次运行时自动从 `~/.config/skillshare/` 迁移到正确的 XDG 位置。
:::

---

### SKILLSHARE_GITLAB_HOSTS

以逗号分隔的自建 GitLab 主机名列表，用于启用嵌套子群组（nested subgroup）支持。适用于没有配置文件的 CI/CD 环境。

```bash
SKILLSHARE_GITLAB_HOSTS=git.company.com,code.internal.io skillshare install git.company.com/team/frontend/ui
```

当环境变量与配置文件中的 [`gitlab_hosts`](/docs/reference/targets/configuration#gitlab_hosts) 同时设置时，两者的值会被**合并**（并去重）。无效条目（包含 scheme、path、port 或为空）会被静默跳过。

**默认值：** 无（仅使用配置文件中的值）

---

### SKILLSHARE_AZURE_HOSTS

以逗号分隔的自建 Azure DevOps Server 主机名列表。

```bash
SKILLSHARE_AZURE_HOSTS=azuredevops.mycompany.com,tfs.internal.io
```

当环境变量与配置文件中的 [`azure_hosts`](/docs/reference/targets/configuration#azure_hosts) 同时设置时，两者的值会被**合并**（并去重）。环境变量中的无效条目会被静默跳过。

**默认值：** _(无)_

### SKILLSHARE_GITEA_HOSTS

以逗号分隔的自建 Gitea 主机名列表。名称中包含 `gitea` 的主机无需在此列出即可被识别。

```bash
SKILLSHARE_GITEA_HOSTS=git.company.com,code.internal.io
```

会与配置文件中的 [`gitea_hosts`](/docs/reference/targets/configuration#gitea_hosts) 合并。

**默认值：** _(无)_

### SKILLSHARE_CNB_HOSTS

以逗号分隔的自建 CNB 主机名列表。`cnb.cool` 无需在此列出即可被识别。

```bash
SKILLSHARE_CNB_HOSTS=cnb.company.com
```

会与配置文件中的 [`cnb_hosts`](/docs/reference/targets/configuration#cnb_hosts) 合并。

**默认值：** _(无)_

---

## Web UI

### SKILLSHARE_UI_BASE_PATH

设置在反向代理后面提供 Web dashboard 时使用的 URL 子路径。

```bash
SKILLSHARE_UI_BASE_PATH=/skillshare skillshare ui --host 0.0.0.0 --no-open
```

等同于 `--base-path /skillshare`。若两者都设置，flag 优先。

**默认值：** 无（dashboard 在根路径 `/` 提供）

参见 [Reverse Proxy](/docs/reference/commands/ui#reverse-proxy) 获取 Nginx 与 Caddy 的示例。

---

## GitHub API

### GITHUB_TOKEN

GitHub 个人访问令牌（personal access token）。

**用途：**
- GitHub API 请求（`skillshare search`、`skillshare upgrade`、版本检查）
- **Git clone 认证** — 通过 HTTPS 安装私有仓库时会自动注入

**创建令牌：**
1. 前往 https://github.com/settings/tokens
2. 生成新令牌（classic）
3. 权限范围：私有仓库选 `repo`，公开仓库无需权限
4. 复制令牌

官方文档：[Managing your personal access tokens](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)

**用法：**
```bash
export GITHUB_TOKEN=ghp_your_token_here
skillshare install https://github.com/org/private-skills.git --track
```

**Windows：**
```powershell
# 当前会话
$env:GITHUB_TOKEN = "ghp_your_token"

# 永久设置
[Environment]::SetEnvironmentVariable("GITHUB_TOKEN", "ghp_your_token", "User")
```

---

### GH_TOKEN

备选的 GitHub 令牌变量。被 [GitHub CLI（`gh`）](https://cli.github.com/) 使用，skillshare 也将其识别为回退方案。

**令牌解析顺序：** `GITHUB_TOKEN` → `GH_TOKEN` → `gh auth token`

如果你已经在使用 `gh` 并通过 `gh auth login` 完成认证，skillshare 会自动获取该令牌 — 无需额外设置环境变量。

```bash
export GH_TOKEN=ghp_your_token_here
skillshare search "react patterns"
```

---

## Git Authentication {#git-authentication}

这些变量用于启用私有仓库的 HTTPS 认证。设置后，skillshare 会在 `install` 与 `update` 时自动注入令牌 — 无需修改 URL。

详情与 CI/CD 示例参见 [Private Repositories](/docs/reference/commands/install#private-repositories)。


### GITLAB_TOKEN

GitLab 个人访问令牌或 CI job token。用于通过 HTTPS clone GitLab 托管的私有仓库。

**创建令牌：**
1. 前往 https://gitlab.com/-/user_settings/personal_access_tokens
2. 添加新令牌
3. 权限范围：`read_repository`（仅拉取）或 `read_repository` + `write_repository`（拉取与推送）
4. 复制令牌（前缀为 `glpat-`）

官方文档：[Token overview](https://docs.gitlab.com/security/tokens/)

:::warning Token types
只有 **Personal Access Token**（`glpat-`）和 **Project/Group Access Token** 可用于 git 操作。Feed Token（`glft-`）**不**具备 git 访问权限。
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

Bitbucket 仓库访问令牌或 app password。用于通过 HTTPS clone Bitbucket 托管的私有仓库。

**创建令牌（repository access token）：**
1. 前往 Repository → Settings → Access tokens
2. 创建具有 **Read** 权限（仅拉取）或 **Read + Write**（拉取与推送）的令牌
3. 复制令牌 — 会自动使用 `x-token-auth`（无需用户名）

**创建令牌（app password）：**
1. 前往 https://bitbucket.org/account/settings/app-passwords/
2. 创建具有 **Repositories: Read**（仅拉取）或 **Repositories: Read + Write**（拉取与推送）权限的 app password
3. 同时设置 `BITBUCKET_USERNAME`（或将用户名写入 URL，如 `https://<username>@bitbucket.org/...`）

官方文档：[Access tokens](https://support.atlassian.com/bitbucket-cloud/docs/access-tokens/)

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

与 `BITBUCKET_TOKEN` 配合使用的 Bitbucket 用户名，当该令牌为 app password 时需要。

```bash
export BITBUCKET_USERNAME=your_bitbucket_username
export BITBUCKET_TOKEN=your_app_password
skillshare install https://bitbucket.org/team/skills.git --track
```

### AZURE_DEVOPS_TOKEN

Azure DevOps 个人访问令牌（PAT）。用于通过 HTTPS clone Azure DevOps 托管的私有仓库。

**创建令牌：**
1. 前往 `https://dev.azure.com/{org}/_usersSettings/tokens`
2. 选择 **+ New Token**
3. 权限范围：**Code → Read**（仅拉取）或 **Code → Read & Write**（拉取与推送）
4. 复制令牌（84 字符字符串，带有 `AZDO` 签名）

官方文档：[Use Personal Access Tokens](https://learn.microsoft.com/en-us/azure/devops/organizations/accounts/use-personal-access-tokens-to-authenticate?view=azure-devops)

```bash
export AZURE_DEVOPS_TOKEN=your_pat_here
skillshare install https://dev.azure.com/org/project/_git/repo --track
```

**Windows：**
```powershell
$env:AZURE_DEVOPS_TOKEN = "your_pat_here"
```

### GITEA_TOKEN

Gitea 访问令牌。用于通过 HTTPS clone `gitea.com` 及自建 Gitea 上的私有仓库，也用于在不做完整 clone 而只安装子目录时调用 Gitea Contents API。

**创建令牌：**
1. 在 Gitea 中前往 **Settings → Applications**
2. 生成新令牌
3. 权限：`repository: Read`（仅拉取）或 `repository: Read and Write`（拉取与推送）

```bash
export GITEA_TOKEN=your_token
skillshare install https://gitea.com/org/skills.git --track
```

**Windows：**
```powershell
$env:GITEA_TOKEN = "your_token"
```

该令牌只会发送给你安装所用的主机。名称中不含 `gitea` 的自建实例需要配置 [`gitea_hosts`](/docs/reference/targets/configuration#gitea_hosts)，否则将改用 `SKILLSHARE_GIT_TOKEN`。

### CNB_TOKEN

拥有仓库读取权限的 [CNB](https://cnb.cool) 访问令牌。用于通过 HTTPS clone `cnb.cool` 及自建 CNB 上的私有仓库。

```bash
export CNB_TOKEN=your_token
skillshare install https://cnb.cool/org/skills --track
```

**Windows：**
```powershell
$env:CNB_TOKEN = "your_token"
```

部署在其他域名上的私有实例需要配置 [`cnb_hosts`](/docs/reference/targets/configuration#cnb_hosts)。

### SKILLSHARE_GIT_TOKEN

适用于任意 HTTPS git 主机的通用回退令牌。在没有设置特定平台令牌时使用。

```bash
export SKILLSHARE_GIT_TOKEN=your_token
skillshare install https://git.example.com/org/skills.git --track
```

**Windows：**
```powershell
$env:SKILLSHARE_GIT_TOKEN = "your_token"
```

**令牌优先级：** 特定平台令牌（`GITHUB_TOKEN`、`GITLAB_TOKEN`、`BITBUCKET_TOKEN`、`AZURE_DEVOPS_TOKEN`、`GITEA_TOKEN`、`CNB_TOKEN`）> `SKILLSHARE_GIT_TOKEN`。

---

## Plugin {#plugins}

### SKILLSHARE_CODEX_CLI

[Plugin 命令](/docs/reference/commands/plugin)要运行的 Codex CLI，只对这台机器有效。Skillshare 自己找不到 Codex 时再设置。它从环境变量读取，所以 macOS 和 Windows 共用同一份 `config.yaml` 时，不需要写一个在另一台机器上错误的路径。

没有设置时，Skillshare 先从 `PATH` 运行 `codex`，再依次尝试这些位置：

| 操作系统 | 位置 |
|---|---|
| macOS | `/opt/homebrew/bin/codex`、`/usr/local/bin/codex`，以及 Codex app 内置的 CLI（`/Applications` 或 `~/Applications` 里的 `ChatGPT.app` 或 `Codex.app`） |
| Windows | 官方 PowerShell 安装程序的 `%LOCALAPPDATA%\Programs\OpenAI\Codex\bin\codex.exe`，再来是 Codex app 安装的 CLI：`%LOCALAPPDATA%\OpenAI\Codex\bin\<version>\codex.exe`（取最新的一份） |

```bash
export SKILLSHARE_CODEX_CLI=/Applications/ChatGPT.app/Contents/Resources/codex-cli/bin/codex
```

**Windows:**
```powershell
$env:SKILLSHARE_CODEX_CLI = "C:\Tools\codex.exe"
```

自己设置了 `cli` 的 [account target](/docs/reference/commands/plugin#accounts) 仍使用那个 `cli`。

---

## Git SSL / TLS {#git-ssl--tls}

这些标准 Git 环境变量会被传递给所有 git 操作。对于使用自签名证书或内部 CA 的自建 Git 服务器（GitLab、Gitea 等）非常有用。

### GIT_SSL_CAINFO

自定义 CA 证书包的路径。当你的自建 Git 服务器使用内部 CA 签发的证书时使用。

```bash
export GIT_SSL_CAINFO=/path/to/ca-bundle.crt
skillshare install https://gitlab.internal.company.com/team/skills.git --track
```

### GIT_SSL_NO_VERIFY

完全禁用 SSL 证书验证。仅在无法配置 CA 证书时作为最后手段使用。

:::warning Security risk
禁用 SSL 验证会使连接容易受到中间人攻击。请优先使用带正确 CA 证书包的 `GIT_SSL_CAINFO`，或改用 SSH。
:::

```bash
GIT_SSL_NO_VERIFY=true skillshare install https://gitlab.internal.company.com/team/skills.git --track
```

**替代方案：使用 SSH 完全避开 SSL：**
```bash
skillshare install git@gitlab.internal.company.com:team/skills.git --track
```

---

## Testing

### SKILLSHARE_TEST_BINARY

覆盖集成测试所用的 CLI 二进制文件路径。

```bash
SKILLSHARE_TEST_BINARY=/path/to/skillshare go test ./tests/integration
```

**默认值：** 项目根目录下的 `bin/skillshare`

---

## Usage Examples

### Temporary override

```bash
# 单次命令
SKILLSHARE_CONFIG=/tmp/test-config.yaml skillshare status

# 多个命令
export SKILLSHARE_CONFIG=/tmp/test-config.yaml
skillshare status
skillshare list
unset SKILLSHARE_CONFIG
```

### Permanent setup (macOS/Linux)

添加到 `~/.bashrc` 或 `~/.zshrc`：
```bash
export GITHUB_TOKEN="ghp_your_token_here"
```

### Permanent setup (Windows)

```powershell
[Environment]::SetEnvironmentVariable("GITHUB_TOKEN", "ghp_your_token", "User")
```

---

## Summary

| Variable | Purpose | Default |
|----------|---------|---------|
| `SKILLSHARE_CONFIG` | 配置文件路径 | `~/.config/skillshare/config.yaml` |
| `SKILLSHARE_UI_BASE_PATH` | 反向代理下 Web UI 的子路径 | 无 |
| `SKILLSHARE_GITLAB_HOSTS` | 自定义 GitLab 主机名（逗号分隔） | 无 |
| `SKILLSHARE_AZURE_HOSTS` | 自定义 Azure DevOps Server 主机名（逗号分隔） | 无 |
| `SKILLSHARE_GITEA_HOSTS` | 自定义 Gitea 主机名（逗号分隔） | 无 |
| `SKILLSHARE_CNB_HOSTS` | 自定义 CNB 主机名（逗号分隔） | 无 |
| `XDG_CONFIG_HOME` | 基础配置目录 | `~/.config`（Linux/macOS）、`%AppData%`（Windows） |
| `XDG_DATA_HOME` | 数据目录（backups、trash） | `~/.local/share` |
| `XDG_STATE_HOME` | 状态目录（logs） | `~/.local/state` |
| `XDG_CACHE_HOME` | 缓存目录（版本检查、UI） | `~/.cache` |
| `GITHUB_TOKEN` | GitHub API + git clone 认证 | 无 |
| `GH_TOKEN` | GitHub API（`GITHUB_TOKEN` 的回退） | 无 |
| `GITLAB_TOKEN` | GitLab git clone 认证 | 无 |
| `BITBUCKET_TOKEN` | Bitbucket git clone 认证 | 无 |
| `BITBUCKET_USERNAME` | 用于 app password 认证的 Bitbucket 用户名 | 无 |
| `AZURE_DEVOPS_TOKEN` | Azure DevOps git clone 认证 | 无 |
| `GITEA_TOKEN` | Gitea git clone + Contents API 认证 | 无 |
| `CNB_TOKEN` | CNB git clone + contents API 认证 | 无 |
| `SKILLSHARE_GIT_TOKEN` | 通用 git clone 认证（回退） | 无 |
| `SKILLSHARE_CODEX_CLI` | 这台机器上 plugin 命令使用的 Codex CLI | `PATH` 上的 `codex`，再找已知的安装位置 |
| `GIT_SSL_CAINFO` | 自定义 CA 证书包路径 | 系统默认 |
| `GIT_SSL_NO_VERIFY` | 禁用 SSL 证书验证 | `false` |
| `SKILLSHARE_TEST_BINARY` | 测试二进制文件路径 | `bin/skillshare` |

---

## Related

- [Configuration](/docs/reference/targets/configuration) — 配置文件参考
- [Windows Issues](/docs/troubleshooting/windows) — Windows 环境设置
