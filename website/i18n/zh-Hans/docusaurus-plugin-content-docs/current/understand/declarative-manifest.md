---
sidebar_position: 8
---

# Declarative Skill Manifest

将你的 Skill 集合定义为代码——从单一 manifest 文件安装、共享和复现配置。

:::tip 什么时候需要用到这个？
当你希望在多台机器上实现可复现的 Skill 配置、用一条命令完成团队上手,或引导开源项目时,使用 declarative manifest。
:::

## 什么是 Skill Manifest?

Skill manifest 是你 Skill 集合的一份**可移植声明**。你不需要逐个手动安装 Skill,而是把它们列在一个 manifest 文件中,然后运行 `skillshare install` 来一次性搭建完成。

Manifest 的位置取决于所处模式:

| Mode | Manifest 位置 | 可提交? |
|------|------------------|-------------|
| **Project** | `.skillshare/config.yaml`(`skills:` 部分) | 是——提交以与团队共享 |
| **Global** | `~/.config/skillshare/skills/.metadata.json` | 否——个人机器状态 |

### Project Mode Manifest

在 Project mode 中,`skills:` 与 `targets:` 一同位于 `config.yaml` 中:

```yaml
# .skillshare/config.yaml
targets:
  - claude
  - cursor

skills:
  - name: react-best-practices
    source: anthropics/skills/skills/react-best-practices
    group: frontend
  - name: _team-skills
    source: my-org/shared-skills
    tracked: true
  - name: commit
    source: anthropics/skills/skills/commit
```

这个文件会被提交到 git——队友克隆仓库后运行 `skillshare install -p` 即可安装所有列出的 Skill。

Manifest 记录的是要安装*什么*。每个 Skill 实际解析到的确切 commit 则记录在它旁边的 `.skillshare/skills.lock.json` 中,该文件由系统自动写入,也应该一并提交。有了这两个文件,即使上游已经继续往前推进,`skillshare install -p` 也能让每位队友得到相同的 commit。参见[锁定文件](./project-skills.md#lockfile)。

### Global Mode Manifest

在 Global mode 中,Skill 记录存储在 `.metadata.json`(集中式元数据存储)中。这个文件还包含运行时追踪数据(哈希、时间戳),并且是自动管理的。

## 工作原理

### 从 Manifest 安装

运行**不带任何参数**的 `skillshare install` 会读取 manifest 并安装所有列出的 Skill:

```bash
# Global mode — installs all skills from ~/.config/skillshare/skills/.metadata.json
skillshare install

# Project mode — installs all skills from .skillshare/config.yaml skills: section
skillshare install -p

# Preview without installing
skillshare install --dry-run
```

已存在的 Skill 会被自动跳过。在 Project mode 中,如果某个 Skill 已安装的 commit 与锁定文件不一致,则会被移动到固定的 commit。

### 自动协调

Manifest 会与你实际的 Skill 集合保持同步:

- **`skillshare install <source>`** —— 自动将已安装的 Skill 添加到 manifest 中
- **`skillshare uninstall <name>...`** —— 自动从 manifest 中移除该条目

在 Project mode 中,更新的是 `config.yaml` 和 `skills.lock.json`。在 Global mode 中,更新的是 `.metadata.json`。你永远不需要手动编辑 manifest(不过你也可以这样做)。

Global 的追踪声明及其审计接受记录会在 checkout 不存在时保留，因此可以使用 `skillshare install` 恢复。要移除声明，请在 checkout 存在时使用 `skillshare uninstall <name>`；如果 checkout 不存在，请先使用 `skillshare install` 恢复，再卸载。普通文件夹和 project mode 的行为不变。

## Skill 条目字段

`skills:` 列表中的每个条目都有以下字段:

| 字段 | 是否必填 | 说明 |
|-------|----------|-------------|
| `name` | 是 | Skill 名称(source 中的目录名) |
| `source` | 是 | 安装来源(GitHub 简写、HTTPS URL、SSH URL) |
| `tracked` | 否 | Tracked repositories 设为 `true`(保留 `.git`) |
| `group` | 否 | 子目录路径(例如 `frontend` 或 `frontend/vue`)。对应安装时的 `--into`。 |

## 使用场景

### 个人配置

跨机器维护你的个人 Skill 集合:

```bash
# On machine A — skills are already installed and tracked in registry
skillshare push   # backup config + registry to git

# On machine B — fresh machine
skillshare pull   # restore config + registry from git
skillshare install  # install all skills from manifest
skillshare sync   # distribute to all targets
```

### 团队上手

新团队成员用一条命令获得相同的 AI 上下文:

```bash
# .skillshare/config.yaml skills: section is committed to the repo
git clone <project-repo>
cd <project-repo>
skillshare install -p   # installs all declared skills
skillshare sync -p      # links to project targets
```

### 开源引导

项目维护者在 `config.yaml` 中声明推荐的 Skill:

```yaml
# .skillshare/config.yaml
targets:
  - claude
  - cursor

skills:
  - name: react-best-practices
    source: anthropics/skills/skills/react-best-practices
    group: frontend
  - name: commit
    source: anthropics/skills/skills/commit
```

:::info Group 字段与 `--into`
当你使用 `--into` 安装时,group 会被自动记录:

```bash
skillshare install anthropics/skills/skills/pdf --into frontend -p
# config.yaml will contain: name: pdf, group: frontend
```

运行不带参数的 `skillshare install -p` 会根据 manifest 重新创建出相同的目录结构。
:::

贡献者克隆仓库并运行 `skillshare install -p`,即可立即获得项目专属的 AI 上下文。

## 工作流总结

```
Project mode:
1. Install skills normally      →  config.yaml skills: auto-updates
2. Commit config.yaml and skills.lock.json via git  →  same skills, same commits for the team
3. Run `skillshare install -p`  →  reproduce on clone
4. Run `skillshare sync`        →  distribute to all targets

Global mode:
1. Install skills normally      →  .metadata.json auto-updates
2. Push/pull config via git     →  portable across machines
3. Run `skillshare install`     →  reproduce on new machine
4. Run `skillshare sync`        →  distribute to all targets
```

## Extras 配置

除了 Skill 之外,`config.yaml` 还可以声明 **extras**——同步到独立目录的非 skill 资源(rules、commands、prompts)。Extras 在 `config.yaml` 的 `extras:` 部分中配置(Global 和 Project 均可):

```yaml
extras:
  - name: rules
    targets:
      - path: ~/.claude/rules
      - path: ~/.cursor/rules
        mode: copy
```

详情参见 [sync extras](/docs/reference/commands/sync#sync-extras)。

## 相关内容

- [Install command](/docs/reference/commands/install) —— 带参数和不带参数的 `skillshare install`
- [Push/Pull](/docs/reference/commands/push) —— 通过 git 备份和恢复配置
- [Project Skills](./project-skills.md) —— 项目级别的 manifest
