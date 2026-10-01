---
sidebar_position: 2
---

# Desktop App

Manage skills, agents, MCP and hooks in a desktop window with **[Skillshare App](https://github.com/runkids/skillshare-app)**. It brings the skillshare dashboard to macOS, Windows and Linux, with guided first-time setup.

**[Download Skillshare App →](https://github.com/runkids/skillshare-app/releases/latest)**

## Install on macOS

For Apple Silicon Macs, Homebrew is the recommended installation method:

```bash
brew tap runkids/tap
brew install --cask skillshare-app
```

Open **skillshare** from Applications after installation. A `.dmg` is also available from the [latest release](https://github.com/runkids/skillshare-app/releases/latest).

## Install on Windows or Linux

Choose your installer from the [latest app release](https://github.com/runkids/skillshare-app/releases/latest):

| Platform | Installer |
|---|---|
| Windows (x64) | `.exe` or `.msi` |
| Linux (x64) | `.deb`, `.AppImage` or `.rpm` |

Install the package for your system, then open Skillshare App.

## First launch

The app runs on the skillshare CLI. Its setup guides you through:

1. Installing the CLI, or selecting a binary you already have.
2. Choosing the AI tools you want to sync to.
3. Running your first sync, then opening the dashboard.

From the dashboard, browse and install skills, preview changes before syncing, and manage agents, MCP and hooks. The desktop app and [`skillshare ui`](../reference/commands/ui.md) use the same dashboard.

## Prefer the terminal?

The CLI remains available for terminal workflows and automation. Start with the [CLI first-sync guide](./first-sync.md), or look up a command in the [quick reference](./quick-reference.md).
