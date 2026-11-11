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

## 从 `npx skills` 迁移 {#from-npx-skills}

你可以继续用 `npx skills` 管理原有 skills，也可以将选定的 skills 交给 Skillshare。两者都支持多 agent 与全局／项目安装；迁移的理由是需要 Skillshare 更广的资源管理能力，而不是另一个工具只能安装。见[比较页](/docs/understand/philosophy/comparison)。

**目前没有自动导入 npx lock 或移交管理权的功能。** Collect 保留本地 skill 文件，不会导入上游跟踪信息。从仓库重新安装会建立 Skillshare 的跟踪信息，但获取的内容可能与现有安装不同。

以下路径与 lock 字段依据 **skills v1.8.1**：[全局 lock 源码](https://github.com/vercel-labs/skills/blob/e781cb4d07433dfe1d19e924ed45cc61738c55ec/src/skill-lock.ts)、[项目 lock 源码](https://github.com/vercel-labs/skills/blob/e781cb4d07433dfe1d19e924ed45cc61738c55ec/src/local-lock.ts)及 [CLI 文档](https://github.com/vercel-labs/skills/blob/e781cb4d07433dfe1d19e924ed45cc61738c55ec/docs/commands.md)。

| 项目 | Global | Project |
|------|--------|---------|
| 共享 skill 位置 | `~/.agents/skills/<name>/` | `.agents/skills/<name>/` |
| Agent 目录 | 通常是指向共享 skills 的链接；`--copy` 会创建独立副本 | 同左 |
| Lock 文件 | 已设置时使用 `$XDG_STATE_HOME/skills/.skill-lock.json`，否则为 `~/.agents/.skill-lock.json` | `skills-lock.json` |

`experimental_sync` 创建的 skills 可能是指向 npm 依赖的链接，而非真实目录。收集前先检查实际路径。

### 第 1 步：决定范围并保存原始配置

第 2–5 步的命令明确使用**全局模式**（`-g`）。项目安装请改看下方的项目说明。

初始化或同步前：

- 决定每个管理工具负责哪些 skills。迁移期间暂停更新、移除与自动同步任务。
- 另外创建原始 skill 目录、实际内容与 symlink 布局的快照。也要包含可能被替换的现有 Skillshare source。
- 单独保存适用的 npx lock。npm 项目也要保存 `package.json` 与包管理器的 lock；这些只记录来源与依赖，不是本地修改的备份。

验证交接完成前保留这份快照。不要删除旧 lock，也不要把 `npx skills remove` 当作清理捷径。

### 第 2 步：初始化但不复制，然后检查备份

若尚未初始化 Skillshare，请在 **stdin 与 stdout 都连接到终端的交互式环境**执行下列命令。不要通过管道或自动化执行：这些参数不会禁用首次同步，非交互初始化在选定 targets 后会自动同步。

```bash
skillshare init -g --no-copy --no-skill --no-git
```

选择要管理的 targets，包含对应 `~/.agents/skills` 的 `universal`，并使用本流程采用的 merge mode。若提示是否立即同步，请拒绝。已有配置时，先检查 source、[targets](/docs/reference/commands/target) 与模式，不要重置。

```bash
skillshare backup -g --dry-run
skillshare backup -g
skillshare backup -g --list
```

检查输出中的失败，确认实际 skill 文件已覆盖。Target 备份会**跳过 symlink**，不会备份链接指向的内容，也不包含 target 目录外的 npx lock。它只是第 1 步快照的补充，不能取代快照。

### 第 3 步：选择本地副本或上游跟踪

**保留当前文件：**从存放真实目录的 target 预览并收集：

```bash
skillshare collect universal -g --dry-run
skillshare collect universal -g
```

这会收集该 target 中所有符合条件的 skills，不是单一选定的 skill。若副本位于某个 agent 目录，请换成配置中对应的 target 名称。只有打算收集所有配置 targets 时才用 `collect --all -g`。部分交接时，改为只将选定的真实 skill 目录复制到 Skillshare source。

Collect 会跳过 skill symlink，包括指向 `node_modules` 的链接。这些 skills 可以继续由 npm 管理，或明确将解析后的真实 skill 目录复制为本地快照；副本之后不会随包更新。除非强制覆盖，collect 也会跳过 source 中已存在的名称。检查跳过／失败结果，并自行比较重复内容；预览不是理解 lock 的冲突报告。不要用 `collect --force` 盲目决定保留哪份内容。

收集的 skills 没有导入仓库跟踪信息，因此 `skillshare update` 无法从原始来源更新它们。

**保留上游更新：**从来源安装前，先检查保存的每个 lock 条目。`sourceType` 区分 Git、npm、本地与其他来源；`sourceUrl`、`ref`、`skillPath` 可能提供额外信息。不是每个条目都有仓库或 `skillPath`。

例如 GitHub 条目 `source: "anthropics/skills"`、`skillPath: "skills/pdf/SKILL.md"` 对应的是包含该文件的 skill 目录：

```bash
skillshare install anthropics/skills/skills/pdf -g --dry-run
skillshare install anthropics/skills/skills/pdf -g
```

若 lock 记录了 Git ref，请在上述两个命令都加上 `--branch <recorded-ref>`（见 [install](/docs/reference/commands/install)）；没有指定时，示例会使用远程默认分支。这是重新安装，不是重放 npx 的内容 hash。请与保存的文件比较并合并本地修改。若已收集的 skill 占用目标位置，先停止并处理；不要直接强制安装，因为可能覆盖修改并覆盖 audit 阻止结果。npm 与本地条目需要各自决定来源，不能直接转成 `owner/repo`。

### 第 4 步：预览并明确移交管理权

```bash
skillshare sync -g --dry-run
```

检查**所有配置的 targets**，不只是 `universal`。Merge mode 会保留同名本地目录，但即使没有 force，同名的 agent symlink 也可能被改为指向 Skillshare source。确认只影响打算交接的 skills 后才应用：

```bash
skillshare sync -g
```

若也想把保留的真实目录换成链接，先确认 source 中是要保留的版本，且原始快照完整：

```bash
skillshare sync -g --force --dry-run
# Apply only after reviewing every replacement:
skillshare sync -g --force
```

Force 会作用于配置的 targets，不是单个 skill 的迁移开关。若预览包含仍由 npx skills 管理的资源，请停止。两个工具的管理范围应分开；见[并存说明](/docs/troubleshooting/faq#using-universal-alongside-npx-skills)。

### 第 5 步：验证并保留单一管理者

检查结果文件／链接，并确认各 AI 工具能加载预期的 skills。执行 `skillshare status -g`，检查保留的本地 skills 或警告；sync 成功不代表每个原始 skill 都已迁移。

Skillshare 不会移除旧 npx lock 条目。交接后不要对已迁移的 skills 使用 `npx skills update`、`remove`、`experimental_install` 或 `experimental_sync`。部分迁移时，避免未指定范围而包含它们的批量命令。

若要回滚，先暂停两个管理工具，再一起恢复保存的原始文件、链接布局与 lock。Skillshare target 备份可协助恢复本地文件，但无法单独重建原始 npx symlink 或 lock。

### 项目安装与 npm 同步

变更前保存项目文件与 lock，包含尚未提交或被忽略的本地修改。以 `skillshare init -p` 初始化并配置项目 targets，再对 `collect`、`install`、`sync` 使用 `-p`，而非上述全局命令。相同的来源跟踪与 symlink 限制仍适用。Skillshare 不会在项目模式创建 skill-target 备份；请使用已验证的项目快照，或确实包含相关文件的版本控制。

若 `package.json` scripts 或 CI 会调用 `experimental_sync`，交接重叠的 skills 前先调整任务与 skill 声明，否则下次安装依赖可能重新链接它们。继续让 npx skills 管理 npm skills 完全合理；不要把链接复制到 `.skillshare/skills/` 就以为已改为独立管理。

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
