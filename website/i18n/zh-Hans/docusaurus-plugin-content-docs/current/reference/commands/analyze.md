---
sidebar_position: 4
---

# analyze

分析每个 target 中 skills 的上下文窗口用量和 skill 质量。

```bash
skillshare analyze                    # 交互式 TUI（默认）
skillshare analyze claude             # 单个 target 的详情
skillshare analyze --verbose          # 描述最长的 10 个 skill
skillshare analyze --json             # 机器可读输出
skillshare analyze -p                 # Project mode
```

## 何时使用

### 优化上下文预算

找出哪些 skills 消耗了最多的上下文窗口 token：

```bash
skillshare analyze           # 交互式浏览所有 target
```

### 跨 Target 比较

查看不同 target（例如 Claude 与 Cursor）之间上下文用量的差异：

```bash
skillshare analyze           # 在 TUI 中按 Tab 切换 target
```

### 检查 Skill 质量

找出缺少字段、描述过短，或没有触发短语的 skills：

```bash
skillshare analyze           # TUI 中会出现 lint 图标（✗/⚠）
```

### CI/脚本化

获取机器可读的上下文指标和 lint 结果：

```bash
skillshare analyze --json | jq '.targets[].always_loaded.estimated_tokens'
skillshare analyze --json | jq '.targets[].skills[] | select(.lint_issues | length > 0)'
```

## 它做了什么

`analyze` 为每个 skill 计算两层上下文开销：

1. **Always loaded** —— 来自 SKILL.md frontmatter 的 `name + description`（每次请求都会为 skill 匹配加载到上下文中）
2. **On-demand** —— frontmatter 之后的 skill 正文（仅在该 skill 被触发时加载）

token 估算使用 `chars / 4` 作为近似值。

### Skill 质量 Lint

除了 token 分析之外，`analyze` 还会对每个 skill 运行一个内置的 lint 引擎。lint 规则检查 SKILL.md 的结构和描述质量，并在 TUI 和 JSON 输出中直接展示问题。

| 规则 | 严重程度 | 检查内容 |
|------|----------|----------------|
| `missing-name` | error | `name` 字段为空或缺失 |
| `missing-description` | error | `description` 字段为空或缺失 |
| `empty-body` | error | skill 正文（frontmatter 之后）为空 |
| `description-too-short` | warning | 描述不足 50 个字符 |
| `description-too-long` | warning | 描述超过 1024 字符的目标上限 |
| `description-near-limit` | warning | 描述介于 900–1024 字符之间 |
| `no-trigger-phrase` | warning | 描述缺少触发短语（例如 "Use when…"） |

在 TUI 中，存在 lint 问题的 skills 会在名称旁显示 ✗（error）或 ⚠（warning）图标。详情面板包含一个 **Quality** 区块，列出所有 findings。

## 交互式 TUI

在 TTY 上，`analyze` 会打开交互式界面，一次查看一个 target：左侧是按 token 用量排序的 skills，右侧是所选 skill 的 token 明细、lint 问题和描述。打开 skill 会显示文件原文（含 front matter），可以直接对照 lint 问题。按键列在界面底部。使用 `--no-tui` 改为输出纯文本。

### 颜色编码

token 消耗等级使用每个 target 各自动态计算的百分位阈值：

| 颜色 | 含义 |
|-------|---------|
| 🔴 红色 | P75+（消耗最高的前 25%） |
| 🟡 黄色 | P25–P75（中间的 50%） |
| 🟢 绿色 | 低于 P25（最低的 25%） |

## 示例输出

### 默认（--no-tui）

```
claude · 7 skills
  Always loaded  ~362 tokens
  On-demand max  ~22 tokens

  Add -v for the 10 largest descriptions
```

### Verbose

```
skillshare analyze --verbose

claude · 7 skills
  Always loaded  ~362 tokens
  On-demand max  ~22 tokens

  Largest descriptions
    my-big-skill   ~180 tokens
    another-skill  ~120 tokens
    …
```

### 单个 Target

传入一个 target 名称会自动启用 verbose 输出：

```bash
skillshare analyze claude
```

### 按 Group 过滤

```bash
# 查看所有 frontend skills 的总 token 开销
skillshare analyze claude --json --filter frontend

# 预先填充 TUI 中的搜索框
skillshare analyze --filter marketing
```

## Options

| Flag | 说明 |
|------|------|
| `[target]` | 显示单个 target 的详情（自动启用 verbose） |
| `--verbose`, `-v` | 显示每个 target 描述最长的 10 个 skill |
| `--no-tui` | 关闭交互式 TUI，改为打印纯文本 |
| `--project`, `-p` | 分析 project 级 skills（`.skillshare/`） |
| `--global`, `-g` | 分析 global 级 skills（`~/.config/skillshare`） |
| `--filter <text>` | 按名称/路径子串过滤 skills |
| `--json` | 以 JSON 格式输出（用于脚本/CI） |
| `--help`, `-h` | 显示帮助信息 |

:::tip 自动检测
如果既没有指定 `--project` 也没有指定 `--global`，skillshare 会自动检测：如果当前目录存在 `.skillshare/config.yaml`，默认使用 project mode；否则使用 global mode。
:::

## JSON 输出

```bash
skillshare analyze --json
```

```json
{
  "targets": [
    {
      "name": "claude",
      "skill_count": 7,
      "always_loaded": {
        "chars": 1448,
        "estimated_tokens": 362
      },
      "on_demand_max": {
        "chars": 88,
        "estimated_tokens": 22
      },
      "skills": [
        {
          "name": "my-skill",
          "description_chars": 180,
          "description_tokens": 45,
          "body_chars": 400,
          "body_tokens": 100,
          "lint_issues": [
            {
              "rule": "no-trigger-phrase",
              "severity": "warning",
              "category": "format",
              "message": "Description lacks trigger phrases (e.g. 'Use when...'); agents may not know when to invoke this skill"
            }
          ]
        }
      ]
    }
  ]
}
```

没有 lint 问题的 skills 会省略 `lint_issues` 字段。

## Project Mode

```bash
skillshare analyze -p                  # project skills 的交互式 TUI
skillshare analyze -p --verbose        # verbose 文本输出
skillshare analyze -p claude           # 单个 target 的详情
skillshare analyze -p --json           # JSON 输出
```

## 过滤

使用 `--filter` 把结果缩小到某个子集的 skills。该过滤器对 skill 的相对路径（包含 group 目录）执行不区分大小写的子串匹配。

例如，如果你用 `--into frontend` 安装了 skills：
- `--filter frontend` 匹配 `frontend/` group 下的所有 skills
- `--filter react` 匹配路径中包含 "react" 的任意 skill

在 TUI 模式下，`--filter` 会预先填充过滤输入框。你也可以用 `/` 键交互式地开始过滤。

在 JSON 模式下，输出会包含一个带汇总 token 计数的 `filtered_summary`：

```json
{
  "filter": "frontend",
  "matched_count": 5,
  "total_count": 50,
  "filtered_summary": {
    "always_loaded": { "chars": 2400, "tokens": 600 },
    "on_demand": { "chars": 8000, "tokens": 2000 },
    "total": { "chars": 10400, "tokens": 2600 }
  },
  "skills": [...]
}
```

当搜索或过滤生效时，Web UI 也会显示一个动态的 token 汇总栏。

## Budget Warnings

当配置了 `context_budget` 阈值时，如果任何 target 超出预算，`analyze` 会显示一条警告。配置细节参见 [sync — Context Cost](/docs/reference/commands/sync#context-cost)。

## 另请参阅

- [list](/docs/reference/commands/list) —— 查看已安装的 skills
- [audit](/docs/reference/commands/audit) —— 扫描 skills 的安全威胁
- [tui](/docs/reference/commands/tui) —— 开关交互式 TUI
