---
sidebar_position: 5
---

# tui

全域切換互動式 TUI 模式。

## When to Use

- 你所有指令都偏好純文字輸出，而非互動式 TUI
- 你在 CI/CD pipeline 或非互動式環境中執行 skillshare
- 你想在停用 TUI 之後重新啟用它

## Synopsis

```bash
skillshare tui          # 顯示目前狀態
skillshare tui on       # 為所有指令啟用 TUI
skillshare tui off      # 為所有指令停用 TUI（純文字輸出）
```

## Behavior

當 TUI 被停用時，原本會啟動互動式介面的指令（`list`、`log`、`search`、`audit rules`、`trash`、`restore`、`diff`、`target list`、`mcp`）會改用純文字輸出——等同於在每個指令上都加上 `--no-tui`。

| 狀態 | 說明 |
|-------|---------|
| `on (default)` | 設定中沒有 TUI 這個 key — TUI 為啟用狀態 |
| `on` | 明確啟用 |
| `off` | 明確停用 |

此設定會以 `tui: false` 的形式儲存在 `config.yaml` 中。移除這個 key 會還原為預設值（啟用）。

## Priority

個別指令上的 `--no-tui` 旗標一律優先於全域設定。舉例來說，即使設定了 `tui on`，`skillshare list --no-tui` 仍會停用 TUI。

## Example

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

## See Also

- [list](./list.md) — 列出 skill（啟用時使用 TUI）
- [log](./log.md) — 查看操作紀錄（啟用時使用 TUI）
- [target](./target.md) — Target 清單（啟用時使用 TUI）
- [audit rules](./audit-rules.md) — Audit rules 瀏覽器（啟用時使用 TUI）
