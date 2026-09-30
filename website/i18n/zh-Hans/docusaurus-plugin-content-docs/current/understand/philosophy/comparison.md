---
sidebar_position: 9
---

# 比较 Skill 管理方案

本页比较 AI CLI Skill 管理的两种主要架构方案：**命令式**（逐条命令安装）和**声明式**（配置 + sync）。

如果你正在评估各种工具或考虑切换，这份分析能帮助你理解其中根本的设计差异。

## 架构总览

### 命令式（逐条命令安装）

命令式工具采用逐条命令安装的模式——每次安装都是一次独立的操作：

```
tool add owner/repo → select agents → choose method → done
tool add owner/repo → select agents → choose method → done
tool add owner/repo → select agents → choose method → done
```

每次操作都需要用户输入。没有持久化状态来描述“哪里应该安装什么”。

### 声明式（配置 + Sync）

skillshare 采用声明式模型——你只需定义一次期望状态，然后执行 sync：

```yaml
# config.yaml — define once
source: ~/.config/skillshare/skills
targets:
  claude: ~/.claude
  cursor: ~/.cursor/skills
  codex: ~/.codex/skills
```

```bash
skillshare sync  # reconcile actual state to desired state
```

一条命令，无需任何提示，每次结果都是确定的。

## 功能对比

| 能力 | 命令式（逐条命令安装） | 声明式（skillshare） |
|------------|------------------------|--------------------------|
| **配置** | 无配置文件；每次运行都要提示 | `config.yaml`——设置一次，永久复用 |
| **Agent 选择** | 每次都要交互式提示 | 在配置中定义；`sync` 统一处理 |
| **安装方式** | 每次操作都要选择 copy/symlink | 配置中的 `sync_mode`（merge、copy 或 symlink） |
| **单一事实来源** | Skills 被独立复制到每个 agent | Source 目录 → 符号链接指向所有 Target |
| **从某个 agent 移除 Skill** | 可能删除 source 文件，破坏其他 agent | 只影响该 Target 的符号链接 |
| **可复现的环境搭建** | 没有内置的方式在新机器上恢复 | `config.yaml` + source 目录 = 完整恢复 |
| **项目范围的 Skills** | 锁定文件只追踪全局层级 | `skillshare init -p` 用于按仓库管理 Skills |
| **跨机器同步** | 手动（通过 dotfiles 同步锁定文件） | 内置的 `push` / `pull`，基于 git |
| **双向流动** | 单向（仅安装） | `collect` 可以把 Target 上的改进拉回来 |
| **区分自有 Skills 与已安装 Skills** | 混在同一目录下 | 被追踪的仓库使用 `_` 前缀 |
| **离线操作** | CLI 本身就需要 npx + 网络 | 单一二进制文件，安装后可离线工作 |
| **Web 控制台** | 无 | `skillshare ui`——可视化管理 |
| **备份 / 恢复** | 无 | `skillshare backup` / `skillshare restore` |
| **Git 平台支持** | update/check 仅支持 GitHub（写死使用 GitHub Trees API） | 任何 Git 远程仓库——GitHub、GitLab、Bitbucket、Azure DevOps、Gitea、AtomGit、Gitee、自建实例 |
| **运行时依赖** | Node.js + npm | 无（单一 Go 二进制文件） |

## 常见痛点及解决方案

### “每次安装都要重新选择 agent”

使用 skillshare，你只需配置一次 Target：

```yaml
targets:
  claude: ~/.claude
  cursor: ~/.cursor/skills
```

之后每次 `sync`、`install` 或 `collect` 都知道该同步到哪里，无需任何提示。

### “从一个 agent 移除 Skill 会连带破坏其他 agent”

在命令式工具中，从某个 agent 移除 Skill 可能会删除共享的 source 文件，导致其他 agent 出现失效的符号链接。

skillshare 的架构从根本上避免了这个问题——source 目录是唯一的事实来源。Target 的符号链接指向 source，而不是反过来。移除某个 Target 只会移除该 Target 自己的符号链接，source 文件不受影响。

```
Source: ~/.config/skillshare/skills/my-skill/SKILL.md  (always preserved)
  ├── ~/.claude/skills/my-skill → symlink to source  ✓
  ├── ~/.cursor/skills/my-skill  → symlink to source  ✓  (unaffected)
  └── ~/.codex/skills/my-skill  → symlink to source  ✓  (unaffected)
```

### “没法在新机器上恢复我的配置”

使用 skillshare，你的整套配置都是可移植的：

1. 对 `~/.config/skillshare/`（source + 配置）进行版本控制
2. 在新机器上：`git clone` 你的配置仓库
3. 运行 `skillshare sync`

所有 Target 会立即被重新创建。

### “update 和 check 在 GitLab / Bitbucket / Azure DevOps 上不起作用”

命令式工具在检查更新时通常依赖 GitHub Trees API，这意味着 `update` 和 `check` 会默默跳过来自非 GitHub 来源的 Skills。

skillshare 使用**本地 git 操作**（`git fetch` + 目录树哈希比对）——它可以配合任何 Git 远程仓库使用，包括 GitLab、Bitbucket、Azure DevOps、Gitea、AtomGit、Gitee，以及任何自建实例，不需要平台专属的 API。

```bash
# All of these support install, update, and check:
skillshare install https://gitlab.com/team/skills
skillshare install git@bitbucket.org:company/private-skills.git
skillshare install https://git.mycompany.com/org/repo
skillshare update   # checks all sources, regardless of host
```

### “大仓库 clone 起来要等很久”

skillshare 对非追踪安装默认使用浅克隆（`--depth 1`），大幅缩短下载时间。如果需要完整历史记录的追踪仓库，使用 `--track`。

### “我的 Skills 分散在各个 agent 目录里”

skillshare 把所有内容集中放在一个地方：

```
~/.config/skillshare/skills/
├── my-custom-skill/          # Your own skills
├── react-best-practices/     # Installed skills
├── _team-repo/               # Tracked repos (prefixed with _)
│   ├── frontend-guidelines/
│   └── code-review/
└── _another-org-repo/
```

`_` 前缀清楚地把被追踪的（团队/组织）仓库与你的个人 Skills 区分开。

## 迁移到 skillshare

如果你已经在使用其他 Skill 管理工具：

### 第一步：安装 skillshare

```bash
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

只有安装器显示 PATH 设置提示时，才需要按提示设置后再运行下方命令。没有 PATH 警告就不需要额外设置。

```bash

# Homebrew
brew install skillshare
```

### 第二步：初始化并收集现有 Skills

```bash
skillshare init              # Creates config and detects targets
skillshare collect --all     # Imports existing skills from all detected targets
```

### 第三步：Sync

```bash
skillshare sync              # Symlinks source skills to all targets
```

你现有的 Skills 现在都统一由一处管理。详细步骤请参见[迁移指南](/docs/how-to/advanced/migration)。

## 如何选择合适的工具

**在以下情况选择命令式工具：**
- 你很少安装 Skills，也不介意交互式提示
- 你只使用一个 AI CLI
- 你不需要跨机器或团队协作的工作流

**在以下情况选择 skillshare：**
- 你使用多个 AI CLI，并希望它们保持同步
- 你想要一次设置、之后无需再管的配置
- 你在多台机器上工作
- 你与团队或组织共享 Skills
- 你希望 Skills 能够备份、恢复并进行版本控制
- 你把 Skills 托管在 GitLab、Bitbucket、Azure DevOps 或自建 Git 上
- 你更喜欢没有运行时依赖的单一二进制文件
- 你不希望安装/下载行为被本地工作流之外的系统追踪

---

## 另请参阅

- [迁移](/docs/how-to/advanced/migration) — 迁移指南
- [核心概念](/docs/understand) — skillshare 的工作原理
