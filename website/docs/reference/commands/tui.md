---
sidebar_position: 5
---

# tui

Toggle interactive TUI mode globally.

## When to Use

- You prefer plain text output over interactive TUI for all commands
- You're running skillshare in a CI/CD pipeline or non-interactive environment
- You want to re-enable TUI after disabling it

## Synopsis

```bash
skillshare tui          # Show current status
skillshare tui on       # Enable TUI for all commands
skillshare tui off      # Disable TUI for all commands (plain text output)
```

## Behavior

When TUI is disabled, commands that normally launch an interactive interface (`list`, `log`, `search`, `audit rules`, `trash`, `restore`, `diff`, `target list`) fall back to plain text output — equivalent to passing `--no-tui` on every command.

| State | Meaning |
|-------|---------|
| `on (default)` | TUI key is absent from config — TUI enabled |
| `on` | Explicitly enabled |
| `off` | Explicitly disabled |

The setting is stored as `tui: false` in `config.yaml`. Removing the key restores the default (enabled).

## Priority

The `--no-tui` flag on individual commands always takes priority over the global setting. For example, `skillshare list --no-tui` disables TUI even if `tui on` is set.

## Example

```
$ skillshare tui
ℹ TUI: on (default)

$ skillshare tui off
✔ TUI disabled

$ skillshare tui
ℹ TUI: off

$ skillshare tui on
✔ TUI enabled
```

## See Also

- [list](./list.md) — List skills (uses TUI when enabled)
- [log](./log.md) — View operation log (uses TUI when enabled)
- [target](./target.md) — Target list (uses TUI when enabled)
- [audit rules](./audit-rules.md) — Audit rules browser (uses TUI when enabled)
