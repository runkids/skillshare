---
sidebar_position: 4
---

# Hub Index Guide

为你的组织建立集中式 Skill 目录——无需 GitHub API 或 token。

## Why Use a Hub Index?

Hub index 是一个 JSON 文件（`skillshare-hub.json`），列出各 Skill 的名称、描述与来源。将它托管在内部，团队所有成员就能搜索并安装其中的 Skill。

| 使用场景 | GitHub Search | Hub Index |
|----------|--------------|-----------|
| 组织级 Skill 目录 | 否 | **是** |
| 私有/内部 Skill | 否 | **是** |
| 内网隔离 / 仅限 VPN 的环境 | 否 | **是** |
| 经过筛选、核准的 Skill 集合 | 否 | **是** |
| 不需要 GitHub token | 否 | **是** |

真实案例可参见 [Public Hub](#public-hub) 一节。

## Quick Start

### 1. 建立 Index

```bash
# 从你的 global Skill 建立
skillshare hub index

# 从 project 建立
skillshare hub index -p

# 输出：<source>/skillshare-hub.json
```

### 2. 搜索 Index

```bash
# 本地文件
skillshare search react --hub ./skillshare-hub.json

# 远端 URL
skillshare search react --hub https://internal.corp/skills/skillshare-hub.json

# 浏览所有 Skill（不带查询字符串）
skillshare search --hub ./skillshare-hub.json --json
```

### 3. 从结果安装

交互式搜索流程与 GitHub search 相同——选择一个 Skill 即可安装。

## Audit Enrichment

为你的 index 加上安全风险评分，让团队成员一眼看出 Skill 的安全性：

```bash
# 建立带 audit 评分的 index
skillshare hub index --audit

# 搭配完整元数据
skillshare hub index --full --audit
```

使用 `--audit` 时，每个 Skill 都会以 `skillshare audit` 的规则扫描，index 会包含 `riskScore`（0–100）、`riskLabel`（clean/low/medium/high/critical）以及 `auditedAt` 时间戳。扫描失败的 Skill 会被纳入但不含风险字段。

来自已 audit index 的搜索结果会显示风险徽章：

```
  1. safe-skill               owner/repo/safe-skill         [clean]
  2. risky-skill              owner/repo/risky-skill        [high]
```

## Sharing Strategies

### File Share（最简单）

将 index 文件复制到共享位置：

```bash
skillshare hub index -o /shared/team/skillshare-hub.json
```

团队成员这样搜索：
```bash
skillshare search --hub /shared/team/skillshare-hub.json
```

### HTTP Server

先在本地生成 index，再上传到你的托管服务：

```bash
# 步骤 1：生成
skillshare hub index -o ./skillshare-hub.json

# 步骤 2：上传（使用你偏好的方式）
scp ./skillshare-hub.json server:/var/www/skills/
# 或：aws s3 cp ./skillshare-hub.json s3://my-bucket/
# 或：rsync、FTP 等
```

团队成员这样搜索：
```bash
skillshare search --hub https://skills.company.com/skillshare-hub.json
```

### Git Repository

将 index commit 到共享仓库，让团队成员可以拉取：

```bash
skillshare hub index -o ./skillshare-hub.json
git add skillshare-hub.json && git commit -m "Update skill index"
git push
```

团队成员可以透过 raw URL、SSH，或克隆到本地后搜索：
```bash
# 透过 raw URL
skillshare search --hub https://raw.githubusercontent.com/team/skills/main/skillshare-hub.json

# 透过 SSH —— 会自动克隆仓库并读取 index（无需手动 clone）
skillshare search --hub git@github.com:team/skills.git
skillshare search --hub git@ghe.corp.com:team/skills.git//hubs/team.json

# 或克隆后在本地搜索
git pull
skillshare search --hub ./skillshare-hub.json
```

:::tip Private 与 GitHub Enterprise 仓库
SSH hub 来源会使用你的 SSH agent/密钥来克隆，因此适用于私有仓库以及 raw HTTPS URL 会被重定向到登录页的 GitHub Enterprise（GHE）主机。仓库内的 index 路径来自 `//path` 后缀，默认为仓库根目录下的 `skillshare-hub.json`。scp 风格（`git@host:org/repo.git`）与 scheme 风格（`ssh://git@host/org/repo.git`）的 URL 都可使用。用 [`hub add`](/docs/reference/commands/hub#hub-add) 保存一次后，即可用标签（label）搜索。

当以 SSH 方式加载 GitHub/GHE 的 hub 时，同一主机、带域名前缀的 Skill 来源会继承该 hub 的 SSH 身份。举例来说，hub URL 为 `acme@acme.ghe.com:Org/skills.git//hubs/team.json` 时，条目来源 `acme.ghe.com/Org/skills/skills/reviewer` 就能以 SSH 方式安装。若 hub 是透过 HTTP、本地文件或不同主机加载，带域名前缀的来源仍会维持 HTTPS 来源。
:::

## Web Dashboard

### 不写 JSON 也能建立 Hub

在仪表板（`skillshare ui`）中打开 **Skill → Hubs**，再选择 **添加或创建 Hub → 创建新的 Hub**。新的 Hub 会直接进入编辑。

1. 为 Hub 填写 **名称** 和可选的 **说明**。名称会成为你分享的 `skillshare hub add` 指令中的 `--label`；两者都不会包含在导出的 index 中。
2. 选择 **添加 skill**。在 **粘贴网址** 标签页输入 **Git 网址**，选择 **查找**，挑选 **版本**，再勾选要添加的 Skill。在 **已安装的** 标签页可挑选本机已安装的 Skill。也可以选择 **找不到？手动填写来源** 添加一行空白条目。
3. 编辑每个 Skill 的 **名称**、**来源** 与 **版本**。例如，`runkids/demo-skills/skills/pdf` 用来指向远端仓库中的某个 Skill。展开该行可编辑 **技能说明**、**标签（逗号分隔）** 与 **技能选择器（可选）**，后者用于在包含多个 Skill 的仓库中选择 Skill。
4. 选择 **保存**。此页面会检查每个条目；若有别人装不了的 Skill，编辑器会保持打开并标示该行。
5. 选择 **分享 → 下载 skillshare-hub.json**。在用 **编辑** 修好被标示的 Skill 之前，无法下载。
6. 将下载的文件 commit 到你自己的 Git 仓库，或上传到 HTTP 服务器。在 **分享** 对话框中粘贴该网址，即可复制一条给接收者使用的 `skillshare hub add` 指令。网址会随 Hub 一起保存。

下载并不会发布任何内容。此目录只引用 Skill，不会打包其文件内容。来源验证只检查语法，不检查仓库是否存在或接收者是否具有权限。私有仓库仍然需要相应的访问权限。

:::tip 本地 Skill 也可留在 Hub 中
没有已知远端来源的已安装 Skill，仍会保留本地来源，你可以将它保存进 Hub。下载会被阻挡，直到你提供远端安装来源或移除该条目；建构器绝不会默默地略过它。
:::

### 恢复或导入目录

你自己的 Hub 会在 Hub 列表中标为 **我的**。它们保存在运行仪表板的机器上，位于当前设置文件旁的 `hub-drafts/` 目录。Global 与 project 设置各自有独立的 Hub。重新加载前请先选择 **保存**。编辑期间 Hub 列表会锁定；带着未保存的变更离开时会提示你是否放弃。来自过期窗口的保存会被拒绝，以免覆盖更新的版本。**取消** 会重新加载最新保存的版本。

对既有的 v1 版 `skillshare-hub.json`（最大 4 MB）使用 **添加或创建 Hub → 导入 skillshare-hub.json**。不支持的版本与无效的字段类型会产生错误。显示名称相同的条目仍会各自独立保留。额外的 JSON 字段与 `skill` 选择器会被保留。若旧版 index 含有 `sourcePath`，相对来源会依照既有 index 读取器的方式解析为本地路径；在导出前必须先改为远端来源。

**更多操作 → 删除 Hub** 会要求确认，且只会删除该 Hub，不会卸载 Skill、删除已托管的 index，或移除已订阅的 Hub。

### 搜索已分享的 Hub

1. 打开 **Skill → 安装**，选择 **搜索**。
2. 在 **来源** 选择器中选择一个 Hub。若要添加 URL、SSH 仓库或本地 index 路径，选择选择器旁的 **管理 Hub**，再在 Hubs 页面选择 **添加或创建 Hub → 添加已有的 Hub**。
3. 搜索、预览并安装 Skill。

你也可以在 Hubs 页面选择一个 Hub，直接过滤并安装其中的 Skill。已订阅的 Hub 来源会保存在当前的 skillshare 设置中，并与 CLI 共享。它们与你自己的 Hub 是分开的。

既有的 `skillshare hub index` 指令与 `/api/hub/index` 端点会继续照旧生成 index，包含对本地来源的支持。上述可移植导出规则同样适用于仪表板中的建构器。

## Index Schema

此 index 遵循 Schema v1：

```json
{
  "schemaVersion": 1,
  "generatedAt": "2026-02-12T10:00:00Z",
  "sourcePath": "/home/user/.config/skillshare/skills",
  "skills": [
    {
      "name": "my-skill",
      "description": "Does something useful",
      "source": "owner/repo/.claude/skills/my-skill",
      "tags": ["workflow", "productivity"]
    }
  ]
}
```

### 必要字段（消费端契约）

| 字段 | 是否必要 | 说明 |
|-------|----------|------|
| `name` | 是 | Skill 显示名称 |
| `source` | 是 | 安装来源（GitHub 简写、URL 或本地路径） |
| `description` | 建议 | 用于搜索匹配的简短描述 |
| `skill` | 否 | 多 Skill 仓库中的特定 Skill 名称（搭配 `install -s` 使用） |
| `tags` | 否 | 用于筛选与分组的分类标签 |

### 文件级字段

| 字段 | 说明 |
|-------|------|
| `schemaVersion` | 恒为 `1` |
| `generatedAt` | RFC 3339 时间戳 |
| `sourcePath` | 用于解析相对来源的基准路径 |

### Source Path Resolution

当设置了 `sourcePath` 且某个 Skill 的 `source` 是相对路径时，搜索消费端会将两者合并：

```
sourcePath: /home/user/.config/skillshare/skills
source:     _team/frontend-skill
→ resolved: /home/user/.config/skillshare/skills/_team/frontend-skill
```

这可避免相对路径被误判为 GitHub 简写（`owner/repo`）。

### 将来源固定到 Tag 或 Commit

若要将某个条目固定到特定版本，请使用在路径中带有 ref 的网页 URL。`tree/` 或 `blob/`（GitHub）、`-/tree/` 或 `-/blob/`（GitLab）、`src/`（Bitbucket）之后的分支、tag 或 commit SHA 会作为安装的 ref，效果与 `install --branch` 相同：

```json
{
  "name": "reviewer",
  "source": "github.com/owner/repo/tree/v1.2.0/skills/reviewer"
}
```

所有从此 hub 安装的人都会得到该版本，`skillshare update` 也会保留该版本。若要移动固定点，请编辑 index 中的 ref。远程不存在的 ref 会让安装失败，而不会回退到默认分支。

绝对路径、URL 与带域名前缀的路径永远不会被合并：

| 来源模式 | 是否合并？ |
|----------------|---------|
| `_team/my-skill` | 是 |
| `subdir/skill` | 是 |
| `/absolute/path` | 否 |
| `github.com/owner/repo/skill` | 否 |
| `https://...` | 否 |

## Hand-Written Indexes

你也可以不使用 `hub index`，手动建立 index。这对托管在私有基础设施上的内部 Skill 特别有用——那些来源是 GitHub Search 与公开工具永远无法触及的：

```json
{
  "schemaVersion": 1,
  "skills": [
    {
      "name": "company-style",
      "description": "Company coding standards and review checklist",
      "source": "ghe.internal.company.com/platform/ai-skills/company-style",
      "tags": ["quality", "workflow"]
    },
    {
      "name": "deploy-helper",
      "description": "Internal deployment automation",
      "source": "gitlab.internal.company.com/ops/skills/deploy-helper",
      "tags": ["devops"]
    },
    {
      "name": "onboarding",
      "description": "New hire onboarding skill for AI assistants",
      "source": "ghe.internal.company.com/hr/ai-skills/onboarding",
      "tags": ["workflow"]
    }
  ]
}
```

:::tip 为什么不直接用 GitHub Search？
`skillshare search` 只能找到 github.com 上的公开仓库。Hub index 却能指向**任何**来源——GitHub Enterprise、私有 GitLab、内部服务器——这些只有在 VPN 之后的员工才能访问。这正是 hub 成为组织级 Skill 分发首选方案的原因。
:::

手写 index 的小提示：
- `sourcePath` 是可选的——若所有来源都是绝对路径可省略
- `tags` 是可选的——有助于在网站或搜索中筛选
- `name` 为空的 Skill 会被跳过
- 结果会按名称字母顺序排序
- 若只透过 SSH 安装 GitHub Enterprise，建议使用明确的 SSH 来源（`user@host:owner/repo.git//path`），或以 SSH 方式加载 hub 本身，让同一主机的 GitHub/GHE 带域名前缀条目继承该 SSH 身份

## Organization Deployment

私有 hub 提供可搜索的已审查 skill 目录。将目录与 skill 来源放在组织掌控的基础设施；认证与访问控制由 Git 主机或 HTTP 服务器提供。

### 1. 策展 skills 与来源

通过 PR 审查 skill 与目录的变更。若 Git 主机只支持 SSH，请在[索引条目](#hand-written-indexes)使用明确的 SSH 来源，例如：

```json
{
  "schemaVersion": 1,
  "skills": [
    {
      "name": "code-review",
      "description": "Team code-review checklist",
      "source": "git@ghe.example.com:platform/ai-skills.git//skills/code-review"
    }
  ]
}
```

也可以使用 `skillshare hub index --audit`，从已安装的远端 skills 生成目录。发布前，确认每个来源都能让团队成员访问。从本地文件创建的索引可能含有该机器的本地路径，请替换成共享来源。Audit 标记描述的是某个时间点的扫描结果，不是永久批准。

### 2. 使用经审查的 CLI 版本审计变更

在 skill 仓库中，使用固定的 CLI 版本与严重级别阈值，作为 PR 的检查关卡：

```yaml
name: Validate shared skills
on:
  pull_request:
    paths: ['skills/**', 'skillshare-hub.json']

jobs:
  audit:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      # Tags 仅为便于阅读；请将每个 Action 固定至经过审查的 commit SHA
      - uses: actions/checkout@v4
      - uses: runkids/setup-skillshare@v1
        with:
          version: '0.23.5' # 示例：选择团队已审查的 CLI 版本
          source: ./skills
          audit: true
          audit-threshold: high
```

此示例假设 skills 位于 checkout 后的仓库的 `skills/` 目录。若使用内部 Git 服务器，请应用 CI runner 的 checkout 与访问配置；扫描命令相同。其他 CI 系统请见[CI/CD Skill 验证](/docs/how-to/recipes/ci-cd-skill-validation)。

Action 的 `version` input 固定的是 CLI release，不是 Action 本身或 skill 内容。示例为了易读而使用 tags；请按组织策略，将每个 Action 固定至经过审查的完整 commit SHA。使用[项目 lockfile](/docs/understand/project-skills#lockfile)记录远端 skill commit，并分别审查这几类更新。`hub index --audit` 将扫描结果加到目录；要拒绝达到该严重级别的发现，请使用 `skillshare audit --threshold high` 或上述流水线关卡。

### 3. 私下发布目录

审查后，将 `skillshare-hub.json` 提交到内部 skill 仓库的根目录，通过 Git 主机授予团队成员读取权限。若团队环境能获取索引，也可以使用内部 HTTP 托管。不需要 fork 公开 hub，也不必提供公开 raw URL。

### 4. 注册、搜索与同步

[初始化 skillshare](/docs/getting-started/first-sync)后，团队成员只需注册私有目录一次：

```bash
skillshare hub add git@ghe.example.com:platform/ai-skills.git --label company -g
skillshare search code-review --hub company -g
# Select a skill to install, then distribute it to global targets
skillshare sync -g
```

SSH hub URL 默认从仓库根目录读取 `skillshare-hub.json`。若目录放在其他位置，请在 URL 后附上路径，例如 `git@ghe.example.com:platform/ai-skills.git//catalog/skillshare-hub.json`。SSH 访问沿用团队成员已有的 SSH 配置。Git 主机必须授权访问目录与各 skill 来源。Hub 是发现机制，不会阻止从其他来源安装。

### 5. 记录项目依赖

单个项目需要的 skills，请用 Project mode 安装，并提交产生的 config 与 lockfile。团队成员在 clone 或 pull 更新后，运行 `skillshare install -p`、audit 与 sync。顺序请见[团队入职](/docs/how-to/recipes/team-onboarding-recipe)。目录策展、skill 更新与 CLI 升级都应作为明确且经过审查的变更。

## Public Hub

[skillshare-hub](https://github.com/runkids/skillshare-hub) 是一个精选的高质量 Skill 目录。它是**默认 hub**——当你运行 `search --hub` 且未指定来源时，就会搜索这里：

```bash
skillshare search --hub              # 浏览 public hub 中的所有 Skill
skillshare search react --hub        # 搜索 "react" 相关的 Skill
```

它也可作为建立你自己组织 hub 的参考：

- **Index 结构** —— 如何以名称、描述、来源与标签组织 `skillshare-hub.json`
- **CI validation** —— 每次 PR 都会自动检查 JSON 格式并执行 `skillshare audit` 安全扫描
- **Contribution workflow** —— Fork → 新增条目 → PR，并有 CI 关卡把关

想为团队建立内部 hub？Fork 此仓库，将其中的 Skill 换成你组织自己的目录，并依你的安全政策调整 CI 流水线。

## Tips

- **自动生成 index** —— 在 Skill 变更后的 CI 流水线中加入 `skillshare hub index`
- **audit 时使用 `--full`** —— Full 模式包含版本、安装日期与类型信息
- **搭配 project mode** —— `skillshare hub index -p` 只会为 project 层级的 Skill 建立 index

---

## See Also

- [search](/docs/reference/commands/search) — 从 hub 搜索 Skill
- [hub](/docs/reference/commands/hub) — 管理 hub 来源
- [install](/docs/reference/commands/install) — 安装找到的 Skill
