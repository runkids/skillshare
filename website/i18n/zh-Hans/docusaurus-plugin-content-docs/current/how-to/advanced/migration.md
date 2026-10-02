---
sidebar_position: 5
---

# Migration

从其他 Skill 管理方式迁移到 skillshare。

## 从手动管理迁移

如果你一直在各个 AI CLI 之间手动复制 Skill：

### 第 1 步：初始化 skillshare

```bash
skillshare init
```

### 第 2 步：收集现有 Skill

```bash
# 从各个 AI CLI 收集
skillshare collect claude
skillshare collect pi
skillshare collect codex

# 或一次性收集全部
skillshare collect --all
```

### 第 3 步：处理重复项

如果同一个 Skill 存在于多个位置，`collect` 会发出警告，由你选择保留哪一个。

### 第 4 步：Sync

```bash
skillshare sync
```

现在所有 Target 都已符号链接到你唯一的 Source。

---

## 从其他安装工具迁移

如果你使用过 `npx install-skill` 或类似工具：

### 第 1 步：初始化 skillshare

```bash
skillshare init
```

### 第 2 步：备份现有 Skill

```bash
skillshare backup
```

### 第 3 步：收集或重新安装

**方式 A：收集现有内容**（保留当前版本）
```bash
skillshare collect --all
```

**方式 B：从 Source 重新安装**（获取最新版本）
```bash
# 查看元数据
cat ~/.config/skillshare/skills/.metadata.json

# 重新安装
skillshare install anthropics/skills/skills/pdf
```

### 第 4 步：Sync

```bash
skillshare sync
```

---

## 从 `npx skills` 迁移

如果你使用 [npx skills CLI](https://github.com/vercel-labs/skills)（`npx skills add ...`）安装过 Skill，skillshare 可以接管相同的目录。你也可以[让两个工具并存使用](/docs/troubleshooting/faq#using-universal-alongside-npx-skills)，只迁移你希望由 skillshare 管理的 Skill。

`npx skills` 的存放位置：

| 项目 | Global | Project |
|------|--------|---------|
| Skill 文件 | `~/.agents/skills/<name>/`（真实目录） | `.agents/skills/<name>/` |
| Agent 目录 | 指向上述文件的 symlink（例如 `~/.claude/skills/<name>`） | 同左 |
| Lock file | `~/.agents/.skill-lock.json`（或 `$XDG_STATE_HOME/skills/.skill-lock.json`） | `skills-lock.json` |

使用 `--copy` 安装的 Skill 在每个 Agent 目录中都是真实目录，而不是 symlink。

### 第 1 步：初始化 skillshare

```bash
skillshare init
```

### 第 2 步：备份现有 Skill

```bash
skillshare backup
```

### 第 3 步：收集 Skill

`collect` 会复制真实目录并跳过 symlink，因此 Agent 目录中的 symlink 不会被重复收集。

```bash
skillshare collect universal --dry-run   # 预览
skillshare collect universal             # ~/.agents/skills
```

如果你使用过 `--copy`，或在其他 Agent 目录中也有 Skill，请改为执行 `skillshare collect --all`，并按照[从手动管理迁移](#从手动管理迁移)中的说明处理重复项。

收集到的 Skill 是普通的本地副本。它们不会记录来自哪个仓库，因此 `skillshare update` 无法更新它们。

### 第 4 步：重新安装需要持续更新的 Skill（可选）

Lock file 记录了每个 Skill 的来源。`source` 是仓库，`skillPath` 是 Skill 在仓库中的位置。

```bash
cat ~/.agents/.skill-lock.json

# "source": "anthropics/skills", "skillPath": "skills/pdf/SKILL.md"
skillshare install anthropics/skills/skills/pdf
```

如果该 Skill 已在第 3 步中进入你的 Source，请加上 `--force`。

### 第 5 步：Sync

```bash
skillshare sync
```

在 merge mode 下，Sync 会保留 Target（`~/.agents/skills`）中同名的真实目录，并将其报告为保留的本地 Skill。其他 Agent 目录（例如 `~/.claude/skills`）中由 `npx skills` 创建的 symlink 会被重新指向你的 skillshare Source。如果还想把这些真实目录也替换为 symlink，请在第 3 步收集完成后执行：

```bash
skillshare sync --force
```

`sync --force` 会在替换任何内容之前备份 Target。此后请只用 skillshare 管理这些 Skill。`npx skills` 在自己的 lock file 中记录安装信息，因此请避免对它们执行 `npx skills update` 或 `npx skills remove`。

对于使用 `npx skills` 的项目，请按照[从已提交的项目 Skill 迁移](#从已提交的项目-skill-迁移)操作，并将 `.agents/skills/` 作为要迁移的目录。

---

## 从 Git Submodule 迁移

如果你一直在使用 git submodule：

### 第 1 步：导出 submodule 内容

```bash
# 在你现有的 Skill 仓库中
git submodule foreach 'cp -r $toplevel/$sm_path ~/temp-skills/$name'
```

### 第 2 步：初始化 skillshare

```bash
skillshare init
```

### 第 3 步：导入 Skill

```bash
# 复制到 Source
cp -r ~/temp-skills/* ~/.config/skillshare/skills/

# 或以 tracked repo 形式安装
skillshare install github.com/org/skill-repo --track
```

### 第 4 步：Sync

```bash
skillshare sync
```

---

## 从已提交的项目 Skill 迁移

如果你的仓库已经在 `.claude/skills/`、`.cursor/skills/` 或类似目录中提交了 Skill：

### 第 1 步：初始化 Project mode

```bash
cd my-project
skillshare init -p
```

### 第 2 步：将 Skill 移动到 `.skillshare/skills/`

```bash
# 将现有 Skill 复制到 skillshare 的 Source
cp -r .claude/skills/my-skill .skillshare/skills/
cp -r .claude/skills/api-guide .skillshare/skills/

# 删除原始文件（Sync 会将其重新创建为符号链接）
rm -rf .claude/skills/my-skill .claude/skills/api-guide
```

### 第 3 步：Sync

```bash
skillshare sync
```

现在 `.claude/skills/my-skill` 是指向 `.skillshare/skills/my-skill` 的符号链接 — 其他所有 Target（Pi、Windsurf 等）也会自动获得相同的 Skill。

### 第 4 步：提交迁移结果

```bash
git add .skillshare/ .claude/skills/ .cursor/skills/
git commit -m "Migrate project skills to skillshare"
```

:::tip 多工具优势
之前：Skill 只能在一种 AI CLI 中使用。之后：相同的 Skill 会自动在每个已配置的 Target 中可用。
:::

---

## 从团队特定方案迁移

如果你的团队有自定义的 Skill 共享方式：

### 第 1 步：识别当前方案

- Skill 存放在哪里？
- 如何共享？
- 如何更新？

### 第 2 步：选择迁移路径

**方式 A：Global mode** — 每台机器上的所有项目都能使用 Skill。

```bash
# 创建团队 Skill 仓库
cp -r /current/team/skills ~/new-team-skills
cd ~/new-team-skills && git init && git add . && git commit -m "Migrate to skillshare"
git push origin main

# 团队成员进行全局安装
skillshare install github.com/org/team-skills --track && skillshare sync
```

**方式 B：Project mode** — Skill 限定于特定仓库，通过 git 共享。

```bash
cd my-project
skillshare init -p

# 将团队 Skill 移入项目的 Source
cp -r /current/team/skills/* .skillshare/skills/

# Sync 并提交
skillshare sync
git add .skillshare/
git commit -m "Add team skills via skillshare"
```

新团队成员只需执行以下命令即可获得一切：
```bash
git clone github.com/org/my-project
cd my-project
skillshare install -p && skillshare sync
```

**方式 C：两者兼具** — 组织级标准使用 Global mode，项目特定的 Skill 逐仓库配置。

```bash
# 组织标准（Global）
skillshare install github.com/org/standards --track && skillshare sync

# 项目特定 Skill（Project mode）
cd my-project
skillshare init -p
skillshare install github.com/org/project-skills -p && skillshare sync
```

:::tip 该如何选择？
- **Global**：编码规范、安全审计 — 每个项目都需要的内容
- **Project**：API 约定、领域规则、部署指南 — 特定于某个仓库的内容
- **两者兼具**：大多数团队随着规模增长最终都会走到这一步
:::

---

## 从 Global 迁移到 Project

如果你在 Global mode 中有属于某个特定项目的 Skill：

### 第 1 步：初始化 Project mode

```bash
cd my-project
skillshare init -p
```

### 第 2 步：从 Global Source 复制 Skill

```bash
# 复制指定 Skill
cp -r ~/.config/skillshare/skills/api-guide .skillshare/skills/
cp -r ~/.config/skillshare/skills/deploy-rules .skillshare/skills/
```

### 第 3 步：从 Global 中移除（可选）

```bash
skillshare uninstall api-guide
skillshare uninstall deploy-rules
skillshare sync   # 清理 Global 符号链接
```

### 第 4 步：Sync 并提交

```bash
skillshare sync   # 自动检测为 Project mode
git add .skillshare/
git commit -m "Move project-specific skills to project mode"
```

之后，这些 Skill 就限定于此仓库，并通过 git 与团队共享 — 不再占用你的 Global 配置。

---

## 保留历史记录

如果你想保留 git 历史：

### 个人 Skill

```bash
# 将现有仓库 clone 到 skillshare 的位置
git clone your-existing-repo ~/.config/skillshare/skills

# 使用现有 Source 初始化 skillshare
skillshare init --source ~/.config/skillshare/skills
```

### 团队仓库

```bash
# 使用 --track 保留 .git
skillshare install github.com/team/skills --track
```

---

## 回滚

如果迁移出现问题：

### 从备份恢复

```bash
skillshare restore claude
skillshare restore cursor
```

### 重新开始

```bash
rm ~/.config/skillshare/config.yaml
skillshare init
```

---

## 检查清单

迁移前：

- [ ] 列出当前所有 Skill 位置
- [ ] 识别重复项
- [ ] 记录任何自定义配置
- [ ] 创建备份

迁移后：

- [ ] 确认所有 Skill 都出现在 `skillshare list` 中
- [ ] 在每个 AI CLI 中测试 Skill
- [ ] 设置 git remote（如需要）
- [ ] 向团队分享新的工作流程

---

## 另请参阅

- [From Existing Skills](/docs/getting-started/from-existing-skills) — 快速迁移路径
- [collect](/docs/reference/commands/collect) — 从 Target 收集
- [Comparison](/docs/understand/philosophy/comparison) — 方案对比
