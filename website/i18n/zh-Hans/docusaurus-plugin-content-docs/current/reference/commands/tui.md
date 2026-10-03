---
sidebar_position: 5
---

# tui

全局切换交互式 TUI 模式。

## 何时使用

- 你希望所有命令都输出纯文本而不是交互式 TUI
- 你在 CI/CD 流水线或非交互式环境中运行 skillshare
- 你想在禁用 TUI 后重新启用它

## 语法

```bash
skillshare tui          # 显示当前状态
skillshare tui on       # 为所有命令启用 TUI
skillshare tui off      # 为所有命令禁用 TUI（纯文本输出）
```

## 行为

当 TUI 被禁用时，原本会启动交互式界面的命令（`list`、`log`、`search`、`audit rules`、`trash`、`restore`、`diff`、`target list`、`mcp`）会回退为纯文本输出——效果等同于在每个命令上都加上 `--no-tui`。

| 状态 | 含义 |
|-------|------|
| `on (default)` | 配置中不存在 TUI 键——TUI 已启用 |
| `on` | 显式启用 |
| `off` | 显式禁用 |

该设置以 `tui: false` 的形式存储在 `config.yaml` 中。移除该键会恢复默认值（启用）。

## 优先级

单个命令上的 `--no-tui` 标志始终优先于全局设置。例如，即使设置了 `tui on`，`skillshare list --no-tui` 仍会禁用 TUI。

## 示例

```
$ skillshare tui
  TUI       on · default

$ skillshare tui off
✓ TUI disabled

$ skillshare tui
  TUI       off

$ skillshare tui on
✓ TUI enabled
```

## 另请参阅

- [list](./list.md) — 列出 skills（启用 TUI 时使用）
- [log](./log.md) — 查看操作日志（启用 TUI 时使用）
- [target](./target.md) — Target 列表（启用 TUI 时使用）
- [audit rules](./audit-rules.md) — Audit 规则浏览器（启用 TUI 时使用）
