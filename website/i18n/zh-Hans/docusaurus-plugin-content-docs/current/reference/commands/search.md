---
sidebar_position: 5
---

# search

从 GitHub 仓库中发现并安装 skills。

## 何时使用

- 从 hub 索引中发现社区 skills
- 按名称、标签或描述查找 skills
- 在安装前浏览可用的 skills

## 快速开始

```bash
skillshare search vercel       # 按关键词搜索
skillshare search              # 浏览热门 skills
```

此命令会在 GitHub 中搜索包含 `SKILL.md` 文件、且匹配你查询内容的仓库。

## 浏览模式

不提供查询时，`search` 会浏览 GitHub 上的热门 skills：

```bash
skillshare search              # 浏览热门 skills
skillshare search --list       # 列出热门 skills
```

它使用 `filename:SKILL.md` 作为 GitHub 查询条件，并按 star 数排序，优先展示最受欢迎的 skill 仓库。

## 工作原理

```
skillshare search [query]
        │
        ▼
GitHub Code Search API (filename:SKILL.md + query)
        │
        ▼
Fetch star counts for each repository
        │
        ▼
Sort by stars (most popular first)
        │
        ▼
Interactive selector → Install selected skill
```

## 选择要安装的项目

在终端中，结果会在选择器里打开，当前所在 skill 的详情显示在列表旁。可以选中多个一起安装；没有选中时，`Enter` 会安装当前所在的那一个。按键列在屏幕底部。

## 选项

| 标志 | 说明 |
|------|------------|
| `--project`, `-p` | 安装到 project 级别配置（`.skillshare/`） |
| `--global`, `-g` | 安装到 global 配置（`~/.config/skillshare`） |
| `--hub [URL]` | 从 hub 索引中搜索（默认：[skillshare-hub](https://github.com/runkids/skillshare-hub)；或自定义 URL/路径） |
| `--list`, `-l` | 仅列出结果，不提示安装 |
| `--json` | 以 JSON 输出（用于脚本化） |
| `--limit N`, `-n N` | 最大结果数（默认：20，最大：100） |
| `--help`, `-h` | 显示帮助 |

:::tip 自动检测
如果既未指定 `--project` 也未指定 `--global`，skillshare 会自动检测：如果当前目录存在 `.skillshare/config.yaml`，则默认使用 project 模式；否则使用 global 模式。
:::

## 示例

### 浏览热门

```bash
skillshare search              # Browse popular skills (no query)
```

### 基础搜索

```bash
skillshare search pdf           # Interactive search and install
skillshare search "code review" # Multi-word search
```

### 列表模式

```bash
skillshare search commit --list
```

输出：
```
  fix            facebook/react/.claude/skills/fix         ★ 242.7k
                 Use when you have lint errors, formatting issues...
  verify         facebook/react/.claude/skills/verify      ★ 242.7k
                 Use when you want to validate changes before committing...
  commit-helper  ...ockroach/.claude/skills/commit-helper  ★ 31.8k
                 Help create git commits and PRs with properly formatted messages...

Found 3 skills

Next
  skillshare install <source>  install one
```

### JSON 输出

```bash
skillshare search react --json --limit 5
```

```json
[
  {
    "Name": "react-patterns",
    "Description": "React and Next.js performance optimization...",
    "Source": "facebook/react/.claude/skills/react-patterns",
    "Stars": 242700,
    "Owner": "facebook",
    "Repo": "react",
    "Path": ".claude/skills/react-patterns"
  }
]
```

### Project 模式

```bash
skillshare search pdf -p           # Search and install to project
skillshare search react --project  # Same thing, long flag
```

已安装的 skills 会保存到 `.skillshare/skills/`，project 配置也会自动更新。如果该 project 尚未初始化，skillshare 会先运行 `init -p`。

### 限制结果数量

```bash
skillshare search frontend -n 5   # 仅显示前 5 个结果
```

## 身份验证 {#authentication}

GitHub Code Search API 需要身份验证。skillshare 会自动检测你的凭证：

1. **GitHub CLI**（推荐）—— 如果你已通过 `gh` 登录：
   ```bash
   gh auth login
   ```

2. **环境变量** —— 设置 `GITHUB_TOKEN` 或 `GH_TOKEN`：
   ```bash
   export GITHUB_TOKEN=ghp_your_token_here
   ```

### 创建 Token

如果你不使用 `gh` CLI：

1. 前往 [GitHub Settings → Tokens](https://github.com/settings/tokens)
2. 生成新 token（classic）
3. 公开仓库无需任何 scope
4. 设置该 token：
   ```bash
   export GITHUB_TOKEN=ghp_your_token_here
   ```

## 结果排序方式

1. **搜索** —— GitHub Code Search 查找匹配你查询内容的 `SKILL.md` 文件
2. **过滤** —— 移除 fork 出来的仓库（重复项）
3. **获取 Star 数** —— 获取每个唯一仓库的 star 数
4. **排序** —— 按 star 数排序（最受欢迎的排在前面）
5. **限制** —— 返回前 N 个结果

这可以确保高质量、受欢迎的 skills 排在前面。

## 社区 Hub

浏览并安装来自 [skillshare-hub](https://github.com/runkids/skillshare-hub) 的社区精选 skills：

```bash
skillshare search --hub                # Browse all skills in skillshare-hub
skillshare search react --hub          # Search "react" in skillshare-hub
```

当 `--hub` 未指定 URL 时，默认使用社区的 [skillshare-hub](https://github.com/runkids/skillshare-hub) 索引。

想把你的 skill 分享给社区？[提交一个 PR](https://github.com/runkids/skillshare-hub) 来添加你的 skill —— CI 会对每次提交运行 `skillshare audit`。

## 已保存的 Hub 标签

使用 [`hub add`](./hub.md#hub-add) 保存 hub，之后可以用标签而不是完整 URL 进行搜索：

```bash
# Save a hub once
skillshare hub add https://internal.corp/hub.json --label team

# Search by label
skillshare search react --hub team

# Set as default for bare --hub
skillshare hub default team
skillshare search --hub              # Uses "team" hub
```

`--hub <value>` 的解析顺序：
1. URL 或路径（以 `http`、`/`、`.`、`~`、`file://` 开头，或形如 `git@…`/`ssh://…` 的 SSH URL）→ 直接使用
2. 否则 → 按标签查找已保存的 hub
3. 单独的 `--hub`（不带值）→ 配置中的默认值 → 社区 hub 兜底

hub 管理参见 [`hub`](./hub.md)。

## 私有索引搜索 {#private-index-search}

从私有 hub 索引而非 GitHub 进行搜索：

```bash
# Local file
skillshare search react --hub ./skillshare-hub.json

# HTTP URL
skillshare search react --hub https://internal.corp/skills/skillshare-hub.json

# SSH URL — clones the repo and reads the index (works with private/GHE hosts)
skillshare search react --hub git@github.com:org/skills.git
skillshare search react --hub git@ghe.corp.com:team/skills.git//hubs/team.json

# Browse all skills (empty query)
skillshare search --hub ./skillshare-hub.json --json

# Equals syntax also works
skillshare search react --hub=./skillshare-hub.json
```

:::note SSH hub 来源
SSH 形式的 `--hub` 值会通过浅克隆（shallow clone）该仓库（使用你的 SSH agent/密钥）来解析，并从中读取索引文件。仓库内的文件路径由 `//path` 后缀指定——`git@host:org/repo.git//hubs/team.json`——如果省略，则默认为仓库根目录下的 `skillshare-hub.json`。scp 风格（`git@host:org/repo.git`）和 scheme 风格（`ssh://git@host/org/repo.git`）的 URL 均受支持。

在 [web dashboard](./ui.md) 中，SSH hub 来源必须先[被保存](./hub.md#hub-add)；服务端只会克隆已保存的 hub。
:::

使用 [`hub index`](./hub.md) 构建索引：

```bash
skillshare hub index                           # Generate skillshare-hub.json
skillshare search --hub ./skillshare-hub.json  # Search it
```

:::tip 默认 Hub
`skillshare search --hub`（不带 URL）默认使用社区 [skillshare-hub](https://github.com/runkids/skillshare-hub) 索引，因此你无需每次都输入完整 URL。也可以用 `skillshare hub default <label>` 设置你自己的默认值。
:::

更多详情参见 [Hub Index Guide](/docs/how-to/sharing/hub-index)。

## 使用技巧

### 查找官方 Skills

搜索知名组织：
```bash
skillshare search anthropic    # Anthropic's skills
skillshare search facebook     # Meta/Facebook skills
skillshare search vercel       # Vercel's skills
```

### 查找特定功能

按你想做的事情搜索：
```bash
skillshare search "pull request"
skillshare search deployment
skillshare search testing
skillshare search database
```

### 连续搜索

找不到结果，或在选择器中按 `Esc` 时，会再询问新的关键字：

```
? Search again (or press Enter to quit): react
```

## 故障排查

### "GitHub Code Search API requires authentication"

运行 `gh auth login` 或设置 `GITHUB_TOKEN`。参见[身份验证](#authentication)。

### "GitHub API rate limit exceeded"

- 已认证用户：Code Search 限速为 30 次请求/分钟
- 等待一分钟后重试
- 使用 `--limit` 减少 API 调用次数

### 找不到新仓库

GitHub 索引新仓库存在延迟（数小时到数天）。如果找不到你的仓库：
- 直接安装：`skillshare install owner/repo/path/to/skill`
- 等待 GitHub 完成索引

### 结果与查询不匹配

GitHub Code Search 匹配的是 `SKILL.md` 文件内的内容。一个在描述中提到 "vercel" 的 skill，即便它本身与 Vercel 无关，也会出现在 vercel 相关的搜索结果中。

安装前请使用 `--list` 查看结果。
