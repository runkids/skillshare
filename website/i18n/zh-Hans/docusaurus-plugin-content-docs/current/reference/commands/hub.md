---
sidebar_position: 8
---

# hub

管理 skill hub —— 用于组织范围内 skill 发现的已保存 hub 来源。

## 何时使用

- 为 `search` 查询设置组织的 skill 目录
- 在多个 hub 之间切换（例如公司范围 vs 团队专属）
- 列出或移除已保存的 hub 来源

## hub add

把一个 hub 来源保存到 config 中，以便在 [`search --hub`](./search.md) 中重复使用。

```bash
skillshare hub add <url> [options]
```

| Flag | 说明 |
|------|------|
| `--label`, `-l` | 自定义标签（默认：从 URL 主机名推导） |
| `--project`, `-p` | 保存到 project config |
| `--global`, `-g` | 保存到 global config |

第一个添加的 hub 会自动设为默认。标签不区分大小写。

hub URL 可以是 HTTP(S) URL、本地文件路径，或 SSH URL —— scp 风格（`git@host:org/repo.git`）或 scheme 风格（`ssh://git@host/org/repo.git`）。对于 SSH 来源，索引文件通过 `//path` 后缀从仓库中读取，默认为仓库根目录下的 `skillshare-hub.json`。SSH 来源使用你的 SSH agent/keys 进行克隆，因此可用于私有仓库和 GitHub Enterprise host。

当一个 SSH hub 包含同一 host 上带域名前缀的 GitHub 或 GitHub Enterprise 条目时，Skillshare 会通过该 hub 的 SSH identity 来安装它们。例如，一个以 `acme@acme.ghe.com:Org/skills.git//hubs/team.json` 添加的 hub，可以包含 `acme.ghe.com/Org/skills/skills/reviewer`，搜索结果会把它安装为 `acme@acme.ghe.com:Org/skills.git//skills/reviewer`。在 SSH hub 之外，带域名前缀的来源仍然意味着 HTTPS。

```bash
skillshare hub add https://internal.corp/hub.json --label team
skillshare hub add ./local-hub.json                          # 推导出的标签："local-hub"
skillshare hub add git@ghe.corp.com:team/skills.git --label ghe
skillshare hub add git@ghe.corp.com:team/skills.git//hubs/team.json --label ghe-team
```

## hub list

列出已保存的 hub。`*` 标记默认 hub。

```bash
skillshare hub list [options]
```

| Flag | 说明 |
|------|------|
| `--project`, `-p` | 显示 project hub |
| `--global`, `-g` | 显示 global hub |

```
$ skillshare hub list
  team      https://internal.corp/hub.json · default
  local     ./local-hub.json

2 hubs
```

别名：`hub ls`

## hub remove

按标签移除一个已保存的 hub。

```bash
skillshare hub remove <label> [options]
```

| Flag | 说明 |
|------|------|
| `--project`, `-p` | 从 project config 中移除 |
| `--global`, `-g` | 从 global config 中移除 |

如果被移除的 hub 是默认 hub，默认设置会被清除。

别名：`hub rm`

## hub default

显示或设置 `search --hub`（不带值的 flag）所使用的默认 hub。

```bash
skillshare hub default [label] [options]
```

| Flag | 说明 |
|------|------|
| `--reset` | 清除默认设置（还原为社区 hub） |
| `--project`, `-p` | 使用 project config |
| `--global`, `-g` | 使用 global config |

```bash
skillshare hub default              # 显示当前默认值
skillshare hub default team         # 把默认值设为 "team"
skillshare hub default --reset      # 清除默认值 → 社区 hub
```

## hub index

从已安装的 skills 构建一个 `skillshare-hub.json` 索引文件。生成的索引可以被 [`search --hub`](./search.md#private-index-search) 消费，用于私有、离线的 skill 发现。

### 用法

```bash
skillshare hub index [options]
```

### Options

| Flag | 说明 |
|------|------|
| `--source`, `-s` | 要扫描的 source 目录（默认：自动检测） |
| `--output`, `-o` | 输出文件路径（默认：`<source>/skillshare-hub.json`） |
| `--full` | 包含完整元数据（flatName、type、version 等） |
| `--audit` | 对每个 skill 运行安全审计并纳入风险评分 |
| `--project`, `-p` | 使用 project mode（`.skillshare/`） |
| `--global`, `-g` | 使用 global mode（`~/.config/skillshare`） |
| `--help`, `-h` | 显示帮助信息 |

### 输出模式

**Minimal（默认）**—— 仅包含用于搜索和安装的必要字段：

```json
{
  "schemaVersion": 1,
  "generatedAt": "2026-02-12T10:00:00Z",
  "sourcePath": "/home/user/.config/skillshare/skills",
  "skills": [
    {
      "name": "my-skill",
      "description": "A useful skill",
      "source": "owner/repo/.claude/skills/my-skill",
      "tags": ["workflow"]
    }
  ]
}
```

**Full（`--full`）**—— 包含用于审计和管理的元数据：

```json
{
  "name": "my-skill",
  "description": "A useful skill",
  "source": "github.com/owner/repo/.claude/skills/my-skill",
  "tags": ["workflow"],
  "flatName": "my-skill",
  "type": "github-subdir",
  "repoUrl": "https://github.com/owner/repo.git",
  "version": "abc1234",
  "installedAt": "2026-02-10T03:49:06Z",
  "isInRepo": false
}
```

**Audit（`--audit`）**—— 从 `skillshare audit` 添加安全风险评分：

```json
{
  "name": "my-skill",
  "description": "A useful skill",
  "source": "owner/repo/.claude/skills/my-skill",
  "riskScore": 0,
  "riskLabel": "clean",
  "auditedAt": "2026-02-22T10:00:00Z"
}
```

`--audit` 可以与 `--full` 结合，同时包含元数据和风险评分。风险标签：`clean`（0）、`low`（1–25）、`medium`（26–50）、`high`（51–75）、`critical`（76–100）。

元数据字段使用 `omitempty` —— 冗余值会被省略：
- `flatName` 在等于 `name` 时省略
- `relPath` 在等于 `source` 时省略
- `isInRepo` 在为 `false` 时省略

### 示例

```bash
# 构建 minimal 索引（默认）
skillshare hub index

# 构建包含完整元数据的索引
skillshare hub index --full

# 构建包含安全风险评分的索引
skillshare hub index --audit

# 完整元数据 + 风险评分
skillshare hub index --full --audit

# 自定义输出路径
skillshare hub index -o /shared/team/skillshare-hub.json

# 自定义 source 目录
skillshare hub index -s ~/my-skills

# Project mode
skillshare hub index -p
```

### 工作流程

一个典型的私有 hub 工作流程：

```
1. 安装 skills               → skillshare install ...
2. 构建索引                  → skillshare hub index
3. 分享索引文件               → 提交/托管 skillshare-hub.json（HTTP、文件或 Git 仓库）
4. 团队成员搜索                → skillshare search --hub [path-url-or-ssh]
```

当索引位于私有或 GitHub Enterprise 仓库中时，团队成员可以通过 SSH（`git@host:org/repo.git`）直接把 `--hub` 指向它，而无需先各自本地克隆。

更多细节参见 [Hub Index Guide](/docs/how-to/sharing/hub-index)。

## Config 格式

已保存的 hub 存储在 `config.yaml` 的 `hub:` 键下：

```yaml
hub:
  default: team
  hubs:
    - label: team
      url: https://internal.corp/hub.json
    - label: local
      url: ./local-hub.json
    - label: ghe
      url: git@ghe.corp.com:team/skills.git//hubs/team.json
```

[public hub](https://github.com/runkids/skillshare-hub) 是内置的默认值，不需要保存。当没有设置自定义默认值时，`search --hub` 会自动回退到它。fork 这个仓库即可搭建你自己组织的 hub。
