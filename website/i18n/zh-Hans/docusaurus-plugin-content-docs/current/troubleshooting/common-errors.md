---
sidebar_position: 2
---

# Common Errors

错误信息与对应的解决方式。

## Config Errors

### `config not found: run 'skillshare init' first`

**原因：** 不存在配置文件。

**解决方式：**
```bash
skillshare init
```

如果想使用自定义路径，加上 `--source`：
```bash
skillshare init --source ~/my-skills
```

---

### `failed to load project config: ...`

**原因：** `.skillshare/config.yaml` 存在，但无法解析（YAML 格式错误、类型不对等）。变更类命令（`uninstall`、`new`、`enable`/`disable`、`check`）在这种状态下会拒绝执行，以避免在你使用自定义 `sources` 配置时不小心动到默认的 `.skillshare/skills/` 目录。

**解决方式：** 修正 YAML 后重新执行命令。常见问题：

```yaml
# 错误 — targets 必须是 list
targets: {}

# 正确
targets: []
```

```yaml
# 错误 — skills 必须是 list
skills: my-skill

# 正确
skills:
  - name: my-skill
    source: github.com/org/my-skill
```

用任何 YAML 检查工具验证该文件，或者如果有备份，可暂时从 `.skillshare/backups/` 还原。

---

### `target "<name>": skills target path X overlaps skills source Y`

**原因：** 你的 `sources.skills` 解析出来的目录与某个 Target 的 skills 路径相同（或两者互相包含）。例如同时设置 `sources.skills: .claude/skills` 和一个 `claude` Target——两者都指向 `.claude/skills/`。如果没有这项检查，`sync --force` 会把 Source 误认为 Target 目录并删除其内容。

**解决方式：** 选择一个不会与任何 Target 重叠的 Source 路径。常见的安全选择：

```yaml
# 与项目文件放在一起
sources:
  skills: ./docs/skills

# 保留在 .skillshare/ 下（默认值 — 直接移除 sources 键即可）
```

同样的检查也适用于 `sources.agents` 与 Agent Target 路径之间。

---

## Target Errors

### `target add: path does not exist`

**原因：** skills 目录尚不存在。

**解决方式：**
```bash
mkdir -p ~/.myapp/skills
skillshare target add myapp ~/.myapp/skills
```

### `target path does not end with 'skills'`

**原因：** 提示路径不符合惯例的警告。

**解决方式：** 这只是警告，不是错误。如果路径是刻意这样设置的，可以继续；或者修正它：
```bash
skillshare target add myapp ~/.myapp/skills  # 建议做法
```

### `target directory already exists with files`

**原因：** Target 中已有文件，可能会被覆盖。

**解决方式：**
```bash
skillshare backup
skillshare sync
```

---

## Sync Errors

### `deleting a symlinked target removed source files`

**原因：** 你在 symlink 模式下对某个 Target 执行了 `rm -rf`。

**解决方式：**
```bash
# 如果已初始化 git
cd ~/.config/skillshare/skills
git checkout -- .

# 或从 backup 还原
skillshare restore <target>
```

**预防方式：** 用 `skillshare target remove` 取代手动删除。

### `sync` keeps showing the same changes {#sync-keeps-showing-the-same-changes}

**原因：** 两个 Target 以不同的 `include` 或 `exclude` 过滤器把 skills 同步到同一个文件夹。每次 sync 都会加入其中一个 Target 需要的内容，又移除另一个 Target 过滤掉的内容，所以这个文件夹永远无法稳定下来。`sync` 会指出是哪两个 Target：

```
! codex and universal sync skills to ~/.agents/skills with different filters, so each sync undoes the other
  keep one: skillshare target codex --skills=false
```

**解决方式：** 只让其中一个 Target 写入这个文件夹，并为另一个 Target 关闭 skills。它的 agents、MCP servers 和 instructions 仍然由 skillshare 管理，对应的工具也仍会读取共享文件夹里的 skills：

```bash
skillshare target codex --skills=false --dry-run
skillshare target codex --skills=false
```

在仪表板中，**Sync** 页面会显示同样的警告，并提供一个按钮，用来停止为该 Target 同步 skills。让两个 Target 使用相同的过滤器也同样可行。

### `sync seems stuck or slow`

**原因：** skills 目录中有大文件。

**解决方式：** 加上忽略规则：
```yaml
# ~/.config/skillshare/config.yaml
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
  - "**/node_modules/**"
```

### `no space left on device` / `ENOSPC` during sync

**原因：** 有东西把卷占满了。先检查 backup 目录，再检查你的 Source。

**解决方式：**
```bash
df -h ~                                     # 确认卷已满
du -sh ~/.local/share/skillshare/backups    # backup 用量
du -sh ~/.config/skillshare/skills          # Source 用量
```

如果 backup 占用很大，就清理它——保留策略会在每次 `sync` 后自动运行，但如果目录在此之前就已经变大，也可以手动清理：

```bash
skillshare backup --cleanup --dry-run   # 预览
skillshare backup --cleanup
```

如果卷被钉在 100% 满，`rm` 可能会因为 "Permission denied" 而失败，直到腾出一点空间为止。先删掉一个大文件，再执行清理。

如果是 **Source** 太大，问题往往出在 skills 内部的产物文件。backup 不会复制这些内容（symlink 的 skills 会被跳过），但处于 copy 模式的每个 Target 都会复制。请把运行时缓存、模型权重、浏览器 profile 移到 skill 目录之外，或用 `ignore:` 排除它们。

关于 backup 范围与 `.gitignore`、`ignore:` 的差异，参见 [Backups & Disk Space](/docs/reference/commands/backup#backups--disk-space)。

---

## Git Errors

### `Could not read from remote repository`

**原因：** SSH key 未设置，或远程仓库地址不正确。

**解决方式：**
```bash
# 检查 SSH 访问权限
ssh -T git@github.com

# 如果没有设置 SSH，改用 HTTPS
git -C ~/.config/skillshare/skills remote set-url origin https://github.com/you/my-skills.git

# 或设置 SSH key
ssh-keygen -t ed25519 -C "you@example.com"
# 然后前往 GitHub → Settings → SSH keys 添加该公钥
```

### `push: remote has changes`

**原因：** 远程仓库比本地领先。

**解决方式：**
```bash
skillshare pull   # 先取得远程变更
skillshare push   # 现在可以 push 了
```

### `pull: local has uncommitted changes`

**原因：** 你有尚未 push 的本地变更。

**解决方式：**
```bash
# 选项 1：先 push 你的变更
skillshare push -m "Local changes"
skillshare pull

# 选项 2：放弃本地变更
cd ~/.config/skillshare/skills
git checkout -- .
skillshare pull
```

### `pull stopped: this machine and the remote both changed ...`

**原因：** 同一个文件在两台机器上都被编辑过。`pull` 已撤销合并，所以仓库没有变化。只有 `.metadata.json` 冲突时不会出现这个错误，它会被自动解决。

**解决方式：**
```bash
cd ~/.config/skillshare/skills
git pull --no-rebase          # 重新合并并保留冲突
# 编辑冲突文件
git add .
git commit --no-edit
skillshare push
skillshare sync
```

### `Git had no identity`

**原因：** `skillshare init` 创建 source repo 时，git 没有 `user.name` / `user.email`。skillshare 把兜底值（`skillshare@local`）写进该 repo 自己的配置，让 commit 能正常进行。repo 的配置优先于 `git config --global`，所以之后设置 global 身份并不会取代它。

你自己创建的 repo 不会被改动：skillshare 只在那一次初始 commit 使用兜底值。

**解决方式：** 在该 repo 设置你的身份（路径以消息打印的为准，以下是默认值）：
```bash
git -C ~/.config/skillshare/skills config user.name "Your Name"
git -C ~/.config/skillshare/skills config user.email "you@example.com"
```

或删除 repo 内的配置，改用 global 身份：`git -C ~/.config/skillshare/skills config --unset user.name`（`user.email` 同理）。

### `Git root mismatch`

**原因：** `config.yaml` 中的 `git_root` 指向一个没有 git 仓库的 scope 目录，而另一个 scope 目录却有。这通常发生在你只改了 `git_root` 却没有搬动仓库时——切换 scope 的意思是「开始为另一个目录建立版本控制」，而不是「搬移既有的历史记录」。参见 [`git_root`](/docs/reference/targets/configuration#git-root)。

**解决方式：** 从错误信息给出的三个选项中选一个：
```bash
# 在配置的 scope 上建立全新仓库（不保留历史）
skillshare init --git-root <scope>

# 把既有仓库搬过去，保留历史
mv <old-scope>/.git <new-scope>/.git

# 或继续使用既有仓库：把 config.yaml 的 git_root 改回去
#   git_root: <scope-that-has-the-repo>
```

### `tracked repository clone is missing`

**原因：** 某个 tracked repo 已记录在 `.metadata.json` 中，但对应的克隆目录（例如 `skills/_team-skills/`）在本地却不存在。这常发生在你把 skillshare 的 Source 仓库克隆到新机器之后，因为 tracked repo 的目录会被有意列入受管理的 `.gitignore` 区块中。

**解决方式：** 从 metadata 重新还原缺失的 tracked repo 克隆：
```bash
skillshare install
skillshare sync
```

在 Project mode 下：
```bash
skillshare install -p
skillshare sync -p
```

`status`、`check`、`update --all` 和 `doctor` 都会报告这种状态，并建议执行 `skillshare install`。

### `nested git repositories must be disabled first`

**原因：** 当 `git_root: root` 时，某个子目录（例如 `skills/_org/` 下被 track 的 skill 仓库）自带 `.git`。Git 会把它当成一个**空的 submodule** 上传，导致其中的文件被悄悄丢弃，因此在每个嵌套仓库被停用之前，`commit`/`push` 会中止。

**解决方式：**
```bash
# 停用每个被报告的嵌套仓库（可逆——改回名字即可重新启用）
mv ~/.config/skillshare/<dir>/.git ~/.config/skillshare/<dir>/.git.disabled
```
也可以在 Web UI 的 Git Sync 页面上一键停用。skillshare 也会自动把 `config.yaml` 排除在 root-scope 仓库之外（因为它包含机器专属的路径）。

### `Invalid git_root`

**原因：** `config.yaml` 中的 `git_root` 被设置成无法识别的值（例如拼写错误）。

**解决方式：** 使用 `skills`、`agents`、`extras`、`root` 之一——或留空（默认为 `skills`）。

---

## Install Errors

### `skill already exists`

**原因：** 已经安装了同名的 Skill。

**解决方式：**
```bash
# 更新已安装的 Skill
skillshare install <source> --update

# 或强制覆盖
skillshare install <source> --force
```

### `git failed (exit 128): repository not found or authentication required`

**原因：** 仓库地址不正确、仓库不存在，或缺少身份验证信息。

skillshare 现在会为常见的 git 失败提供可操作的错误信息，而不是只给出原始退出码。错误信息中会包含建议：

```
Error: git failed (exit 128): repository not found or authentication required
```

如果使用了 token 但被拒绝：

```
Error: git failed (exit 128): authentication token was rejected — check permissions and expiry
```

**解决方式：** 参见下方的身份验证选项。

### `Authentication failed` / `Access denied`

**原因：** HTTPS 凭证缺失、已过期，或 token 类型不对。

**解决方式 — 选项 1：设置 token 环境变量：**

```bash
# GitHub
export GITHUB_TOKEN=ghp_xxxx

# GitLab（必须是 Personal Access Token，前缀为 glpat-）
export GITLAB_TOKEN=glpat-xxxx

# Bitbucket
export BITBUCKET_TOKEN=your_app_password
```

**Windows（PowerShell）：**
```powershell
$env:GITLAB_TOKEN = "glpat-xxxx"

# 永久生效（重启后仍保留）
[Environment]::SetEnvironmentVariable("GITLAB_TOKEN", "glpat-xxxx", "User")
```

**解决方式 — 选项 2：使用 SSH 地址：**
```bash
skillshare install git@github.com:team/private-skills.git
skillshare install git@gitlab.com:team/skills.git
skillshare install git@bitbucket.org:team/skills.git
```

**解决方式 — 选项 3：Git 凭证助手：**
```bash
gh auth login          # GitHub CLI
git credential approve # 或平台专属的 credential manager
```

**所需的 token 权限：**

| 平台 | Token 类型 | 权限范围 |
|----------|-----------|---------------------|
| GitHub | Personal Access Token（`ghp_`） | `repo`（私有仓库），公开仓库无需权限 |
| GitLab | Personal Access Token（`glpat-`） | `read_repository` + `write_repository` |
| Bitbucket | Repository Access Token | 读 + 写 |
| Bitbucket | App Password + `BITBUCKET_USERNAME` | Repositories: 读 + 写 |

:::warning GitLab token types
只有 **Personal Access Token**（`glpat-`）可用于 git 操作。Feed Token（`glft-`）**没有** git 访问权限。
:::

参见 [Environment Variables](/docs/reference/appendix/environment-variables#git-authentication) 与 [Private Repositories](/docs/reference/commands/install#private-repositories)。

### `SSL certificate problem` / `certificate verification failed`

**原因：** Git 服务器使用自签名证书或你系统不信任的内部 CA。常见于自建的 GitLab、Gitea 或 Gogs 实例。

**解决方式 — 选项 1：自定义 CA bundle（推荐）：**
```bash
export GIT_SSL_CAINFO=/path/to/company-ca-bundle.crt
skillshare install https://gitlab.internal.company.com/team/skills.git --track
```

**解决方式 — 选项 2：改用 SSH（完全避开 SSL）：**
```bash
skillshare install git@gitlab.internal.company.com:team/skills.git --track
```

**解决方式 — 选项 3：停用 SSL 验证（不推荐）：**
```bash
GIT_SSL_NO_VERIFY=true skillshare install https://gitlab.internal.company.com/team/skills.git --track
```

:::warning
停用 SSL 验证存在安全风险。请优先选择选项 1 或 2。
:::

参见 [Environment Variables — Git SSL / TLS](/docs/reference/appendix/environment-variables#git-ssl--tls)。

### `invalid skill: SKILL.md not found`

**原因：** 该 Source 没有有效的 SKILL.md 文件。

**解决方式：** 确认 Source 路径正确，且指向一个 Skill 目录。

---

## Update Errors

### `pull stopped: this machine and the remote both changed ...`（tracked 仓库）

**原因：** tracked 仓库里有本地 commit，与远程发生冲突。`update` 会合并分歧的历史，但遇到冲突的文件会停止，并撤销合并。

**解决方式：**
```bash
# 强制更新（用远程内容取代本地）
skillshare update --force

# 或手动解决
cd ~/.config/skillshare/skills/_repo-name
git pull --no-rebase
# 编辑冲突文件，然后
git add . && git commit --no-edit
```

:::tip
`skillshare update` 和 `skillshare install` 现在会针对 git 失败（身份验证、SSL、分支分歧）显示可操作的错误信息，而不是原始退出码。
:::

---

## Audit Errors

### `security audit failed — critical threats detected`

**原因：** 该 Skill 包含匹配严重安全威胁的模式（prompt injection、数据外泄、凭证访问）。

**解决方式：**
```bash
# 查看检测结果
skillshare audit <skill-name>

# 如果你信任该来源，强制安装
skillshare install <source> --force
```

### `audit HIGH: Hidden zero-width Unicode characters detected`

**原因：** 该 Skill 中含有不可见的 Unicode 字符，可能是复制粘贴产生的残留，也可能是刻意混淆。

**解决方式：** 用能显示隐藏字符的编辑器打开文件并移除它们，或者如果信任该来源，可以强制安装。

---

## Upgrade Errors

### `GitHub API rate limit exceeded`

**原因：** 未经身份验证的 API 请求太多。

**解决方式：**
```bash
# 选项 1：设置 GitHub token（推荐）
export GITHUB_TOKEN=ghp_your_token_here
skillshare upgrade

# 选项 2：强制升级
skillshare upgrade --cli --force
```

在这里创建 token：https://github.com/settings/tokens （公开仓库不需要任何 scope）

---

## Skill Errors

### `skill not appearing in AI CLI`

**原因：**
1. Skill 未同步
2. SKILL.md 格式无效
3. AI CLI 有缓存

**解决方式：**
```bash
# 1. Sync
skillshare sync

# 2. 检查格式
skillshare doctor

# 3. 重新启动 AI CLI
```

### Antigravity does not load synced skills {#antigravity-does-not-load-synced-skills}

**原因：** Antigravity 应用的 Skill 扫描器只会发现**真实目录**——它会跳过 symlink。skillshare 默认的 `merge` 模式会为每个 Skill 创建一个 symlink（在 Windows 上是 NTFS junction），所以没有一个会被扫描到。在 Windows 上会表现为 `Incorrect function` 错误；在 macOS 与 Linux 上，Skill 则是悄悄地不出现。

这是 Antigravity 那一侧的限制，不是 skillshare 的 bug。它只影响 `antigravity` target（应用，`~/.gemini/config/skills`）；独立的 `agy` CLI 是另一个 `antigravity-cli` target，读取 `~/.gemini/antigravity-cli/skills`。有两种解决方式：

**选项 1 — 把该 Target 切换为 `copy` 模式**

```bash
skillshare target antigravity --mode copy
skillshare sync --force
```

这样会写入真实目录而不是 symlink。代价是：每次编辑完 Source 中的 Skill 后都需要重新执行 `skillshare sync`。

**选项 2 — 让 Antigravity 直接指向你的 Source 目录**

在 Antigravity 中：**Settings → Customizations → Skill Custom Paths → "+ Add"**，然后输入 skillshare Source 的**绝对路径**（例如 `/Users/you/.config/skillshare/skills`）。`~` 缩写不会被展开，所以必须填写完整路径。

无论用哪种方式，都要重启 Antigravity 以重新加载 Skill。

### `skill name 'X' is defined in multiple places`

**原因：** 多个 Skill 使用了相同的 `name` 字段，并且落在同一个 Target 上。

**解决方式：** 在 SKILL.md 中重新命名其中一个，或用 `include`/`exclude` 过滤器把它们分流到不同的 Target：
```yaml
# 选项 1：在 SKILL.md 中加上命名空间
name: team-a-skill-name

# 选项 2：用过滤器分流（global 配置）
targets:
  codex:
    path: ~/.codex/skills
    include: [_team-a__*]
  claude:
    path: ~/.claude/skills
    include: [_team-b__*]

# 选项 2：用过滤器分流（project 配置）
targets:
  - name: claude
    exclude: [codex-*]
  - name: codex
    include: [codex-*]
```

:::tip
如果过滤器已经把重复项隔离开来，sync 只会显示信息提示而不是警告——不需要额外处理。
完整语法参见 [Target Filters](/docs/reference/targets/configuration#include--exclude-target-filters)。
:::

---

## Agent Errors

### Warning: `No agents folder: <targets>`

**原因：** 你执行了 `skillshare sync`（或 `skillshare sync agents`），而某些配置的 Target 没有定义 agents 目录。只有 Claude、Cursor、Augment 和 OpenCode 内置了 agents 路径；其他 Target 在 agent 同步时会被自动跳过。

**解决方式：**

1. 如果这些 Target 不需要 agents，可以忽略这个警告。
2. 在 `config.yaml` 中为该 Target 加上 `agents:` 子键，为它启用 agent 同步：

```yaml
targets:
  myapp:
    path: ~/myapp/skills
    agents:
      path: ~/myapp/agents
```

然后重新执行 `skillshare sync agents`。

### `backup is not supported in project mode (except for agents)`

**原因：** 你执行了 `skillshare backup -p`（或 `skillshare backup -p <target>`），却没有加上 `agents` 过滤条件。在 Project mode 下，只支持 agent 的 backup——Skill 的 backup 仅限 Global mode。

**解决方式：** 加上 `agents` 位置参数，或使用 `--all`：

```bash
skillshare backup -p agents          # Project mode 下的 agent Target
skillshare backup -p agents claude   # 指定某个 Target
skillshare backup -p --all           # 效果相同（在 Project mode 下会收窄为 agents）
```

`restore` 也适用同样的规则：`restore is not supported in project mode (except for agents)`。

### `agent name 'X' has invalid characters`

**原因：** agent 的文件名或 `name:` frontmatter 字段包含不允许的字符。

**解决方式：** agent 名称只能使用 `a-z`、`0-9`、`_`、`-`、`.`。重命名该文件（并同步更新 `name:` 字段），让两者使用同一个规范名称。

### `.agentignore` patterns not taking effect

**原因：**

1. 文件放错了位置。它必须位于 agents source 的根目录：`~/.config/skillshare/agents/.agentignore`（Global mode）或 `.skillshare/agents/.agentignore`（Project mode）。
2. 你的匹配规则命中了非预期的路径片段——该文件使用 [gitignore 语法](https://git-scm.com/docs/gitignore)。

**解决方式：** 用 `skillshare doctor` 确认文件路径，并重新检查匹配规则。agent 是按 basename（不含 `.md`）匹配的，所以 `draft-*` 会匹配 `draft-experiment.md`。可以用 `skillshare disable <agent> --kind agent` 让 CLI 帮你写入该条目。

---

## Plugin Errors {#plugin-errors}

### `<agent> CLI is not installed or not on PATH` {#plugin-cli-not-on-path}

**原因：** Plugin 命令会在运行 Skillshare 的机器上调用 Agent 的原生 CLI（`claude`、`codex` 等），但找不到该 CLI。计划任务或由服务启动的 dashboard，`PATH` 通常比你的终端短。

**解决方式：**

1. 在那台机器上安装 Agent 的 CLI。
2. 如果已经安装，把它所在的目录加入启动 Skillshare 的程序的 `PATH`，例如计划任务的环境变量。
3. 对于 [account target](/docs/reference/commands/plugin#accounts)，也可以把 `cli` 设为可执行文件的绝对路径。

### `Codex CLI not found on the machine running Skillshare` {#plugin-codex-not-found}

**原因：** Skillshare 在 `PATH`、Homebrew 的文件夹和 Codex 桌面 app 里找 Codex CLI，都没有找到。消息会列出找过的每个位置。

**解决方式：**

1. 在那台机器上安装 Codex app 或 Codex CLI。
2. 如果 Codex 装在其他地方，在启动 Skillshare 的环境（例如计划任务）把 [`SKILLSHARE_CODEX_CLI`](/docs/reference/appendix/environment-variables#skillshare_codex_cli) 设为它的路径。这个设置只留在那台机器，不会影响与其他操作系统共用的 config。

### `The native marketplace X is gone` {#plugin-marketplace-gone}

**原因：** 该 plugin 是导入的，因此 Skillshare 会从它原来的原生 marketplace 重新安装，但 Agent 中没有注册该 marketplace。这在第二台机器上很常见：导入只记录在第一台机器上。

**解决方式：**

1. 在 Agent 中重新添加该 marketplace，然后运行 `skillshare sync plugins`。
2. 或从该 plugin 移除这个 Agent，再从 source 重新添加 plugin。请参阅 [Cross-Machine Sync — Plugins](/docs/how-to/sharing/cross-machine-sync#plugins)。

---

## Binary Errors

### `integration tests cannot find the binary`

**原因：** 二进制文件未构建，或路径不正确。

**解决方式：**
```bash
go build -o bin/skillshare ./cmd/skillshare
# 或设置
export SKILLSHARE_TEST_BINARY=/path/to/skillshare
```

---

## Still Having Issues?

参见 [Troubleshooting Workflow](./troubleshooting-workflow.md)，了解系统化的排查方式。
