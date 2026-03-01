---
sidebar_position: 6
---

# version

Show the current skillshare version.

## When to Use

- Check which version you're running before reporting a bug
- Verify an upgrade was successful
- Compare your version against [the latest release](https://github.com/runkids/skillshare/releases)

## Synopsis

```bash
skillshare version
skillshare -v
skillshare --version
```

## Example Output

```
skillshare version 0.16.6
```

## Update Notifications

When a newer version is available, `skillshare` displays an update notification after commands that support it. The notification is **Homebrew-aware**: if skillshare was installed via Homebrew, it queries `brew info` for the latest formula version and suggests `brew upgrade skillshare`; otherwise it checks GitHub releases and suggests `skillshare upgrade`.

Detection is automatic — skillshare resolves its own executable path and checks whether it resides under a Homebrew Cellar prefix (e.g. `/opt/homebrew/Cellar/skillshare/`).

Version checks are cached for 24 hours at `~/.cache/skillshare/version-check.json`.

## See Also

- [upgrade](./upgrade.md) — Upgrade to latest version
- [doctor](./doctor.md) — Full environment diagnostics
- [status](./status.md) — Show sync state including version info
