---
sidebar_position: 9
---

# 比较 Skill 管理方式

**Vercel 的 `npx skills`** 和 **Skillshare** 都能跨多个 AI 工具管理 skills。差别不是“一个只能安装，另一个才能同步”，而是你需要管理 skills，还是更完整的 AI 工作环境。

本页依据已发布的 **skills v1.8.1** [包信息](https://registry.npmjs.org/skills/1.8.1)及其[官方 CLI 文档](https://github.com/vercel-labs/skills/blob/e781cb4d07433dfe1d19e924ed45cc61738c55ec/docs/commands.md)。实验性命令可能变更或移除；这是指定版本的比较，不是对所有安装工具的概括。

## 架构概览

### npx skills：发现与管理 skills

[skills.sh](https://skills.sh) 提供发现入口。CLI 支持多 agent、项目与全局安装、指向共享副本的符号链接或独立副本，以及更新检查与更新。Agent 选择及确认可通过参数提供，不一定需要交互提示。

它也会通过 lock 文件记录已安装的 skills。实验性命令支持项目恢复与 npm 同步，因此不能将它描述为没有持久状态或只能安装。

### Skillshare：跨工具管理工作环境

Skillshare 将源资源与 target 配置分开，再同步到各工具。除了 skills，也管理 agent 定义、rules 等 extras、MCP 连接设置，以及支持客户端的原生 hooks。本地 Web dashboard 提供可视化管理界面。

真正的差别是将这些资源一起纳入跨工具、跨机器与团队工作流，而不是独有的符号链接或多 agent 安装能力。资源格式与客户端能力仍有差异，并非每种资源都适用于每个工具。

## 功能比较

| 能力 | npx skills v1.8.1 | Skillshare |
|------|------------------|------------|
| **发现** | skills.sh 与 `skills find` | CLI 搜索与本地 Web dashboard |
| **多 agent 与范围** | 多 agent；项目与全局安装 | 配置中的 targets；项目与全局模式 |
| **共享 skill 文件** | 指向共享副本的符号链接；可选 copy | 源目录搭配 merge、copy 或整个目录的 symlink 模式 |
| **更新** | `skills check`（有更新或本地修改时以退出码 1 结束）与 `skills update` | `skillshare check` 与 `skillshare update` |
| **项目恢复** | 实验性 `skills experimental_install`（或不带来源的 `skills install`）从 `skills-lock.json` 恢复支持的来源 | 声明式 skill 配置与源资源；见 [manifest](/docs/understand/declarative-manifest) |
| **npm 项目同步** | 实验性 `skills experimental_sync` 读取已安装依赖与 `package.json` 的 skill 声明 | 源到 target 的同步；不是相同的 npm 依赖工作流 |
| **跨机器工作流** | 项目 lock 恢复与 npm 同步可协助重建项目 skills | Git `push` / `pull`，范围由 `git_root` 决定 |
| **Skills 以外** | 所引用的 CLI 文档以 skills 为主 | Agent 定义、rules 等 extras、MCP 设置与支持客户端的 hooks |
| **本地管理与恢复** | 所引用的 CLI 文档未列出本地 dashboard 或 target 备份命令 | `skillshare ui`、target 备份与恢复，以及受管理文件恢复 |
| **运行环境** | Node.js 与 npm/npx | Go 可执行文件；Git 工作流需要 Git |

### 实验性命令涵盖什么

- **`experimental_install`**（也可用不带来源的 `skills install` 执行）将支持的 lock 条目恢复到 `.agents/skills/`，并链接到 agent 目录（以 copy 模式记录的 skills 则为复制）：优先使用 `--agent` 指定的 agent，其次是上次选择的 agent，再其次是检测到的 agent。它不会安装 npm 依赖。已安装且文件与 lock 内容 hash 一致的 skills 不会重新获取，只补上缺少的 agent 链接；内容不一致的 skills 会从来源恢复，`--force` 则重新安装所有 skills。Frozen-lock 模式会检查内容 hash，且为 CI 默认值。
- **`experimental_sync`** 链接已安装 npm 依赖附带的 skills，并解析 `package.json` 的 skill 声明。它通过 `skills-lock.json` 跟踪这些条目、移除不再提供的 skills，并保留另外安装的 skills。使用 copy 时，依赖更新后需要再次同步。

这些确实是同步与恢复能力，但不是整套 AI 工具配置的备份。

## 选择合适的工具

**继续使用 npx skills**：如果主要需求是发现、安装与更新公开或私有 skills，即使同时使用多个 agent 也很合理。若希望 skills 跟随 npm 项目依赖，且能接受此工作流仍属实验性，也适合考虑。

**选择 Skillshare**：当问题不只是安装 skills，例如：“我同时用 Claude Code 和 Codex，希望把受管理的 skills、rules、MCP 设置与支持的 hooks 带到另一台电脑。”它的源／target 配置、Git 工作流、本地 dashboard 与恢复工具服务于这个更广的管理需求。

可移植性仍需要配置：选择 Git 范围、在新机器配置 targets，并另外准备凭据与外部依赖。`push` 默认只处理 skills 源，不会自动发布全部配置或备份整台机器。见 [Git root 范围](/docs/reference/commands/push)与 [sync](/docs/reference/commands/sync)。

## 迁移至 Skillshare

不必立刻取代运行良好的安装工具。只移交希望由 Skillshare 管理的资源，并避免两个工具同时更新或移除同一个受管理的 skill。

目前迁移并非无损的 lock 导入：收集的 skills 是本地副本，不保留原始 repository 的跟踪信息。若需要后续通过 Skillshare 更新，请从记录的来源重新安装。移交管理权前先备份与预览；请遵循 [npx skills 迁移指南](/docs/how-to/advanced/migration)，不要将 collect 本身视为完整迁移。

## 另请参阅

- [迁移](/docs/how-to/advanced/migration)
- [核心概念](/docs/understand)
- [备份](/docs/reference/commands/backup)
