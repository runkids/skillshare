---
sidebar_position: 5.5
---

# 使用文件夹组织 Skill

随着你的 Skill 集合不断增长，将它们组织到文件夹中可以让一切保持易于管理——而 skillshare 会自动处理剩下的部分。

## 为什么要组织？

20 个以上 Skill 的扁平列表会变得难以浏览：

```
~/.config/skillshare/skills/
├── accessibility/
├── ascii-box-check/
├── core-web-vitals/
├── frontend-design/
├── performance/
├── react-best-practices/
├── remotion/
├── seo/
├── skill-creator/
├── ui-skills/
├── vue-best-practices/
├── vue-debug-guides/
├── web-artifacts-builder/
└── ... 还有 20 个以上
```

使用文件夹，你可以获得逻辑分组，同时 skillshare 会针对 AI CLI 自动扁平化：

```
SOURCE（已组织）                       TARGET（自动扁平化）
───────────────────────────────────    ──────────────────────────────────
~/.config/skillshare/skills/           ~/.claude/skills/
├── frontend/                          ├── frontend__frontend-design
│   ├── frontend-design/               ├── frontend__react__react-best-..
│   ├── react/                         ├── frontend__ui-skills
│   │   └── react-best-practices/      ├── frontend__vue__vue-best-prac..
│   ├── ui-skills/                     ├── frontend__vue__vue-debug-gui..
│   └── vue/                           ├── utils__ascii-box-check
│       ├── vue-best-practices/        ├── utils__remotion
│       ├── vue-debug-guides/          ├── utils__skill-creator
│       └── ...                        ├── web-dev__accessibility
├── utils/                             ├── web-dev__core-web-vitals
│   ├── ascii-box-check/               └── ...
│   ├── remotion/
│   └── skill-creator/
└── web-dev/
    ├── accessibility/
    ├── core-web-vitals/
    └── ...
```

![Source vs Target comparison](/img/organizing-skills-comparison.png)

:::tip 真实案例
参见 [runkids/my-skills](https://github.com/runkids/my-skills)，其中使用此模式建立了一个完整、有组织的 Skill 集合。
:::

---

## 自动扁平化的工作原理

skillshare 使用 `__`（双底线）作为分隔符，将文件夹路径转换为扁平名称：

| Source 路径 | 同步后的 Target 名称 |
|---|---|
| `frontend/react/react-best-practices/` | `frontend__react__react-best-practices` |
| `utils/remotion/` | `utils__remotion` |
| `web-dev/accessibility/` | `web-dev__accessibility` |

**关键点：**
- 只有包含 `SKILL.md` 的目录才会被视为 Skill
- 中间文件夹（例如 `frontend/` 本身）仅用于组织——它们不需要 `SKILL.md`
- `list` 与 `sync` 会在任意深度发现嵌套的 Skill
- `check` 与 `update` 也支持嵌套的 Skill

:::note Agent 不支持嵌套
本页讲的是如何组织 **Skill**。Agent 永远是单一的 `.md` 文件，直接放在 `~/.config/skillshare/agents/`（Project mode 下则是 `.skillshare/agents/`）之下——它们不支持文件夹嵌套或自动扁平化。若要组织 Agent，请使用命名规范（例如 `frontend-reviewer.md`、`backend-auditor.md`）以及 `.agentignore` 模式。
:::

---

## 使用嵌套 Skill

### list

同一目录下的 Skill 会自动分组显示：

```bash
$ skillshare list -g

  frontend/vue/
    → vue-best-practices     github.com/vuejs-ai/skills/...

  utils/
    → remotion               github.com/remotion-dev/skills/...

  web-dev/
    → accessibility          github.com/addyosmani/web-quality-...
```

在每个分组中，Skill 会显示其基本名称（而非完整的扁平名称）。顶层 Skill 会在底部以未分组的方式显示。如果所有 Skill 都是顶层的，输出就会是一个扁平列表——与旧格式相同。

### check

检测嵌套的 Skill 并显示相对路径：

```bash
$ skillshare check -g
! utils/remotion  update available · github.com/remotion-dev/skills

! Updates available for 1 skill, 14 up to date · 2.1s

Next
  skillshare update --all  pull the updates
```

### update

同时支持**完整路径**与**短名称**：

```bash
# 完整相对路径
skillshare update -g frontend/react/react-best-practices

# 短名称（basename）——自动解析
skillshare update -g react-best-practices

# 更新所有内容
skillshare update -g --all
```

当短名称匹配多个 Skill 时，skillshare 会要求你更明确地指定：

```
'my-skill' matches multiple items:
  - frontend/my-skill
  - backend/my-skill
Please specify the full path
```

### enable / disable

文件夹让你可以一次性切换整个类别的开关。`disable`/`enable` 接受 glob 模式，因此可以直接指向该文件夹：

```bash
# 停用 frontend/ 下的所有 Skill（任意深度）
skillshare disable "frontend/**"

# 使用相同模式重新启用整个文件夹
skillshare enable "frontend/**"

# 应用到 Target
skillshare sync
```

这会在 `.skillignore` 中写入一行 `frontend/**`，并会持续覆盖你日后添加到该文件夹中的内容。若要切换单个 Skill，请改为传入其名称（`skillshare disable frontend/react/react-best-practices`）。

:::tip 为模式加上引号
将文件夹模式用引号包起来（`"frontend/**"`），以免你的 shell 先展开了 `*`。
:::

详情参见 [enable / disable](/docs/reference/commands/enable) 与 [.skillignore 语法](/docs/reference/filtering#skillignore)。

---

## 直接安装到文件夹中 {#install-directly-into-folders}

使用 `--into` 可以一步将 Skill 安装到子目录中——无需手动 `mv`：

```bash
# 安装到分类文件夹中
skillshare install anthropics/skills -s pdf --into frontend
# → ~/.config/skillshare/skills/frontend/pdf/

# 多层嵌套
skillshare install ~/my-skill --into frontend/react
# → ~/.config/skillshare/skills/frontend/react/my-skill/

# 也可搭配 --track 使用
skillshare install github.com/team/skills --track --into devops
# → ~/.config/skillshare/skills/devops/_team-skills/

# 在 Project mode 下也可使用
skillshare install anthropics/skills -s pdf --into tools -p
# → .skillshare/skills/tools/pdf/
```

执行 `skillshare sync` 后，Target 会显示自动扁平化后的名称：
- `frontend/pdf/` → `frontend__pdf`
- `frontend/react/my-skill/` → `frontend__react__my-skill`
- `devops/_team-skills/frontend/ui/` → `devops___team-skills__frontend__ui`

:::tip
`--into` 会自动创建中间目录，无需先手动 `mkdir`。
:::

---

## 建议的文件夹结构

### 依领域划分

```
skills/
├── frontend/
│   ├── react/
│   ├── vue/
│   └── css/
├── backend/
│   ├── api-design/
│   └── database/
├── devops/
│   ├── docker/
│   └── ci-cd/
└── utils/
    ├── git-workflow/
    └── code-review/
```

### 依工具生态划分

```
skills/
├── vue/
│   ├── vue-best-practices/
│   ├── vue-debug-guides/
│   ├── vue-pinia-best-practices/
│   └── vue-router-best-practices/
├── react/
│   └── react-best-practices/
└── web/
    ├── accessibility/
    ├── performance/
    └── seo/
```

### 混合式：个人 + 已追踪的仓库

```
skills/
├── frontend/              # 个人组织的 Skill
│   └── vue/
├── utils/                 # 个人工具类 Skill
│   └── ascii-box-check/
├── _team-skills/          # 已追踪的仓库（自动更新）
│   ├── code-review/
│   └── deploy/
└── _org-standards/        # 另一个已追踪的仓库
    └── security/
```

---

## 对你的 Skill 进行版本控制

将 Skill 组织到文件夹中，与 git 天然契合：

```bash
skillshare init --remote git@github.com:yourname/my-skills.git
skillshare push -m "organize skills into categories"
```

这会为你带来：
- **历史记录**：跨机器追踪 Skill 变更
- **备份**：透过 GitHub/GitLab
- **共享**——其他人可以浏览并 fork 你的集合
- **跨机器同步**：透过 `skillshare pull`（参见 [Cross-Machine Sync](/docs/how-to/sharing/cross-machine-sync)）

---

## 从扁平结构迁移到文件夹结构

:::tip 全新安装
若是新的 Skill，请使用 `--into` 直接安装到正确的文件夹——参见上方的[直接安装到文件夹中](#install-directly-into-folders)。
:::

你自己建立的 Skill（用 `skillshare new` 或手动创建）没有安装记录，可以直接用 `mv` 移动：

```bash
cd ~/.config/skillshare/skills
mkdir -p frontend/react
mv my-react-notes frontend/react/
skillshare sync
```

执行 `sync` 后，Target 会自动更新——旧的扁平符号链接会被清理，并创建新的扁平化名称。

:::warning 通过 `skillshare install` 安装的 Skill
不要用 `mv` 移动已安装的 Skill。`.metadata.json` 里的安装记录以路径为键，移动后的副本会失去来源链接，记录仍指向旧位置。请改为重新安装到文件夹中：

```bash
skillshare list -v react-best-practices            # 记下 Source
skillshare uninstall react-best-practices
skillshare install <source> --into frontend/react
skillshare sync
```

原本安装时用过的参数（例如 `--track`）也要一并加上。
:::

---

## 另请参阅

- [Source & Targets](/docs/understand/source-and-targets) —— 扁平化的工作原理
- [Tracked Repositories](/docs/understand/tracked-repositories) —— 仓库中的嵌套 Skill
- [Best Practices](./best-practices.md) —— 命名规范
- [install](/docs/reference/commands/install) —— 使用 `--into` 安装到子目录
