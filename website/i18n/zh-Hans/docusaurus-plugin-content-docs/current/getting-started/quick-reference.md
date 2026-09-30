---
sidebar_position: 4
---

# 速查表

skillshare 命令速查。

## 核心命令

| 命令 | 说明 |
|---------|-------------|
| `init` | 首次配置 |
| `install <source>` | 添加一个 skill |
| `uninstall <name>...` | 移除一个或多个 skills |
| `list` | 列出所有 skills |
| `search <query>` | 搜索 skills |
| `sync` | 推送到所有 Targets |
| `status` | 显示 Sync 状态 |

## Skill 管理

| 命令 | 说明 |
|---------|-------------|
| `new <name>` | 创建一个新 skill |
| `update <name>` | 更新一个 skill（git pull） |
| `update --all` | 更新所有 tracked repos |
| `check` | 检查 skill 更新 |
| `check --json` | 检查更新（JSON 输出） |
| `upgrade` | 升级 CLI 与内置 skill |
| `hub list` | 列出已配置的 skill Hub |
| `hub add <url>` | 添加一个 skill Hub |

## Target 管理

| 命令 | 说明 |
|---------|-------------|
| `target list` | 列出所有 Targets |
| `target <name>` | 显示 Target 详情 |
| `target <name> --mode <mode>` | 修改 Sync 模式 |
| `target add <name> <path>` | 添加自定义 Target |
| `target remove <name>` | 安全地移除 Target |
| `diff [target]` | 显示差异 |

## Extras 管理

| 命令 | 说明 |
|---------|-------------|
| `extras init <name> --target <path>` | 在配置中添加一条 extras 记录 |
| `extras init <name> --file <file> --target <path>` | 添加一个只同步单个文件的 extra（`--as` 可在 targets 中重命名该文件） |
| `extras list` | 列出已配置的 extras 及其同步状态 |
| `extras remove <name>` | 从配置中移除一条 extras 记录 |
| `extras <name> --add-target <path>` | 为已有 extras 记录添加一个 Target |
| `extras <name> --remove-target <path>` | 移除一个 Target（加 `--prune` 可删除已同步的文件） |
| `extras collect <name>` | 把 extras Target 中的本地文件 collect 回 Source |

## Agent 管理

| 命令 | 说明 |
|---------|-------------|
| `list agents` | 列出已安装的 agents |
| `install <source> --kind agent` | 只安装 repo 中的 agents |
| `install <source> -a <name>` | 按名称安装指定的 agent |
| `uninstall --kind agent <name>` | 移除一个 agent |
| `sync agents` | 只把 agents 同步到 Targets |
| `check agents` | 检查 agents 的更新 |
| `audit agents` | 对 agents 做安全扫描 |
| `enable --kind agent <name>` | 重新启用被禁用的 agent |
| `disable --kind agent <name>` | 通过 `.agentignore` 禁用一个 agent |

## Plugin 管理

| 命令 | 说明 |
|---------|-------------|
| `plugin` | 打开交互式 plugin 管理器 |
| `plugin list` | 显示受管 plugins 及原生安装状态 |
| `plugin discover <source>` | 检视一个目录或 Git repo |
| `plugin add [source]` | 安装一个完整的原生 plugin |
| `plugin import [plugin@market] --from claude` | 接管已有的原生安装 |
| `plugin inspect <name>` | 检视一个受管的包 |
| `plugin check [name]` | 检查来源变更但不应用 |
| `plugin update [name] --target claude` | 用已审阅的来源更新某个受支持的 Target |
| `plugin enable [name] --target codex` | 为下一次 Sync 选中一个 Target |
| `plugin disable [name] --target codex` | 为下一次 Sync 取消选中一个 Target |
| `plugin remove [name]` | 卸载受管绑定并移除定义 |
| `sync plugins [name]` | 应用 plugin 的 Sync 选择；等同于 `plugin sync` |

Targets：Claude Code、Codex、Cursor、Antigravity（`agy`）、Pi 和 OpenCode。
Project mode 支持 Claude、Antigravity、Pi 和 OpenCode。

enable/disable 只保存选择结果。下一次 plugin sync 才会安装被选中的绑定，
或卸载被取消选中的绑定（同时保留其定义）。
Plugins 不包含在 `sync --all` 中。用 `--dry-run --json` 预览将要发生的变更；
自动化场景请用 `--no-tui` 并显式给出输入。原生客户端要求与受支持的 Targets
见 [plugin](/docs/reference/commands/plugin)。

## Sync 操作

| 命令 | 说明 |
|---------|-------------|
| `sync extras` | 同步非 skill 资源（rules、commands 等） |
| `sync mcp` | 同步 MCP 连接设置 |
| `sync --all` | 同步 skills + agents + extras + MCP + hooks（不含 plugins） |
| `collect <target>` | 把 Target 中的 skills collect 回 Source |
| `collect --all` | 从所有 Targets collect |
| `backup [target]` | 创建备份 |
| `backup --list` | 列出备份 |
| `restore <target>` | 从备份恢复 |
| `commit [-m "msg"]` | 只创建本地 git commit，不 push |
| `push [-m "msg"]` | commit 并 push 到 git remote |
| `pull` | 从 git pull 并 Sync |
| `trash list` | 列出软删除的 skills |
| `trash restore <name>` | 恢复一个软删除的 skill |

## 实用工具

| 命令 | 说明 |
|---------|-------------|
| `analyze` | 分析上下文窗口占用（交互式 TUI） |
| `analyze --filter <text>` | 按名称／路径子串过滤 skills |
| `analyze --json` | 以 JSON 输出上下文占用 |
| `doctor` | 诊断问题 |
| `doctor --json` | 诊断问题（JSON 输出，供 CI 使用） |
| `log` | 查看操作日志与审计日志 |
| `ui` | 在 `localhost:19420` 启动 Web 仪表盘 |
| `ui -p` | 以 Project mode 启动 Web 仪表盘 |
| `completion <shell> --install` | 安装 shell 自动补全（bash/zsh/fish/powershell/nushell） |
| `version` | 显示 CLI 版本 |
| `make test-docker` | 运行离线 Docker sandbox 测试 |
| `make playground` | 启动 playground 并进入 shell（一步完成） |
| `make playground-down` | 停止并移除 playground |
| `./scripts/sandbox.sh <cmd>` | 高级 sandbox 管理（up/down/shell/reset/status/logs/bare） |
| `make ui-build` | 构建前端 |
| `make build-all` | 构建包含前端的完整二进制 |

---

## 常见工作流

### 安装并同步一个 skill
```bash
skillshare install anthropics/skills/skills/pdf
skillshare sync
```

### 创建并部署一个 skill
```bash
skillshare new my-skill
# Edit ~/.config/skillshare/skills/my-skill/SKILL.md
skillshare sync
```

### 跨机器 Sync
```bash
# Setup (pick one)
# Interactive (guided prompts)
skillshare init --remote git@github.com:you/my-skills.git

# Non-interactive (no prompts, auto-detect installed targets)
skillshare init --remote git@github.com:you/my-skills.git --no-copy --all-targets --no-skill

# Optional local checkpoint without pushing
skillshare commit -m "Save local skill edits"

# Machine A: push changes
skillshare push -m "Add new skill"

# Machine B: pull and sync
skillshare pull
```

之后可选（仅当你在配置完成后又安装了新的 AI CLI）：

```bash
skillshare init --discover
```

在 discover 时覆盖模式，只会影响新添加的 Targets：

```bash
skillshare init --discover --select cursor --mode copy
```

### 团队 skill 共享
```bash
# Install team repo
skillshare install github.com/team/skills --track

# Install from specific branch (works with or without --track)
skillshare install github.com/team/skills --branch develop --all
skillshare install github.com/team/skills --track --branch develop

# 固定到 tag 或 commit SHA（仅限普通安装）
skillshare install github.com/team/skills --branch v1.2.0 --all

# Update from team
skillshare update --all
skillshare sync
```

### Sandbox playground 会话
```bash
make playground          # start + enter shell
skillshare --help
ss status
exit                     # leave shell
make playground-down     # stop container
```

---

## 关键路径

| 路径 | 说明 |
|------|-------------|
| `~/.config/skillshare/config.yaml` | 配置文件 |
| `~/.config/skillshare/skills/.metadata.json` | 已安装 skill 的元数据（自动维护） |
| `~/.config/skillshare/skills/` | Skill Source 目录 |
| `~/.config/skillshare/agents/` | Agent Source 目录 |
| `~/.config/skillshare/extras/<name>/` | Extras Source 目录 |
| `~/.local/state/skillshare/logs/` | 操作日志与审计日志 |
| `~/.local/share/skillshare/backups/` | 备份目录 |

---

## 大多数命令都支持的 Flags

| Flag | 说明 |
|------|-------------|
| `--dry-run`、`-n` | 预览而不做实际改动 |
| `--help`、`-h` | 显示帮助 |

---

## 另见

- [命令参考](/docs/reference/commands) —— 完整命令文档
- [概念](/docs/understand) —— 核心概念详解
