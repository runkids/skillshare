---
sidebar_position: 4
---

# trash

管理已卸载的 skills 和 agents 的 trash 目录。

```bash
skillshare trash list                    # 交互式 TUI（TTY 下）
skillshare trash list --no-tui           # 纯文本输出
skillshare trash restore my-skill        # 从 trash 中恢复
skillshare trash restore my-skill -p     # 以 project 模式恢复
skillshare trash delete my-skill         # 从 trash 中永久删除
skillshare trash empty                   # 清空 trash
skillshare trash agents list             # 列出已放入 trash 的 agents
skillshare trash agents restore tutor    # 从 trash 中恢复某个 agent
skillshare trash --all list              # 列出 trash 中的 skills + agents
```

## 何时使用

- 恢复最近卸载的 skill 或 agent（7 天内）
- 永久删除 trash 中的项目以释放空间
- 在 trash 自动过期前检查其中的内容

## 交互式 TUI

在 TTY 上，`trash list` 会打开交互式的回收站列表，最新的在前面。可以选择一个或多个条目来恢复或永久删除，也可以清空整个回收站；每个操作都会先确认。打开条目会显示它的文件，恢复前可以先确认内容。按键列在界面底部。使用 `--all` 或未指定类型时，skills 和 agents 会列在一起。

如果部分条目失败（例如要恢复的 skill 名称已存在于 source），其余条目仍会继续处理，结果会列出失败的条目。

使用 `--no-tui` 可跳过 TUI，改为打印纯文本：

```bash
skillshare trash list --no-tui           # 纯文本输出
skillshare trash list --no-tui | less    # 手动通过 pager 分页
```

## 类型过滤

默认情况下，trash 作用于**skills**。使用 `agents` 位置关键字可作用于 agents，使用 `--all` 可同时包含两者：

```bash
skillshare trash list                    # 仅 skills（默认）
skillshare trash agents list             # 仅 agents
skillshare trash --all list              # skills 和 agents 都包含
skillshare trash agents restore tutor    # 恢复某个已放入 trash 的 agent
skillshare trash agents empty            # 仅清空 agent trash
```

## 子命令

### list（别名：`ls`）

显示 trash 中当前的所有项目。在终端中会启动交互式 TUI，使用 `--no-tui` 或在非 TTY 环境下则打印纯文本：

```bash
skillshare trash list
skillshare trash agents list
skillshare trash --all list --no-tui
```

纯文本输出：

```
Trash
  my-skill      1.2 KB · 2d ago
  old-helper    800 B · 5d ago

2 items, 2.0 KB
  Each item is removed for good 7 days after it was trashed
```

### restore

将最近一次放入 trash 的版本恢复回 source 目录：

```bash
skillshare trash restore my-skill
skillshare trash agents restore tutor
```

```
✓ Restore   my-skill → ~/.config/skillshare/skills · trashed 2d ago

Next
  skillshare sync  link it into your targets again
```

对于 agents，恢复提示会改为建议运行 `skillshare sync agents`。

如果 source 中已存在同名项目，恢复会失败。请先卸载已存在的项目，或使用不同的名称。

### delete（别名：`rm`）

从 trash 中永久删除单个项目：

```bash
skillshare trash delete my-skill
skillshare trash agents delete tutor
```

```
✓ Permanently deleted my-skill
```

### empty

从 trash 中永久删除所有项目（需确认提示）：

```bash
skillshare trash empty
skillshare trash agents empty
```

```
! This will permanently delete 3 items from trash
? Continue? [y/N] y
✓ Emptied trash: 3 items permanently deleted · 0.1s
```

## Backup 与 Trash 的区别

这两种安全机制保护的对象不同：

| | backup | trash |
|---|---|---|
| **保护对象** | target 目录（同步快照） | source skills 和 agents（卸载） |
| **位置** | `~/.local/share/skillshare/backups/` | `~/.local/share/skillshare/trash/`（skills）、`.../trash/agents/`（agents） |
| **触发方式** | `sync`、`target remove` | `uninstall` |
| **恢复方式** | `skillshare restore <target>` | `skillshare trash restore <name>` |
| **自动清理** | 手动（`backup --cleanup`） | 7 天 |

## 选项

| 标志 | 说明 |
|------|------------|
| `agents` | 位置关键字 —— 作用于 agents 而非 skills |
| `--all` | 同时包含 skills 和 agents |
| `--no-tui` | 禁用交互式 TUI，使用纯文本输出 |
| `--project, -p` | 使用 project 级别的 trash（`.skillshare/trash/`） |
| `--global, -g` | 使用 global trash |
| `--help, -h` | 显示帮助 |

## 自动清理

已过期的 trash 项目（超过 7 天）会在你运行 `uninstall` 或 `sync` 时自动清理。无需 cron 或定时任务。

## 另请参阅

- [uninstall](/docs/reference/commands/uninstall) — 移除 skills（移入 trash）
- [backup](/docs/reference/commands/backup) — 备份 target 目录
- [restore](/docs/reference/commands/restore) — 从 backup 恢复 targets
