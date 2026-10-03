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

在 TTY 中，`trash list` 会启动一个支持多选、筛选和内联恢复/删除操作的交互式 TUI。每个项目都会显示一个类型徽标：`[S]` 表示 skills，`[A]` 表示 agents。

```
Trash (global) — 5 items

  [ ] [S] my-skill    (512 B, 2d ago)
  [x] [S] old-tool    (1.2 KB, 5d ago)
  [ ] [A] tutor       (2.0 KB, 3d ago)
  [ ] [S] another     (128 B, 1d ago)

  ─────────────────────────────────────────
  Name:         old-tool
  Type:         Skill
  Trashed:      2026-02-27 14:30:05
  Size:         1.2 KB
  Path:         ~/.local/share/skillshare/trash/old-tool_...

  ── SKILL.md ──────────────────────────────
  ---
  name: old-tool
  description: A helpful tool
  ---
  # old-tool
  ...

  ↑↓ navigate  / filter  space select  r restore(1)  d delete(1)  D empty  q quit
```

使用 `--all` 或不带类型过滤条件时，TUI 会将 skills 和 agents 合并为一个按日期排序（最新在前）的列表。

### 快捷键

| 按键 | 操作 |
|-----|--------|
| `↑`/`↓` | 浏览项目 |
| `←`/`→` | 切换页面 |
| `/` | 进入筛选模式（按名称子串匹配） |
| `Space` | 切换选中当前项目 |
| `a` | 切换全选当前可见项目 |
| `r` | 恢复选中的项目（需确认） |
| `d` | 永久删除选中的项目（需确认） |
| `D` | 清空整个 trash（忽略选择，需确认） |
| `Ctrl+d`/`Ctrl+u` | 向下/向上滚动详情面板 |
| `q`/`Ctrl+C` | 退出 |

在确认模式下：`y`/`Enter` 确认，`n`/`Esc` 取消。

### 批量操作

选中多个项目时，`r` 和 `d` 会作用于所有选中项。如果其中部分项目失败（例如恢复的 skill 在 source 中已存在同名项），TUI 会继续处理剩余项目，并显示合并后的结果：

```
Restored 2 item(s)  Failed: my-skill: already exists
```

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
  my-skill      (1.2 KB, 2d ago)
  old-helper    (800 B, 5d ago)

2 item(s), 2.0 KB total
Items are automatically cleaned up after 7 days
```

### restore

将最近一次放入 trash 的版本恢复回 source 目录：

```bash
skillshare trash restore my-skill
skillshare trash agents restore tutor
```

```
✓ Restored: my-skill
ℹ Trashed 2d ago, now back in ~/.config/skillshare/skills
ℹ Run 'skillshare sync' to update targets
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
✓ Permanently deleted: my-skill
```

### empty

从 trash 中永久删除所有项目（需确认提示）：

```bash
skillshare trash empty
skillshare trash agents empty
```

```
⚠ This will permanently delete 3 item(s) from trash
✓ Emptied trash: 3 item(s) permanently deleted
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
