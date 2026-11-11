---
sidebar_position: 3
---

# Recipe：私有企業 Skills

> 使用 token 認證安裝來自私有 repository 的 skills。

## 情境

你的組織將內部 skills 存放在私有的 GitHub/GitLab repository 中。你需要在不將憑證暴露於 config 檔案的情況下安裝並更新這些 skills。

## 解決方案

### 步驟 1：設定認證

skillshare 會從環境變數偵測 token，平台專屬的變數優先於通用的 fallback：

| 平台 | 環境變數 |
|----------|---------------------|
| `github.com` | `GITHUB_TOKEN` |
| `gitlab.com` | `GITLAB_TOKEN` |
| Bitbucket | `BITBUCKET_TOKEN`（+ 選用的 `BITBUCKET_USERNAME`） |
| Azure DevOps | `AZURE_DEVOPS_TOKEN` |
| Gitea / 自架 Gitea | `GITEA_TOKEN` |
| CNB | `CNB_TOKEN` |
| 任何平台（fallback） | `SKILLSHARE_GIT_TOKEN` |

平台權杖僅傳送至使用 HTTPS 預設連接埠（或 443）的 `github.com`、`gitlab.com`、`bitbucket.org`、`dev.azure.com`、`*.visualstudio.com`、`gitea.com` 和 `cnb.cool`。自架 Gitea/CNB 即使名稱包含平台名，也必須在 `gitea_hosts`/`cnb_hosts` 中設定完整相符的主機名稱。自架 GitHub/GitLab 等主機應使用依主機設定的 Git credential helper，或僅為目標呼叫設定 `SKILLSHARE_GIT_TOKEN`。此通用 fallback 可傳送給**任何 HTTPS 來源**；類似平台的名稱不會授權傳送平台權杖。

```bash
# 選項 A：Git credential helper（建議用於 GitHub）
gh auth login   # 為 HTTPS 設定 git credential helper

# 選項 B：平台專屬的環境變數
export GITHUB_TOKEN=ghp_xxxxxxxxxxxxx      # GitHub
export GITLAB_TOKEN=glpat-xxxxxxxxxxxxx    # GitLab
export AZURE_DEVOPS_TOKEN=your-pat-here    # Azure DevOps

# 選項 C：通用 fallback（適用於任何 HTTPS host）
export SKILLSHARE_GIT_TOKEN=your-token-here
```

### 步驟 2：從私有 repo 安裝

```bash
skillshare install your-org/internal-skills --track
```

skillshare 會自動從上述環境變數中偵測 token。

### 步驟 3：驗證追蹤狀態

```bash
skillshare list
```

已安裝的 repo 會以 `_` 前綴顯示（tracked repository）：

```
_your-org-internal-skills/
├── code-review/
├── testing-standards/
└── deployment-checklist/
```

### 步驟 4：更新循環

```bash
skillshare check    # 偵測上游變更
skillshare update   # 拉取最新版本
skillshare sync     # 推送到 targets
```

## 驗證

- `skillshare list` 顯示已追蹤的 repo
- `skillshare check` 能連線到 remote 並比對 hash
- `skillshare sync` 在所有 targets 中建立 symlinks

## 變化

- **選擇性安裝**：`skillshare install your-org/internal-skills --skill code-review` 只安裝單一 skill
- **CI/CD token**：在 pipeline 中，從 CI secrets 設定平台專屬的環境變數（例如 `GITHUB_TOKEN`）
- **自架 GitHub/GitLab**：使用依主機設定的 Git credential helper，或僅為這次呼叫提供 token：`SKILLSHARE_GIT_TOKEN=your-token skillshare install https://gitlab.internal.com/team/skills.git --track`。`GITHUB_TOKEN`／`GITLAB_TOKEN` 不會傳送給自架服務。
- **自架 Gitea**：設定 `GITEA_TOKEN`，並一律將確切主機名稱列入 [`gitea_hosts`](/docs/reference/targets/configuration#gitea_hosts)。
- **Gitee / AtomGit**：透過 `SKILLSHARE_GIT_TOKEN` 搭配 HTTPS URL 支援

## 相關

- [`install` 指令參考](/docs/reference/commands/install)
- [`update` 指令參考](/docs/reference/commands/update)
- [組織分享指南](/docs/how-to/sharing/organization-sharing)
- [URL 格式參考](/docs/reference/appendix/url-formats)
