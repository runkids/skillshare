---
sidebar_position: 4
---

# log

查看持久化的操作日志和 audit 日志，用于调试和合规审计。

```bash
skillshare log                    # 交互式 TUI（TTY 下默认）
skillshare log --audit            # 仅显示 audit log
skillshare log --tail 50          # 显示最近 50 条记录
skillshare log --cmd sync         # 仅显示 sync 相关记录
skillshare log --status error     # 仅显示出错的记录
skillshare log --since 2d         # 最近 2 天的记录
skillshare log --stats            # 显示汇总统计
skillshare log --json             # 以 JSONL 格式输出
skillshare log --no-tui           # 纯文本输出
skillshare log --clear            # 清空操作日志
skillshare log -p                 # project 的操作日志与 audit 日志
```

## 何时使用

- 交互式浏览和筛选日志记录
- 调试某次失败操作时发生了什么
- 出于合规审计或问题排查目的查看审计轨迹
- 按命令、状态或时间范围筛选日志以便调查

## 交互式 TUI

在 TTY 上，`skillshare log` 会打开交互式的操作列表，旁边显示所选条目的详情，并提供过滤和按命令分类的统计界面。操作记录与 audit 记录会合并并按时间排序。按键列在界面底部。

使用 `--no-tui` 可跳过 TUI，改为打印纯文本：

```bash
skillshare log --no-tui           # 纯文本输出
skillshare log --no-tui | less    # 手动通过 pager 分页
```

## 记录内容

每一次会产生变更的 CLI 和 Web UI 操作，都会以 JSONL 记录的形式保存，包含时间戳、命令、状态、耗时和上下文参数。

| 命令 | 日志文件 |
|---------|----------|
| `install`、`uninstall`、`sync`、`push`、`pull`、`collect`、`backup`、`restore`、`update`、`target`、`trash`、`config`、`check`、`diff`、`init`、`upgrade` | `operations.log` |
| `audit` | `audit.log` |

调用这些 API 的 Web UI 操作，其记录方式与 CLI 操作相同。

## 日志类型

### 默认视图
在一份输出中同时显示**两个部分**：
- 操作日志
- Audit 日志

```bash
skillshare log
```

### 仅 Audit 视图

将安全 audit 扫描记录与常规操作分开记录。

```bash
skillshare log --audit
```

### 筛选

按命令、状态或时间范围缩小结果范围。当 `--cmd` 指定的命令只存在于某个日志中时（例如 `--cmd audit` 只出现在 audit.log），无关的部分会被自动跳过。

```bash
skillshare log --cmd install              # 仅显示 install 记录
skillshare log --status error             # 仅显示出错记录
skillshare log --since 1h                 # 最近一小时（也支持 30m、2d、1w）
skillshare log --since 2026-01-15         # 自指定日期起
skillshare log --cmd sync --status error  # 组合筛选
```

### JSON 输出

输出原始 JSONL，便于脚本化和自动化：

```bash
skillshare log --json                     # 以 JSONL 输出所有记录
skillshare log --json --cmd sync          # 输出筛选后的 JSONL
```

## 示例输出（纯文本）

在 terminal 使用 `--no-tui` 时（输出被 pipe 时，每条的细节会合并成一行）：

```
Operations · last 2
✓ sync      2026-02-10 14:35 · 0.3s
            targets: 3
            scope: global

✗ sync      2026-02-10 14:31 · 0.8s · error
            targets: 3
            failed: 1
            scope: global
  ~/.local/state/skillshare/logs/operations.log

Audit · last 1
✗ audit     2026-02-10 14:36 · 1.1s · blocked
            scope: all-skills
            scanned: 12
            passed: 11
            failed: 1
            failed skills:
              - prompt-injection-skill
              - data-exfil-skill
  ~/.local/state/skillshare/logs/audit.log
```

## 日志格式

记录以 JSONL 格式存储（每行一个 JSON 对象）：

```json
{"ts":"2026-02-10T14:30:00Z","cmd":"install","args":{"source":"anthropics/skills/pdf"},"status":"ok","ms":1200}
```

| 字段 | 说明 |
|-------|-------------|
| `ts` | ISO 8601 时间戳 |
| `cmd` | 命令名称 |
| `args` | 命令相关上下文（source、name、target 等） |
| `status` | `ok`、`error`、`partial` 或 `blocked` |
| `msg` | 错误消息（当 status 不是 ok 时） |
| `ms` | 耗时（毫秒） |

## 日志位置

```
~/.local/state/skillshare/logs/operations.log    # Global operations
~/.local/state/skillshare/logs/audit.log         # Global audit
<project>/.skillshare/logs/operations.log   # Project operations
<project>/.skillshare/logs/audit.log        # Project audit
```

## 在 Git 中追踪日志（Project 模式）

Project 模式默认会忽略 `.skillshare/logs/`，以避免产生噪音提交。

如果你的团队希望对日志文件进行版本控制，请在 `.skillshare/.gitignore` 中托管块之后添加以下**用户覆盖**规则：

```gitignore
# User override: track logs
!logs/
!logs/*.log
```

如果你的仓库根目录 `.gitignore` 也忽略了 `.skillshare/`，请同样在其中添加对应的取消忽略规则。

## 选项

| 标志 | 说明 |
|------|------------|
| `-a`, `--audit` | 仅显示 audit log |
| `-t`, `--tail <N>` | 显示最近 N 条记录（默认：20） |
| `--cmd <name>` | 按命令名称筛选（例如 `sync`、`install`、`audit`） |
| `--status <status>` | 按状态筛选（`ok`、`error`、`partial`、`blocked`） |
| `--since <dur\|date>` | 按时间筛选（`30m`、`2h`、`2d`、`1w`，或 `2006-01-02`） |
| `--stats` | 显示汇总统计（总数、成功率、按命令细分） |
| `--json` | 输出原始 JSONL（每行一个 JSON 对象） |
| `--no-tui` | 禁用交互式 TUI，使用纯文本输出 |
| `-c`, `--clear` | 清空指定的日志文件（默认为操作日志，配合 `--audit` 则为 audit log） |
| `-p`, `--project` | 使用 project 级别的日志 |
| `-g`, `--global` | 使用 global 日志 |
| `-h`, `--help` | 显示帮助 |

## Web UI

日志同样可在 `/log` 的 web dashboard 中查看：

```bash
skillshare ui
# Navigate to Log page
```

Log 页面提供：
- **标签页**：`All`、`Operations` 和 `Audit`
- **筛选项**：命令、状态和时间范围（1h、24h、7d、30d）
- **表格视图**，展示时间、命令、详情、状态和耗时
- **Audit 详情行**，在存在失败/警告 skill 时显示其名称
- **Clear** 和 **Refresh** 控件

## 统计视图

### CLI

```bash
skillshare log --stats                # 全部操作的汇总
skillshare log --stats --cmd sync     # 仅 sync 的统计
skillshare log --stats --since 7d     # 最近 7 天的统计
```

## 日志保留

日志会自动截断以防止无限增长。默认限制为**每个日志文件 1000 条记录**（每次 CLI 或 Web UI 操作 = 1 条记录）。`operations.log` 和 `audit.log` 分别独立计数。

如需覆盖默认值，请在 `config.yaml` 中添加：

```yaml
log:
  max_entries: 500  # entries per file; 0 = unlimited (default: 1000)
```

## 另请参阅

- [audit](/docs/reference/commands/audit) — 安全扫描（记录在 audit.log 中）
- [status](/docs/reference/commands/status) — 显示当前同步状态
- [doctor](/docs/reference/commands/doctor) — 诊断配置问题
